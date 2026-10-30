package server

import (
	"log"

	"github.com/correspondenceadg-cmyk/Xochi/guardian"
)

type Notifier interface {
	Notify(t Trip, a guardian.Alert) error
}

type LogNotifier struct{}

func (LogNotifier) Notify(t Trip, a guardian.Alert) error {
	log.Printf("ALERT %s trip=%s contacts=%d loc=%v", a.Kind, t.ID, len(t.Contacts), t.HasLoc)
	return nil
}
