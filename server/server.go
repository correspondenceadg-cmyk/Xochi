package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/correspondenceadg-cmyk/Xochi/guardian"
)

type Server struct {
	Store      Store
	Notifier   Notifier
	Now        func() int64
	TickSecret string
	GraceSecs  int64
	lastTick   atomic.Int64
	rl         *rateLimiter
}

func New(st Store, n Notifier, tickSecret string) *Server {
	return &Server{Store: st, Notifier: n, TickSecret: tickSecret, GraceSecs: 300,
		Now: func() int64 { return time.Now().Unix() }, rl: newRateLimiter()}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/interval", s.open)
	mux.HandleFunc("POST /v1/interval/{id}/extend", s.extend)
	mux.HandleFunc("POST /v1/interval/{id}/safe", s.safe)
	mux.HandleFunc("POST /v1/interval/{id}/distress", s.distress)
	mux.HandleFunc("POST /v1/interval/{id}/ping", s.ping)
	mux.HandleFunc("GET /v1/interval/{id}/status", s.status)
	mux.HandleFunc("POST /v1/tick", s.tick)
	mux.HandleFunc("POST /v1/test-alert", s.testAlert)
	mux.HandleFunc("GET /v1/contact/verify/{token}", s.verify)
	mux.HandleFunc("GET /v1/contact/ack/{token}", s.ack)
	mux.HandleFunc("GET /v1/contact/revoke/{token}", s.revoke)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		notifierType := "log"
		if _, ok := s.Notifier.(BrevoNotifier); ok {
			notifierType = "brevo"
		}
		writeJSON(w, 200, map[string]any{"status": "ok", "last_tick_at": s.lastTick.Load(), "notifier": notifierType})
	})
	return cors(s.rl.limit(mux))
}

// newID() -> 32 hex chars; possession authorizes every call on that interval
func newID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "not found"})
	case errors.Is(err, guardian.ErrClosed):
		writeJSON(w, 409, map[string]string{"error": "interval closed"})
	default:
		writeJSON(w, 400, map[string]string{"error": err.Error()})
	}
}

func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	return json.NewDecoder(r.Body).Decode(v)
}

func (s *Server) open(w http.ResponseWriter, r *http.Request) {
	var in struct {
		WindowMinutes int      `json:"window_minutes"`
		GraceSecs     int64    `json:"grace_secs"`
		Contacts      []string `json:"contacts"`
		Message       string   `json:"message"`
	}
	if err := decode(w, r, &in); err != nil {
		fail(w, err)
		return
	}
	if in.WindowMinutes < 1 || in.WindowMinutes > 720 {
		fail(w, errors.New("window_minutes must be 1-720"))
		return
	}
	if in.GraceSecs < 0 {
		in.GraceSecs = 0
	}
	if len(in.Contacts) < 1 || len(in.Contacts) > 5 {
		fail(w, errors.New("1-5 contacts required"))
		return
	}
	for _, c := range in.Contacts {
		if !strings.Contains(c, "@") {
			fail(w, errors.New("contacts must be email addresses"))
			return
		}
	}
	now := s.Now()
	t := &Trip{
		Interval: *guardian.New(newID(), now, int64(in.WindowMinutes)*60, in.GraceSecs),
		Contacts: in.Contacts, Message: in.Message,
		NextContactPriority: 1,
	}
	if err := s.Store.Create(r.Context(), t); err != nil {
		slog.Error("create_trip_failed", "error", err)
		writeJSON(w, 500, map[string]string{"error": "internal"})
		return
	}

	for _, c := range in.Contacts {
		verified, err := s.Store.GetConsent(r.Context(), c)
		if err != nil {
			continue
		}
		if !verified {
			token := newID()
			hash := sha256.Sum256([]byte(token))
			s.Store.CreateConsent(r.Context(), c, hex.EncodeToString(hash[:]))
			s.Notifier.SendConsent(c, "Guardian", token)
		}
	}

	slog.Info("trip_opened", "trip", t.ID[:8])
	writeJSON(w, 201, map[string]any{"id": t.ID, "expires_at": t.ExpiresAt, "grace_secs": in.GraceSecs})
}

func (s *Server) extend(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Minutes int `json:"minutes"`
	}
	if err := decode(w, r, &in); err != nil || in.Minutes < 1 || in.Minutes > 720 {
		fail(w, errors.New("minutes must be 1-720"))
		return
	}
	var exp int64
	err := s.Store.Update(r.Context(), r.PathValue("id"), func(t *Trip) error {
		if err := t.Extend(s.Now(), int64(in.Minutes)*60); err != nil {
			return err
		}
		exp = t.ExpiresAt
		return nil
	})
	if err != nil {
		fail(w, err)
		return
	}
	slog.Info("trip_extended", "trip", r.PathValue("id")[:8])
	writeJSON(w, 200, map[string]any{"state": guardian.Open, "expires_at": exp})
}

func (s *Server) safe(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.Store.Update(r.Context(), id, func(t *Trip) error {
		if err := t.Release(s.Now()); err != nil {
			return err
		}
		t.Notified = true
		return nil
	})
	if err != nil {
		fail(w, err)
		return
	}
	slog.Info("trip_safe", "trip", id[:8])
	writeJSON(w, 200, map[string]any{"state": guardian.Safe})
}

func (s *Server) distress(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var alert *guardian.Alert
	var contactsToNotify []string
	var ackTokens []string
	err := s.Store.Update(r.Context(), id, func(t *Trip) error {
		a, err := t.Distress(s.Now())
		if err != nil {
			return err
		}
		alert = a
		t.Notified = true
		for _, c := range t.Contacts {
			contactsToNotify = append(contactsToNotify, c)
			ackTokens = append(ackTokens, newID())
		}
		return nil
	})
	if err != nil {
		fail(w, err)
		return
	}
	var deliveryErrors int
	for i, c := range contactsToNotify {
		s.Store.SetAckToken(r.Context(), id, c, ackTokens[i])
		if err := s.deliver(r.Context(), id, c, ackTokens[i], *alert); err != nil {
			slog.Error("deliver_failed", "trip", id[:8], "contact", c, "error", err)
			deliveryErrors++
		}
	}
	if deliveryErrors > 0 {
		writeJSON(w, 500, map[string]string{"error": "Email delivery failed. Check Brevo API configuration on server."})
		return
	}
	slog.Warn("trip_distress", "trip", id[:8])
	writeJSON(w, 200, map[string]any{"state": guardian.Distress})
}

func (s *Server) ping(w http.ResponseWriter, r *http.Request) {
	var in struct{ Lat, Lon float64 }
	if err := decode(w, r, &in); err != nil || in.Lat < -90 || in.Lat > 90 || in.Lon < -180 || in.Lon > 180 {
		fail(w, errors.New("bad coordinates"))
		return
	}
	var st guardian.State
	var exp int64
	err := s.Store.Update(r.Context(), r.PathValue("id"), func(t *Trip) error {
		if t.State.Closed() {
			return guardian.ErrClosed
		}
		t.HasLoc, t.Lat, t.Lon, t.LocAt = true, in.Lat, in.Lon, s.Now()
		st, exp = t.State, t.ExpiresAt
		return nil
	})
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"state": st, "expires_at": exp})
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	t, err := s.Store.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{
		"state": t.State, "expires_at": t.ExpiresAt, "extended": t.Extended,
	})
}

func (s *Server) tick(w http.ResponseWriter, r *http.Request) {
	got := []byte(r.Header.Get("X-Tick-Secret"))
	if s.TickSecret == "" || subtle.ConstantTimeCompare(got, []byte(s.TickSecret)) != 1 {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}

	s.lastTick.Store(s.Now())
	s.Store.Prune(r.Context(), s.Now())

	processed, alerts := 0, 0
	for _, id := range s.Store.Pending(r.Context()) {
		var alert *guardian.Alert
		var contactToNotify string
		var ackToken string

		err := s.Store.Update(r.Context(), id, func(t *Trip) error {
			if !t.State.Closed() {
				alert = t.Tick(s.Now())
			} else if !t.Notified {
				if t.NextContactPriority <= len(t.Contacts) && (t.LastAlertAt == 0 || s.Now()-t.LastAlertAt >= 300) {
					contactToNotify = t.Contacts[t.NextContactPriority-1]
					ackToken = newID()

					t.LastAlertAt = s.Now()
					t.NextContactPriority++
					if t.NextContactPriority > len(t.Contacts) {
						t.Notified = true
					}

					if alert == nil {
						alert = &guardian.Alert{Kind: t.State, At: t.ClosedAt}
					}
				}
			}
			return nil
		})
		if err != nil {
			continue
		}
		processed++
		if alert != nil && contactToNotify != "" {
			alerts++
			s.Store.SetAckToken(r.Context(), id, contactToNotify, ackToken)
			err := s.deliver(r.Context(), id, contactToNotify, ackToken, *alert)
			if err != nil {
				s.Store.Update(r.Context(), id, func(t *Trip) error {
					if t.LastAlertAt == s.Now() {
						t.LastAlertAt = 0
						t.NextContactPriority--
					}
					return nil
				})
			}
		}
	}
	writeJSON(w, 200, map[string]any{"processed": processed, "alerts": alerts, "at": s.Now()})
}

func (s *Server) testAlert(w http.ResponseWriter, r *http.Request) {
	var in struct{ Email string }
	if err := decode(w, r, &in); err != nil || !strings.Contains(in.Email, "@") {
		fail(w, errors.New("valid email required"))
		return
	}
	t := Trip{Interval: guardian.Interval{ID: "test1234"}}
	if err := s.Notifier.SendAlert(t, in.Email, "test-token"); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "sent"})
}

func (s *Server) deliver(ctx context.Context, id, contact, ackToken string, a guardian.Alert) error {
	t, err := s.Store.Get(ctx, id)
	if err != nil {
		return err
	}
	slog.Info("alert_dispatching", "trip", id[:8], "kind", a.Kind)
	return s.Notifier.SendAlert(t, contact, ackToken)
}

func (s *Server) verify(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	hash := sha256.Sum256([]byte(token))
	err := s.Store.VerifyConsent(r.Context(), hex.EncodeToString(hash[:]))
	if err != nil {
		fail(w, err)
		return
	}
	w.Write([]byte("verified"))
}

func (s *Server) ack(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	err := s.Store.Ack(r.Context(), token)
	if err != nil {
		fail(w, err)
		return
	}
	w.Write([]byte("acknowledged"))
}

func (s *Server) revoke(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	hash := sha256.Sum256([]byte(token))
	err := s.Store.RevokeConsent(r.Context(), hex.EncodeToString(hash[:]))
	if err != nil {
		fail(w, err)
		return
	}
	w.Write([]byte("revoked"))
}
