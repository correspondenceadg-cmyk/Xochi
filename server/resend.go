package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type ResendNotifier struct {
	APIKey string
	From   string
}

func (r ResendNotifier) send(to, subject, text string) error {
	body := map[string]any{
		"from":    r.From,
		"to":      []string{to},
		"subject": subject,
		"text":    text,
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+r.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("resend error: %d", resp.StatusCode)
	}
	return nil
}

func (r ResendNotifier) SendConsent(to, from, token string) error {
	text := fmt.Sprintf("%s named you as a safety contact. Accept here: https://xochi.app/v1/contact/verify/%s", from, token)
	return r.send(to, "Safety Contact Request", text)
}

func (r ResendNotifier) SendAlert(t Trip, to, ackToken string) error {
	text := fmt.Sprintf("No check-in from trip %s.\nAck here: https://xochi.app/v1/contact/ack/%s", t.ID, ackToken)
	return r.send(to, "[Guardian] Missed check-in", text)
}
