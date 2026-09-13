// Package history ≈ features/vehicle-history ของ front-end — เส้นทางย้อนหลังของรถหนึ่งคัน
// endpoint: GET /api/vehicles/{id}/history?from=&to=
package history

// Point ต้องตรงกับ HistoryPoint ใน src/features/vehicle-history/types.ts
type Point struct {
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
	Timestamp  int64   `json:"timestamp"` // epoch ms
	SpeedKph   int     `json:"speedKph"`
	HeadingDeg int     `json:"headingDeg"`
	Address    string  `json:"address"`
}

// Sample คือจุดดิบจากฐานข้อมูล — มี IgnitionOn เพิ่มเพื่อใช้หา events แต่ไม่ส่งออกไป
type Sample struct {
	Point
	IgnitionOn bool
}

type EventType string

const (
	EventStart  EventType = "start"
	EventStop   EventType = "stop"
	EventMoving EventType = "moving"
	EventEnd    EventType = "end"
)

// Event ต้องตรงกับ HistoryEvent — PointIndex คือลำดับของจุดใน Result.Points (หลังลดจุดแล้ว)
type Event struct {
	ID         string    `json:"id"`
	Type       EventType `json:"type"`
	Timestamp  int64     `json:"timestamp"`
	Title      string    `json:"title"`
	Detail     string    `json:"detail"`
	PointIndex int       `json:"pointIndex"`
}

// Summary คือค่าสรุปที่ต้องคำนวณจากจุดดิบทั้งหมด (ก่อนลดจุด)
type Summary struct {
	DistanceKm      float64 `json:"distanceKm"`
	MovingMinutes   int     `json:"movingMinutes"`
	StoppedMinutes  int     `json:"stoppedMinutes"`
	MaxSpeedKph     int     `json:"maxSpeedKph"`
	AverageSpeedKph int     `json:"averageSpeedKph"`
}

// Result ≈ HistoryTrip ใน types.ts
// Summary ฝังไว้แบบไม่มีชื่อ ฟิลด์ของมันจึงแบนอยู่ระดับเดียวกับ points ใน JSON
type Result struct {
	Points []Point `json:"points"`
	Events []Event `json:"events"`
	Summary
}
