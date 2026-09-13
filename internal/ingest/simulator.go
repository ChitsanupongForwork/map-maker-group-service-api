package ingest

import (
	"context"
	"fmt"
	"log"
	"math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5"

	"fleet-monitor-server/internal/platform/geo"
)

const (
	simulateEvery = 3 * time.Second
	// รถที่จอดส่งทุก ๆ 10 รอบ (30 วินาที) เหมือนอุปกรณ์จริงที่ส่งห่างขึ้นตอนจอดเพื่อประหยัดแบต
	parkedEveryRounds = 10
)

// Simulator จำลองอุปกรณ์ GPS ส่งตำแหน่งเข้าฐานข้อมูลจริง ใช้ตอนมี DB แต่ยังไม่มีอุปกรณ์ (SIMULATE=true)
// เขียนผ่าน Store.Write ตัวเดียวกับ POST /api/positions — ทดสอบเส้นทางเขียนจริงไปในตัว
//
// แตะเฉพาะรถที่ device_online = true รถที่ออฟไลน์อยู่ก็ออฟไลน์ต่อไป
type Simulator struct {
	store  *Store
	random *rand.Rand
}

func NewSimulator(store *Store) *Simulator {
	return &Simulator{store: store, random: rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 7))}
}

func (s *Simulator) Run(ctx context.Context) {
	ticker := time.NewTicker(simulateEvery)
	defer ticker.Stop()

	for round := 0; ; round++ {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if err := s.tick(ctx, round%parkedEveryRounds == 0); err != nil && ctx.Err() == nil {
			log.Printf("simulator: %v", err)
		}
	}
}

const onlineVehiclesSQL = `
SELECT vehicle_id, lat, lng, speed_kph::int, heading_deg::int, ignition_on, fuel_pct::int
FROM vehicle_live_state
WHERE device_online`

func (s *Simulator) tick(ctx context.Context, includeParked bool) error {
	rows, err := s.store.pool.Query(ctx, onlineVehiclesSQL)
	if err != nil {
		return fmt.Errorf("read live state: %w", err)
	}
	vehicles, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Position, error) {
		var p Position
		err := row.Scan(&p.VehicleID, &p.Lat, &p.Lng, &p.SpeedKph, &p.HeadingDeg, &p.IgnitionOn, &p.FuelPct)
		return p, err
	})
	if err != nil {
		return fmt.Errorf("scan live state: %w", err)
	}

	now := time.Now().UnixMilli()
	batch := make([]Position, 0, len(vehicles))
	for _, p := range vehicles {
		moving := p.IgnitionOn && p.SpeedKph > 0
		if !moving && !includeParked {
			continue
		}
		p.RecordedAt = now
		if moving {
			s.drive(&p)
		}
		batch = append(batch, p)
	}
	if len(batch) == 0 {
		return nil
	}

	_, err = s.store.Write(ctx, batch)
	return err
}

// ขอบเขตคร่าว ๆ ของกรุงเทพฯ และปริมณฑล รถวิ่งหลุดออกไปจะเลี้ยวกลับเข้าหาใจกลางเมือง
const (
	minLat, maxLat = 13.55, 14.05
	minLng, maxLng = 100.30, 100.90
	centerLat      = 13.7466
	centerLng      = 100.5605
)

func (s *Simulator) drive(p *Position) {
	heading := float64(p.HeadingDeg) + (s.random.Float64()-0.5)*14
	if p.Lat < minLat || p.Lat > maxLat || p.Lng < minLng || p.Lng > maxLng {
		heading = float64(geo.BearingDeg(p.Lat, p.Lng, centerLat, centerLng))
	}
	p.HeadingDeg = geo.NormalizeHeading(heading)

	km := float64(p.SpeedKph) * simulateEvery.Hours()
	lat, lng := geo.Move(p.Lat, p.Lng, p.HeadingDeg, km)
	p.Lat, p.Lng = geo.Round5(lat), geo.Round5(lng)
	p.SpeedKph = min(110, max(10, p.SpeedKph+s.random.IntN(11)-5))
}
