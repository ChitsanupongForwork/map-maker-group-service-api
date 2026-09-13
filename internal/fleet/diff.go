package fleet

// Patch คือส่วนที่เปลี่ยนของรถหนึ่งคัน — id มีเสมอ ฟิลด์อื่นมีเฉพาะที่เปลี่ยน (สเปกหัวข้อ 4)
//
// ใช้ pointer คู่กับ omitempty:
//
//	nil        → ไม่เปลี่ยน ไม่โผล่ใน JSON
//	ชี้ไปที่ 0  → เปลี่ยนเป็น 0 โผล่ใน JSON เป็น "speedKph":0
//
// ถ้าใช้ int ธรรมดา + omitempty รถที่เพิ่งจอด (ความเร็วเปลี่ยนเป็น 0) จะหายไปจาก patch
type Patch struct {
	ID              string   `json:"id"`
	Plate           *string  `json:"plate,omitempty"`
	Label           *string  `json:"label,omitempty"`
	Make            *string  `json:"make,omitempty"`
	Model           *string  `json:"model,omitempty"`
	Status          *Status  `json:"status,omitempty"`
	SpeedKph        *int     `json:"speedKph,omitempty"`
	HeadingDeg      *int     `json:"headingDeg,omitempty"`
	Lat             *float64 `json:"lat,omitempty"`
	Lng             *float64 `json:"lng,omitempty"`
	LastUpdate      *int64   `json:"lastUpdate,omitempty"`
	DriverID        *string  `json:"driverId,omitempty"`
	DriverName      *string  `json:"driverName,omitempty"`
	GroupID         *string  `json:"groupId,omitempty"`
	AreaID          *string  `json:"areaId,omitempty"`
	Address         *string  `json:"address,omitempty"`
	FuelPct         *int     `json:"fuelPct,omitempty"`
	OdometerKm      *int     `json:"odometerKm,omitempty"`
	EngineHours     *int     `json:"engineHours,omitempty"`
	TodayDistanceKm *float64 `json:"todayDistanceKm,omitempty"`
}

// Diff เทียบรถทีละคันระหว่างสองรอบ คืนเฉพาะคันที่มีอะไรเปลี่ยน
//
// sameFleet = false แปลว่ารายชื่อรถไม่ตรงกัน (มีคันเพิ่ม หาย หรือลำดับเปลี่ยน)
// patch อธิบายเรื่องนี้ไม่ได้ ผู้เรียกต้องส่งก้อนเต็มแทน
func Diff(prev, next []Vehicle) (patches []Patch, sameFleet bool) {
	if len(prev) != len(next) {
		return nil, false
	}

	patches = []Patch{}
	for i := range next {
		a, b := prev[i], next[i]
		if a.ID != b.ID {
			return nil, false
		}

		patch := Patch{
			ID:              b.ID,
			Plate:           changed(a.Plate, b.Plate),
			Label:           changed(a.Label, b.Label),
			Make:            changed(a.Make, b.Make),
			Model:           changed(a.Model, b.Model),
			Status:          changed(a.Status, b.Status),
			SpeedKph:        changed(a.SpeedKph, b.SpeedKph),
			HeadingDeg:      changed(a.HeadingDeg, b.HeadingDeg),
			Lat:             changed(a.Lat, b.Lat),
			Lng:             changed(a.Lng, b.Lng),
			LastUpdate:      changed(a.LastUpdate, b.LastUpdate),
			DriverID:        changed(a.DriverID, b.DriverID),
			DriverName:      changed(a.DriverName, b.DriverName),
			GroupID:         changed(a.GroupID, b.GroupID),
			AreaID:          changed(a.AreaID, b.AreaID),
			Address:         changed(a.Address, b.Address),
			FuelPct:         changed(a.FuelPct, b.FuelPct),
			OdometerKm:      changed(a.OdometerKm, b.OdometerKm),
			EngineHours:     changed(a.EngineHours, b.EngineHours),
			TodayDistanceKm: changed(a.TodayDistanceKm, b.TodayDistanceKm),
		}
		// ทุก pointer เป็น nil = ไม่มีอะไรเปลี่ยน ไม่ต้องส่งคันนี้
		if patch != (Patch{ID: b.ID}) {
			patches = append(patches, patch)
		}
	}
	return patches, true
}

func changed[T comparable](before, after T) *T {
	if before == after {
		return nil
	}
	return &after
}
