# Xochi Architecture

Xochi is a multi-platform safety application. It relies on three distinct codebases communicating via a shared REST API.

## Components

1. **Backend (Go)**: The source of truth. Handles the state machine for intervals (Open -> Grace -> Closed). It uses Postgres to store trips, location pings, and consent records. It relies on an external cron job (or the `/v1/tick` endpoint) to process expirations and send emails via Brevo.
2. **Web (TypeScript/Vite)**: A PWA that acts as the primary interface for desktop and mobile web users. It communicates directly with the Go backend.
3. **Android (Kotlin/Compose)**: A native application that provides reliable background location tracking using a Foreground Service. It contains a "Cover Mode" (a fake timer app) to hide the safety features from casual observers.

## Data Flow

- When a trip is opened, the client receives a `trip_id` and `expires_at` timestamp.
- Clients ping the server periodically with their location via `/v1/interval/{id}/ping`.
- If the timer expires, the server transitions the state to `GRACE`.
- If the grace period expires without a `/safe` check-in, the server sends an email to the emergency contact with the last known coordinates.
