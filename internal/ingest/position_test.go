package ingest

import (
	"strings"
	"testing"
	"time"
)

func validPosition(now time.Time) Position {
	return Position{
		VehicleID:  "v-0001",
		RecordedAt: now.Add(-10 * time.Second).UnixMilli(),
		Lat:        13.80123,
		Lng:        100.55412,
		SpeedKph:   68,
		HeadingDeg: 285,
		IgnitionOn: true,
	}
}

func TestValidate(t *testing.T) {
	now := time.Now()
	fuel := 101
	tests := map[string]struct {
		change  func(*Position)
		wantErr string
	}{
		"valid":              {change: func(*Position) {}},
		"missing vehicle":    {change: func(p *Position) { p.VehicleID = " " }, wantErr: "vehicleId"},
		"seconds not ms":     {change: func(p *Position) { p.RecordedAt /= 1000 }, wantErr: "13 digits"},
		"from the future":    {change: func(p *Position) { p.RecordedAt = now.Add(time.Hour).UnixMilli() }, wantErr: "future"},
		"missing coordinate": {change: func(p *Position) { p.Lat, p.Lng = 0, 0 }, wantErr: "required"},
		"heading 360":        {change: func(p *Position) { p.HeadingDeg = 360 }, wantErr: "headingDeg"},
		"negative heading":   {change: func(p *Position) { p.HeadingDeg = -90 }, wantErr: "headingDeg"},
		"fuel over 100":      {change: func(p *Position) { p.FuelPct = &fuel }, wantErr: "fuelPct"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			p := validPosition(now)
			tt.change(&p)
			err := p.Validate(now)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Fatalf("error = %v, want one mentioning %q", err, tt.wantErr)
			}
		})
	}
}

func TestDecodePositionsAcceptsObjectOrArray(t *testing.T) {
	one, err := decodePositions(strings.NewReader(`{"vehicleId":"v-0001","recordedAt":1789765430000,"lat":13.8,"lng":100.5}`))
	if err != nil || len(one) != 1 || one[0].VehicleID != "v-0001" {
		t.Fatalf("object: %v, %v", one, err)
	}

	many, err := decodePositions(strings.NewReader(` [{"vehicleId":"v-0001"},{"vehicleId":"v-0002"}]`))
	if err != nil || len(many) != 2 {
		t.Fatalf("array: %v, %v", many, err)
	}
}

func TestDecodePositionsRejectsMisspelledField(t *testing.T) {
	_, err := decodePositions(strings.NewReader(`{"vehicleId":"v-0001","heading":90}`))
	if err == nil {
		t.Fatal("want error for unknown field \"heading\" (should be headingDeg)")
	}
}
