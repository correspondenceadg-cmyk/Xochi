package server

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PgStore struct {
	pool *pgxpool.Pool
}

func NewPgStore(pool *pgxpool.Pool) *PgStore {
	return &PgStore{pool: pool}
}

func (s *PgStore) Create(ctx context.Context, t *Trip) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO trips (id, state, bound_at, expires_at, grace_secs, closed_at, extended, message, has_loc, lat, lon, loc_at, notified)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		t.ID, t.State, t.BoundAt, t.ExpiresAt, t.GraceSecs, t.ClosedAt, t.Extended, t.Message, t.HasLoc, t.Lat, t.Lon, t.LocAt, t.Notified)
	if err != nil {
		return err
	}

	for i, c := range t.Contacts {
		_, err = tx.Exec(ctx, `INSERT INTO trip_contacts (trip_id, email, priority) VALUES ($1, $2, $3)`, t.ID, c, i+1)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *PgStore) Update(ctx context.Context, id string, fn func(*Trip) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var t Trip
	err = tx.QueryRow(ctx, `
		SELECT state, bound_at, expires_at, grace_secs, closed_at, extended, message, has_loc, lat, lon, loc_at, notified
		FROM trips WHERE id = $1 FOR UPDATE`, id).
		Scan(&t.State, &t.BoundAt, &t.ExpiresAt, &t.GraceSecs, &t.ClosedAt, &t.Extended, &t.Message, &t.HasLoc, &t.Lat, &t.Lon, &t.LocAt, &t.Notified)
	if err == pgx.ErrNoRows {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	t.ID = id

	rows, err := tx.Query(ctx, `SELECT email FROM trip_contacts WHERE trip_id = $1 ORDER BY priority`, id)
	if err != nil {
		return err
	}
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return err
		}
		t.Contacts = append(t.Contacts, email)
	}

	if err := fn(&t); err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		UPDATE trips SET state=$1, bound_at=$2, expires_at=$3, grace_secs=$4, closed_at=$5, extended=$6, message=$7, has_loc=$8, lat=$9, lon=$10, loc_at=$11, notified=$12
		WHERE id=$13`,
		t.State, t.BoundAt, t.ExpiresAt, t.GraceSecs, t.ClosedAt, t.Extended, t.Message, t.HasLoc, t.Lat, t.Lon, t.LocAt, t.Notified, t.ID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *PgStore) Get(ctx context.Context, id string) (Trip, error) {
	var t Trip
	err := s.pool.QueryRow(ctx, `
		SELECT state, bound_at, expires_at, grace_secs, closed_at, extended, message, has_loc, lat, lon, loc_at, notified
		FROM trips WHERE id = $1`, id).
		Scan(&t.State, &t.BoundAt, &t.ExpiresAt, &t.GraceSecs, &t.ClosedAt, &t.Extended, &t.Message, &t.HasLoc, &t.Lat, &t.Lon, &t.LocAt, &t.Notified)
	if err == pgx.ErrNoRows {
		return Trip{}, ErrNotFound
	} else if err != nil {
		return Trip{}, err
	}
	t.ID = id

	rows, err := s.pool.Query(ctx, `SELECT email FROM trip_contacts WHERE trip_id = $1 ORDER BY priority`, id)
	if err != nil {
		return Trip{}, err
	}
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return Trip{}, err
		}
		t.Contacts = append(t.Contacts, email)
	}

	return t, nil
}

func (s *PgStore) Pending(ctx context.Context) []string {
	rows, err := s.pool.Query(ctx, `
		SELECT id FROM trips 
		WHERE state IN ('OPEN', 'GRACE') 
		   OR (state IN ('UNKNOWN', 'DISTRESS') AND NOT notified)`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

func (s *PgStore) Prune(ctx context.Context, now int64) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE trips SET has_loc=false, lat=NULL, lon=NULL 
		WHERE closed_at > 0 AND closed_at < $1 - 86400`, now)
	return err
}
