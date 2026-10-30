# Threshold Guardian

Trip-scoped safety check-in. See PROJECT_OPMAP for the full plan.

## Run locally
    TICK_SECRET=dev go run ./cmd/guardian
    go test ./...

## A full walk with curl
    curl -s localhost:8080/v1/interval -d '{"window_minutes":1,"contacts":["me@example.com"],"message":"Missed my window"}'
    # -> {"id":"<ID>",...}
    curl -s -XPOST localhost:8080/v1/interval/<ID>/safe
    # or let it expire, then drive time forward:
    curl -s -XPOST localhost:8080/v1/tick -H 'X-Tick-Secret: dev'

## Deploy model
Free hosts sleep, so alerts are driven by an external pinger hitting
POST /v1/tick every minute (header X-Tick-Secret). Tick is idempotent
and retries undelivered alerts.
