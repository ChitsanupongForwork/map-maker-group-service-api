package history

import (
	"fmt"
	"math"
)

// BuildEvents สร้างไทม์ไลน์ของหน้า history จากจุดดิบ
//
// ใช้ ignition_on ตัดสินว่า "จอด" ไม่ใช่ความเร็วเป็น 0 เพราะติดไฟแดงความเร็วก็เป็น 0 เหมือนกัน
//
//	start  → จุดแรกของช่วงที่ขอ
//	stop   → เครื่องยนต์ดับ (ชื่อบอกว่าจอดนานเท่าไหร่)
//	moving → หลังติดเครื่อง จุดแรกที่รถขยับจริง
//	end    → จุดสุดท้ายของช่วงที่ขอ
func BuildEvents(samples []Sample) []Event {
	events := []Event{}
	if len(samples) == 0 {
		return events
	}

	add := func(eventType EventType, index int, title, detail string) int {
		events = append(events, Event{
			ID:         fmt.Sprintf("e-%d", len(events)+1),
			Type:       eventType,
			Timestamp:  samples[index].Timestamp,
			Title:      title,
			Detail:     detail,
			PointIndex: index,
		})
		return len(events) - 1
	}

	first := samples[0]
	add(EventStart, 0, "เริ่มต้นเส้นทาง", place(first.Point))

	parked := !first.IgnitionOn
	waitingToMove := !(first.IgnitionOn && first.SpeedKph > 0)
	departed := !waitingToMove // จุดแรกก็วิ่งอยู่แล้ว
	openStop := -1             // ตำแหน่งใน events ของการจอดที่ยังไม่รู้ว่าจบเมื่อไหร่

	for i := 1; i < len(samples); i++ {
		current := samples[i]

		switch {
		case !parked && !current.IgnitionOn:
			parked = true
			openStop = add(EventStop, i, "", place(current.Point))
		case parked && current.IgnitionOn:
			parked = false
			waitingToMove = true
			if openStop >= 0 {
				events[openStop].Title = stopTitle(current.Timestamp - events[openStop].Timestamp)
				openStop = -1
			}
		}

		if !parked && waitingToMove && current.SpeedKph > 0 {
			title := "เดินทางต่อ"
			if !departed {
				title = "ออกเดินทาง"
			}
			add(EventMoving, i, title, fmt.Sprintf("ความเร็ว %d กม./ชม. · %s", current.SpeedKph, place(current.Point)))
			waitingToMove, departed = false, true
		}
	}

	last := len(samples) - 1
	if openStop >= 0 {
		// ยังจอดอยู่จนสุดช่วงที่ขอ — นับถึงจุดสุดท้ายที่มี
		events[openStop].Title = stopTitle(samples[last].Timestamp - events[openStop].Timestamp)
	}
	if last > 0 {
		add(EventEnd, last, "สิ้นสุดเส้นทาง", place(samples[last].Point))
	}
	return events
}

func stopTitle(durationMs int64) string {
	minutes := int(math.Round(float64(durationMs) / 60_000))
	switch {
	case minutes < 1:
		return "จอดรถไม่ถึง 1 นาที"
	case minutes < 60:
		return fmt.Sprintf("จอดรถ %d นาที", minutes)
	default:
		return fmt.Sprintf("จอดรถ %d ชม. %d นาที", minutes/60, minutes%60)
	}
}

// place คือข้อความบอกตำแหน่ง — ใช้ที่อยู่ถ้ามี ไม่มีก็ใช้พิกัด (position_events.address เป็น NULL ได้)
func place(point Point) string {
	if point.Address != "" {
		return point.Address
	}
	return fmt.Sprintf("%.5f, %.5f", point.Lat, point.Lng)
}
