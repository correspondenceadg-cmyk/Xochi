package server

import (
	"context"
	"errors"
	"sync"

	"github.com/correspondenceadg-cmyk/Xochi/guardian"
)

var ErrNotFound = errors.New("not found")

type Trip struct {
	guardian.Interval
	Contacts            []string
	Message             string
	HasLoc              bool
	Lat, Lon            float64
	LocAt               int64
	Notified            bool
	LastAlertAt         int64
	NextContactPriority int
}

type Store interface {
	Create(ctx context.Context, t *Trip) error
	Update(ctx context.Context, id string, fn func(*Trip) error) error
	Get(ctx context.Context, id string) (Trip, error)
	Pending(ctx context.Context) []string
	Prune(ctx context.Context, now int64) error
	GetConsent(ctx context.Context, email string) (bool, error)
	CreateConsent(ctx context.Context, email, tokenHash string) error
	VerifyConsent(ctx context.Context, tokenHash string) error
	RevokeConsent(ctx context.Context, tokenHash string) error
	SetAckToken(ctx context.Context, tripID, email, token string) error
	Ack(ctx context.Context, token string) error
}

type MemStore struct {
	mu sync.Mutex
	m  map[string]*Trip
}

func NewMemStore() *MemStore { return &MemStore{m: map[string]*Trip{}} }

func (s *MemStore) Create(ctx context.Context, t *Trip) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.NextContactPriority == 0 {
		t.NextContactPriority = 1
	}
	s.m[t.ID] = t
	return nil
}

func (s *MemStore) Update(ctx context.Context, id string, fn func(*Trip) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.m[id]
	if !ok {
		return ErrNotFound
	}
	return fn(t)
}

func (s *MemStore) Get(ctx context.Context, id string) (Trip, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.m[id]
	if !ok {
		return Trip{}, ErrNotFound
	}
	return *t, nil
}

func (s *MemStore) Pending(ctx context.Context) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ids []string
	for id, t := range s.m {
		switch t.State {
		case guardian.Open, guardian.Grace, guardian.Unknown, guardian.Distress:
			ids = append(ids, id)
		}
	}
	return ids
}

func (s *MemStore) Prune(ctx context.Context, now int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.m {
		if t.ClosedAt > 0 && t.ClosedAt < now-86400 {
			t.HasLoc = false
			t.Lat, t.Lon = 0, 0
			t.LocAt = 0
		}
	}
	return nil
}

func (s *MemStore) GetConsent(ctx context.Context, email string) (bool, error)         { return true, nil }
func (s *MemStore) CreateConsent(ctx context.Context, email, tokenHash string) error   { return nil }
func (s *MemStore) VerifyConsent(ctx context.Context, tokenHash string) error          { return nil }
func (s *MemStore) RevokeConsent(ctx context.Context, tokenHash string) error          { return nil }
func (s *MemStore) SetAckToken(ctx context.Context, tripID, email, token string) error { return nil }
func (s *MemStore) Ack(ctx context.Context, token string) error                        { return nil }
