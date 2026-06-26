package main

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/term"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"os/exec"
)

var validServices = []string{
	"admin",
	"auth",
	"blog",
	"cbt_exam",
	"courses",
	"dashboard_profile",
	"enrollments",
	"notes",
	"teacher",
	"test_series",
}

func main() {
	// 1. Load root .env file
	// Search in workspace root first
	_ = godotenv.Load(".env")
	_ = godotenv.Load("../.env")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("Error: DATABASE_URL is not set in environment or env files.")
	}

	if len(os.Args) < 2 {
		printHelp()
		return
	}

	command := strings.ToLower(os.Args[1])
	switch command {
	case "migrate":
		runMigrate(dbURL)
	case "makemigrations":
		if len(os.Args) < 4 {
			fmt.Println("Error: makemigrations requires service name and description.")
			fmt.Println("Usage: go run manage.go makemigrations <service_name> <description>")
			fmt.Println("Example: go run manage.go makemigrations auth \"add_profile_fields\"")
			return
		}
		runMakeMigrations(os.Args[2], os.Args[3])
	case "flushall":
		runFlushAll(dbURL)
	case "createadmin":
		runCreateAdmin(dbURL)
	case "updatepass":
		runUpdatePass(dbURL)
	case "help":
		printHelp()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printHelp()
	}
}

func printHelp() {
	fmt.Println("\n==================================================")
	fmt.Println("ClaSynq Go Database Management Tool")
	fmt.Println("==================================================")
	fmt.Println("Usage:")
	fmt.Println("  go run manage.go <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  migrate                              Apply all pending SQL migrations across all services")
	fmt.Println("  makemigrations <service> <desc>      Generate a new SQL migration template for a specific service")
	fmt.Println("  flushall                             Truncate all data tables (clears DB but keeps schemas)")
	fmt.Println("  createadmin                          Create a new admin query and output insert SQL file")
	fmt.Println("  updatepass                           Update an admin password and output update SQL file")
	fmt.Println("  help                                 Show this help screen")
	fmt.Println("\nValid Services:")
	for _, s := range validServices {
		fmt.Printf("  - %s\n", s)
	}
	fmt.Println("==================================================")
}

func connectDB(dbURL string) *gorm.DB {
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	return db
}

// ----------------------------------------------------
// MIGRATE COMMAND
// ----------------------------------------------------
func runMigrate(dbURL string) {
	// Populate missing referral codes for existing users before AutoMigrate runs,
	// to prevent duplicate key constraint violations when creating the unique index.
	dbInit := connectDB(dbURL)
	populateMissingReferralCodes(dbInit)
	if sqlDB, err := dbInit.DB(); err == nil {
		sqlDB.Close()
	}

	// 1. Run AutoMigrate for each service using temporary generator files
	runGormAutoMigrate(dbURL)

	fmt.Println("\n[Migrate] Connecting to database to apply schema updates...")
	db := connectDB(dbURL)

	// Create global schema_migrations table if not exists
	err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			service VARCHAR(100),
			version VARCHAR(255),
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (service, version)
		)
	`).Error
	if err != nil {
		log.Fatalf("Failed to create schema_migrations table: %v", err)
	}

	// Dynamic upgrade of the table if it was created by the old script without the 'service' column
	if !db.Migrator().HasColumn("schema_migrations", "service") {
		fmt.Println("[Migrate] Upgrading legacy schema_migrations table to support multi-service migrations...")
		err = db.Transaction(func(tx *gorm.DB) error {
			// 1. Add service column
			if err := tx.Exec("ALTER TABLE schema_migrations ADD COLUMN service VARCHAR(100)").Error; err != nil {
				return err
			}
			// 2. Set default value for pre-existing records (which all belonged to the 'notes' service)
			if err := tx.Exec("UPDATE schema_migrations SET service = 'notes' WHERE service IS NULL").Error; err != nil {
				return err
			}
			// 3. Set service column as NOT NULL
			if err := tx.Exec("ALTER TABLE schema_migrations ALTER COLUMN service SET NOT NULL").Error; err != nil {
				return err
			}
			// 4. Drop old primary key
			if err := tx.Exec("ALTER TABLE schema_migrations DROP CONSTRAINT IF EXISTS schema_migrations_pkey").Error; err != nil {
				return err
			}
			// 5. Add new composite primary key
			if err := tx.Exec("ALTER TABLE schema_migrations ADD PRIMARY KEY (service, version)").Error; err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			log.Fatalf("Failed to upgrade schema_migrations table: %v", err)
		}
		fmt.Println("[Migrate] schema_migrations table upgraded successfully.")
	}

	// Query already applied migrations
	type MigrationKey struct {
		Service string
		Version string
	}
	var appliedList []MigrationKey
	err = db.Raw("SELECT service, version FROM schema_migrations").Scan(&appliedList).Error
	if err != nil {
		log.Fatalf("Failed to query applied migrations: %v", err)
	}

	appliedMap := make(map[string]bool)
	for _, m := range appliedList {
		key := fmt.Sprintf("%s:%s", m.Service, m.Version)
		appliedMap[key] = true
	}

	// Scan through all microservices for migrations
	rootPath := findWorkspaceRoot()
	migrationsApplied := 0

	for _, service := range validServices {
		serviceMigDir := filepath.Join(rootPath, service, "migrations")
		if _, err := os.Stat(serviceMigDir); os.IsNotExist(err) {
			continue // No migrations directory for this service
		}

		files, err := ioutil.ReadDir(serviceMigDir)
		if err != nil {
			log.Printf("Warning: Failed to read migrations for service %s: %v", service, err)
			continue
		}

		var sqlFiles []string
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".sql") {
				sqlFiles = append(sqlFiles, file.Name())
			}
		}

		// Sort migrations alphabetically so they execute in order (e.g. 0001, 0002)
		sort.Strings(sqlFiles)

		for _, sqlFile := range sqlFiles {
			key := fmt.Sprintf("%s:%s", service, sqlFile)
			if appliedMap[key] {
				continue // Already applied
			}

			filePath := filepath.Join(serviceMigDir, sqlFile)
			queryBytes, err := ioutil.ReadFile(filePath)
			if err != nil {
				log.Fatalf("Failed to read migration file %s: %v", filePath, err)
			}

			query := string(queryBytes)
			if strings.TrimSpace(query) == "" {
				// Empty file, mark as applied directly
				fmt.Printf("[%s] Marking empty migration as applied: %s\n", service, sqlFile)
			} else {
				fmt.Printf("[%s] Applying migration: %s...\n", service, sqlFile)
				err = db.Transaction(func(tx *gorm.DB) error {
					if err := tx.Exec(query).Error; err != nil {
						return err
					}
					return tx.Exec("INSERT INTO schema_migrations (service, version) VALUES (?, ?)", service, sqlFile).Error
				})
				if err != nil {
					log.Fatalf("Fatal: Migration failed in file %s: %v", sqlFile, err)
				}
			}

			migrationsApplied++
		}
	}

	if migrationsApplied == 0 {
		fmt.Println("Database is already up to date. No pending migrations.")
	} else {
		fmt.Printf("Success: Applied %d migration(s) successfully.\n", migrationsApplied)
	}
}

func populateMissingReferralCodes(db *gorm.DB) {
	fmt.Println("[Migrate] Checking for users with missing referral codes...")

	type User struct {
		ID           int64
		ReferralCode string `gorm:"column:referral_code"`
	}

	var users []User
	err := db.Raw("SELECT id, referral_code FROM users WHERE referral_code = '' OR referral_code IS NULL").Scan(&users).Error
	if err != nil {
		log.Printf("Warning: Failed to fetch users with missing referral codes: %v", err)
		return
	}

	if len(users) == 0 {
		fmt.Println("[Migrate] All existing users have valid referral codes.")
		return
	}

	fmt.Printf("[Migrate] Generating unique referral codes for %d user(s)...\n", len(users))
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	for _, u := range users {
		var code string
		for {
			result := make([]byte, 8)
			for i := 0; i < 8; i++ {
				num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
				result[i] = charset[num.Int64()]
			}
			code = "CSQ-" + string(result)

			var count int64
			err = db.Table("users").Where("referral_code = ?", code).Count(&count).Error
			if err == nil && count == 0 {
				break
			}
		}

		err = db.Exec("UPDATE users SET referral_code = ? WHERE id = ?", code, u.ID).Error
		if err != nil {
			log.Printf("Warning: Failed to update referral code for user ID %d: %v", u.ID, err)
		} else {
			fmt.Printf("[Migrate] Assigned code %s to user ID %d\n", code, u.ID)
		}
	}
}

// ----------------------------------------------------
// MAKEMIGRATIONS COMMAND
// ----------------------------------------------------
func runMakeMigrations(service, description string) {
	service = strings.ToLower(service)

	// Validate service name
	isValid := false
	for _, s := range validServices {
		if s == service {
			isValid = true
			break
		}
	}

	if !isValid {
		fmt.Printf("Error: '%s' is not a valid service name.\n", service)
		fmt.Println("Valid services are:", strings.Join(validServices, ", "))
		return
	}

	// Clean description to be safe for filenames
	reg := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	cleanDesc := reg.ReplaceAllString(description, "_")
	cleanDesc = strings.ToLower(cleanDesc)

	rootPath := findWorkspaceRoot()
	serviceMigDir := filepath.Join(rootPath, service, "migrations")

	// Ensure service migrations folder exists
	if err := os.MkdirAll(serviceMigDir, 0755); err != nil {
		log.Fatalf("Failed to create migrations directory: %v", err)
	}

	// Scan existing files to find the next sequence index
	files, err := ioutil.ReadDir(serviceMigDir)
	if err != nil {
		log.Fatalf("Failed to read migrations directory: %v", err)
	}

	maxSeq := 0
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".sql") {
			parts := strings.SplitN(file.Name(), "_", 2)
			if len(parts) > 0 {
				seq, err := strconv.Atoi(parts[0])
				if err == nil && seq > maxSeq {
					maxSeq = seq
				}
			}
		}
	}

	nextSeq := maxSeq + 1
	filename := fmt.Sprintf("%04d_%s.sql", nextSeq, cleanDesc)
	filePath := filepath.Join(serviceMigDir, filename)

	// Create migration template contents
	template := fmt.Sprintf(`-- Migration: %s
-- Created At: %s
-- Description: %s

-- Write your UP SQL migration queries here:
-- E.g. ALTER TABLE table_name ADD COLUMN column_name data_type;

`, filename, time.Now().Format(time.RFC3339), description)

	err = ioutil.WriteFile(filePath, []byte(template), 0644)
	if err != nil {
		log.Fatalf("Failed to write migration template file: %v", err)
	}

	// Print link-style path for convenience
	relPath := filepath.Join(service, "migrations", filename)
	fmt.Printf("\nSuccess: Created migration file template at:\n%s\n", relPath)
}

// ----------------------------------------------------
// FLUSHALL COMMAND
// ----------------------------------------------------
func runFlushAll(dbURL string) {
	fmt.Println("\nWARNING: This will truncate all data tables in the database!")
	fmt.Println("This clears all records but keeps database tables and schemas intact.")
	fmt.Print("Are you sure you want to proceed? (y/N): ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalf("Failed to read input: %v", err)
	}

	confirm := strings.TrimSpace(strings.ToLower(input))
	if confirm != "y" && confirm != "yes" {
		fmt.Println("Operation cancelled.")
		return
	}

	fmt.Println("\n[FlushAll] Connecting to database...")
	db := connectDB(dbURL)

	fmt.Println("[FlushAll] Truncating all tables in public schema...")
	flushSQL := `
		DO $$ DECLARE
			r RECORD;
		BEGIN
			FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = 'public') LOOP
				-- Skip schema_migrations table so we don't lose migration history
				IF r.tablename != 'schema_migrations' THEN
					EXECUTE 'TRUNCATE TABLE ' || quote_ident(r.tablename) || ' RESTART IDENTITY CASCADE;';
				END IF;
			END LOOP;
		END $$;
	`

	err = db.Exec(flushSQL).Error
	if err != nil {
		log.Fatalf("Fatal: Database flush failed: %v", err)
	}

	fmt.Println("Success: Database tables truncated and primary key sequences reset to 1 successfully.")
}

func findWorkspaceRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}

	dir := cwd
	for {
		workFile := filepath.Join(dir, "go.work")
		if _, err := os.Stat(workFile); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // Reached the root of the filesystem
		}
		dir = parent
	}

	return "." // fallback
}

// ----------------------------------------------------
// CREATEADMIN COMMAND
// ----------------------------------------------------
func runCreateAdmin(dbURL string) {
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("  ClaSynq Admin SQL Query Generator (Go)")
	fmt.Println(strings.Repeat("=", 60))

	reader := bufio.NewReader(os.Stdin)

	// Prompt for email
	fmt.Print("Enter Admin Email: ")
	emailInput, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Error reading email: %v\n", err)
		os.Exit(1)
	}
	email := strings.ToLower(strings.TrimSpace(emailInput))
	if email == "" {
		fmt.Println("Error: Email cannot be empty.")
		os.Exit(1)
	}

	// Prompt for password
	password, err := readPassword(reader, "Enter Admin Password: ")
	if err != nil {
		fmt.Printf("Error reading password: %v\n", err)
		os.Exit(1)
	}
	if password == "" {
		fmt.Println("Error: Password cannot be empty.")
		os.Exit(1)
	}

	// Confirm password
	confirmPassword, err := readPassword(reader, "Confirm Admin Password: ")
	if err != nil {
		fmt.Printf("Error reading confirmation: %v\n", err)
		os.Exit(1)
	}
	if password != confirmPassword {
		fmt.Println("Error: Passwords do not match.")
		os.Exit(1)
	}

	// Generate Django PBKDF2 hash
	fmt.Println("\nHashing password...")
	salt := GenerateSalt(12)
	hashedPassword := EncodeDjangoPassword(password, salt, 390000)

	// Generate SQL Statement
	sqlQuery := fmt.Sprintf(`INSERT INTO admin (email, password, created_at)
VALUES (
    '%s', 
    '%s', 
    NOW()
);`, email, hashedPassword)

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("  GENERATED SQL QUERY (Copy and run this in your database client)")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println(sqlQuery)
	fmt.Println(strings.Repeat("=", 60))

	// Write to a file as well for convenience
	filename := "create_admin.sql"
	absPath, err := filepath.Abs(filename)
	if err != nil {
		absPath = filename
	}

	err = os.WriteFile(filename, []byte(sqlQuery), 0644)
	if err != nil {
		fmt.Printf("\nWarning: Could not save query to file: %v\n", err)
	} else {
		fmt.Printf("\nSaved query to file: %s\n", absPath)
	}

	// Ask if user wants to execute the query immediately on the database
	fmt.Print("\nDo you want to execute this query on the database immediately? (y/N): ")
	confirmInput, err := reader.ReadString('\n')
	if err == nil {
		confirm := strings.TrimSpace(strings.ToLower(confirmInput))
		if confirm == "y" || confirm == "yes" {
			fmt.Println("Connecting to database...")
			db := connectDB(dbURL)
			if err := db.Exec(sqlQuery).Error; err != nil {
				fmt.Printf("Error executing query: %v\n", err)
			} else {
				fmt.Println("Success: Query executed and applied to database successfully!")
			}
		}
	}
}

// ----------------------------------------------------
// UPDATEPASS COMMAND
// ----------------------------------------------------
func runUpdatePass(dbURL string) {
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("  ClaSynq Admin Password Update SQL Generator (Go)")
	fmt.Println(strings.Repeat("=", 60))

	reader := bufio.NewReader(os.Stdin)

	// Prompt for email
	fmt.Print("Enter Admin Email to Update: ")
	emailInput, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Error reading email: %v\n", err)
		os.Exit(1)
	}
	email := strings.ToLower(strings.TrimSpace(emailInput))
	if email == "" {
		fmt.Println("Error: Email cannot be empty.")
		os.Exit(1)
	}

	// Prompt for password
	password, err := readPassword(reader, "Enter New Admin Password: ")
	if err != nil {
		fmt.Printf("Error reading password: %v\n", err)
		os.Exit(1)
	}
	if password == "" {
		fmt.Println("Error: Password cannot be empty.")
		os.Exit(1)
	}

	// Confirm password
	confirmPassword, err := readPassword(reader, "Confirm New Admin Password: ")
	if err != nil {
		fmt.Printf("Error reading confirmation: %v\n", err)
		os.Exit(1)
	}
	if password != confirmPassword {
		fmt.Println("Error: Passwords do not match.")
		os.Exit(1)
	}

	// Generate Django PBKDF2 hash
	fmt.Println("\nHashing password...")
	salt := GenerateSalt(12)
	hashedPassword := EncodeDjangoPassword(password, salt, 390000)

	// Generate SQL Statement
	sqlQuery := fmt.Sprintf(`UPDATE admin 
SET password = '%s' 
WHERE email = '%s';`, hashedPassword, email)

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("  GENERATED SQL QUERY (Copy and run this in your database client)")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println(sqlQuery)
	fmt.Println(strings.Repeat("=", 60))

	// Write to a file as well for convenience
	filename := "update_admin.sql"
	absPath, err := filepath.Abs(filename)
	if err != nil {
		absPath = filename
	}

	err = os.WriteFile(filename, []byte(sqlQuery), 0644)
	if err != nil {
		fmt.Printf("\nWarning: Could not save query to file: %v\n", err)
	} else {
		fmt.Printf("\nSaved query to file: %s\n", absPath)
	}

	// Ask if user wants to execute the query immediately on the database
	fmt.Print("\nDo you want to execute this query on the database immediately? (y/N): ")
	confirmInput, err := reader.ReadString('\n')
	if err == nil {
		confirm := strings.TrimSpace(strings.ToLower(confirmInput))
		if confirm == "y" || confirm == "yes" {
			fmt.Println("Connecting to database...")
			db := connectDB(dbURL)
			if err := db.Exec(sqlQuery).Error; err != nil {
				fmt.Printf("Error executing query: %v\n", err)
			} else {
				fmt.Println("Success: Query executed and applied to database successfully!")
			}
		}
	}
}

// Helper functions for password generation
func readPassword(reader *bufio.Reader, prompt string) (string, error) {
	fmt.Print(prompt)
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		bytePassword, err := term.ReadPassword(fd)
		fmt.Println() // Print newline after entering password
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(bytePassword)), nil
	}
	// Fallback to normal scanner
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func GenerateSalt(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[num.Int64()]
	}
	return string(result)
}

func EncodeDjangoPassword(plainPassword, salt string, iterations int) string {
	dk := pbkdf2.Key([]byte(plainPassword), []byte(salt), iterations, 32, sha256.New)
	hashBase64 := base64.StdEncoding.EncodeToString(dk)
	return fmt.Sprintf("pbkdf2_sha256$%d$%s$%s", iterations, salt, hashBase64)
}

func runGormAutoMigrate(dbURL string) {
	fmt.Println("\n[Migrate] Running GORM AutoMigrate for all service models...")
	rootPath := findWorkspaceRoot()
	dbURLQuoted := strconv.Quote(dbURL)

	// Clean up conflicting Django-style unique constraints ending in '_key' 
	// to prevent GORM AutoMigrate from crashing when dropping constraints.
	db := connectDB(dbURL)
	dropSQL := `
		-- Drop the invalid polymorphic UserNotification foreign key if it exists
		ALTER TABLE user_notifications DROP CONSTRAINT IF EXISTS fk_user_notifications_recipient;

		DO $$ 
		DECLARE 
			r RECORD;
		BEGIN
			FOR r IN (
				SELECT 
					table_name, 
					constraint_name 
				FROM 
					information_schema.table_constraints 
				WHERE 
					constraint_schema = 'public' 
					AND constraint_type = 'UNIQUE' 
					AND constraint_name LIKE '%\_key'
			) LOOP
				EXECUTE 'ALTER TABLE ' || quote_ident(r.table_name) || ' DROP CONSTRAINT ' || quote_ident(r.constraint_name) || ';';
			END LOOP;
		END $$;
	`
	if err := db.Exec(dropSQL).Error; err != nil {
		log.Printf("[Migrate] Warning: Failed to drop conflicting Django constraints: %v", err)
	} else {
		fmt.Println("[Migrate] Successfully cleared conflicting legacy unique constraints.")
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.Close()
	}

	// 1. auth
	authMigrateCode := `package main

import (
	"log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"clasynq/api/auth/internal/domain"
)

func main() {
	dbURL := ` + dbURLQuoted + `
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("db connection failed: %v", err)
	}
	err = db.AutoMigrate(
		&domain.User{},
		&domain.Student{},
		&domain.Admin{},
		&domain.Teacher{},
		&domain.PendingRegistration{},
		&domain.PasswordResetOTP{},
		&domain.Follow{},
		&domain.UserNotification{},
	)
	if err != nil {
		log.Fatalf("AutoMigrate failed: %v", err)
	}
}
`

	// 2. notes
	notesMigrateCode := `package main

import (
	"log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"clasynq/api/notes/internal/domain"
)

func main() {
	dbURL := ` + dbURLQuoted + `
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("db connection failed: %v", err)
	}
	err = db.AutoMigrate(
		&domain.Note{},
		&domain.NoteAccess{},
	)
	if err != nil {
		log.Fatalf("AutoMigrate failed: %v", err)
	}
}
`

	// 3. courses
	coursesMigrateCode := `package main

import (
	"log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"clasynq/api/courses/internal/domain"
)

func main() {
	dbURL := ` + dbURLQuoted + `
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("db connection failed: %v", err)
	}
	err = db.AutoMigrate(
		&domain.Subject{},
		&domain.Teacher{},
		&domain.Course{},
		&domain.ClassSchedule{},
	)
	if err != nil {
		log.Fatalf("AutoMigrate failed: %v", err)
	}
}
`

	// 4. blog
	blogMigrateCode := `package main

import (
	"log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"clasynq/api/blog/internal/domain"
)

func main() {
	dbURL := ` + dbURLQuoted + `
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("db connection failed: %v", err)
	}
	err = db.AutoMigrate(
		&domain.BlogPost{},
		&domain.BlogLike{},
		&domain.BlogComment{},
		&domain.PostView{},
		&domain.Repost{},
		&domain.SavedPost{},
		&domain.ActivityLog{},
	)
	if err != nil {
		log.Fatalf("AutoMigrate failed: %v", err)
	}
}
`

	// 5. enrollments
	enrollmentsMigrateCode := `package main

import (
	"log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"clasynq/api/enrollments/internal/domain"
)

func main() {
	dbURL := ` + dbURLQuoted + `
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("db connection failed: %v", err)
	}
	err = db.AutoMigrate(
		&domain.PaymentOrder{},
		&domain.ReferralTransaction{},
		&domain.WebhookEvent{},
		&domain.PaymentAuditLog{},
		&domain.Enrollment{},
	)
	if err != nil {
		log.Fatalf("AutoMigrate failed: %v", err)
	}
}
`

	// 6. cbt_exam
	cbtMigrateCode := `package main

import (
	"log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"clasynq/api/cbt_exam/internal/domain"
)

func main() {
	dbURL := ` + dbURLQuoted + `
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("db connection failed: %v", err)
	}
	err = db.AutoMigrate(
		&domain.TestSeries{},
		&domain.Test{},
		&domain.QuestionOption{},
		&domain.Question{},
		&domain.StudentTestAttempt{},
		&domain.StudentAnswer{},
		&domain.TestResult{},
		&domain.TestSeriesAccess{},
	)
	if err != nil {
		log.Fatalf("AutoMigrate failed: %v", err)
	}
}
`

	// 7. admin
	adminMigrateCode := `package main

import (
	"log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"clasynq/api/admin/internal/domain"
)

func main() {
	dbURL := ` + dbURLQuoted + `
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("db connection failed: %v", err)
	}
	err = db.AutoMigrate(
		&domain.AdminActivity{},
		&domain.AdminDashboard{},
	)
	if err != nil {
		log.Fatalf("AutoMigrate failed: %v", err)
	}
}
`

	// 8. teacher
	teacherMigrateCode := `package main

import (
	"log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"clasynq/api/teacher/internal/domain"
)

func main() {
	dbURL := ` + dbURLQuoted + `
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("db connection failed: %v", err)
	}
	err = db.AutoMigrate(
		&domain.TeacherActivity{},
	)
	if err != nil {
		log.Fatalf("AutoMigrate failed: %v", err)
	}
}
`

	servicesToMigrate := []struct {
		name string
		code string
	}{
		{"auth", authMigrateCode},
		{"notes", notesMigrateCode},
		{"courses", coursesMigrateCode},
		{"blog", blogMigrateCode},
		{"enrollments", enrollmentsMigrateCode},
		{"cbt_exam", cbtMigrateCode},
		{"admin", adminMigrateCode},
		{"teacher", teacherMigrateCode},
	}

	for _, s := range servicesToMigrate {
		fmt.Printf("[%s] Recreating tables via GORM AutoMigrate...\n", s.name)
		tmpFile := filepath.Join(rootPath, s.name, "tmp_migrate.go")
		err := ioutil.WriteFile(tmpFile, []byte(s.code), 0644)
		if err != nil {
			log.Fatalf("[%s] Failed to write temporary migrate file: %v", s.name, err)
		}
		// Defer deletion so it always cleans up
		defer os.Remove(tmpFile)

		cmd := exec.Command("go", "run", "tmp_migrate.go")
		cmd.Dir = filepath.Join(rootPath, s.name)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatalf("[%s] GORM AutoMigrate execution failed: %v", s.name, err)
		}
	}
	fmt.Println("[Migrate] GORM AutoMigrate completed successfully for all services.")
}
