# Map Maker Group Service API

The service reads and writes the `map-maker-db` PostgreSQL schema and sends live vehicle changes through Server-Sent Events.

Set `DATABASE_URL` only in your local terminal or ignored `.env` file, then run `go mod tidy` once and `go run .`.

On startup, the service tops up its fictional `FV-` demo records to 1,000 vehicles without touching non-demo records. PostgreSQL changes to vehicles, live states, trips, drivers, and new position events are sent to dashboard clients through SSE. Set `SIMULATOR_ENABLED=true` only when you want the optional 40-vehicle movement demo every two seconds; production defaults to database-driven updates so manual edits are never overwritten.

Endpoints: `GET /healthz`, `GET /api/fleet`, `POST /api/fleet/seed`, and `GET /api/fleet/stream`.

## Deployment

Set the secret `DATABASE_URL`, `DB_SCHEMA=map-maker-db`, `SIMULATOR_ENABLED=false`, and `FRONTEND_ORIGINS` as a comma-separated allow-list such as `http://localhost:3000,https://map-maker-group.vercel.app`.
