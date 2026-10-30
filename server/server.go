package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/correspondenceadg-cmyk/Xochi/guardian"
)

type Server struct {
	Store      Store
	Notifier   Notifier
	Now        func() int64
	TickSecret string
	GraceSecs  int64
}

func New(st Store, n Notifier, tickSecret string) *Server {
	return &Server{Store: st, Notifier: n, TickSecret: tickSecret, GraceSecs: 300,
		Now: func() int64 { return time.Now().Unix() }}
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
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	return mux
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
		Interval: *guardian.New(newID(), now, int64(in.WindowMinutes)*60, s.GraceSecs),
		Contacts: in.Contacts, Message: in.Message,
	}
	if err := s.Store.Create(t); err != nil {
		writeJSON(w, 500, map[string]string{"error": "internal"})
		return
	}
	writeJSON(w, 201, map[string]any{"id": t.ID, "expires_at": t.ExpiresAt})
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
	err := s.Store.Update(r.PathValue("id"), func(t *Trip) error {
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
	writeJSON(w, 200, map[string]any{"state": guardian.Open, "expires_at": exp})
}

func (s *Server) safe(w http.ResponseWriter, r *http.Request) {
	err := s.Store.Update(r.PathValue("id"), func(t *Trip) error {
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
	writeJSON(w, 200, map[string]any{"state": guardian.Safe})
}

// distress response -> identical shape to safe/extend (no delivery info; cover-mode)
func (s *Server) distress(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var alert *guardian.Alert
	err := s.Store.Update(id, func(t *Trip) error {
		a, err := t.Distress(s.Now())
		alert = a
		return err
	})
	if err != nil {
		fail(w, err)
		return
	}
	s.deliver(id, *alert)
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
	err := s.Store.Update(r.PathValue("id"), func(t *Trip) error {
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
	t, err := s.Store.Get(r.PathValue("id"))
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
	processed, alerts := 0, 0
	for _, id := range s.Store.Pending() {
		var alert *guardian.Alert
		err := s.Store.Update(id, func(t *Trip) error {
			if !t.State.Closed() {
				alert = t.Tick(s.Now())
			} else if !t.Notified {
				alert = &guardian.Alert{Kind: t.State, At: t.ClosedAt}
			}
			return nil
		})
		if err != nil {
			continue
		}
		processed++
		if alert != nil {
			alerts++
			s.deliver(id, *alert)
		}
	}
	writeJSON(w, 200, map[string]any{"processed": processed, "alerts": alerts, "at": s.Now()})
}

func (s *Server) deliver(id string, a guardian.Alert) {
	t, err := s.Store.Get(id)
	if err != nil {
		return
	}
	if s.Notifier.Notify(t, a) == nil {
		s.Store.Update(id, func(t *Trip) error { t.Notified = true; return nil })
	}
}
