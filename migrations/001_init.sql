CREATE TABLE trips (
  id          text PRIMARY KEY,
  state       text   NOT NULL,
  bound_at    bigint NOT NULL,
  expires_at  bigint NOT NULL,
  grace_secs  int    NOT NULL,
  closed_at   bigint NOT NULL DEFAULT 0,
  extended    int    NOT NULL DEFAULT 0,
  message     text   NOT NULL DEFAULT '',
  has_loc     boolean NOT NULL DEFAULT false,
  lat double precision, lon double precision, loc_at bigint,
  notified    boolean NOT NULL DEFAULT false
);
CREATE INDEX trips_pending ON trips (state, notified);

CREATE TABLE trip_contacts (
  trip_id  text REFERENCES trips(id) ON DELETE CASCADE,
  email    text NOT NULL,
  priority int  NOT NULL DEFAULT 1,
  PRIMARY KEY (trip_id, email)
);

CREATE TABLE contact_consent (
  email       text PRIMARY KEY,
  token_hash  text NOT NULL,
  verified_at timestamptz,
  revoked_at  timestamptz
);

CREATE TABLE closures (
  trip_id text REFERENCES trips(id) ON DELETE CASCADE,
  state text NOT NULL, recorded_at bigint NOT NULL, signed_hash text
);

CREATE TABLE acks (
  trip_id text REFERENCES trips(id) ON DELETE CASCADE,
  email text NOT NULL, response text NOT NULL, responded_at bigint NOT NULL
);
