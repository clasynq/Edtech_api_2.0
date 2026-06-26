package utils

import (
	"fmt"
	"net/smtp"
	"strconv"
)

// SendOTPEmail sends an email containing the OTP code using SMTP configuration
func SendOTPEmail(toEmail, code, name, emailType, fromEmail, smtpHost, smtpPortStr, smtpUser, smtpPass string, ttlSeconds int) error {
	minutes := ttlSeconds / 60
	if minutes < 1 {
		minutes = 1
	}

	var subject, body string
	if emailType == "reset" {
		subject = "Reset your ClaSynq password"
		body = fmt.Sprintf(
			"Hi %s,\n\n"+
				"You requested to reset your password. Your ClaSynq OTP is: %s\n\n"+
				"This code expires in %d minutes. If you did not request this, you can safely ignore this email.\n\n"+
				"— ClaSynq",
			name, code, minutes,
		)
	} else {
		subject = "Your ClaSynq verification code"
		body = fmt.Sprintf(
			"Hi %s,\n\n"+
				"Your ClaSynq verification code is: %s\n\n"+
				"This code expires in %d minutes. If you did not request this, you can safely ignore the email.\n\n"+
				"— ClaSynq",
			name, code, minutes,
		)
	}

	// Prepare MIME email format
	mime := "MIME-Version: 1.0\r\nContent-Type: text/plain; charset=\"utf-8\"\r\n"
	message := fmt.Sprintf(
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"%s\r\n"+
			"%s",
		fromEmail, toEmail, subject, mime, body,
	)

	// Set up SMTP Auth
	var auth smtp.Auth
	if smtpUser != "" && smtpPass != "" {
		auth = smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	}

	// Convert port string to int
	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		return fmt.Errorf("invalid SMTP port: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", smtpHost, smtpPort)

	// Send mail
	err = smtp.SendMail(addr, auth, fromEmail, []string{toEmail}, []byte(message))
	if err != nil {
		return fmt.Errorf("failed to send SMTP email: %w", err)
	}

	return nil
}

// SendBirthdayEmail sends a birthday wish email using SMTP configuration
func SendBirthdayEmail(toEmail, name, fromEmail, smtpHost, smtpPortStr, smtpUser, smtpPass string) error {
	subject := fmt.Sprintf("Happy Birthday, %s!", name)
	body := fmt.Sprintf(
		"Hi %s,\n\n"+
			"Wishing you a very Happy Birthday from all of us at ClaSynq! 🎂🎉\n\n"+
			"Thank you for being a part of our learning community. Have a wonderful day!\n\n"+
			"— The ClaSynq Team",
		name,
	)

	// Prepare MIME email format
	mime := "MIME-Version: 1.0\r\nContent-Type: text/plain; charset=\"utf-8\"\r\n"
	message := fmt.Sprintf(
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"%s\r\n"+
			"%s",
		fromEmail, toEmail, subject, mime, body,
	)

	// Set up SMTP Auth
	var auth smtp.Auth
	if smtpUser != "" && smtpPass != "" {
		auth = smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	}

	// Convert port string to int
	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		return fmt.Errorf("invalid SMTP port: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", smtpHost, smtpPort)

	// Send mail
	err = smtp.SendMail(addr, auth, fromEmail, []string{toEmail}, []byte(message))
	if err != nil {
		return fmt.Errorf("failed to send SMTP birthday email: %w", err)
	}

	return nil
}

