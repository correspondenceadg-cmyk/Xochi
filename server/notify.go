package server

import (
	"log"
)

type Notifier interface {
	SendConsent(to, from, token string) error
	SendAlert(t Trip, to, ackToken string) error
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
