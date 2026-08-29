package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const fleetSize = 1000

var demoDrivers = []struct{ name, initials string }{{"Narin S.", "NS"}, {"Pimchanok K.", "PK"}, {"Thanawat R.", "TR"}, {"Mali C.", "MC"}}
var demoVehicles = []struct{ make, model string }{{"Toyota", "Hilux Revo"}, {"Isuzu", "D-Max"}, {"Honda", "City Hatchback"}, {"Ford", "Ranger"}}
var demoPlaces = []string{"North depot", "Rama IX hub", "Bang Na yard", "Riverside depot", "Lat Krabang hub", "Chatuchak depot"}

type vehicle struct {
	ID             string  `json:"id"`
	TenantID       string  `json:"-"`
	DeviceID       string  `json:"-"`
	DriverID       string  `json:"-"`
	Code           string  `json:"code"`
	Label          string  `json:"label"`
	Status         string  `json:"status"`
	SpeedKph       int     `json:"speedKph"`
	AccOn          bool    `json:"accOn"`
	HeadingDeg     int     `json:"headingDeg"`
	DriverName     string  `json:"driverName"`
	DriverInitials string  `json:"driverInitials"`
	DriverPhone    string  `json:"driverPhone"`
	LicensePlate   string  `json:"licensePlate"`
	Make           string  `json:"make"`
	Model          string  `json:"model"`
	Origin         string  `json:"origin"`
	Destination    string  `json:"destination"`
	Lat            float64 `json:"lat"`
	Lng            float64 `json:"lng"`
	LastUpdate     string  `json:"lastUpdate"`
}

type fleetEvent struct {
	Type     string    `json:"type"`
	Vehicles []vehicle `json:"vehicles"`
}

type simulator struct {
	mu       sync.RWMutex
	vehicles []vehicle
	clients  map[chan []byte]struct{}
	random   *rand.Rand
}

func newSimulator() *simulator {
	s := &simulator{clients: make(map[chan []byte]struct{}), random: rand.New(rand.NewPCG(9042, 108))}
	s.vehicles = make([]vehicle, fleetSize)
	for i := range s.vehicles {
		status := "moving"
		if i%5 == 3 {
			status = "stopped"
		} else if i%5 == 4 {
			status = "offline"
		}
		moving := status == "moving"
		driver := demoDrivers[i%len(demoDrivers)]
		vehicleInfo := demoVehicles[i%len(demoVehicles)]
		s.vehicles[i] = vehicle{
			ID: fmt.Sprintf("fleet-%d", i+1), Code: fmt.Sprintf("FV-%05d", i+1), Label: fmt.Sprintf("Fleet vehicle %d", i+1),
			Status: status, SpeedKph: speedFor(s.random, moving), AccOn: status != "offline", HeadingDeg: s.random.IntN(360),
			DriverName: driver.name, DriverInitials: driver.initials, DriverPhone: fmt.Sprintf("Demo +66 80-000-%04d", i%9999+1), LicensePlate: fmt.Sprintf("DEMO %03d", i%999+1), Make: vehicleInfo.make, Model: vehicleInfo.model, Origin: demoPlaces[i%len(demoPlaces)], Destination: demoPlaces[(i+2)%len(demoPlaces)],
			Lat: 13.55 + s.random.Float64()*0.40, Lng: 100.35 + s.random.Float64()*0.60, LastUpdate: now(),
		}
	}
	return s
}

func speedFor(random *rand.Rand, moving bool) int {
	if !moving {
		return 0
	}
	return 25 + random.IntN(78)
}

func now() string { return time.Now().Format("15:04:05") }

func (s *simulator) snapshot() []vehicle {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]vehicle(nil), s.vehicles...)
}

func (s *simulator) subscribe() chan []byte {
	client := make(chan []byte, 8)
	s.mu.Lock()
	s.clients[client] = struct{}{}
	s.mu.Unlock()
	return client
}

func (s *simulator) unsubscribe(client chan []byte) {
	s.mu.Lock()
	delete(s.clients, client)
	s.mu.Unlock()
}

func (s *simulator) tick() {
	s.mu.Lock()
	updates := make([]vehicle, 0, 32)
	for range 32 {
		index := s.random.IntN(len(s.vehicles))
		item := &s.vehicles[index]
		if item.Status == "offline" && s.random.IntN(10) == 0 {
			item.Status, item.AccOn = "stopped", true
		} else if item.Status == "stopped" && s.random.IntN(5) == 0 {
			item.Status, item.AccOn = "moving", true
		}
		if item.Status == "moving" {
			item.SpeedKph = speedFor(s.random, true)
			item.HeadingDeg = (item.HeadingDeg + s.random.IntN(31) - 15 + 360) % 360
			radians := float64(item.HeadingDeg) * math.Pi / 180
			item.Lat += math.Cos(radians) * 0.00035
			item.Lng += math.Sin(radians) * 0.00035
		} else {
			item.SpeedKph = 0
		}
		item.LastUpdate = now()
		updates = append(updates, *item)
	}
	payload, _ := json.Marshal(fleetEvent{Type: "vehicle-updates", Vehicles: updates})
	clients := make([]chan []byte, 0, len(s.clients))
	for client := range s.clients {
		clients = append(clients, client)
	}
	s.mu.Unlock()

	for _, client := range clients {
		select {
		case client <- payload:
		default:
		}
	}
}

func (s *simulator) run(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

func withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if origin := allowedCORSOrigin(r.Header.Get("Origin")); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func allowedCORSOrigin(origin string) string {
	if origin == "" {
		return ""
	}

	configuredOrigins := os.Getenv("FRONTEND_ORIGINS")
	if configuredOrigins == "" {
		configuredOrigins = "http://localhost:3000,https://map-maker-group.vercel.app"
	}
	for _, allowed := range strings.Split(configuredOrigins, ",") {
		if strings.TrimSpace(allowed) == origin {
			return origin
		}
	}
	return ""
}

// legacyInMemoryMain is kept temporarily as a reference for the original demo.
// The active main function now lives in db_service.go and uses PostgreSQL.
func legacyInMemoryMain() {
	sim := newSimulator()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sim.run(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", withCORS(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	mux.HandleFunc("GET /api/fleet", withCORS(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sim.snapshot())
	}))
	mux.HandleFunc("GET /api/fleet/stream", withCORS(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming is unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		client := sim.subscribe()
		defer sim.unsubscribe(client)
		fmt.Fprint(w, ": connected\n\n")
		flusher.Flush()
		for {
			select {
			case <-r.Context().Done():
				return
			case payload := <-client:
				fmt.Fprintf(w, "data: %s\n\n", payload)
				flusher.Flush()
			}
		}
	}))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("fleet simulator listening on http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
