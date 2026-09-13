package ingest

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"fleet-monitor-server/internal/platform/geo"
)

var ErrUnknownVehicle = errors.New("unknown vehicle")

// WriteResult ตอบกลับให้เห็นว่าแต่ละจุดไปจบที่ไหน — ใช้เช็กข้อ "ยิงซ้ำ" กับ "ยิงจุดเก่า" ใน Postman ได้ตรง ๆ
type WriteResult struct {
	Received int `json:"received"`
	// Inserted = แถวใหม่ใน position_events, Duplicates = จุดเดิมซ้ำ (vehicle_id + recorded_at ชนกัน)
	Inserted   int `json:"inserted"`
	Duplicates int `json:"duplicates"`
	// LiveStateUpdated = จำนวนครั้งที่ vehicle_live_state ถูกเขียน จุดที่เก่ากว่าของเดิมจะไม่นับ
	LiveStateUpdated int `json:"liveStateUpdated"`
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

const insertEventSQL = `
INSERT INTO position_events
  (vehicle_id, recorded_at, lat, lng, speed_kph, heading_deg, ignition_on, fuel_pct, address)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT DO NOTHING`

// FOR UPDATE ล็อกแถวไว้จนจบทรานแซกชัน สองคำขอของรถคันเดียวกันจะได้ไม่บวกระยะทางทับกัน
const lastLiveSQL = `
SELECT recorded_at, lat, lng, speed_kph::int
FROM vehicle_live_state
WHERE vehicle_id = $1
FOR UPDATE`

// ต่างจากสเปกหัวข้อ 6.7 สามจุด:
//   - fuel_pct / address ที่ไม่ได้ส่งมา (NULL) ใช้ค่าเดิม ไม่ลบค่าที่เคยรู้ทิ้ง
//   - today_distance_km บวกระยะจากจุดก่อนหน้า และเริ่มนับใหม่เมื่อข้ามวัน (เวลาไทย)
//   - บรรทัด WHERE สุดท้ายเหมือนสเปกทุกตัวอักษร: จุดที่มาถึงช้าห้ามเขียนทับจุดที่ใหม่กว่า
const upsertLiveSQL = `
INSERT INTO vehicle_live_state AS s
  (vehicle_id, recorded_at, lat, lng, speed_kph, heading_deg, ignition_on, fuel_pct, address,
   today_distance_km, device_online)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, true)
ON CONFLICT (vehicle_id) DO UPDATE SET
  recorded_at   = EXCLUDED.recorded_at,
  lat           = EXCLUDED.lat,
  lng           = EXCLUDED.lng,
  speed_kph     = EXCLUDED.speed_kph,
  heading_deg   = EXCLUDED.heading_deg,
  ignition_on   = EXCLUDED.ignition_on,
  fuel_pct      = COALESCE(EXCLUDED.fuel_pct, s.fuel_pct),
  address       = COALESCE(EXCLUDED.address, s.address),
  today_distance_km = CASE
    WHEN (EXCLUDED.recorded_at AT TIME ZONE 'Asia/Bangkok')::date = (s.recorded_at AT TIME ZONE 'Asia/Bangkok')::date
    THEN s.today_distance_km + EXCLUDED.today_distance_km
    ELSE EXCLUDED.today_distance_km
  END,
  device_online = true
WHERE EXCLUDED.recorded_at > s.recorded_at`

// Write เขียนทุกจุดในทรานแซกชันเดียว — จุดไหนพัง ทั้งก้อนย้อนกลับ ไม่มีครึ่ง ๆ กลาง ๆ
func (s *Store) Write(ctx context.Context, positions []Position) (WriteResult, error) {
	// เรียงตามรถแล้วตามเวลา: ระยะทางสะสมบวกตามลำดับที่รถวิ่งจริง
	// และทุกทรานแซกชันล็อกแถวรถเรียงลำดับเดียวกัน จึงไม่เกิด deadlock ระหว่างสองคำขอ
	sorted := slices.Clone(positions)
	slices.SortStableFunc(sorted, func(a, b Position) int {
		return cmp.Or(strings.Compare(a.VehicleID, b.VehicleID), cmp.Compare(a.RecordedAt, b.RecordedAt))
	})

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return WriteResult{}, fmt.Errorf("ingest begin: %w", err)
	}
	// Commit แล้วเรียก Rollback จะไม่มีผล — ใส่ไว้กันลืมตอน return กลางทาง
	defer func() { _ = tx.Rollback(ctx) }()

	result := WriteResult{Received: len(positions)}
	for _, p := range sorted {
		inserted, liveUpdated, err := writeOne(ctx, tx, p)
		if err != nil {
			return WriteResult{}, err
		}
		if inserted {
			result.Inserted++
		} else {
			result.Duplicates++
		}
		if liveUpdated {
			result.LiveStateUpdated++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return WriteResult{}, fmt.Errorf("ingest commit: %w", err)
	}
	return result, nil
}

func writeOne(ctx context.Context, tx pgx.Tx, p Position) (inserted, liveUpdated bool, err error) {
	recordedAt := time.UnixMilli(p.RecordedAt)

	tag, err := tx.Exec(ctx, insertEventSQL,
		p.VehicleID, recordedAt, p.Lat, p.Lng, p.SpeedKph, p.HeadingDeg, p.IgnitionOn, p.FuelPct, p.Address)
	if err != nil {
		return false, false, writeError(p, err)
	}
	inserted = tag.RowsAffected() == 1

	segmentKm, err := segmentFromLive(ctx, tx, p, recordedAt)
	if err != nil {
		return false, false, writeError(p, err)
	}

	tag, err = tx.Exec(ctx, upsertLiveSQL,
		p.VehicleID, recordedAt, p.Lat, p.Lng, p.SpeedKph, p.HeadingDeg, p.IgnitionOn, p.FuelPct, p.Address, segmentKm)
	if err != nil {
		return false, false, writeError(p, err)
	}
	return inserted, tag.RowsAffected() == 1, nil
}

// segmentFromLive คือระยะจากตำแหน่งล่าสุดใน vehicle_live_state มาถึงจุดนี้ ใช้บวก today_distance_km
func segmentFromLive(ctx context.Context, tx pgx.Tx, p Position, recordedAt time.Time) (float64, error) {
	var lastAt time.Time
	var lastLat, lastLng float64
	var lastSpeed int

	err := tx.QueryRow(ctx, lastLiveSQL, p.VehicleID).Scan(&lastAt, &lastLat, &lastLng, &lastSpeed)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil // รถคันนี้ยังไม่เคยมีสถานะล่าสุด
	}
	if err != nil {
		return 0, err
	}

	// จุดที่เก่ากว่าสถานะล่าสุดจะไม่ถูกเขียนทับอยู่แล้ว ไม่ต้องนับระยะ
	// ความเร็ว 0 ทั้งสองจุด = GPS แกว่งตอนจอด ไม่ใช่รถขยับ
	if !recordedAt.After(lastAt) || (lastSpeed == 0 && p.SpeedKph == 0) {
		return 0, nil
	}
	return geo.DistanceKm(lastLat, lastLng, p.Lat, p.Lng), nil
}

func writeError(p Position, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation: ไม่มีรถ id นี้ในตาราง vehicles
		return fmt.Errorf("%w: %s", ErrUnknownVehicle, p.VehicleID)
	}
	return fmt.Errorf("ingest %s at %d: %w", p.VehicleID, p.RecordedAt, err)
}
