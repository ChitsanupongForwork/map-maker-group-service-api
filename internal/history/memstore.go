package history

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"fleet-monitor-server/internal/platform/geo"
)

// MemStore สร้างเส้นทางจำลองของรถ v-0001 … v-NNNN ≈ lib/mock-history.ts
// ใช้ตอน DEMO_MODE คู่กับ fleet.MemStore — รับจำนวนคันจาก main เพราะฟีเจอร์ห้าม import กันเอง
//
// ทุกจุดคำนวณจาก timestamp ล้วน ๆ ขอช่วงเวลาที่ทับกันกี่ครั้งก็ได้จุดเดิม
// หนึ่งรอบยาว 3 ชั่วโมง เริ่มและจบที่ต้นทางเสมอ เส้นทางจึงไม่กระโดดข้ามรอบ:
//
//	0:00–0:22  วิ่งไปปลายทาง      1:30–1:52  วิ่งกลับต้นทาง
//	0:22–0:25  ติดเครื่องรอ        1:52–1:55  ติดเครื่องรอ
//	0:25–1:30  ดับเครื่องจอด       1:55–3:00  ดับเครื่องจอด
//
// รอบที่เริ่มช่วง 22:00–06:00 เวลาไทย จอดดับเครื่องที่ต้นทางทั้งรอบ
type MemStore struct {
	fleetSize int
}

func NewMemStore(fleetSize int) *MemStore {
	return &MemStore{fleetSize: fleetSize}
}

const (
	sampleEvery = 30 * time.Second
	tripCycle   = 3 * time.Hour
	driveFor    = 22 * time.Minute
	idleFor     = 3 * time.Minute
)

// ประเทศไทยไม่มี daylight saving ใช้ FixedZone ได้ ไม่ต้องพึ่ง tzdata ของเครื่อง
var bangkok = time.FixedZone("Asia/Bangkok", 7*60*60)

func (s *MemStore) Samples(_ context.Context, vehicleID string, from, to time.Time) ([]Sample, error) {
	seed, ok := s.seedOf(vehicleID)
	if !ok {
		return nil, ErrVehicleNotFound
	}

	step := sampleEvery.Milliseconds()
	end := min(to.UnixMilli(), time.Now().UnixMilli()) // ไม่มีข้อมูลจากอนาคต
	samples := []Sample{}
	for t := ceilTo(from.UnixMilli(), step); t <= end; t += step {
		samples = append(samples, sampleAt(seed, t))
	}
	return samples, nil
}

// seedOf รับเฉพาะ id รูปแบบเดียวกับ fleet.MemStore (v-0001) และไม่เกินจำนวนคัน
func (s *MemStore) seedOf(vehicleID string) (int, bool) {
	number, err := strconv.Atoi(strings.TrimPrefix(vehicleID, "v-"))
	if err != nil || number < 1 || number > s.fleetSize || fmt.Sprintf("v-%04d", number) != vehicleID {
		return 0, false
	}
	return number, true
}

// sampleAt คือสภาพของรถคัน seed ณ เวลา t
func sampleAt(seed int, t int64) Sample {
	// แต่ละคันเยื้องเส้นทางกันเล็กน้อย เลือกหลายคันมาดูแล้วเส้นไม่ทับกันสนิท
	offsetLat := float64(seed%5-2) * 0.0003
	offsetLng := float64(seed%7-3) * 0.00035

	state := stateAt(seed, t)
	lat, lng, address := demoRoute.at(state.fraction)

	sample := Sample{
		Point: Point{
			Lat:        geo.Round5(lat + offsetLat),
			Lng:        geo.Round5(lng + offsetLng),
			Timestamp:  t,
			HeadingDeg: demoRoute.headingAt(state.fraction, state.outbound),
			Address:    address,
		},
		IgnitionOn: state.ignitionOn,
	}

	if state.driving {
		// ความเร็ว = ระยะที่ขยับจากจุดก่อนหน้าหารเวลา สรุประยะทางกับความเร็วจึงสอดคล้องกันเสมอ
		before := stateAt(seed, t-sampleEvery.Milliseconds())
		prevLat, prevLng, _ := demoRoute.at(before.fraction)
		km := geo.DistanceKm(prevLat, prevLng, lat, lng)
		sample.SpeedKph = int(math.Round(km / sampleEvery.Hours()))
	}
	return sample
}

type legState struct {
	fraction   float64 // 0 = ต้นทาง, 1 = ปลายทาง
	outbound   bool
	driving    bool
	ignitionOn bool
}

func stateAt(seed int, t int64) legState {
	// แต่ละคันออกรถไม่พร้อมกัน เลื่อนรอบไปคันละ 7 นาที
	local := t + int64(seed)*(7*time.Minute).Milliseconds()
	phase := time.Duration(floorMod(local, tripCycle.Milliseconds())) * time.Millisecond
	cycleStart := t - phase.Milliseconds()

	if hour := time.UnixMilli(cycleStart).In(bangkok).Hour(); hour >= 22 || hour < 6 {
		return legState{fraction: 0}
	}

	state := legState{outbound: phase < tripCycle/2}
	if !state.outbound {
		phase -= tripCycle / 2
	}

	progress := 1.0
	switch {
	case phase < driveFor:
		u := float64(phase) / float64(driveFor)
		// เร่ง-ผ่อนเป็นช่วง ๆ ความเร็วจะแกว่งแทนที่จะคงที่ทั้งทาง
		// อนุพันธ์ 1 - 0.6·cos(6πu) ไม่ติดลบ รถจึงไม่ถอยหลัง
		progress = u - 0.6*math.Sin(6*math.Pi*u)/(6*math.Pi)
		state.driving, state.ignitionOn = true, true
	case phase < driveFor+idleFor:
		state.ignitionOn = true
	}

	state.fraction = progress
	if !state.outbound {
		state.fraction = 1 - progress
	}
	return state
}

func floorMod(a, b int64) int64 {
	return ((a % b) + b) % b
}

// ceilTo ปัด value ขึ้นให้ลงตัวกับ step — จุดจะอยู่บนเวลาเดิมเสมอไม่ว่าขอช่วงไหน
func ceilTo(value, step int64) int64 {
	return value + floorMod(-value, step)
}

// ─── เส้นทางจำลอง ชุดเดียวกับ ROUTE ใน lib/mock-history.ts ───

type waypoint struct {
	lat, lng float64
	name     string
}

type route struct {
	points     []waypoint
	cumulative []float64 // ระยะสะสมจากต้นทางถึงจุดที่ i (กม.)
}

func newRoute(points []waypoint) route {
	cumulative := make([]float64, len(points))
	for i := 1; i < len(points); i++ {
		cumulative[i] = cumulative[i-1] + geo.DistanceKm(points[i-1].lat, points[i-1].lng, points[i].lat, points[i].lng)
	}
	return route{points: points, cumulative: cumulative}
}

// at คือพิกัดที่สัดส่วน fraction ของระยะทางทั้งเส้น พร้อมชื่อของ waypoint ที่ใกล้ที่สุด
func (r route) at(fraction float64) (lat, lng float64, name string) {
	target := min(1, max(0, fraction)) * r.cumulative[len(r.cumulative)-1]

	i := 1
	for i < len(r.points)-1 && r.cumulative[i] < target {
		i++
	}
	a, b := r.points[i-1], r.points[i]

	part := 0.0
	if span := r.cumulative[i] - r.cumulative[i-1]; span > 0 {
		part = min(1, (target-r.cumulative[i-1])/span)
	}

	name = a.name
	if part >= 0.5 {
		name = b.name
	}
	return a.lat + (b.lat-a.lat)*part, a.lng + (b.lng-a.lng)*part, name
}

// headingAt คือทิศของถนน ณ fraction ตามทิศที่รถวิ่ง — ตอนจอดก็ยังชี้ทางที่วิ่งมา
func (r route) headingAt(fraction float64, outbound bool) int {
	const delta = 0.002
	lo := max(0, fraction-delta)
	hi := min(1, lo+2*delta)
	loLat, loLng, _ := r.at(lo)
	hiLat, hiLng, _ := r.at(hi)
	if outbound {
		return geo.BearingDeg(loLat, loLng, hiLat, hiLng)
	}
	return geo.BearingDeg(hiLat, hiLng, loLat, loLng)
}

var demoRoute = newRoute([]waypoint{
	{13.7365, 100.5267, "ถนนสาทรเหนือ เขตบางรัก"},
	{13.7391, 100.5312, "แยกนราธิวาส–สาทร"},
	{13.7427, 100.5382, "ถนนพระราม 4 แขวงสีลม"},
	{13.7462, 100.5447, "แยกศาลาแดง เขตปทุมวัน"},
	{13.7489, 100.5516, "ถนนวิทยุ แขวงลุมพินี"},
	{13.7518, 100.5589, "ถนนเพลินจิต เขตวัฒนา"},
	{13.7558, 100.5669, "แยกอโศก ถนนสุขุมวิท"},
	{13.7606, 100.5751, "ถนนเพชรบุรีตัดใหม่"},
	{13.7669, 100.5834, "ถนนพระราม 9 เขตห้วยขวาง"},
	{13.7724, 100.5912, "แยก อ.ส.ม.ท. เขตห้วยขวาง"},
	{13.7798, 100.5979, "ถนนรัชดาภิเษก"},
	{13.7884, 100.6021, "แยกสุทธิสาร เขตดินแดง"},
	{13.7967, 100.5972, "ถนนลาดพร้าว แขวงจอมพล"},
	{13.8027, 100.5891, "แยกรัชดา–ลาดพร้าว"},
	{13.8075, 100.5804, "ถนนพหลโยธิน เขตจตุจักร"},
	{13.8112, 100.5712, "ห้าแยกลาดพร้าว"},
	{13.8148, 100.5622, "ถนนวิภาวดีรังสิต"},
	{13.8192, 100.5541, "สถานีกลางกรุงเทพอภิวัฒน์"},
})
