-- Map Maker — โครงฐานข้อมูลสำหรับหน้า /map และ /history
--
-- รันด้วย: psql $env:DATABASE_URL -f migrations/001_map_history_core.sql
-- ต้องการ PostgreSQL 14 ขึ้นไป และรันซ้ำได้โดยไม่พัง (idempotent)
--
-- แนวคิดหลัก: แยก "ล่าสุด" ออกจาก "ประวัติ" เป็นสองตาราง
--   1) vehicle_live_state  → แถวเดียวต่อรถ สำหรับหน้า /map ที่อ่านบ่อยมาก
--   2) position_events     → เขียนอย่างเดียวไม่แก้ สำหรับหน้า /history
--
-- ถ้าเก็บแต่ตารางประวัติแล้วให้หน้า map ไปหา "แถวล่าสุดของแต่ละคัน" เอง
-- query จะช้าลงเรื่อย ๆ ตามจำนวนแถวที่สะสม จนใช้ไม่ได้ในที่สุด
-- ส่วน vehicle_live_state มี 1,000 แถวตลอดกาลไม่ว่าจะเก็บประวัติมานานแค่ไหน

BEGIN;

CREATE SCHEMA IF NOT EXISTS "map-maker-db-new";
SET search_path TO "map-maker-db-new", public;


-- ─────────────────────────────────────────────────────────────
-- ตารางอ้างอิง — ป้อนตัวเลือกให้ dropdown ตัวกรองบนหน้าเว็บ
--
-- id เป็น text ไม่ใช่ uuid เพราะค่านี้ไปโผล่ใน JSON ที่ front-end
-- ใช้เป็นคีย์ตรง ๆ อ่านออกเวลา debug และ URL หน้า history สั้นกว่า
-- ─────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS vehicle_groups (
  id   text PRIMARY KEY,
  name text NOT NULL
);

CREATE TABLE IF NOT EXISTS areas (
  id   text PRIMARY KEY,
  name text NOT NULL
);

CREATE TABLE IF NOT EXISTS drivers (
  id    text PRIMARY KEY,
  name  text NOT NULL,
  phone text NOT NULL DEFAULT ''
);


-- ─────────────────────────────────────────────────────────────
-- ตารางรถ — ข้อมูลที่ไม่ค่อยเปลี่ยน
-- ─────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS vehicles (
  id           text PRIMARY KEY,
  plate        text NOT NULL UNIQUE,
  make         text NOT NULL DEFAULT '',
  model        text NOT NULL DEFAULT '',
  group_id     text REFERENCES vehicle_groups(id) ON DELETE SET NULL,
  area_id      text REFERENCES areas(id)          ON DELETE SET NULL,
  driver_id    text REFERENCES drivers(id)        ON DELETE SET NULL,
  odometer_km  integer NOT NULL DEFAULT 0 CHECK (odometer_km >= 0),
  engine_hours integer NOT NULL DEFAULT 0 CHECK (engine_hours >= 0),
  created_at   timestamptz NOT NULL DEFAULT now()
);


-- ─────────────────────────────────────────────────────────────
-- ประวัติตำแหน่ง — หัวใจของหน้า /history
--
-- PRIMARY KEY (vehicle_id, recorded_at) ทำงานสองอย่างพร้อมกัน:
--   1) เป็น index ที่ตรงกับ query ของหน้า history พอดี
--      WHERE vehicle_id = ? AND recorded_at BETWEEN ? AND ?
--      จึงไม่ต้องสร้าง index เพิ่มอีกตัว
--   2) กันข้อมูลซ้ำ เวลาอุปกรณ์ส่งจุดเดิมมาสองครั้ง (เกิดบ่อยเวลาสัญญาณกระตุก)
--      ใช้คู่กับ INSERT ... ON CONFLICT DO NOTHING
--
-- recorded_at คือเวลาที่ "อุปกรณ์วัดได้" ไม่ใช่เวลาที่เซิร์ฟเวอร์รับ
-- รถเข้าอุโมงค์ 10 นาทีแล้วโผล่ออกมา อุปกรณ์จะส่งจุดที่ค้างไว้พรวดเดียว
-- ถ้าใช้เวลาที่รับ เส้นทางบนแผนที่จะผิดหมด และ lastUpdate จะบอกว่าสด
-- ทั้งที่ความจริงข้อมูลเก่าไปแล้ว 10 นาที
-- ─────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS position_events (
  vehicle_id  text        NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
  recorded_at timestamptz NOT NULL,
  lat         double precision NOT NULL CHECK (lat BETWEEN -90 AND 90),
  lng         double precision NOT NULL CHECK (lng BETWEEN -180 AND 180),
  speed_kph   smallint NOT NULL DEFAULT 0 CHECK (speed_kph BETWEEN 0 AND 400),
  heading_deg smallint NOT NULL DEFAULT 0 CHECK (heading_deg BETWEEN 0 AND 359),
  ignition_on boolean  NOT NULL DEFAULT false,
  fuel_pct    smallint CHECK (fuel_pct IS NULL OR fuel_pct BETWEEN 0 AND 100),
  address     text,
  PRIMARY KEY (vehicle_id, recorded_at)
);

-- ยังไม่ได้แบ่ง partition โดยตั้งใจ — เริ่มแบบง่ายก่อน
-- ถ้าวันหนึ่งช้าจนต้องแบ่งจริง วิธีคือสร้างตารางใหม่แบบ PARTITION BY RANGE (recorded_at)
-- แล้วคัดลอกข้อมูลข้ามมา (ALTER ตารางเดิมให้กลายเป็น partitioned ไม่ได้)
-- ตัวอย่างที่ทำไว้แล้ว: git show main:migrations/001_fleet_realtime_postgres.sql


-- ─────────────────────────────────────────────────────────────
-- สถานะล่าสุด — หัวใจของหน้า /map
--
-- สังเกตว่าไม่มีคอลัมน์ status และไม่มีคอลัมน์สถานะข้อมูล
-- ทั้งสองอย่างคำนวณได้จากค่าที่เก็บอยู่แล้ว การเก็บค่าที่คำนวณได้
-- ลงฐานข้อมูลคือการสร้างโอกาสให้มันไม่ตรงกับความจริง
--   status      → คำนวณใน view fleet_snapshot ข้างล่าง
--   สถานะข้อมูล → front-end คำนวณจาก recorded_at เทียบกับเวลาปัจจุบัน
-- ─────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS vehicle_live_state (
  vehicle_id        text PRIMARY KEY REFERENCES vehicles(id) ON DELETE CASCADE,
  recorded_at       timestamptz NOT NULL,
  lat               double precision NOT NULL CHECK (lat BETWEEN -90 AND 90),
  lng               double precision NOT NULL CHECK (lng BETWEEN -180 AND 180),
  speed_kph         smallint NOT NULL DEFAULT 0 CHECK (speed_kph BETWEEN 0 AND 400),
  heading_deg       smallint NOT NULL DEFAULT 0 CHECK (heading_deg BETWEEN 0 AND 359),
  ignition_on       boolean  NOT NULL DEFAULT false,
  device_online     boolean  NOT NULL DEFAULT true,
  fuel_pct          smallint CHECK (fuel_pct IS NULL OR fuel_pct BETWEEN 0 AND 100),
  address           text,
  today_distance_km numeric(8,2) NOT NULL DEFAULT 0 CHECK (today_distance_km >= 0)
);

-- ใช้ตอนกวาดหารถที่ขาดการติดต่อ (ดู mark_stale_devices_offline ข้างล่าง)
CREATE INDEX IF NOT EXISTS vehicle_live_state_recorded_at_idx
  ON vehicle_live_state (recorded_at)
  WHERE device_online;


-- ─────────────────────────────────────────────────────────────
-- View ที่หน้า /map ใช้ — SELECT * FROM fleet_snapshot; จบในบรรทัดเดียว
--
-- ลำดับของ CASE สำคัญ: ต้องเช็ก device_online ก่อนเสมอ
-- เพราะรถที่อุปกรณ์ตายไปแล้วอาจมี ignition_on ค้างเป็น true
-- จากข้อมูลชุดสุดท้ายที่ส่งมาได้
-- ─────────────────────────────────────────────────────────────

CREATE OR REPLACE VIEW fleet_snapshot AS
SELECT
  v.id,
  v.plate,
  v.make,
  v.model,
  trim(v.make || ' ' || v.model)        AS label,
  v.group_id,
  v.area_id,
  v.driver_id,
  COALESCE(d.name, '')                  AS driver_name,
  v.odometer_km,
  v.engine_hours,
  s.recorded_at,
  s.lat,
  s.lng,
  s.speed_kph,
  s.heading_deg,
  COALESCE(s.fuel_pct, 0)               AS fuel_pct,
  COALESCE(s.address, '')               AS address,
  s.today_distance_km,
  CASE
    WHEN NOT s.device_online THEN 'offline'
    WHEN NOT s.ignition_on   THEN 'parking'
    WHEN s.speed_kph > 0     THEN 'running'
    ELSE                          'engine-on'
  END                                   AS status
FROM vehicles v
JOIN vehicle_live_state s ON s.vehicle_id = v.id
LEFT JOIN drivers d       ON d.id = v.driver_id;


-- ─────────────────────────────────────────────────────────────
-- งานเบื้องหลัง
-- ─────────────────────────────────────────────────────────────

-- ไม่มีอุปกรณ์ไหนส่งข้อความมาบอกว่า "ฉันออฟไลน์แล้ว" ต้องมีคนคอยตรวจเอง
-- เรียกจาก Go ด้วย ticker ทุก ๆ 1 นาที:  SELECT mark_stale_devices_offline();
--
-- ค่าเริ่มต้น 30 นาที ตั้งให้ยาวกว่าเกณฑ์ stale ของ front-end (15 นาที)
-- จะได้เห็นลำดับที่สมเหตุสมผลบนหน้าจอ:
--   ข้อมูลเริ่มไม่เรียลไทม์ → ไม่อัพเดต → แล้วค่อยประกาศว่าอุปกรณ์ออฟไลน์
CREATE OR REPLACE FUNCTION mark_stale_devices_offline(cutoff interval DEFAULT interval '30 minutes')
RETURNS integer
LANGUAGE sql
SET search_path = "map-maker-db-new", public
AS $$
  WITH touched AS (
    UPDATE vehicle_live_state
    SET device_online = false
    WHERE device_online
      AND recorded_at < now() - cutoff
    RETURNING vehicle_id
  )
  SELECT count(*)::integer FROM touched;
$$;

-- ลบประวัติที่เก่าเกินความจำเป็น หน้า history ดูย้อนหลังแค่ 1 เดือน
-- เรียกวันละครั้ง:  SELECT prune_position_events();
CREATE OR REPLACE FUNCTION prune_position_events(keep interval DEFAULT interval '90 days')
RETURNS integer
LANGUAGE sql
SET search_path = "map-maker-db-new", public
AS $$
  WITH removed AS (
    DELETE FROM position_events
    WHERE recorded_at < now() - keep
    RETURNING vehicle_id
  )
  SELECT count(*)::integer FROM removed;
$$;

COMMIT;


-- ยืนยันผลทันทีหลังรัน — ควรได้ 6 ตาราง + 1 view และบอกว่าต่ออยู่ database ไหน
-- ถ้าบรรทัดนี้ไม่ขึ้น แปลว่ามี error ก่อนหน้าและทั้งไฟล์ถูก rollback ไปแล้ว
SELECT current_database()                       AS database,
       current_user                             AS connected_as,
       count(*) FILTER (WHERE table_type = 'BASE TABLE') AS tables,
       count(*) FILTER (WHERE table_type = 'VIEW')       AS views
FROM information_schema.tables
WHERE table_schema = 'map-maker-db-new';


-- ─────────────────────────────────────────────────────────────
-- ภาคผนวก: query ที่ Go จะใช้ คัดลอกไปใช้ได้เลย
-- ─────────────────────────────────────────────────────────────
--
-- GET /api/fleet — รถทุกคัน
--   SELECT * FROM fleet_snapshot;
--
-- GET /api/fleet — ตัวเลือกของ dropdown
--   SELECT id AS value, name AS label FROM vehicle_groups ORDER BY name;
--   SELECT id AS value, name AS label FROM areas          ORDER BY name;
--   SELECT id AS value, name AS label FROM drivers        ORDER BY name;
--
-- GET /api/vehicles/{id}/history?from&to
--   SELECT recorded_at, lat, lng, speed_kph, heading_deg, COALESCE(address, '') AS address
--   FROM position_events
--   WHERE vehicle_id = $1 AND recorded_at BETWEEN $2 AND $3
--   ORDER BY recorded_at;
--
-- speedHistory 48 จุด (endpoint แยก เรียกตอนผู้ใช้คลิกเลือกรถเท่านั้น
-- อย่าใส่มาใน /api/fleet เพราะจะกลายเป็นการรัน aggregate 1,000 ครั้งต่อ request)
--   SELECT to_timestamp(floor(extract(epoch FROM recorded_at) / 1800) * 1800) AS bucket,
--          max(speed_kph) AS speed
--   FROM position_events
--   WHERE vehicle_id = $1 AND recorded_at > now() - interval '24 hours'
--   GROUP BY bucket
--   ORDER BY bucket;
--
-- รับข้อมูลเข้า 1 จุด — เขียนสองที่ในทรานแซกชันเดียว
--   BEGIN;
--   INSERT INTO position_events
--     (vehicle_id, recorded_at, lat, lng, speed_kph, heading_deg, ignition_on, fuel_pct)
--   VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
--   ON CONFLICT DO NOTHING;
--
--   INSERT INTO vehicle_live_state AS s
--     (vehicle_id, recorded_at, lat, lng, speed_kph, heading_deg, ignition_on, fuel_pct, device_online)
--   VALUES ($1, $2, $3, $4, $5, $6, $7, $8, true)
--   ON CONFLICT (vehicle_id) DO UPDATE SET
--     recorded_at   = EXCLUDED.recorded_at,
--     lat           = EXCLUDED.lat,
--     lng           = EXCLUDED.lng,
--     speed_kph     = EXCLUDED.speed_kph,
--     heading_deg   = EXCLUDED.heading_deg,
--     ignition_on   = EXCLUDED.ignition_on,
--     fuel_pct      = EXCLUDED.fuel_pct,
--     device_online = true
--   WHERE EXCLUDED.recorded_at > s.recorded_at;
--   COMMIT;
--
-- บรรทัดสุดท้ายคือตัวกันไม่ให้ข้อมูลเก่าที่มาถึงช้า เขียนทับข้อมูลใหม่ที่มาถึงก่อน
