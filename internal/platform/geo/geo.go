// Package geo คำนวณระยะทางและทิศจากพิกัด WGS84
package geo

import "math"

const (
	earthRadiusKm = 6371.0
	kmPerDegree   = 111.32
	radPerDegree  = math.Pi / 180
)

// DistanceKm คือระยะทางตามผิวโลก (haversine) ระหว่างสองพิกัด
func DistanceKm(lat1, lng1, lat2, lng2 float64) float64 {
	dLat := (lat2 - lat1) * radPerDegree
	dLng := (lng2 - lng1) * radPerDegree
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*radPerDegree)*math.Cos(lat2*radPerDegree)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return earthRadiusKm * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// BearingDeg คือทิศจากจุดแรกไปจุดที่สอง 0–359 (0 = เหนือ ตามเข็มนาฬิกา)
// ความหมายเดียวกับ headingDeg ที่ front-end ใช้หมุนลูกศร
func BearingDeg(lat1, lng1, lat2, lng2 float64) int {
	dLng := (lng2 - lng1) * radPerDegree
	y := math.Sin(dLng) * math.Cos(lat2*radPerDegree)
	x := math.Cos(lat1*radPerDegree)*math.Sin(lat2*radPerDegree) -
		math.Sin(lat1*radPerDegree)*math.Cos(lat2*radPerDegree)*math.Cos(dLng)
	return NormalizeHeading(math.Atan2(y, x) / radPerDegree)
}

// NormalizeHeading ปัดองศาใด ๆ ให้อยู่ในช่วง 0–359 ไม่ใช่ -180 ถึง 180 (สเปกหัวข้อ 11)
func NormalizeHeading(deg float64) int {
	heading := int(math.Round(deg)) % 360
	if heading < 0 {
		heading += 360
	}
	return heading
}

// Move เลื่อนพิกัดไปตามทิศ headingDeg เป็นระยะ km
// ประมาณแบบระนาบ แม่นพอสำหรับระยะไม่กี่ร้อยเมตรต่อก้าว
func Move(lat, lng float64, headingDeg int, km float64) (float64, float64) {
	rad := float64(headingDeg) * radPerDegree
	nextLat := lat + math.Cos(rad)*km/kmPerDegree
	nextLng := lng + math.Sin(rad)*km/(kmPerDegree*math.Cos(lat*radPerDegree))
	return nextLat, nextLng
}

// Round5 ปัดทศนิยม 5 ตำแหน่ง (≈1 เมตร) พอสำหรับหมุดบนแผนที่ และทำให้ JSON สั้นลง
func Round5(value float64) float64 {
	return math.Round(value*100_000) / 100_000
}
