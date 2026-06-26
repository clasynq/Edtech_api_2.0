package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
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
	fmt.Println("\n[Migrate] Connecting to database...")
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
