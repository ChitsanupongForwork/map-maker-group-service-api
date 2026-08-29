# Map Maker Group Service API

The service reads and writes the `map-maker-db` PostgreSQL schema and sends live vehicle changes through Server-Sent Events.

Set `DATABASE_URL` only in your local terminal or ignored `.env` file, then run `go mod tidy` once and `go run .`.

On startup, the service tops up its fictional `FV-` demo records to 1,000 vehicles without touching non-demo records. It writes only 40 changing positions every two seconds so the portfolio remains responsive.

Endpoints: `GET /healthz`, `GET /api/fleet`, `POST /api/fleet/seed`, and `GET /api/fleet/stream`.

## Deployment

For the first Go deployment, set `DEMO_MODE=true`. It starts the same 1,000-vehicle live simulator without a database, so the public API can be verified before PostgreSQL is provisioned.

When PostgreSQL is ready, remove `DEMO_MODE`, set the secret `DATABASE_URL`, and set `DB_SCHEMA=map-maker-db`. Set `FRONTEND_ORIGINS` to a comma-separated allow-list such as `http://localhost:3000,https://map-maker-group.vercel.app`.
