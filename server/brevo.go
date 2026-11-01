package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type BrevoNotifier struct {
	APIKey string
	Sender string
}

func (b BrevoNotifier) send(to, subject, text string) error {
	if b.Sender == "" {
		b.Sender = os.Getenv("BREVO_SENDER_EMAIL")
	}
	if b.Sender == "" {
		b.Sender = "noreply@xochi.app"
	}

	payload := map[string]any{
		"sender":      map[string]string{"name": "Xochi", "email": b.Sender},
		"to":          []map[string]string{{"email": to}},
		"subject":     subject,
		"textContent": text,
	}

	bData, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "https://api.brevo.com/v3/smtp/email", bytes.NewBuffer(bData))
	req.Header.Set("api-key", b.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("brevo error: %d", resp.StatusCode)
	}
	return nil
}

func (b BrevoNotifier) SendConsent(to, from, token string) error {
	text := fmt.Sprintf("%s named you as a safety contact. Accept here: https://xochi.app/v1/contact/verify/%s", from, token)
	return b.send(to, "Safety Contact Request", text)
}

func (b BrevoNotifier) SendAlert(t Trip, to, ackToken string) error {
	text := fmt.Sprintf("No check-in from trip %s.\nAck here: https://xochi.app/v1/contact/ack/%s", t.ID, ackToken)
	return b.send(to, "[Guardian] Missed check-in", text)
}
