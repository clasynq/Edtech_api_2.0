package utils

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime/multipart"
	"net/smtp"
	"net/textproto"
	"strconv"
)

func SendEmail(to, subject, bodyText, from, smtpHost, smtpPortStr, smtpUser, smtpPass string) error {
	mime := "MIME-Version: 1.0\r\nContent-Type: text/plain; charset=\"utf-8\"\r\n"
	message := fmt.Sprintf(
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"%s\r\n"+
			"%s",
		from, to, subject, mime, bodyText,
	)

	return sendSMTP(to, from, smtpHost, smtpPortStr, smtpUser, smtpPass, []byte(message))
}

func SendEmailWithAttachment(to, subject, bodyText, from, smtpHost, smtpPortStr, smtpUser, smtpPass, fileName string, fileData []byte) error {
	buf := bytes.NewBuffer(nil)
	buf.WriteString(fmt.Sprintf("From: %s\r\n", from))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", to))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	buf.WriteString("MIME-Version: 1.0\r\n")

	writer := multipart.NewWriter(buf)
	buf.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s\r\n\r\n", writer.Boundary()))

	// Body text part
	textHeader := make(textproto.MIMEHeader)
	textHeader.Set("Content-Type", "text/plain; charset=UTF-8")
	textPart, err := writer.CreatePart(textHeader)
	if err != nil {
		return err
	}
	_, _ = textPart.Write([]byte(bodyText))

	// Attachment part
	attachHeader := make(textproto.MIMEHeader)
	attachHeader.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	attachHeader.Set("Content-Type", "application/pdf")
	attachHeader.Set("Content-Transfer-Encoding", "base64")
	attachPart, err := writer.CreatePart(attachHeader)
	if err != nil {
		return err
	}

	encoder := base64.NewEncoder(base64.StdEncoding, attachPart)
	_, _ = encoder.Write(fileData)
	_ = encoder.Close()

	_ = writer.Close()

	return sendSMTP(to, from, smtpHost, smtpPortStr, smtpUser, smtpPass, buf.Bytes())
}

func sendSMTP(to, from, smtpHost, smtpPortStr, smtpUser, smtpPass string, message []byte) error {
	var auth smtp.Auth
	if smtpUser != "" && smtpPass != "" {
		auth = smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	}

	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		return fmt.Errorf("invalid SMTP port: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", smtpHost, smtpPort)
	return smtp.SendMail(addr, auth, from, []string{to}, message)
}
