package history

import (
	"math"

	"fleet-monitor-server/internal/platform/geo"
)

// Summarize คำนวณระยะทาง นาทีวิ่ง/จอด และความเร็วจากจุดดิบที่เรียงตามเวลาแล้ว
//
// เวลาระหว่างสองจุดนับตามสภาพของ "จุดต้นช่วง": จุดต้นวิ่งอยู่ = ช่วงนั้นคือเวลาวิ่ง
// ระยะทางไม่นับช่วงที่ความเร็วเป็น 0 ทั้งสองฝั่ง เพราะคือ GPS แกว่งตอนจอด ไม่ใช่รถขยับ
func Summarize(samples []Sample) Summary {
	var summary Summary
	var movingMs, stoppedMs int64
	var speedTimesMs float64 // Σ ความเร็ว × เวลา ของช่วงที่วิ่ง → ใช้หาความเร็วเฉลี่ยถ่วงตามเวลา

	for i, current := range samples {
		summary.MaxSpeedKph = max(summary.MaxSpeedKph, current.SpeedKph)
		if i == 0 {
			continue
		}

		previous := samples[i-1]
		elapsed := current.Timestamp - previous.Timestamp
		if previous.SpeedKph > 0 {
			movingMs += elapsed
			speedTimesMs += float64(previous.SpeedKph) * float64(elapsed)
		} else {
			stoppedMs += elapsed
		}

		if previous.SpeedKph > 0 || current.SpeedKph > 0 {
			summary.DistanceKm += geo.DistanceKm(previous.Lat, previous.Lng, current.Lat, current.Lng)
		}
	}

	summary.DistanceKm = math.Round(summary.DistanceKm*10) / 10
	summary.MovingMinutes = int(math.Round(float64(movingMs) / 60_000))
	summary.StoppedMinutes = int(math.Round(float64(stoppedMs) / 60_000))
	if movingMs > 0 {
		summary.AverageSpeedKph = int(math.Round(speedTimesMs / float64(movingMs)))
	}
	return summary
}
