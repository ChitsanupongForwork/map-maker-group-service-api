-- Fleet Monitor — PostgreSQL schema for real-time GPS tracking
--
-- Run with: psql "$DATABASE_URL" -f database/001_fleet_realtime_postgres.sql
-- PostgreSQL 14+ recommended. This migration is safe to run on a new database.
-- It creates two hot paths:
--   1) vehicle_live_states  : one current row per vehicle for the map
--   2) position_events      : append-only GPS history, partitioned by month

BEGIN;

CREATE SCHEMA IF NOT EXISTS "map-maker-db";
SET search_path TO "map-maker-db", public;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE connection_status AS ENUM ('online', 'stale', 'offline');
CREATE TYPE operational_status AS ENUM ('moving', 'stopped', 'idle', 'unknown');
CREATE TYPE trip_status AS ENUM ('planned', 'active', 'completed', 'cancelled');

CREATE TABLE tenants (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE drivers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  display_name TEXT NOT NULL,
  phone TEXT,
  employee_code TEXT,
  active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, employee_code)
);

CREATE TABLE vehicles (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  code TEXT NOT NULL,
  label TEXT NOT NULL,
  license_plate TEXT NOT NULL,
  make TEXT,
  model TEXT,
  category TEXT,
  active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, code),
  UNIQUE (tenant_id, license_plate)
);

CREATE TABLE gps_devices (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  unique_id TEXT NOT NULL, -- IMEI, serial number, or tracker-specific ID
  protocol TEXT,
  model TEXT,
  active BOOLEAN NOT NULL DEFAULT true,
  last_seen_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, unique_id)
);

-- Keeps assignment history. The partial unique indexes allow only one active
-- driver and one active GPS device for a vehicle at a time.
CREATE TABLE vehicle_assignments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  vehicle_id UUID NOT NULL REFERENCES vehicles(id),
  driver_id UUID REFERENCES drivers(id),
  device_id UUID REFERENCES gps_devices(id),
  assigned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  unassigned_at TIMESTAMPTZ,
  CHECK (driver_id IS NOT NULL OR device_id IS NOT NULL),
  CHECK (unassigned_at IS NULL OR unassigned_at >= assigned_at)
);

CREATE UNIQUE INDEX vehicle_assignments_one_active_driver
  ON vehicle_assignments(vehicle_id)
  WHERE driver_id IS NOT NULL AND unassigned_at IS NULL;

CREATE UNIQUE INDEX vehicle_assignments_one_active_device
  ON vehicle_assignments(vehicle_id)
  WHERE device_id IS NOT NULL AND unassigned_at IS NULL;

-- Immutable events: do not store driver name, phone, make, or destination here.
-- Those fields belong to their master tables/trips and would multiply storage.
CREATE TABLE position_events (
  received_at TIMESTAMPTZ NOT NULL,
  event_id UUID NOT NULL DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  vehicle_id UUID NOT NULL REFERENCES vehicles(id),
  device_id UUID NOT NULL REFERENCES gps_devices(id),
  sequence_no BIGINT,
  device_time TIMESTAMPTZ NOT NULL,
  fix_time TIMESTAMPTZ,
  valid BOOLEAN NOT NULL DEFAULT true,

  latitude DOUBLE PRECISION NOT NULL CHECK (latitude BETWEEN -90 AND 90),
  longitude DOUBLE PRECISION NOT NULL CHECK (longitude BETWEEN -180 AND 180),
  altitude_m DOUBLE PRECISION,
  accuracy_m DOUBLE PRECISION CHECK (accuracy_m IS NULL OR accuracy_m >= 0),
  speed_kph DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (speed_kph >= 0),
  course_deg SMALLINT CHECK (course_deg BETWEEN 0 AND 359),

  ignition BOOLEAN,
  motion BOOLEAN,
  odometer_m BIGINT CHECK (odometer_m IS NULL OR odometer_m >= 0),
  engine_hours_s BIGINT CHECK (engine_hours_s IS NULL OR engine_hours_s >= 0),
  fuel_level_pct NUMERIC(5,2) CHECK (fuel_level_pct IS NULL OR fuel_level_pct BETWEEN 0 AND 100),
  battery_voltage NUMERIC(7,3),
  external_power_voltage NUMERIC(7,3),
  satellites SMALLINT CHECK (satellites IS NULL OR satellites >= 0),
  hdop NUMERIC(6,2) CHECK (hdop IS NULL OR hdop >= 0),

  geofence_ids UUID[] NOT NULL DEFAULT '{}',
  attributes JSONB NOT NULL DEFAULT '{}', -- device-specific values only
  raw_payload JSONB,                      -- optional; use a retention policy
  PRIMARY KEY (received_at, event_id)
) PARTITION BY RANGE (received_at);

-- Create partitions for this month and next month. Deployment automation should
-- create the following monthly partition before each month begins.
DO $$
DECLARE
  month_start TIMESTAMPTZ := date_trunc('month', now());
  partition_start TIMESTAMPTZ;
  partition_end TIMESTAMPTZ;
  partition_name TEXT;
BEGIN
  FOR offset_month IN 0..1 LOOP
    partition_start := month_start + make_interval(months => offset_month);
    partition_end := partition_start + INTERVAL '1 month';
    partition_name := format('position_events_%s', to_char(partition_start, 'YYYY_MM'));
    EXECUTE format(
      'CREATE TABLE IF NOT EXISTS %I PARTITION OF position_events FOR VALUES FROM (%L) TO (%L)',
      partition_name, partition_start, partition_end
    );
  END LOOP;
END $$;

-- These indexes are inherited by each partition created above.
CREATE INDEX position_events_vehicle_received_at_idx
  ON position_events (vehicle_id, received_at DESC);
CREATE INDEX position_events_tenant_received_at_idx
  ON position_events (tenant_id, received_at DESC);
CREATE INDEX position_events_device_time_idx
  ON position_events (device_id, device_time DESC);

-- Map read model. The API should read this table, not scan position_events.
CREATE TABLE vehicle_live_states (
  vehicle_id UUID PRIMARY KEY REFERENCES vehicles(id),
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  device_id UUID NOT NULL REFERENCES gps_devices(id),
  driver_id UUID REFERENCES drivers(id),
  latest_event_id UUID NOT NULL,
  device_time TIMESTAMPTZ NOT NULL,
  received_at TIMESTAMPTZ NOT NULL,
  last_seen_at TIMESTAMPTZ NOT NULL,
  valid BOOLEAN NOT NULL,
  latitude DOUBLE PRECISION NOT NULL CHECK (latitude BETWEEN -90 AND 90),
  longitude DOUBLE PRECISION NOT NULL CHECK (longitude BETWEEN -180 AND 180),
  speed_kph DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (speed_kph >= 0),
  course_deg SMALLINT CHECK (course_deg BETWEEN 0 AND 359),
  ignition BOOLEAN,
  motion BOOLEAN,
  connection_status connection_status NOT NULL DEFAULT 'online',
  operational_status operational_status NOT NULL DEFAULT 'unknown',
  geofence_ids UUID[] NOT NULL DEFAULT '{}',
  version BIGINT NOT NULL DEFAULT 1,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX vehicle_live_states_tenant_status_idx
  ON vehicle_live_states (tenant_id, connection_status, operational_status);
CREATE INDEX vehicle_live_states_last_seen_idx
  ON vehicle_live_states (last_seen_at);

CREATE TABLE trips (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  vehicle_id UUID NOT NULL REFERENCES vehicles(id),
  driver_id UUID REFERENCES drivers(id),
  status trip_status NOT NULL DEFAULT 'planned',
  origin_name TEXT,
  destination_name TEXT,
  started_at TIMESTAMPTZ,
  ended_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (ended_at IS NULL OR started_at IS NULL OR ended_at >= started_at)
);

CREATE INDEX trips_active_vehicle_idx
  ON trips (vehicle_id, status)
  WHERE status IN ('planned', 'active');

-- Durable delivery log for SSE/WebSocket reconnection. Send event_id to the
-- client as SSE `id:` and resume after Last-Event-ID when the client reconnects.
CREATE TABLE fleet_realtime_outbox (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  vehicle_id UUID NOT NULL REFERENCES vehicles(id),
  event_type TEXT NOT NULL,
  event_id UUID NOT NULL DEFAULT gen_random_uuid(),
  payload JSONB NOT NULL,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (event_id)
);

CREATE INDEX fleet_realtime_outbox_tenant_id_idx
  ON fleet_realtime_outbox (tenant_id, id);

COMMIT;

-- ---------------------------------------------------------------------------
-- Realtime write transaction template (use parameterized queries from Go).
-- A delayed packet must never overwrite a newer live position.
--
-- 1. INSERT INTO position_events (... received_at, event_id, tenant_id,
--      vehicle_id, device_id, sequence_no, device_time, valid, latitude,
--      longitude, speed_kph, course_deg, ignition, motion, attributes)
--    VALUES (...);
--
-- 2. INSERT INTO vehicle_live_states (...)
--    VALUES (...)
--    ON CONFLICT (vehicle_id) DO UPDATE SET
--      latest_event_id = EXCLUDED.latest_event_id,
--      device_time = EXCLUDED.device_time,
--      received_at = EXCLUDED.received_at,
--      last_seen_at = EXCLUDED.last_seen_at,
--      valid = EXCLUDED.valid,
--      latitude = EXCLUDED.latitude,
--      longitude = EXCLUDED.longitude,
--      speed_kph = EXCLUDED.speed_kph,
--      course_deg = EXCLUDED.course_deg,
--      ignition = EXCLUDED.ignition,
--      motion = EXCLUDED.motion,
--      connection_status = EXCLUDED.connection_status,
--      operational_status = EXCLUDED.operational_status,
--      geofence_ids = EXCLUDED.geofence_ids,
--      version = vehicle_live_states.version + 1,
--      updated_at = now()
--    WHERE EXCLUDED.device_time >= vehicle_live_states.device_time;
--
-- 3. INSERT INTO fleet_realtime_outbox (tenant_id, vehicle_id, event_type, payload)
--    VALUES (..., 'position.updated', jsonb_build_object(...));
