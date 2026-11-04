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
	mux.HandleFunc("GET /v1/contact/verify/{token}", s.verifyPage)
	mux.HandleFunc("POST /v1/contact/verify/{token}", s.verifyAction)
	mux.HandleFunc("GET /v1/contact/ack/{token}", s.ack)
	mux.HandleFunc("GET /v1/contact/revoke/{token}", s.revokePage)
	mux.HandleFunc("POST /v1/contact/revoke/{token}", s.revokeAction)
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
		if !strings.Contains(c, "@") && !strings.HasPrefix(c, "+") {
			fail(w, errors.New("contacts must be email addresses or phone numbers (e.g. +15550100)"))
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
		// Do not set Notified=true here. Let tick() process the alert and retries.
		return nil
	})
	if err != nil {
		fail(w, err)
		return
	}
	var deliveryErrors int
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
		if s.processTrip(id) {
			alerts++
		}
		processed++
	}
	writeJSON(w, 200, map[string]any{"processed": processed, "alerts": alerts, "at": s.Now()})
}

func (s *Server) processTrip(id string) bool {
	var alert *guardian.Alert
	var contactToNotify string
	var ackToken string
	var now = s.Now()

	err := s.Store.Update(context.Background(), id, func(t *Trip) error {
		if !t.State.Closed() {
			alert = t.Tick(now)
		} else if !t.Notified {
			verified, _ := s.Store.GetConsent(context.Background(), t.Contacts[t.NextContactPriority-1])
			if !verified {
				t.NextContactPriority++
				return nil
			}
			if t.NextContactPriority <= len(t.Contacts) && (t.LastAlertAt == 0 || now-t.LastAlertAt >= 300) {
				contactToNotify = t.Contacts[t.NextContactPriority-1]
				ackToken = newID()

				t.LastAlertAt = now
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
		return false
	}
	
	if alert != nil && contactToNotify != "" {
		s.Store.SetAckToken(context.Background(), id, contactToNotify, ackToken)
		err := s.deliver(context.Background(), id, contactToNotify, ackToken, *alert)
		if err != nil {
			s.Store.Update(context.Background(), id, func(t *Trip) error {
				if t.LastAlertAt == now {
					t.LastAlertAt = 0
					t.NextContactPriority--
				}
				return nil
			})
			return false
		}
		return true
	}
	return false
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

func (s *Server) verifyPage(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(fmt.Sprintf(`<html><body style="background:#0f172a;color:#fff;font-family:sans-serif;text-align:center;padding:50px;"><h2>Verify Contact</h2><form method="POST"><button style="padding:10px 20px;font-size:16px;background:#38bdf8;border:none;border-radius:5px;cursor:pointer;">Click to Verify</button></form></body></html>`, token)))
}
func (s *Server) verifyAction(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	hash := sha256.Sum256([]byte(token))
	err := s.Store.VerifyConsent(r.Context(), hex.EncodeToString(hash[:]))
	if err != nil {
		fail(w, err)
		return
	}
	w.Write([]byte("verified"))
}

func (s *Server) ackPage(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<html><body style="background:#0f172a;color:#fff;font-family:sans-serif;text-align:center;padding:50px;"><h2>Acknowledge Alert</h2><form method="POST"><button style="padding:10px 20px;font-size:16px;background:#10b981;color:#fff;border:none;border-radius:5px;cursor:pointer;">Acknowledge</button></form></body></html>`))
}
func (s *Server) ackAction(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	err := s.Store.Ack(r.Context(), token)
	if err != nil {
		fail(w, err)
		return
	}
	w.Write([]byte("acknowledged"))
}

func (s *Server) revokePage(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<html><body style="background:#0f172a;color:#fff;font-family:sans-serif;text-align:center;padding:50px;"><h2>Revoke Consent</h2><form method="POST"><button style="padding:10px 20px;font-size:16px;background:#ef4444;border:none;border-radius:5px;cursor:pointer;">Revoke</button></form></body></html>`))
}
func (s *Server) revokeAction(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	hash := sha256.Sum256([]byte(token))
	err := s.Store.RevokeConsent(r.Context(), hex.EncodeToString(hash[:]))
	if err != nil {
		fail(w, err)
		return
	}
	w.Write([]byte("revoked"))
}
