// Package fleet ≈ features/fleet ของ front-end — ข้อมูลรถทุกคันสำหรับหน้า /map
// endpoint: GET /api/fleet, GET /api/fleet/stream
package fleet

// Vehicle ต้องตรงกับ src/features/fleet/types.ts ของ map-maker-group ทุกฟิลด์
type Vehicle struct {
	ID              string  `json:"id"`
	Plate           string  `json:"plate"`
	Label           string  `json:"label"`
	Make            string  `json:"make"`
	Model           string  `json:"model"`
	Status          Status  `json:"status"`
	SpeedKph        int     `json:"speedKph"`
	HeadingDeg      int     `json:"headingDeg"`
	Lat             float64 `json:"lat"`
	Lng             float64 `json:"lng"`
	LastUpdate      int64   `json:"lastUpdate"` // epoch ms: recordedAt.UnixMilli()
	DriverID        string  `json:"driverId"`
	DriverName      string  `json:"driverName"`
	GroupID         string  `json:"groupId"`
	AreaID          string  `json:"areaId"`
	Address         string  `json:"address"`
	FuelPct         int     `json:"fuelPct"`
	OdometerKm      int     `json:"odometerKm"`
	EngineHours     int     `json:"engineHours"`
	TodayDistanceKm float64 `json:"todayDistanceKm"`
	SpeedHistory    []int   `json:"speedHistory"` // ต้องเป็น []int{} ห้ามเป็น nil (nil → null)
}

// Status คือ "รถกำลังทำอะไร" — คนละเรื่องกับสถานะข้อมูล (realtime/delayed/stale)
// ซึ่ง front-end คำนวณเองจาก LastUpdate และจงใจไม่มีใน Go
type Status string

const (
	StatusRunning  Status = "running"
	StatusEngineOn Status = "engine-on"
	StatusParking  Status = "parking"
	StatusOffline  Status = "offline"
)

type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type Snapshot struct {
	GeneratedAt int64     `json:"generatedAt"`
	Groups      []Option  `json:"groups"`
	Areas       []Option  `json:"areas"`
	Drivers     []Option  `json:"drivers"`
	Vehicles    []Vehicle `json:"vehicles"`
}
