package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/correspondenceadg-cmyk/Xochi/guardian"
)

type fakeNotifier struct {
	sent []guardian.State
	fail bool
}

func (f *fakeNotifier) SendConsent(to, from, token string) error { return nil }

func (f *fakeNotifier) SendAlert(t Trip, to, ackToken string) error {
	if f.fail {
		return errors.New("smtp down")
	}
	f.sent = append(f.sent, t.State)
	return nil
}

type harness struct {
	t   *testing.T
	srv *Server
	n   *fakeNotifier
	now int64
	h   http.Handler
}

func newHarness(t *testing.T) *harness {
	n := &fakeNotifier{}
	hs := &harness{t: t, n: n}
	hs.srv = New(NewMemStore(), n, "s3cret")
	hs.srv.Now = func() int64 { return hs.now }
	hs.h = hs.srv.Handler()
	return hs
}

func (h *harness) do(method, path, body string, hdr ...string) (int, map[string]any) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func (h *harness) tick() map[string]any {
	code, out := h.do("POST", "/v1/tick", "", "X-Tick-Secret", "s3cret")
	if code != 200 {
		h.t.Fatalf("tick: %d", code)
	}
	return out
}

func (h *harness) open() string {
	code, out := h.do("POST", "/v1/interval",
		`{"window_minutes":10,"contacts":["a@example.com"],"message":"help"}`)
	if code != 201 {
		h.t.Fatalf("open: %d %v", code, out)
	}
	return out["id"].(string)
}

func TestFullWalkSafe(t *testing.T) {
	h := newHarness(t)
	id := h.open()
	h.now = 300
	h.do("POST", "/v1/interval/"+id+"/ping", `{"lat":41.88,"lon":-87.63}`)
	if code, _ := h.do("POST", "/v1/interval/"+id+"/safe", ""); code != 200 {
		t.Fatal("safe failed")
	}
	h.now = 99999
	h.tick()
	if len(h.n.sent) != 0 {
		t.Fatal("safe trip must never alert")
	}
}

func TestTimeoutAlertsOnce(t *testing.T) {
	h := newHarness(t)
	h.open()
	h.now = 600
	h.tick()
	h.now = 899
	h.tick()
	if len(h.n.sent) != 0 {
		t.Fatal("alert during grace")
	}
	h.now = 900
	h.tick()
	h.now = 960
	h.tick()
	if len(h.n.sent) != 1 || h.n.sent[0] != guardian.Unknown {
		t.Fatalf("want exactly one UNKNOWN alert, got %v", h.n.sent)
	}
}

func TestFailedDeliveryRetries(t *testing.T) {
	h := newHarness(t)
	h.open()
	h.n.fail = true
	h.now = 900
	h.tick()
	if len(h.n.sent) != 0 {
		t.Fatal("should not have delivered")
	}
	h.n.fail = false
	h.now = 960
	h.tick()
	if len(h.n.sent) != 1 {
		t.Fatalf("retry should deliver once, got %v", h.n.sent)
	}
	h.tick()
	if len(h.n.sent) != 1 {
		t.Fatal("must not re-deliver after success")
	}
}

func TestDistressImmediate(t *testing.T) {
	h := newHarness(t)
	id := h.open()
	h.now = 5
	h.do("POST", "/v1/interval/"+id+"/distress", "")
	if len(h.n.sent) != 1 || h.n.sent[0] != guardian.Distress {
		t.Fatalf("got %v", h.n.sent)
	}
	h.tick()
	if len(h.n.sent) != 1 {
		t.Fatal("duplicate after distress")
	}
}

func TestExtendPreventsAlert(t *testing.T) {
	h := newHarness(t)
	id := h.open()
	h.now = 550
	h.do("POST", "/v1/interval/"+id+"/extend", `{"minutes":20}`)
	h.now = 1000
	h.tick()
	if len(h.n.sent) != 0 {
		t.Fatal("extended trip alerted early")
	}
}

func TestTickRequiresSecret(t *testing.T) {
	h := newHarness(t)
	if code, _ := h.do("POST", "/v1/tick", ""); code != 401 {
		t.Fatal("tick must require secret")
	}
	if code, _ := h.do("POST", "/v1/tick", "", "X-Tick-Secret", "wrong"); code != 401 {
		t.Fatal("wrong secret accepted")
	}
}

func TestValidation(t *testing.T) {
	h := newHarness(t)
	if code, _ := h.do("POST", "/v1/interval", `{"window_minutes":0,"contacts":["a@b.c"]}`); code != 400 {
		t.Fatal("zero window accepted")
	}
	if code, _ := h.do("POST", "/v1/interval", `{"window_minutes":5,"contacts":[]}`); code != 400 {
		t.Fatal("no contacts accepted")
	}
	if code, _ := h.do("GET", "/v1/interval/nope/status", ""); code != 404 {
		t.Fatal("unknown id should 404")
	}
}
