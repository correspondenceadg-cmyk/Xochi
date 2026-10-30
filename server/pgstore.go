package server

import (
	"context"
	"time"

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
		INSERT INTO trips (id, state, bound_at, expires_at, grace_secs, closed_at, extended, message, has_loc, lat, lon, loc_at, notified, last_alert_at, next_contact_priority)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		t.ID, t.State, t.BoundAt, t.ExpiresAt, t.GraceSecs, t.ClosedAt, t.Extended, t.Message, t.HasLoc, t.Lat, t.Lon, t.LocAt, t.Notified, t.LastAlertAt, t.NextContactPriority)
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
		SELECT state, bound_at, expires_at, grace_secs, closed_at, extended, message, has_loc, lat, lon, loc_at, notified, last_alert_at, next_contact_priority
		FROM trips WHERE id = $1 FOR UPDATE`, id).
		Scan(&t.State, &t.BoundAt, &t.ExpiresAt, &t.GraceSecs, &t.ClosedAt, &t.Extended, &t.Message, &t.HasLoc, &t.Lat, &t.Lon, &t.LocAt, &t.Notified, &t.LastAlertAt, &t.NextContactPriority)
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
		UPDATE trips SET state=$1, bound_at=$2, expires_at=$3, grace_secs=$4, closed_at=$5, extended=$6, message=$7, has_loc=$8, lat=$9, lon=$10, loc_at=$11, notified=$12, last_alert_at=$13, next_contact_priority=$14
		WHERE id=$15`,
		t.State, t.BoundAt, t.ExpiresAt, t.GraceSecs, t.ClosedAt, t.Extended, t.Message, t.HasLoc, t.Lat, t.Lon, t.LocAt, t.Notified, t.LastAlertAt, t.NextContactPriority, t.ID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *PgStore) Get(ctx context.Context, id string) (Trip, error) {
	var t Trip
	err := s.pool.QueryRow(ctx, `
		SELECT state, bound_at, expires_at, grace_secs, closed_at, extended, message, has_loc, lat, lon, loc_at, notified, last_alert_at, next_contact_priority
		FROM trips WHERE id = $1`, id).
		Scan(&t.State, &t.BoundAt, &t.ExpiresAt, &t.GraceSecs, &t.ClosedAt, &t.Extended, &t.Message, &t.HasLoc, &t.Lat, &t.Lon, &t.LocAt, &t.Notified, &t.LastAlertAt, &t.NextContactPriority)
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
	rows, err := s.pool.Query(ctx, `SELECT id FROM trips WHERE state IN ('OPEN', 'GRACE', 'UNKNOWN', 'DISTRESS')`)
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

func (s *PgStore) GetConsent(ctx context.Context, email string) (bool, error) {
	var verified bool
	err := s.pool.QueryRow(ctx, `SELECT verified_at IS NOT NULL FROM contact_consent WHERE email = $1`, email).Scan(&verified)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	return verified, err
}

func (s *PgStore) CreateConsent(ctx context.Context, email, tokenHash string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO contact_consent (email, token_hash) VALUES ($1, $2)
		ON CONFLICT (email) DO UPDATE SET token_hash = $2, verified_at = NULL`, email, tokenHash)
	return err
}

func (s *PgStore) VerifyConsent(ctx context.Context, tokenHash string) error {
	cmd, err := s.pool.Exec(ctx, `UPDATE contact_consent SET verified_at = now() WHERE token_hash = $1`, tokenHash)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PgStore) RevokeConsent(ctx context.Context, tokenHash string) error {
	cmd, err := s.pool.Exec(ctx, `UPDATE contact_consent SET revoked_at = now() WHERE token_hash = $1`, tokenHash)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PgStore) SetAckToken(ctx context.Context, tripID, email, token string) error {
	_, err := s.pool.Exec(ctx, `UPDATE trip_contacts SET ack_token = $1 WHERE trip_id = $2 AND email = $3`, token, tripID, email)
	return err
}

func (s *PgStore) Ack(ctx context.Context, token string) error {
	var tripID, email string
	err := s.pool.QueryRow(ctx, `UPDATE trip_contacts SET ack_token = NULL WHERE ack_token = $1 RETURNING trip_id, email`, token).Scan(&tripID, &email)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO acks (trip_id, email, response, responded_at) VALUES ($1, $2, 'ACK', $3)`, tripID, email, time.Now().Unix())
	return err
}
