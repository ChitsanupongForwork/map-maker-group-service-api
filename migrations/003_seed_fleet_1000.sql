-- Map Maker — สร้างกองรถจำลอง 1,000 คันให้ใกล้เคียงข้อมูลจริง
--
-- รันใน Navicat: คลิกขวาที่ database → Execute SQL File… → Encoding UTF-8 (65001)
-- หรือ PowerShell: psql $env:DATABASE_URL -f migrations/003_seed_fleet_1000.sql
-- ต้องรัน 001 มาก่อน (002 จะรันหรือไม่ก็ได้ ไฟล์นี้ไม่ชนกับข้อมูลของ 002)
--
-- ได้อะไรบ้าง
--   รถ v-0005 ถึง v-1004 (1,000 คัน) ต่อจากรถตัวอย่าง 4 คันของ 002
--   คนขับ d-0005 ถึง d-1004 คันละหนึ่งคน
--   สถานะล่าสุดของทุกคัน → ขึ้นบนหน้า /map ได้ทันที
--
-- รันซ้ำได้: id เดิมจะถูกเขียนทับ ทะเบียนคงเดิม แต่ตำแหน่ง/สถานะ/เวลาจะสุ่มใหม่
-- และเวลาทุกคันอิงจาก now() — ทิ้งไว้นานแล้วรถจะค่อย ๆ กลายเป็น "ไม่อัพเดต"
-- ถ้าอยากให้กลับมาสด แค่รันไฟล์นี้ซ้ำ
--
-- เทคนิคที่ใช้: สุ่มค่าทุกอย่างเก็บลงตารางชั่วคราว fleet_plan ครั้งเดียว
-- แล้วค่อย INSERT ลงตารางจริงจากตารางนั้น — ถ้าสุ่มแยกในแต่ละ INSERT
-- คนขับ รถ และสถานะของ id เดียวกันจะสุ่มได้คนละค่า ข้อมูลจะไม่สอดคล้องกัน

BEGIN;

SET search_path TO "map-maker-db-new", public;


-- ─────────────────────────────────────────────────────────────
-- 1) กลุ่มรถและพื้นที่เพิ่มเติม (ของเดิมจาก 002 ยังอยู่ แค่อัปเดตชื่อ)
-- ─────────────────────────────────────────────────────────────

INSERT INTO vehicle_groups (id, name) VALUES
  ('g-central', 'ขนส่งภาคกลาง'),
  ('g-north',   'ขนส่งภาคเหนือ'),
  ('g-east',    'ขนส่งภาคตะวันออก'),
  ('g-south',   'ขนส่งภาคใต้'),
  ('g-service', 'รถบริการ'),
  ('g-cold',    'รถห้องเย็น'),
  ('g-exec',    'รถผู้บริหาร')
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;

INSERT INTO areas (id, name) VALUES
  ('a-inner',       'กรุงเทพฯ ชั้นใน'),
  ('a-outer',       'กรุงเทพฯ รอบนอก'),
  ('a-bangna',      'บางนา-ลาดกระบัง'),
  ('a-nonthaburi',  'นนทบุรี'),
  ('a-pathumthani', 'ปทุมธานี'),
  ('a-samutprakan', 'สมุทรปราการ'),
  ('a-chonburi',    'ชลบุรี')
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;


-- ─────────────────────────────────────────────────────────────
-- 2) แผนการสร้าง — สุ่มครั้งเดียว เก็บไว้ในตารางชั่วคราว
--    ON COMMIT DROP = ตารางนี้หายไปเองตอน COMMIT ไม่ทิ้งขยะไว้ใน database
-- ─────────────────────────────────────────────────────────────

CREATE TEMP TABLE fleet_plan ON COMMIT DROP AS
WITH
-- ย่านที่รถกระจุกตัว: ยิ่งน้ำหนักมาก ยิ่งมีรถไปอยู่แถวนั้นเยอะ
hubs (name, lat, lng, area_id, address, weight) AS (
  VALUES
    ('จตุจักร',     13.8010, 100.5540, 'a-inner',       'ถนนพหลโยธิน แขวงจอมพล เขตจตุจักร กรุงเทพมหานคร',                 9),
    ('ดินแดง',      13.7700, 100.5540, 'a-inner',       'ถนนรัชดาภิเษก แขวงดินแดง เขตดินแดง กรุงเทพมหานคร',              8),
    ('ห้วยขวาง',    13.7690, 100.5740, 'a-inner',       'ถนนประชาราษฎร์บำเพ็ญ แขวงห้วยขวาง เขตห้วยขวาง กรุงเทพมหานคร',  7),
    ('ปทุมวัน',     13.7460, 100.5340, 'a-inner',       'ถนนพระรามที่ 1 แขวงปทุมวัน เขตปทุมวัน กรุงเทพมหานคร',             6),
    ('คลองเตย',     13.7160, 100.5620, 'a-inner',       'ถนนสุขุมวิท แขวงคลองเตย เขตคลองเตย กรุงเทพมหานคร',              7),
    ('บางกะปิ',     13.7650, 100.6440, 'a-outer',       'ถนนลาดพร้าว แขวงคลองจั่น เขตบางกะปิ กรุงเทพมหานคร',             6),
    ('ลาดพร้าว',    13.8060, 100.6060, 'a-outer',       'ถนนประดิษฐ์มนูธรรม แขวงลาดพร้าว เขตลาดพร้าว กรุงเทพมหานคร',     5),
    ('ธนบุรี',      13.7230, 100.4880, 'a-outer',       'ถนนอิสรภาพ แขวงวัดกัลยาณ์ เขตธนบุรี กรุงเทพมหานคร',              5),
    ('บางแค',       13.7130, 100.3990, 'a-outer',       'ถนนเพชรเกษม แขวงบางแค เขตบางแค กรุงเทพมหานคร',                  3),
    ('มีนบุรี',     13.8130, 100.7370, 'a-outer',       'ถนนรามคำแหง แขวงมีนบุรี เขตมีนบุรี กรุงเทพมหานคร',              3),
    ('พระราม 2',    13.6560, 100.4340, 'a-outer',       'ถนนพระรามที่ 2 แขวงแสมดำ เขตบางขุนเทียน กรุงเทพมหานคร',          3),
    ('บางนา',       13.6680, 100.6040, 'a-bangna',      'ถนนบางนา-ตราด แขวงบางนา เขตบางนา กรุงเทพมหานคร',                5),
    ('ลาดกระบัง',   13.7270, 100.7480, 'a-bangna',      'ถนนฉลองกรุง แขวงลำปลาทิว เขตลาดกระบัง กรุงเทพมหานคร',           4),
    ('นนทบุรี',     13.8590, 100.5140, 'a-nonthaburi',  'ถนนงามวงศ์วาน ตำบลบางกระสอ อำเภอเมืองนนทบุรี นนทบุรี',          4),
    ('ปากเกร็ด',    13.9130, 100.4980, 'a-nonthaburi',  'ถนนแจ้งวัฒนะ ตำบลปากเกร็ด อำเภอปากเกร็ด นนทบุรี',              3),
    ('รังสิต',      13.9850, 100.6170, 'a-pathumthani', 'ถนนพหลโยธิน ตำบลประชาธิปัตย์ อำเภอธัญบุรี ปทุมธานี',            3),
    ('สมุทรปราการ', 13.5990, 100.5990, 'a-samutprakan', 'ถนนสุขุมวิท ตำบลปากน้ำ อำเภอเมืองสมุทรปราการ สมุทรปราการ',     4),
    ('บางพลี',      13.6100, 100.7070, 'a-samutprakan', 'ถนนเทพารักษ์ ตำบลบางพลีใหญ่ อำเภอบางพลี สมุทรปราการ',          3),
    ('แหลมฉบัง',    13.0850, 100.8830, 'a-chonburi',    'ถนนสุขุมวิท ตำบลทุ่งสุขลา อำเภอศรีราชา ชลบุรี',                   3),
    ('ศรีราชา',     13.1740, 100.9290, 'a-chonburi',    'ถนนสุขุมวิท ตำบลศรีราชา อำเภอศรีราชา ชลบุรี',                     2)
),
-- แปลงน้ำหนักเป็นช่วง [lo, hi) บนเส้น 0..total เพื่อเลือก hub ด้วยเลขสุ่มตัวเดียว
weighted AS (
  SELECT h.*,
         sum(weight) OVER (ORDER BY name) - weight AS lo,
         sum(weight) OVER (ORDER BY name)          AS hi,
         sum(weight) OVER ()                       AS total
  FROM hubs h
),
lists AS (
  SELECT
    ARRAY['สมชาย','ประเสริฐ','วิชัย','อนุชา','ธนากร','ศักดิ์ชัย','พงศ์พันธุ์','ณัฐพล','กิตติศักดิ์','สุรชัย',
          'มานพ','ชัยวัฒน์','ธีรพงษ์','ปรีชา','อรรถพล','วีระ','สมศักดิ์','จักรพงษ์','ภาณุพงศ์','ทวีศักดิ์',
          'บุญเลิศ','สุเทพ','ไพโรจน์','ประยุทธ','เอกชัย','วรวุฒิ','ชาญวิทย์','ธวัชชัย','นิพนธ์','สมพงษ์',
          'อำนาจ','ยุทธนา','เกรียงไกร','ศุภชัย','พิชิต','รัฐพล','สุวิทย์','บรรจง','ชูชาติ','วิโรจน์'] AS first_names,
    ARRAY['ใจดี','ศรีสุข','รุ่งเรือง','ทองคำ','พรหมมา','แสงทอง','บุญมี','วงศ์ไทย','สุขสวัสดิ์','เจริญพร',
          'ชัยมงคล','อินทร์ทอง','พูลทรัพย์','แก้วมณี','สายบุญ','ศรีวงศ์','มั่นคง','ทองดี','ปัญญาดี','รักษาศรี',
          'บุญประเสริฐ','จันทร์เพ็ญ','สมบูรณ์ศักดิ์','เพชรรัตน์','นาคสวัสดิ์','กาญจนวงศ์','ศรีประเสริฐ','ธนสาร','วัฒนกุล','เรืองศรี'] AS last_names,
    -- อักษรคู่หน้าทะเบียน — ตัดคู่ที่รถตัวอย่างของ 002 ใช้อยู่ (จธ ขบ บล กท) ออก
    -- ทะเบียนจะได้ไม่มีทางซ้ำกับของเดิม
    ARRAY['กข','กค','กง','กจ','กฉ','กช','กณ','กด','กต','กถ',
          'ขก','ขค','ขง','ขจ','ขฉ','คก','คข','คง','คจ','งก',
          'งข','จก','จข','จค','ฉก','ฉข','ชก','ชข','ชค','ญก',
          'ฐก','ฒก','ณก','ดก','ตก','ถก','ทข','ธก','นก','บก',
          'ปก','ผก','พก','ฟก','ภก','มก','ยก','รก','ลก','วก',
          'ศก','ษก','สก','หก','อก','ฮก','นข','พข','มข','สข'] AS plate_pairs,
    ARRAY['Isuzu|D-Max','Toyota|Hilux Revo','Ford|Ranger','Nissan|Navara','Mitsubishi|Triton',
          'Isuzu|Elf NLR','Hino|300 Series','Mitsubishi|Fuso Canter','Hino|500 Series','Isuzu|FRR'] AS cargo_models,
    ARRAY['Isuzu|FRR','Hino|500 Series','Isuzu|Elf NMR','Mitsubishi|Fuso Canter'] AS cold_models,
    ARRAY['Toyota|Commuter','Suzuki|Carry','Toyota|Hiace','Nissan|Urvan'] AS service_models,
    ARRAY['Toyota|Fortuner','Toyota|Camry','Honda|Accord','Mitsubishi|Pajero Sport'] AS exec_models
),
-- เลขสุ่มทุกตัวที่ต้องใช้ ต่อรถหนึ่งคัน
-- (CTE ที่เรียก random() จะถูกคำนวณครั้งเดียวแล้วเก็บไว้ ค่าเลยไม่เปลี่ยนระหว่างทาง)
rolls AS (
  SELECT i,
         random() AS r_hub,     random() AS r_state,   random() AS r_age,
         random() AS r_lat1,    random() AS r_lat2,    random() AS r_lng1,
         random() AS r_lng2,    random() AS r_speed,   random() AS r_heading,
         random() AS r_fuel,    random() AS r_odo,     random() AS r_today,
         random() AS r_group,   random() AS r_model
  FROM generate_series(1, 1000) AS i
),
planned AS (
  SELECT r.*, w.lat AS hub_lat, w.lng AS hub_lng, w.area_id, w.address,
         -- สัดส่วนสถานะใกล้เคียงกองรถจริงช่วงกลางวัน
         CASE
           WHEN r.r_state < 0.11 THEN 'offline'
           WHEN r.r_state < 0.41 THEN 'parking'
           WHEN r.r_state < 0.55 THEN 'engine-on'
           ELSE                       'running'
         END AS status,
         CASE
           WHEN r.r_group < 0.30 THEN 'g-central'
           WHEN r.r_group < 0.42 THEN 'g-north'
           WHEN r.r_group < 0.56 THEN 'g-east'
           WHEN r.r_group < 0.66 THEN 'g-south'
           WHEN r.r_group < 0.82 THEN 'g-service'
           WHEN r.r_group < 0.94 THEN 'g-cold'
           ELSE                       'g-exec'
         END AS group_id
  FROM rolls r
  JOIN weighted w
    ON r.r_hub * w.total >= w.lo
   AND r.r_hub * w.total <  w.hi
)
SELECT
  'v-' || lpad((p.i + 4)::text, 4, '0') AS id,
  'd-' || lpad((p.i + 4)::text, 4, '0') AS driver_id,

  -- ทะเบียน: เลข 4 หลักคำนวณจาก i ด้วย (i × 7919) mod 9000
  -- 7919 เป็นจำนวนเฉพาะที่ไม่หาร 9000 ลงตัว เลขที่ได้จึงไม่ซ้ำกันเลยใน 1,000 คัน
  -- ต่อให้อักษรคู่หน้าจะซ้ำกัน ทะเบียนทั้งแผ่นก็ยังไม่ซ้ำ
  CASE WHEN p.i % 20 < 7 THEN (1 + p.i % 9)::text ELSE '' END
    || l.plate_pairs[1 + (p.i * 13) % array_length(l.plate_pairs, 1)]
    || ' ' || (1000 + (p.i * 7919) % 9000)::text                              AS plate,

  -- ชื่อคนขับ: หมุนชื่อกับนามสกุลคนละจังหวะ ได้ชื่อต่างกันสูงสุด 40 × 30 = 1,200 แบบ
  l.first_names[1 + p.i % array_length(l.first_names, 1)] || ' ' ||
  l.last_names[1 + (p.i / array_length(l.first_names, 1) + p.i * 7) % array_length(l.last_names, 1)] AS driver_name,

  '08' || (p.i % 10)::text || '-' ||
  lpad(((p.i * 104729) % 1000)::text, 3, '0') || '-' ||
  lpad(((p.i * 7919) % 10000)::text, 4, '0')                                  AS phone,

  -- รุ่นรถตามประเภทงาน
  split_part(
    CASE p.group_id
      WHEN 'g-cold'    THEN l.cold_models   [1 + floor(p.r_model * array_length(l.cold_models, 1))::int]
      WHEN 'g-service' THEN l.service_models[1 + floor(p.r_model * array_length(l.service_models, 1))::int]
      WHEN 'g-exec'    THEN l.exec_models   [1 + floor(p.r_model * array_length(l.exec_models, 1))::int]
      ELSE                  l.cargo_models  [1 + floor(p.r_model * array_length(l.cargo_models, 1))::int]
    END, '|', 1)                                                               AS make,
  split_part(
    CASE p.group_id
      WHEN 'g-cold'    THEN l.cold_models   [1 + floor(p.r_model * array_length(l.cold_models, 1))::int]
      WHEN 'g-service' THEN l.service_models[1 + floor(p.r_model * array_length(l.service_models, 1))::int]
      WHEN 'g-exec'    THEN l.exec_models   [1 + floor(p.r_model * array_length(l.exec_models, 1))::int]
      ELSE                  l.cargo_models  [1 + floor(p.r_model * array_length(l.cargo_models, 1))::int]
    END, '|', 2)                                                               AS model,

  p.group_id,
  p.area_id,
  p.address,
  p.status,

  -- ตำแหน่ง: บวกเลขสุ่มสองตัวเข้าด้วยกันได้การกระจายรูปสามเหลี่ยม
  -- รถจึงหนาแน่นใกล้กลาง hub แล้วบางลงเมื่อห่างออกไป (±2–3 กม.) ดูเป็นธรรมชาติกว่าสุ่มแบน
  p.hub_lat + (p.r_lat1 + p.r_lat2 - 1.0) * 0.022                             AS lat,
  p.hub_lng + (p.r_lng1 + p.r_lng2 - 1.0) * 0.026                             AS lng,

  CASE WHEN p.status = 'running' THEN 15 + floor(p.r_speed * 95)::int ELSE 0 END AS speed_kph,
  floor(p.r_heading * 360)::int                                               AS heading_deg,
  p.status IN ('running', 'engine-on')                                        AS ignition_on,
  p.status <> 'offline'                                                       AS device_online,
  8 + floor(p.r_fuel * 92)::int                                               AS fuel_pct,

  -- เลขไมล์เอียงไปทางรถใหม่ (ยกกำลัง 1.3) ชั่วโมงเครื่องยนต์ = ระยะทาง ÷ ความเร็วเฉลี่ย 38–52 กม./ชม.
  5000 + floor(power(p.r_odo, 1.3) * 440000)::int                             AS odometer_km,
  floor((5000 + power(p.r_odo, 1.3) * 440000) / (38 + p.r_speed * 14))::int   AS engine_hours,

  round((CASE p.status
           WHEN 'running' THEN 20 + p.r_today * 360
           WHEN 'offline' THEN p.r_today * 40
           ELSE                p.r_today * 180
         END)::numeric, 2)                                                    AS today_distance_km,

  -- อายุของข้อมูล (วินาที) — ตั้งให้สอดคล้องกับเกณฑ์ของหน้าเว็บและงานกวาดออฟไลน์
  --   ออฟไลน์            : 35 นาที – 8 ชม.  (เกินเกณฑ์ 30 นาทีของ mark_stale_devices_offline)
  --   ที่เหลือ 80%       : 3–110 วินาที    → เรียลไทม์   (≤ 120 วินาที)
  --   ที่เหลือ 13%       : 150–870 วินาที  → ไม่เรียลไทม์ (≤ 900 วินาที)
  --   ที่เหลือ 7%        : 920–1,750 วินาที → ไม่อัพเดต  (แต่ยังไม่ถึง 30 นาที อุปกรณ์เลยยังนับว่าออนไลน์)
  CASE
    WHEN p.status = 'offline' THEN 2100 + p.r_age * 26700
    WHEN p.r_age < 0.80       THEN 3    + (p.r_age / 0.80) * 107
    WHEN p.r_age < 0.93       THEN 150  + ((p.r_age - 0.80) / 0.13) * 720
    ELSE                           920  + ((p.r_age - 0.93) / 0.07) * 830
  END                                                                         AS age_sec
FROM planned p
CROSS JOIN lists l;


-- ─────────────────────────────────────────────────────────────
-- 3) เขียนลงตารางจริงจากแผนเดียวกัน — ลำดับสำคัญเพราะมี foreign key
--    คนขับ → รถ (อ้างคนขับ) → สถานะล่าสุด (อ้างรถ)
-- ─────────────────────────────────────────────────────────────

INSERT INTO drivers (id, name, phone)
SELECT driver_id, driver_name, phone
FROM fleet_plan
ON CONFLICT (id) DO UPDATE SET
  name  = EXCLUDED.name,
  phone = EXCLUDED.phone;

INSERT INTO vehicles (id, plate, make, model, group_id, area_id, driver_id, odometer_km, engine_hours)
SELECT id, plate, make, model, group_id, area_id, driver_id, odometer_km, engine_hours
FROM fleet_plan
ON CONFLICT (id) DO UPDATE SET
  plate        = EXCLUDED.plate,
  make         = EXCLUDED.make,
  model        = EXCLUDED.model,
  group_id     = EXCLUDED.group_id,
  area_id      = EXCLUDED.area_id,
  driver_id    = EXCLUDED.driver_id,
  odometer_km  = EXCLUDED.odometer_km,
  engine_hours = EXCLUDED.engine_hours;

-- ตอน seed ตั้งใจเขียนทับทุกครั้ง จึงไม่ใส่เงื่อนไข WHERE recorded_at > เดิม
-- (ต่างจากตอนรับข้อมูลจริงจากอุปกรณ์ ที่ต้องกันข้อมูลเก่ามาทับข้อมูลใหม่)
INSERT INTO vehicle_live_state
  (vehicle_id, recorded_at, lat, lng, speed_kph, heading_deg,
   ignition_on, device_online, fuel_pct, address, today_distance_km)
SELECT id,
       now() - make_interval(secs => age_sec),
       lat, lng, speed_kph, heading_deg,
       ignition_on, device_online, fuel_pct, address, today_distance_km
FROM fleet_plan
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

COMMIT;


-- ─────────────────────────────────────────────────────────────
-- ตรวจผล — Navicat จะแสดงผลแยกเป็นแท็บ
-- ─────────────────────────────────────────────────────────────

-- ควรได้ vehicles = unique_plates (ทะเบียนไม่ซ้ำ) และมี 1,000 คันขึ้นไป (รวมรถตัวอย่างของ 002)
SELECT count(*)              AS vehicles,
       count(DISTINCT plate) AS unique_plates
FROM "map-maker-db-new".vehicles;

-- สัดส่วนสถานะรถ — ประมาณ running 45% / parking 30% / engine-on 14% / offline 11%
SELECT status, count(*) AS vehicles
FROM "map-maker-db-new".fleet_snapshot
GROUP BY status
ORDER BY vehicles DESC;

-- สัดส่วนสถานะข้อมูลแบบเดียวกับที่หน้าเว็บคำนวณ (ค่านี้เปลี่ยนตามเวลาที่ผ่านไปหลังรัน)
SELECT CASE
         WHEN now() - recorded_at <= interval '120 seconds' THEN 'realtime'
         WHEN now() - recorded_at <= interval '900 seconds' THEN 'delayed'
         ELSE                                                    'stale'
       END      AS data_status,
       count(*) AS vehicles
FROM "map-maker-db-new".vehicle_live_state
GROUP BY 1
ORDER BY vehicles DESC;
