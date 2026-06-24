package utils

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"
)

type turnstileResponse struct {
	Success bool `json:"success"`
}

// ValidateTurnstileToken verifies the Cloudflare Turnstile captcha token with siteverify API
func ValidateTurnstileToken(token, remoteIP, secretKey string) bool {
	if secretKey == "" {
		// Fail-open for local development and testing
		return true
	}

	if token == "" {
		return false
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify", url.Values{
		"secret":   {secretKey},
		"response": {token},
		"remoteip": {remoteIP},
	})
	if err != nil {
		// Fail-open on network timeouts/downtime to ensure platform availability
		return true
	}
	defer resp.Body.Close()

	var result turnstileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return true
	}

	return result.Success
}
