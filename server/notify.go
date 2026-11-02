package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

type Notifier interface {
	SendConsent(to, from, token string) error
	SendAlert(t Trip, to, ackToken string) error
}

type TextBeeNotifier struct {
	APIKey   string
	DeviceID string
}

func (t TextBeeNotifier) SendConsent(to, from, token string) error {
	log.Printf("CONSENT to=%s from=%s token=%s", to, from, token)
	return nil
}

func (t TextBeeNotifier) SendAlert(trip Trip, to, ackToken string) error {
	text := fmt.Sprintf("No check-in from trip %s. Ack: https://xochiquetzal.de.deplexo.com/v1/contact/ack/%s", trip.ID, ackToken)
	if !strings.HasPrefix(to, "+") {
		return fmt.Errorf("phone number must include country code")
	}
	
	payload := map[string]interface{}{
		"recipients": []string{to},
		"message":    text,
	}
	bData, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", fmt.Sprintf("https://api.textbee.dev/api/v1/gateway/devices/%s/send-sms", t.DeviceID), bytes.NewBuffer(bData))
	req.Header.Set("x-api-key", t.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("textbee error %d: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

type MultiNotifier struct {
	Email Notifier
	SMS   Notifier
}

func (m MultiNotifier) SendConsent(to, from, token string) error {
	if strings.Contains(to, "@") { return m.Email.SendConsent(to, from, token) }
	return m.SMS.SendConsent(to, from, token)
}

func (m MultiNotifier) SendAlert(t Trip, to, ackToken string) error {
	if strings.Contains(to, "@") { return m.Email.SendAlert(t, to, ackToken) }
	return m.SMS.SendAlert(t, to, ackToken)
}

type LogNotifier struct{}

func (LogNotifier) SendConsent(to, from, token string) error {
	log.Printf("CONSENT to=%s from=%s token=%s", to, from, token)
	return nil
}

func (LogNotifier) SendAlert(t Trip, to, ackToken string) error {
	log.Printf("ALERT to=%s trip=%s state=%s ack=%s", to, t.ID, t.State, ackToken)
	return nil
}
