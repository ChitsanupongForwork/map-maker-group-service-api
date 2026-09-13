package fleet

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store ประกาศที่นี่ เพราะ fleet เป็นคนใช้ (GO-STRUCTURE กฎข้อ 4.1)
// PostgresStore กับ MemStore มี method นี้ครบ ก็ถือว่าทำตามแล้วโดยอัตโนมัติ
type Store interface {
	Snapshot(ctx context.Context) (Snapshot, error)
}

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// COALESCE ของ driver_id / group_id / area_id อยู่ตรงนี้ เพราะ view ยังไม่ได้ครอบให้
// ทั้งสามเป็น NULL ได้ (ON DELETE SET NULL) และ scan NULL เข้า string จะพังทั้งก้อนเพราะรถคันเดียว
//
// ORDER BY id สำคัญกับ Hub — Diff เทียบรถทีละตำแหน่ง ลำดับต้องคงที่ทุกรอบ
const snapshotSQL = `
SELECT id, plate, label, make, model, status,
       speed_kph::int, heading_deg::int, lat, lng, recorded_at,
       COALESCE(driver_id, ''), driver_name, COALESCE(group_id, ''), COALESCE(area_id, ''),
       address, fuel_pct::int, odometer_km, engine_hours, today_distance_km::float8
FROM fleet_snapshot
ORDER BY id`

func (s *PostgresStore) Snapshot(ctx context.Context) (Snapshot, error) {
	snap := Snapshot{GeneratedAt: time.Now().UnixMilli()}
	var err error

	if snap.Groups, err = s.options(ctx, "SELECT id, name FROM vehicle_groups ORDER BY name"); err != nil {
		return Snapshot{}, fmt.Errorf("fleet groups: %w", err)
	}
	if snap.Areas, err = s.options(ctx, "SELECT id, name FROM areas ORDER BY name"); err != nil {
		return Snapshot{}, fmt.Errorf("fleet areas: %w", err)
	}
	if snap.Drivers, err = s.options(ctx, "SELECT id, name FROM drivers ORDER BY name"); err != nil {
		return Snapshot{}, fmt.Errorf("fleet drivers: %w", err)
	}

	rows, err := s.pool.Query(ctx, snapshotSQL)
	if err != nil {
		return Snapshot{}, fmt.Errorf("fleet snapshot: %w", err)
	}
	defer rows.Close()

	snap.Vehicles = make([]Vehicle, 0, 128)
	for rows.Next() {
		var v Vehicle
		var recordedAt time.Time
		if err := rows.Scan(
			&v.ID, &v.Plate, &v.Label, &v.Make, &v.Model, &v.Status,
			&v.SpeedKph, &v.HeadingDeg, &v.Lat, &v.Lng, &recordedAt,
			&v.DriverID, &v.DriverName, &v.GroupID, &v.AreaID,
			&v.Address, &v.FuelPct, &v.OdometerKm, &v.EngineHours, &v.TodayDistanceKm,
		); err != nil {
			return Snapshot{}, fmt.Errorf("fleet snapshot scan: %w", err)
		}
		v.LastUpdate = recordedAt.UnixMilli() // ms ไม่ใช่ .Unix() ที่เป็นวินาที
		v.SpeedHistory = []int{}
		snap.Vehicles = append(snap.Vehicles, v)
	}
	if err := rows.Err(); err != nil {
		return Snapshot{}, fmt.Errorf("fleet snapshot rows: %w", err)
	}
	return snap, nil
}

// options อ่านตารางอ้างอิงเป็น {value, label} สำหรับ dropdown ตัวกรอง
func (s *PostgresStore) options(ctx context.Context, sql string) ([]Option, error) {
	rows, err := s.pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	options := make([]Option, 0, 16)
	for rows.Next() {
		var option Option
		if err := rows.Scan(&option.Value, &option.Label); err != nil {
			return nil, err
		}
		options = append(options, option)
	}
	return options, rows.Err()
}
