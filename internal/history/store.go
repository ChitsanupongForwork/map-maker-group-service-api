package history

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrVehicleNotFound = errors.New("vehicle not found")

// Store คืนจุดดิบของรถหนึ่งคันในช่วงเวลา เรียงจากเก่าไปใหม่
// ถ้าไม่มีรถคันนี้อยู่จริงให้คืน ErrVehicleNotFound (ต่างจากมีรถแต่ช่วงนั้นไม่มีจุด → slice ว่าง)
type Store interface {
	Samples(ctx context.Context, vehicleID string, from, to time.Time) ([]Sample, error)
}

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// WHERE vehicle_id = ? AND recorded_at BETWEEN ? AND ? ตรงกับ PRIMARY KEY (vehicle_id, recorded_at) พอดี
// จึงไม่ต้องมี index เพิ่ม
const samplesSQL = `
SELECT recorded_at, lat, lng, speed_kph::int, heading_deg::int, ignition_on, COALESCE(address, '')
FROM position_events
WHERE vehicle_id = $1 AND recorded_at BETWEEN $2 AND $3
ORDER BY recorded_at`

func (s *PostgresStore) Samples(ctx context.Context, vehicleID string, from, to time.Time) ([]Sample, error) {
	rows, err := s.pool.Query(ctx, samplesSQL, vehicleID, from, to)
	if err != nil {
		return nil, fmt.Errorf("history samples: %w", err)
	}
	defer rows.Close()

	samples := []Sample{}
	for rows.Next() {
		var sample Sample
		var recordedAt time.Time
		if err := rows.Scan(&recordedAt, &sample.Lat, &sample.Lng, &sample.SpeedKph, &sample.HeadingDeg,
			&sample.IgnitionOn, &sample.Address); err != nil {
			return nil, fmt.Errorf("history samples scan: %w", err)
		}
		sample.Timestamp = recordedAt.UnixMilli()
		samples = append(samples, sample)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("history samples rows: %w", err)
	}

	if len(samples) == 0 {
		// แยกให้ออกว่า "ไม่มีรถคันนี้" (404) กับ "รถมีแต่ช่วงนั้นไม่มีข้อมูล" (200 + [])
		var exists bool
		if err := s.pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM vehicles WHERE id = $1)", vehicleID).Scan(&exists); err != nil {
			return nil, fmt.Errorf("history vehicle exists: %w", err)
		}
		if !exists {
			return nil, ErrVehicleNotFound
		}
	}
	return samples, nil
}
