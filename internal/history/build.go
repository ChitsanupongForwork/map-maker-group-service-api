package history

import "slices"

// Build ประกอบผลลัพธ์ของหน้า history จากจุดดิบที่เรียงตามเวลาแล้ว
//
// ลำดับสำคัญ: สรุปและหา events จากจุดดิบทั้งหมดก่อน แล้วค่อยลดจุด
// โดยบังคับเก็บจุดที่ events ชี้อยู่ไว้ แล้วแปลง pointIndex ให้ชี้ตำแหน่งใหม่
func Build(samples []Sample, maxPoints int) Result {
	summary := Summarize(samples)
	events := BuildEvents(samples)

	keep := make([]int, len(events))
	for i, event := range events {
		keep[i] = event.PointIndex
	}
	indices := Downsample(len(samples), maxPoints, keep)

	points := make([]Point, len(indices))
	for i, raw := range indices {
		points[i] = samples[raw].Point
	}
	for i := range events {
		// indices เรียงแล้ว และมีจุดของ event แน่นอนเพราะอยู่ใน keep
		events[i].PointIndex, _ = slices.BinarySearch(indices, events[i].PointIndex)
	}

	return Result{Points: points, Events: events, Summary: summary}
}
