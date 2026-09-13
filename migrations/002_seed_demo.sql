-- Map Maker — ข้อมูลตัวอย่างสำหรับทดสอบ
--
-- รันด้วย: psql $env:DATABASE_URL -f migrations/002_seed_demo.sql
-- รันซ้ำได้ ข้อมูลเดิมจะถูกเขียนทับด้วยเวลาปัจจุบัน
--
-- ใส่รถ 4 คันให้ครบทั้ง 4 สถานะ จะได้เห็นหมุดครบทุกแบบบนแผนที่:
--   running   → ลูกศรสีเขียว หมุนตามหัวรถ
--   engine-on → จุดสีเหลือง
--   parking   → จุดสีน้ำเงิน
--   offline   → จุดสีเทา
--
-- สำคัญ: เวลาทั้งหมดอิงจาก now() ไม่ใช่ค่าตายตัว
-- ถ้าใส่เวลาตายตัวไว้ พอเวลาผ่านไปหน้าเว็บจะขึ้นว่า "ไม่อัพเดต" ทั้งกอง
-- เพราะสถานะข้อมูลคำนวณจาก "ข้อมูลเก่าแค่ไหนเทียบกับตอนนี้"

BEGIN;

SET search_path TO "map-maker-db-new", public;

-- ตรวจก่อนว่ารัน 001 มาแล้วจริง ไม่งั้น error ที่ได้จะเป็นแค่
-- "relation vehicle_groups does not exist" ซึ่งไม่ได้บอกว่าต้องทำอะไรต่อ
DO $guard$
BEGIN
  IF to_regclass('"map-maker-db-new".vehicle_groups') IS NULL THEN
    RAISE EXCEPTION
      E'ยังไม่มีตารางในฐานข้อมูลนี้ ให้รัน 001 ก่อน:\n'
      '  psql $env:DATABASE_URL -f migrations/001_map_history_core.sql\n'
      'ถ้ารัน 001 ไปแล้วแต่ยังเจอข้อความนี้ แปลว่า 001 พังกลางทางแล้วถูก rollback ทั้งไฟล์ '
      '(มันห่อด้วย BEGIN/COMMIT) — ให้เลื่อนขึ้นไปอ่าน error ตัวแรกของ 001';
  END IF;
END
$guard$;

-- ─────────────────────────────────────────────────────────────
-- ตัวเลือกของ dropdown
-- ─────────────────────────────────────────────────────────────

INSERT INTO vehicle_groups (id, name) VALUES
  ('g-central', 'ขนส่งภาคกลาง'),
  ('g-north',   'ขนส่งภาคเหนือ'),
  ('g-service', 'รถบริการ')
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;

INSERT INTO areas (id, name) VALUES
  ('a-inner',       'กรุงเทพฯ ชั้นใน'),
  ('a-outer',       'กรุงเทพฯ รอบนอก'),
  ('a-nonthaburi',  'นนทบุรี')
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;

INSERT INTO drivers (id, name, phone) VALUES
  ('d-0001', 'สมชาย ใจดี',      '081-234-5678'),
  ('d-0002', 'ประเสริฐ ศรีสุข', '082-345-6789'),
  ('d-0003', 'วิชัย รุ่งเรือง',  '083-456-7890'),
  ('d-0004', 'อนุชา ทองคำ',     '084-567-8901')
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, phone = EXCLUDED.phone;


-- ─────────────────────────────────────────────────────────────
-- รถ 4 คัน
-- ─────────────────────────────────────────────────────────────

INSERT INTO vehicles (id, plate, make, model, group_id, area_id, driver_id, odometer_km, engine_hours) VALUES
  ('v-0001', '2จธ 4871', 'Isuzu',      'D-Max',       'g-central', 'a-inner',      'd-0001', 128450, 4210),
  ('v-0002', 'ขบ 2161',  'Toyota',     'Hilux Revo',  'g-central', 'a-inner',      'd-0002',  88120, 2980),
  ('v-0003', 'บล 5287',  'Hino',       '500 Series',  'g-north',   'a-outer',      'd-0003', 241030, 9120),
  ('v-0004', 'กท 5440',  'Mitsubishi', 'Fuso Canter', 'g-service', 'a-nonthaburi', 'd-0004',  56780, 1840)
ON CONFLICT (id) DO UPDATE SET
  plate = EXCLUDED.plate, make = EXCLUDED.make, model = EXCLUDED.model,
  group_id = EXCLUDED.group_id, area_id = EXCLUDED.area_id, driver_id = EXCLUDED.driver_id;


-- ─────────────────────────────────────────────────────────────
-- สถานะล่าสุดของแต่ละคัน — คุมให้ได้ครบทั้ง 4 สถานะ
--
-- status ถูกคำนวณจากสามคอลัมน์นี้ (ดู view fleet_snapshot):
--   device_online = false          → offline
--   ignition_on   = false          → parking
--   ignition_on = true, speed > 0  → running
--   ignition_on = true, speed = 0  → engine-on
-- ─────────────────────────────────────────────────────────────

INSERT INTO vehicle_live_state
  (vehicle_id, recorded_at, lat, lng, speed_kph, heading_deg,
   ignition_on, device_online, fuel_pct, address, today_distance_km)
VALUES
  -- กำลังวิ่ง — ข้อมูลสด 20 วินาทีที่แล้ว หัวรถชี้ไปทางตะวันตกเฉียงเหนือ
  ('v-0001', now() - interval '20 seconds', 13.80123, 100.55412, 68, 285,
   true,  true,  68, 'ถนนพหลโยธิน แขวงจอมพล เขตจตุจักร กรุงเทพมหานคร', 182.4),

  -- จอดติดเครื่อง — เครื่องติดแต่ความเร็วเป็น 0
  ('v-0002', now() - interval '45 seconds', 13.76900, 100.57400, 0, 90,
   true,  true,  41, 'ถนนรัชดาภิเษก แขวงดินแดง เขตดินแดง กรุงเทพมหานคร', 76.2),

  -- จอด — เครื่องดับ แต่อุปกรณ์ยังส่งข้อมูลอยู่ (parking + realtime เป็นเรื่องปกติ)
  ('v-0003', now() - interval '70 seconds', 13.72300, 100.48800, 0, 180,
   false, true,  93, 'ถนนบรมราชชนนี แขวงอรุณอมรินทร์ เขตบางกอกน้อย กรุงเทพมหานคร', 0),

  -- ออฟไลน์ — อุปกรณ์ขาดการติดต่อไป 3 ชั่วโมง
  ('v-0004', now() - interval '3 hours', 13.85900, 100.51400, 0, 0,
   false, false, 12, 'ถนนงามวงศ์วาน ตำบลบางเขน อำเภอเมือง นนทบุรี', 24.8)
ON CONFLICT (vehicle_id) DO UPDATE SET
  recorded_at       = EXCLUDED.recorded_at,
  lat               = EXCLUDED.lat,
  lng               = EXCLUDED.lng,
  speed_kph         = EXCLUDED.speed_kph,
  heading_deg       = EXCLUDED.heading_deg,
  ignition_on       = EXCLUDED.ignition_on,
  device_online     = EXCLUDED.device_online,
  fuel_pct          = EXCLUDED.fuel_pct,
  address           = EXCLUDED.address,
  today_distance_km = EXCLUDED.today_distance_km;


-- ─────────────────────────────────────────────────────────────
-- ประวัติเส้นทางของ v-0001 — ย้อนหลัง 1 ชั่วโมง จุดละ 30 วินาที
--
-- เส้นทางมุ่งหน้าไปทางตะวันตกเฉียงเหนือ และไป "จบตรงตำแหน่งปัจจุบัน" ของรถพอดี
-- (จุดสุดท้ายตรงกับ vehicle_live_state) พร้อมจอดสนิทช่วงกลางทาง
-- (จุดที่ 40–48) เพื่อให้หน้า history มีทั้งช่วงวิ่งและช่วงจอดให้ดู
-- ─────────────────────────────────────────────────────────────

INSERT INTO position_events
  (vehicle_id, recorded_at, lat, lng, speed_kph, heading_deg, ignition_on, fuel_pct, address)
SELECT
  'v-0001',
  now() - interval '1 hour' + (step * interval '30 seconds'),
  13.73935 + step * 0.00052,
  100.71001 - step * 0.00131,
  CASE WHEN step BETWEEN 40 AND 48 THEN 0 ELSE 38 + (step % 7) * 6 END,
  CASE WHEN step BETWEEN 40 AND 48 THEN 285 ELSE (280 + (step % 5) * 3) END,
  true,
  68,
  NULL
FROM generate_series(0, 119) AS step
ON CONFLICT DO NOTHING;

COMMIT;


-- ─────────────────────────────────────────────────────────────
-- ตรวจผล
-- ─────────────────────────────────────────────────────────────
--
-- ควรได้ 4 แถว ครบทั้ง 4 สถานะ
--   SELECT id, plate, status, speed_kph, heading_deg, recorded_at FROM fleet_snapshot ORDER BY id;
--
-- ควรได้ 120 จุด
--   SELECT count(*) FROM position_events WHERE vehicle_id = 'v-0001';
--
-- ควรได้ 3 แถว (ทุกคันยกเว้น v-0004 ที่ออฟไลน์อยู่แล้ว)
--   SELECT vehicle_id, recorded_at FROM vehicle_live_state WHERE device_online ORDER BY recorded_at;
--
-- ทดสอบว่างานกวาดอุปกรณ์ออฟไลน์ทำงานถูก — ควรคืน 0 เพราะยังไม่มีคันไหนขาดติดต่อเกิน 30 นาที
-- (v-0004 ถูกตั้ง device_online = false ไว้แล้วตั้งแต่ต้น)
--   SELECT mark_stale_devices_offline();
