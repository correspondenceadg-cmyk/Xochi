ALTER TABLE trips ADD COLUMN last_alert_at bigint NOT NULL DEFAULT 0;
ALTER TABLE trips ADD COLUMN next_contact_priority int NOT NULL DEFAULT 1;
ALTER TABLE trip_contacts ADD COLUMN ack_token text;
CREATE INDEX trip_contacts_ack ON trip_contacts (ack_token);
CREATE INDEX contact_consent_token ON contact_consent (token_hash);
