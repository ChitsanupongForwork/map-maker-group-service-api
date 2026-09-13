# Map Maker Group Service API

Go API for the `map-maker-group` front-end: the `/map` page and the `/history` page.
The contract is `API-REQUIREMENTS.md`; the code layout (level 4, one package per feature under `internal/`) is `GO-STRUCTURE.md`.

## Run locally

```powershell
go run ./cmd/api
```

Without `DATABASE_URL` the server starts in **demo mode**: 107 in-memory vehicles that keep moving, and generated trip history for `v-0001` … `v-0107`. Nothing else to install.

With PostgreSQL: copy `.env.example` to `.env`, set `DATABASE_URL` and `DB_SCHEMA=map-maker-db-new`, run the migrations, then check the connection step by step (read-only, same `.env` as the server):

```powershell
go run ./cmd/dbcheck
```

It reports whether it can connect, whether the schema, tables, view, functions and grants are all there, and how much data exists. Every failure comes with a fix, such as a wrong password, a schema name used as the database name, `?sslmode=require` for Render, or a password that needs URL-encoding. When it prints `พร้อมแล้ว`, run `go run ./cmd/api`. Add `SIMULATE=true` to make online vehicles move until real devices send data.

`DATABASE_URL` examples:

```
# Postgres on this machine
DATABASE_URL=postgres://postgres:PASSWORD@127.0.0.1:5432/postgres?sslmode=disable
# Render: copy the External Database URL from the dashboard, then add sslmode
DATABASE_URL=postgres://USER:PASSWORD@dpg-xxxx.oregon-postgres.render.com/DBNAME?sslmode=require
```

```powershell
go test ./internal/...
```

Use `./internal/...`, not `./...` — the `lessons/` folders are separate programs.

## Endpoints

| Method | Path | Mode | Notes |
|---|---|---|---|
| GET | `/healthz` | both | `{"status":"ok","mode":"demo"}` — pings the database in postgres mode |
| GET | `/api/fleet` | both | Every vehicle in one response, no filters (spec §3) |
| GET | `/api/fleet/stream` | both | SSE: one `snapshot` event on connect, then `patch` events every ~3 s, `: heartbeat` every 20 s |
| GET | `/api/vehicles/{id}/history?from=&to=` | both | `from`/`to` in epoch **ms**, range ≤ 31 days, ≤ 5,000 points, summary + events computed from raw points |
| POST | `/api/positions` | database only | One position object or an array (≤ 1,000). Returns `inserted`, `duplicates`, `liveStateUpdated` |

`POST /api/positions` body:

```json
{
  "vehicleId": "v-0001",
  "recordedAt": 1789765433000,
  "lat": 13.8015,
  "lng": 100.5544,
  "speedKph": 71,
  "headingDeg": 287,
  "ignitionOn": true,
  "fuelPct": 67,
  "address": "ถนนพหลโยธิน แขวงจอมพล เขตจตุจักร กรุงเทพมหานคร"
}
```

`recordedAt` is when the device measured the point, not when the server received it. `fuelPct` and `address` are optional.

## Postman

Import `postman/map-maker-api.postman_collection.json`. `baseUrl` defaults to `http://localhost:8080`.
Every request has tests for the checklist in `API-REQUIREMENTS.md` §11 — run the whole collection with the Collection Runner. Folder 4 (Ingest) needs a database and returns `503` in demo mode.

## Environment

| Variable | Default | |
|---|---|---|
| `PORT` | `8080` | |
| `DATABASE_URL` | — | Empty → demo mode |
| `DB_SCHEMA` | `map-maker-db-new` | Used as `search_path` |
| `FRONTEND_ORIGINS` | localhost:3000, the Vercel domain, and `https://map-maker-group-*.vercel.app` | Comma-separated; `*` matches one Vercel preview label |
| `DEMO_MODE` | `false` | Force demo mode even with `DATABASE_URL` set |
| `DEMO_FLEET_SIZE` | `107` | |
| `SIMULATE` | `false` | Postgres mode only: move online vehicles through the real write path |

In postgres mode a background job calls `mark_stale_devices_offline()` every minute and `prune_position_events()` daily.

## Deployment

Render builds `./cmd/api` (`render.yaml`) and keeps `DEMO_MODE=true` until the database exists. When it does, remove `DEMO_MODE`, set the secret `DATABASE_URL`, and set `DB_SCHEMA`.
