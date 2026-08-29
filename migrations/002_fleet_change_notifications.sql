-- Publish committed database edits so dashboard clients receive an SSE update.
-- The listener resolves the vehicle again from the read model after commit;
-- no mutable vehicle data is placed in a PostgreSQL NOTIFY payload.

BEGIN;

CREATE OR REPLACE FUNCTION fleet_notify_vehicle_change()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
  changed_vehicle_id UUID;
BEGIN
  changed_vehicle_id := CASE WHEN TG_OP = 'DELETE' THEN OLD.id ELSE NEW.id END;
  PERFORM pg_notify('fleet_vehicle_changed', changed_vehicle_id::text);
  RETURN COALESCE(NEW, OLD);
END;
$$;

CREATE OR REPLACE FUNCTION fleet_notify_live_state_change()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
  changed_vehicle_id UUID;
BEGIN
  changed_vehicle_id := CASE WHEN TG_OP = 'DELETE' THEN OLD.vehicle_id ELSE NEW.vehicle_id END;
  PERFORM pg_notify('fleet_vehicle_changed', changed_vehicle_id::text);
  RETURN COALESCE(NEW, OLD);
END;
$$;

CREATE OR REPLACE FUNCTION fleet_notify_trip_change()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
  changed_vehicle_id UUID;
BEGIN
  changed_vehicle_id := CASE WHEN TG_OP = 'DELETE' THEN OLD.vehicle_id ELSE NEW.vehicle_id END;
  PERFORM pg_notify('fleet_vehicle_changed', changed_vehicle_id::text);
  RETURN COALESCE(NEW, OLD);
END;
$$;

CREATE OR REPLACE FUNCTION fleet_notify_driver_change()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  -- A driver may be assigned to multiple vehicles, so request one full refresh.
  PERFORM pg_notify('fleet_vehicle_changed', 'refresh');
  RETURN COALESCE(NEW, OLD);
END;
$$;

-- A GPS event is the write contract for a new current position. This makes
-- manual inserts and future device-ingestion writes update the map read model.
CREATE OR REPLACE FUNCTION fleet_apply_position_event()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  INSERT INTO vehicle_live_states (
    vehicle_id, tenant_id, device_id, latest_event_id, device_time, received_at,
    last_seen_at, valid, latitude, longitude, speed_kph, course_deg, ignition,
    motion, connection_status, operational_status
  ) VALUES (
    NEW.vehicle_id, NEW.tenant_id, NEW.device_id, NEW.event_id, NEW.device_time,
    NEW.received_at, NEW.received_at, NEW.valid, NEW.latitude, NEW.longitude,
    NEW.speed_kph, NEW.course_deg, NEW.ignition, NEW.motion, 'online'::connection_status,
    (CASE WHEN COALESCE(NEW.motion, false) OR NEW.speed_kph > 0 THEN 'moving' ELSE 'idle' END)::operational_status
  ) ON CONFLICT (vehicle_id) DO UPDATE SET
    device_id = EXCLUDED.device_id,
    latest_event_id = EXCLUDED.latest_event_id,
    device_time = EXCLUDED.device_time,
    received_at = EXCLUDED.received_at,
    last_seen_at = EXCLUDED.last_seen_at,
    valid = EXCLUDED.valid,
    latitude = EXCLUDED.latitude,
    longitude = EXCLUDED.longitude,
    speed_kph = EXCLUDED.speed_kph,
    course_deg = EXCLUDED.course_deg,
    ignition = EXCLUDED.ignition,
    motion = EXCLUDED.motion,
    connection_status = EXCLUDED.connection_status,
    operational_status = EXCLUDED.operational_status,
    version = vehicle_live_states.version + 1,
    updated_at = now()
  WHERE EXCLUDED.device_time >= vehicle_live_states.device_time;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS fleet_vehicles_changed ON vehicles;
CREATE TRIGGER fleet_vehicles_changed
AFTER INSERT OR UPDATE OR DELETE ON vehicles
FOR EACH ROW EXECUTE FUNCTION fleet_notify_vehicle_change();

DROP TRIGGER IF EXISTS fleet_vehicle_live_states_changed ON vehicle_live_states;
CREATE TRIGGER fleet_vehicle_live_states_changed
AFTER INSERT OR UPDATE OR DELETE ON vehicle_live_states
FOR EACH ROW EXECUTE FUNCTION fleet_notify_live_state_change();

DROP TRIGGER IF EXISTS fleet_trips_changed ON trips;
CREATE TRIGGER fleet_trips_changed
AFTER INSERT OR UPDATE OR DELETE ON trips
FOR EACH ROW EXECUTE FUNCTION fleet_notify_trip_change();

DROP TRIGGER IF EXISTS fleet_drivers_changed ON drivers;
CREATE TRIGGER fleet_drivers_changed
AFTER INSERT OR UPDATE OR DELETE ON drivers
FOR EACH ROW EXECUTE FUNCTION fleet_notify_driver_change();

DROP TRIGGER IF EXISTS fleet_position_event_inserted ON position_events;
CREATE TRIGGER fleet_position_event_inserted
AFTER INSERT ON position_events
FOR EACH ROW EXECUTE FUNCTION fleet_apply_position_event();

COMMIT;
