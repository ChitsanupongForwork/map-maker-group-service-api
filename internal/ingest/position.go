// Package ingest คือฝั่งเขียนข้อมูล — รับตำแหน่งรถเข้าฐานข้อมูล ไม่มีคู่ใน front-end
// endpoint: POST /api/positions
package ingest

import (
	"errors"
	"strings"
	"time"
)

// Position คือหนึ่งจุดที่อุปกรณ์ส่งเข้ามา
type Position struct {
	VehicleID string `json:"vehicleId"`
	// RecordedAt คือเวลาที่ "อุปกรณ์วัดได้" (epoch ms) ไม่ใช่เวลาที่เซิร์ฟเวอร์รับ
	// รถออกจากอุโมงค์แล้วส่งจุดที่ค้างไว้มาพร้อมกัน เส้นทางจะยังเรียงถูก
	RecordedAt int64   `json:"recordedAt"`
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
	SpeedKph   int     `json:"speedKph"`
	HeadingDeg int     `json:"headingDeg"`
	IgnitionOn bool    `json:"ignitionOn"`
	FuelPct    *int    `json:"fuelPct,omitempty"` // ไม่ส่งมา = ไม่รู้ → NULL
	Address    *string `json:"address,omitempty"` // ไม่ส่งมา = NULL (สเปกหัวข้อ 7.2)
}

// อุปกรณ์กับเซิร์ฟเวอร์นาฬิกาเพี้ยนกันได้นิดหน่อย แต่จุดจากอนาคตไกล ๆ คือข้อมูลผิดแน่นอน
const maxClockSkew = 5 * time.Minute

// Validate ตรวจตามช่วงค่าเดียวกับ CHECK ในตาราง จะได้ตอบ 400 ที่อ่านรู้เรื่อง
// แทนที่จะปล่อยให้ Postgres ตอบ error 500 กลับไป
func (p Position) Validate(now time.Time) error {
	switch {
	case strings.TrimSpace(p.VehicleID) == "":
		return errors.New("vehicleId is required")
	case p.RecordedAt < 100_000_000_000 || p.RecordedAt >= 10_000_000_000_000:
		return errors.New("recordedAt must be epoch milliseconds (13 digits), not seconds")
	case time.UnixMilli(p.RecordedAt).After(now.Add(maxClockSkew)):
		return errors.New("recordedAt is in the future")
	case p.Lat == 0 && p.Lng == 0:
		return errors.New("lat and lng are required")
	case p.Lat < -90 || p.Lat > 90:
		return errors.New("lat must be between -90 and 90")
	case p.Lng < -180 || p.Lng > 180:
		return errors.New("lng must be between -180 and 180")
	case p.SpeedKph < 0 || p.SpeedKph > 400:
		return errors.New("speedKph must be between 0 and 400")
	case p.HeadingDeg < 0 || p.HeadingDeg > 359:
		return errors.New("headingDeg must be between 0 and 359 (0 = north, clockwise)")
	case p.FuelPct != nil && (*p.FuelPct < 0 || *p.FuelPct > 100):
		return errors.New("fuelPct must be between 0 and 100")
	}
	return nil
}
