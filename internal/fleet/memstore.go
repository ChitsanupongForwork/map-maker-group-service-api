package fleet

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"time"

	"fleet-monitor-server/internal/platform/geo"
)

// MemStore คือกองรถจำลองในหน่วยความจำ ≈ lib/mock-fleet.ts ของ front-end
// ใช้ตอน DEMO_MODE (ยังไม่มีฐานข้อมูล) และในเทส
//
// ทุกครั้งที่มีคนขอ Snapshot รถที่อุปกรณ์ยังส่งข้อมูลอยู่จะได้ lastUpdate ใหม่
// และรถที่กำลังวิ่งจะขยับไปตามหัวรถตามเวลาที่ผ่านไปจริง
type MemStore struct {
	mu       sync.Mutex
	random   *rand.Rand
	lastMove time.Time

	vehicles []Vehicle
	homes    [][2]float64 // ย่านของแต่ละคัน วิ่งออกไปไกลเกินจะเลี้ยวกลับ
	// reporting[i] = false คืออุปกรณ์หยุดส่งข้อมูลแล้ว lastUpdate จะค้างให้เห็นสถานะ delayed / stale
	reporting []bool

	groups, areas, drivers []Option
}

func NewMemStore(size int) *MemStore {
	// seed คงที่ รันกี่ครั้งก็ได้ทะเบียน / คนขับ / ตำแหน่งตั้งต้นชุดเดิม เทียบผลใน Postman ได้
	random := rand.New(rand.NewPCG(20260912, 1))
	now := time.Now()
	consonants := []rune(plateConsonants)

	s := &MemStore{
		random:    random,
		lastMove:  now,
		vehicles:  make([]Vehicle, size),
		homes:     make([][2]float64, size),
		reporting: make([]bool, size),
		groups:    demoGroups,
		areas:     demoAreas,
		drivers:   make([]Option, size),
	}
	usedPlates := make(map[string]bool, size)

	for i := range size {
		status := demoStatus(i)
		home := demoHubs[random.IntN(len(demoHubs))]
		model := demoModels[random.IntN(len(demoModels))]
		driverName := demoFirstNames[random.IntN(len(demoFirstNames))] + " " + demoLastNames[random.IntN(len(demoLastNames))]
		reporting, age := demoDataAge(i, status, random)

		plate := ""
		for plate == "" || usedPlates[plate] {
			plate = fmt.Sprintf("%c%c %d",
				consonants[random.IntN(len(consonants))], consonants[random.IntN(len(consonants))], 1000+random.IntN(9000))
			if random.IntN(100) < 45 {
				plate = fmt.Sprint(1+random.IntN(9)) + plate
			}
		}
		usedPlates[plate] = true

		speed := 0
		if status == StatusRunning {
			speed = 18 + random.IntN(93)
		}

		driverID := fmt.Sprintf("d-%04d", i+1)
		s.vehicles[i] = Vehicle{
			ID:              fmt.Sprintf("v-%04d", i+1),
			Plate:           plate,
			Label:           model[0] + " " + model[1],
			Make:            model[0],
			Model:           model[1],
			Status:          status,
			SpeedKph:        speed,
			HeadingDeg:      random.IntN(360),
			Lat:             home[0] + random.NormFloat64()*0.016,
			Lng:             home[1] + random.NormFloat64()*0.019,
			LastUpdate:      now.Add(-age).UnixMilli(),
			DriverID:        driverID,
			DriverName:      driverName,
			GroupID:         demoGroups[random.IntN(len(demoGroups))].Value,
			AreaID:          demoAreas[random.IntN(len(demoAreas))].Value,
			Address:         demoRoads[random.IntN(len(demoRoads))],
			FuelPct:         12 + random.IntN(86),
			OdometerKm:      18_000 + random.IntN(240_000),
			EngineHours:     400 + random.IntN(9_000),
			TodayDistanceKm: float64(random.IntN(340)),
			SpeedHistory:    []int{},
		}
		s.homes[i] = home
		s.reporting[i] = reporting
		s.drivers[i] = Option{Value: driverID, Label: driverName}
	}

	slices.SortFunc(s.drivers, func(a, b Option) int { return strings.Compare(a.Label, b.Label) })
	return s
}

func (s *MemStore) Snapshot(_ context.Context) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	s.advance(now)

	// คืนสำเนาเสมอ — Hub เก็บ snapshot รอบก่อนไว้เทียบ
	// ถ้าส่ง slice ตัวเดิมออกไป รอบหน้าจะเทียบกับตัวเองแล้วไม่เห็นอะไรเปลี่ยนเลย
	vehicles := make([]Vehicle, len(s.vehicles))
	for i, v := range s.vehicles {
		v.Lat, v.Lng = geo.Round5(v.Lat), geo.Round5(v.Lng)
		v.TodayDistanceKm = math.Round(v.TodayDistanceKm*10) / 10
		v.SpeedHistory = []int{}
		vehicles[i] = v
	}

	return Snapshot{
		GeneratedAt: now.UnixMilli(),
		Groups:      slices.Clone(s.groups),
		Areas:       slices.Clone(s.areas),
		Drivers:     slices.Clone(s.drivers),
		Vehicles:    vehicles,
	}, nil
}

// advance เดินเวลาของกองรถไปถึง now — เรียกขณะถือ mu อยู่เท่านั้น
func (s *MemStore) advance(now time.Time) {
	elapsed := now.Sub(s.lastMove)
	if elapsed < time.Second {
		return
	}
	s.lastMove = now
	// ไม่มีใครขอข้อมูลนาน ๆ แล้วค่อยขอ รถจะได้ไม่วาร์ปไปไกล
	hours := min(elapsed, 10*time.Second).Hours()

	for i := range s.vehicles {
		if !s.reporting[i] {
			continue
		}
		v := &s.vehicles[i]
		v.LastUpdate = now.UnixMilli()

		// ติดไฟแดง / ออกตัว — สลับ running ↔ engine-on บ้าง จะได้เห็นสถานะเปลี่ยนผ่าน SSE
		switch {
		case v.Status == StatusRunning && s.random.IntN(100) < 3:
			v.Status, v.SpeedKph = StatusEngineOn, 0
		case v.Status == StatusEngineOn && s.random.IntN(100) < 15:
			v.Status, v.SpeedKph = StatusRunning, 15
		}
		if v.Status != StatusRunning {
			continue
		}

		heading := float64(v.HeadingDeg) + (s.random.Float64()-0.5)*14
		if home := s.homes[i]; math.Abs(v.Lat-home[0]) > 0.06 || math.Abs(v.Lng-home[1]) > 0.06 {
			heading = float64(geo.BearingDeg(v.Lat, v.Lng, home[0], home[1]))
		}
		v.HeadingDeg = geo.NormalizeHeading(heading)

		km := float64(v.SpeedKph) * hours
		v.Lat, v.Lng = geo.Move(v.Lat, v.Lng, v.HeadingDeg, km)
		v.TodayDistanceKm += km
		v.SpeedKph = min(118, max(8, v.SpeedKph+s.random.IntN(11)-5))
	}
}

// demoStatus แจกสถานะตามลำดับคัน ให้สัดส่วนใกล้หน้าจอออกแบบ
// (วิ่ง 48 / จอด 32 / จอดติดเครื่อง 15 / ออฟไลน์ 12)
func demoStatus(index int) Status {
	switch index % 9 {
	case 0, 2, 4, 6:
		return StatusRunning
	case 1, 5, 8:
		return StatusParking
	case 3:
		return StatusEngineOn
	default:
		return StatusOffline
	}
}

// demoDataAge บอกว่าอุปกรณ์ของคันนี้ยังส่งข้อมูลอยู่ไหม และข้อมูลล่าสุดเก่าแค่ไหนตอนเริ่ม
//
//	offline      → หยุดส่งมาแล้ว 31 นาที – 6 ชั่วโมง (เกินเกณฑ์ออฟไลน์ 30 นาทีในสเปกหัวข้อ 6.8)
//	จอดบางคัน    → หยุดส่งมา 2.5 – 14.5 นาที หน้าจอจะมีสถานะ "ไม่เรียลไทม์" ให้เห็น
//	ที่เหลือ      → ส่งปกติ ข้อมูลเก่าไม่เกินหนึ่งนาที
func demoDataAge(index int, status Status, random *rand.Rand) (reporting bool, age time.Duration) {
	switch {
	case status == StatusOffline:
		return false, 31*time.Minute + time.Duration(random.Int64N(int64(329*time.Minute)))
	case status == StatusParking && index%9 == 5:
		return false, 150*time.Second + time.Duration(random.Int64N(int64(720*time.Second)))
	default:
		return true, time.Duration(2+random.IntN(58)) * time.Second
	}
}

// ─── ข้อมูลตั้งต้น ชุดเดียวกับ lib/mock-fleet.ts ───

// ย่านที่รถกระจุกตัวในกรุงเทพฯ และปริมณฑล [lat, lng]
var demoHubs = [][2]float64{
	{13.801, 100.554}, {13.770, 100.554}, {13.769, 100.574}, {13.757, 100.534},
	{13.746, 100.534}, {13.716, 100.562}, {13.668, 100.604}, {13.690, 100.750},
	{13.765, 100.644}, {13.806, 100.606}, {13.723, 100.488}, {13.713, 100.399},
	{13.859, 100.514}, {13.913, 100.498}, {13.599, 100.599}, {13.985, 100.617},
	{13.656, 100.434}, {13.813, 100.737},
}

const plateConsonants = "กขคฆงจฉชฌญฎฐฑณดตถทธนบปผพภมยรลวศษสหฬอฮ"

var demoGroups = []Option{
	{Value: "g-central", Label: "ขนส่งภาคกลาง"},
	{Value: "g-north", Label: "ขนส่งภาคเหนือ"},
	{Value: "g-isan", Label: "ขนส่งภาคอีสาน"},
	{Value: "g-service", Label: "รถบริการ"},
	{Value: "g-exec", Label: "รถผู้บริหาร"},
}

var demoAreas = []Option{
	{Value: "a-inner", Label: "กรุงเทพฯ ชั้นใน"},
	{Value: "a-outer", Label: "กรุงเทพฯ รอบนอก"},
	{Value: "a-nonthaburi", Label: "นนทบุรี"},
	{Value: "a-samutprakan", Label: "สมุทรปราการ"},
	{Value: "a-pathumthani", Label: "ปทุมธานี"},
}

var demoFirstNames = []string{
	"สมชาย", "ประเสริฐ", "วิชัย", "อนุชา", "ธนากร", "ศักดิ์ชัย", "พงศ์พันธุ์",
	"ณัฐพล", "กิตติศักดิ์", "สุรชัย", "มานพ", "ชัยวัฒน์", "ธีรพงษ์", "ปรีชา",
	"อรรถพล", "วีระ", "สมศักดิ์", "จักรพงษ์", "ภาณุพงศ์", "ทวีศักดิ์",
}

var demoLastNames = []string{
	"ใจดี", "ศรีสุข", "รุ่งเรือง", "ทองคำ", "พรหมมา", "แสงทอง", "บุญมี",
	"วงศ์ไทย", "สุขสวัสดิ์", "เจริญพร", "ชัยมงคล", "อินทร์ทอง", "พูลทรัพย์",
}

var demoRoads = []string{
	"ถนนพหลโยธิน แขวงจอมพล เขตจตุจักร กรุงเทพมหานคร",
	"ถนนรัชดาภิเษก แขวงดินแดง เขตดินแดง กรุงเทพมหานคร",
	"ถนนลาดพร้าว แขวงจันทรเกษม เขตจตุจักร กรุงเทพมหานคร",
	"ถนนสุขุมวิท แขวงคลองเตย เขตคลองเตย กรุงเทพมหานคร",
	"ถนนเพชรบุรีตัดใหม่ แขวงบางกะปิ เขตห้วยขวาง กรุงเทพมหานคร",
	"ถนนบางนา-ตราด แขวงบางนา เขตบางนา กรุงเทพมหานคร",
	"ถนนบรมราชชนนี แขวงอรุณอมรินทร์ เขตบางกอกน้อย กรุงเทพมหานคร",
	"ถนนงามวงศ์วาน ตำบลบางเขน อำเภอเมือง นนทบุรี",
	"ถนนสุขสวัสดิ์ ตำบลบางครุ อำเภอพระประแดง สมุทรปราการ",
	"ถนนพระราม 2 แขวงแสมดำ เขตบางขุนเทียน กรุงเทพมหานคร",
}

var demoModels = [][2]string{
	{"Isuzu", "D-Max"}, {"Toyota", "Hilux Revo"}, {"Hino", "500 Series"}, {"Mitsubishi", "Fuso Canter"},
	{"Ford", "Ranger"}, {"Nissan", "Navara"}, {"Toyota", "Commuter"}, {"Isuzu", "Elf NLR"},
}
