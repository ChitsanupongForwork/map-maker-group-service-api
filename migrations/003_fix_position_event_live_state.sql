-- Migration 002 is already recorded on existing deployments. Replace only the
-- position-event projection function with the enum-safe expression.

BEGIN;

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

COMMIT;
