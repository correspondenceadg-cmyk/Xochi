package server

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/correspondenceadg-cmyk/Xochi/guardian"
	"github.com/jackc/pgx/v5/pgxpool"
)

func runStoreTests(t *testing.T, newStore func() Store) {
	ctx := context.Background()
	s := newStore()
	now := time.Now().Unix()

	t1 := &Trip{
		Interval: *guardian.New("t1", now, 600, 300),
		Contacts: []string{"a@b.c", "d@e.f"},
		Message:  "test",
	}
	if err := s.Create(ctx, t1); err != nil {
		t.Fatal(err)
	}

	got, err := s.Get(ctx, "t1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Message != "test" || len(got.Contacts) != 2 {
		t.Fatalf("bad get: %+v", got)
	}

	err = s.Update(ctx, "t1", func(t *Trip) error {
		t.Message = "updated"
		return t.Release(now)
	})
	if err != nil {
		t.Fatal(err)
	}

	got, _ = s.Get(ctx, "t1")
	if got.State != guardian.Safe || got.Message != "updated" {
		t.Fatal("bad update")
	}

	if len(s.Pending(ctx)) != 0 {
		t.Fatal("should not be pending")
	}

	t2 := &Trip{Interval: *guardian.New("t2", now, 600, 300)}
	s.Create(ctx, t2)
	if len(s.Pending(ctx)) != 1 {
		t.Fatal("should be pending")
	}

	s.Prune(ctx, now+86401)
	got2, _ := s.Get(ctx, "t1")
	if got2.HasLoc {
		t.Fatal("prune failed")
	}
}

func TestMemStore(t *testing.T) {
	runStoreTests(t, func() Store { return NewMemStore() })
}

func TestPgStore(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL unset")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	runStoreTests(t, func() Store { return NewPgStore(pool) })
}
