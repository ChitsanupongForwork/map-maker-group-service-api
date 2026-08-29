package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	databaseDemoFleetSize       = 1_000
	databaseSimulationBatchSize = 40
)

type databaseDemoVehicle struct {
	code, label, driver, phone, plate, make, model, origin, destination, status string
	lat, lng                                                                    float64
	speed, heading                                                              int
}

var databaseDemoFleet = []databaseDemoVehicle{
	{"FV-00001", "Bluebird 01", "Narin S.", "+66 81-240-1001", "1กข 4021", "Toyota", "Hilux Revo", "Rama IX hub", "Bang Na yard", "moving", 13.7563, 100.5018, 48, 72},
	{"FV-00002", "Bluebird 02", "Pimchanok K.", "+66 81-240-1002", "2ขค 5824", "Isuzu", "D-Max", "Lat Krabang hub", "Riverside depot", "stopped", 13.7275, 100.7403, 0, 148},
	{"FV-00003", "Bluebird 03", "Thanawat R.", "+66 81-240-1003", "3คง 8137", "Ford", "Ranger", "Chatuchak depot", "Rama IX hub", "moving", 13.8247, 100.5643, 66, 291},
	{"FV-00004", "Bluebird 04", "Mali C.", "+66 81-240-1004", "4งจ 9045", "Honda", "City Hatchback", "North depot", "Bang Na yard", "offline", 13.6844, 100.6127, 0, 214},
	{"FV-00005", "Bluebird 05", "Kittipong N.", "+66 81-240-1005", "5จฉ 1162", "Toyota", "Hiace", "Riverside depot", "Lat Krabang hub", "stopped", 13.7124, 100.4891, 0, 18},
}

func databaseDemoVehicleAt(index int) databaseDemoVehicle {
	if index < len(databaseDemoFleet) {
		return databaseDemoFleet[index]
	}

	drivers := []string{"Anan P.", "Suda W.", "Kriangsak T.", "Ploy N.", "Somsak C.", "Wanwisa R."}
	makes := []struct{ make, model string }{{"Toyota", "Hilux Revo"}, {"Isuzu", "D-Max"}, {"Ford", "Ranger"}, {"Mitsubishi", "Triton"}, {"Honda", "City Hatchback"}}
	places := []string{"Rama IX hub", "Bang Na yard", "Lat Krabang hub", "Riverside depot", "Chatuchak depot", "North depot"}
	statuses := []string{"moving", "moving", "moving", "stopped", "offline"}
	driver := drivers[index%len(drivers)]
	make := makes[index%len(makes)]
	status := statuses[index%len(statuses)]
	sequence := index + 1
	return databaseDemoVehicle{
		code:        fmt.Sprintf("FV-%05d", sequence),
		label:       fmt.Sprintf("Fleet vehicle %d", sequence),
		driver:      driver,
		phone:       fmt.Sprintf("+66 81-240-%04d", sequence),
		plate:       fmt.Sprintf("DEMO %04d", sequence),
		make:        make.make,
		model:       make.model,
		origin:      places[index%len(places)],
		destination: places[(index+2)%len(places)],
		status:      status,
		lat:         13.55 + float64(index%40)*0.01 + float64((index/40)%5)*0.001,
		lng:         100.35 + float64((index/40)%60)*0.01 + float64(index%5)*0.001,
		speed:       25 + (index*17)%78,
		heading:     (index * 37) % 360,
	}
}

type databaseRepository struct{ pool *pgxpool.Pool }

func newDatabaseRepository(ctx context.Context) (*databaseRepository, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	if config.ConnConfig.RuntimeParams == nil {
		config.ConnConfig.RuntimeParams = map[string]string{}
	}
	schema := os.Getenv("DB_SCHEMA")
	if schema == "" {
		schema = "map-maker-db"
	}
	config.ConnConfig.RuntimeParams["search_path"] = databaseQuoteIdentifier(schema) + ",public"
	config.MaxConns = 8
	config.MinConns = 1
	config.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &databaseRepository{pool: pool}, nil
}

func databaseQuoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func (r *databaseRepository) ensureDemoFleet(ctx context.Context) (bool, error) {
	var demoCount int
	if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM vehicles WHERE code LIKE 'FV-%'").Scan(&demoCount); err != nil {
		return false, err
	}
	if demoCount >= databaseDemoFleetSize {
		return false, nil
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var tenantID string
	if demoCount == 0 {
		if err := tx.QueryRow(ctx, `INSERT INTO tenants (name) VALUES ('Map Maker demo fleet') RETURNING id::text`).Scan(&tenantID); err != nil {
			return false, err
		}
	} else if err := tx.QueryRow(ctx, `SELECT tenant_id::text FROM vehicles WHERE code LIKE 'FV-%' ORDER BY code LIMIT 1`).Scan(&tenantID); err != nil {
		return false, err
	}

	for index := demoCount; index < databaseDemoFleetSize; index++ {
		item := databaseDemoVehicleAt(index)
		if err := r.insertDemoVehicle(ctx, tx, tenantID, index, item); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (r *databaseRepository) insertDemoVehicle(ctx context.Context, tx pgx.Tx, tenantID string, index int, item databaseDemoVehicle) error {
	var driverID, deviceID, vehicleID, eventID string
	if err := tx.QueryRow(ctx, `INSERT INTO drivers (tenant_id, display_name, phone, employee_code) VALUES ($1::uuid, $2, $3, $4) RETURNING id::text`, tenantID, item.driver, item.phone, fmt.Sprintf("DRV-%03d", index+1)).Scan(&driverID); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `INSERT INTO gps_devices (tenant_id, unique_id, protocol, model) VALUES ($1::uuid, $2, 'demo', 'Map Maker simulator') RETURNING id::text`, tenantID, fmt.Sprintf("demo-device-%03d", index+1)).Scan(&deviceID); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `INSERT INTO vehicles (tenant_id, code, label, license_plate, make, model, category) VALUES ($1::uuid, $2, $3, $4, $5, $6, 'fleet') RETURNING id::text`, tenantID, item.code, item.label, item.plate, item.make, item.model).Scan(&vehicleID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO vehicle_assignments (vehicle_id, driver_id, device_id, assigned_at) VALUES ($1::uuid, $2::uuid, $3::uuid, now())`, vehicleID, driverID, deviceID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO trips (tenant_id, vehicle_id, driver_id, status, origin_name, destination_name, started_at) VALUES ($1::uuid, $2::uuid, $3::uuid, 'active', $4, $5, now())`, tenantID, vehicleID, driverID, item.origin, item.destination); err != nil {
		return err
	}
	connection, operational, ignition := databaseStatesFor(item.status)
	if err := tx.QueryRow(ctx, `
		INSERT INTO position_events (received_at, tenant_id, vehicle_id, device_id, sequence_no, device_time, fix_time, valid, latitude, longitude, accuracy_m, speed_kph, course_deg, ignition, motion, satellites, hdop, attributes, raw_payload)
		VALUES (now(), $1::uuid, $2::uuid, $3::uuid, 1, now(), now(), true, $4, $5, 8, $6, $7, $8, $9, 12, 0.8, '{}'::jsonb, '{}'::jsonb)
		RETURNING event_id::text`, tenantID, vehicleID, deviceID, item.lat, item.lng, item.speed, item.heading, ignition, item.status == "moving").Scan(&eventID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO vehicle_live_states (vehicle_id, tenant_id, device_id, driver_id, latest_event_id, device_time, received_at, last_seen_at, valid, latitude, longitude, speed_kph, course_deg, ignition, motion, connection_status, operational_status)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5::uuid, now(), now(), now(), true, $6, $7, $8, $9, $10, $11, $12::connection_status, $13::operational_status)`, vehicleID, tenantID, deviceID, driverID, eventID, item.lat, item.lng, item.speed, item.heading, ignition, item.status == "moving", connection, operational)
	return err
}

func (r *databaseRepository) snapshot(ctx context.Context) ([]vehicle, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT v.id::text, v.tenant_id::text, COALESCE(ls.device_id::text, ''), COALESCE(ls.driver_id::text, ''), v.code, v.label,
			CASE WHEN ls.connection_status = 'offline' THEN 'offline' WHEN ls.operational_status = 'moving' THEN 'moving' ELSE 'stopped' END,
			COALESCE(ls.speed_kph, 0)::integer, COALESCE(ls.ignition, false), COALESCE(ls.course_deg, 0)::integer,
			COALESCE(d.display_name, 'Unassigned driver'), COALESCE(d.phone, '-'), v.license_plate, COALESCE(v.make, '-'), COALESCE(v.model, '-'),
			COALESCE(trip.origin_name, 'Unassigned'), COALESCE(trip.destination_name, 'Unassigned'), COALESCE(ls.latitude, 13.7563), COALESCE(ls.longitude, 100.5018),
			to_char(COALESCE(ls.last_seen_at, now()) AT TIME ZONE 'Asia/Bangkok', 'HH24:MI:SS')
		FROM vehicles v
		LEFT JOIN vehicle_live_states ls ON ls.vehicle_id = v.id
		LEFT JOIN drivers d ON d.id = ls.driver_id
		LEFT JOIN LATERAL (SELECT origin_name, destination_name FROM trips WHERE vehicle_id = v.id AND status = 'active' ORDER BY started_at DESC LIMIT 1) trip ON true
		WHERE v.active ORDER BY v.code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]vehicle, 0)
	for rows.Next() {
		var item vehicle
		if err := rows.Scan(&item.ID, &item.TenantID, &item.DeviceID, &item.DriverID, &item.Code, &item.Label, &item.Status, &item.SpeedKph, &item.AccOn, &item.HeadingDeg, &item.DriverName, &item.DriverPhone, &item.LicensePlate, &item.Make, &item.Model, &item.Origin, &item.Destination, &item.Lat, &item.Lng, &item.LastUpdate); err != nil {
			return nil, err
		}
		item.DriverInitials = databaseInitials(item.DriverName)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *databaseRepository) recordPosition(ctx context.Context, item vehicle) error {
	connection, operational, ignition := databaseStatesFor(item.Status)
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var eventID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO position_events (received_at, tenant_id, vehicle_id, device_id, sequence_no, device_time, fix_time, valid, latitude, longitude, accuracy_m, speed_kph, course_deg, ignition, motion, satellites, hdop, attributes, raw_payload)
		VALUES (now(), $1::uuid, $2::uuid, $3::uuid, extract(epoch FROM now())::bigint, now(), now(), true, $4, $5, 8, $6, $7, $8, $9, 12, 0.8, '{}'::jsonb, '{}'::jsonb)
		RETURNING event_id::text`, item.TenantID, item.ID, item.DeviceID, item.Lat, item.Lng, item.SpeedKph, item.HeadingDeg, ignition, item.Status == "moving").Scan(&eventID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE vehicle_live_states SET latest_event_id = $2::uuid, device_time = now(), received_at = now(), last_seen_at = now(), valid = true,
			latitude = $3, longitude = $4, speed_kph = $5, course_deg = $6, ignition = $7, motion = $8,
			connection_status = $9::connection_status, operational_status = $10::operational_status, version = version + 1, updated_at = now()
		WHERE vehicle_id = $1::uuid`, item.ID, eventID, item.Lat, item.Lng, item.SpeedKph, item.HeadingDeg, ignition, item.Status == "moving", connection, operational); err != nil {
		return err
	}
	payload, _ := json.Marshal(item)
	if _, err := tx.Exec(ctx, `INSERT INTO fleet_realtime_outbox (tenant_id, vehicle_id, event_type, event_id, payload) VALUES ($1::uuid, $2::uuid, 'vehicle.position.updated', $3::uuid, $4::jsonb)`, item.TenantID, item.ID, eventID, string(payload)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func databaseStatesFor(status string) (connection, operational string, ignition bool) {
	switch status {
	case "moving":
		return "online", "moving", true
	case "offline":
		return "offline", "unknown", false
	default:
		return "online", "idle", true
	}
}

func databaseInitials(name string) string {
	parts := strings.Fields(name)
	letters := make([]string, 0, 2)
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		letters = append(letters, strings.ToUpper(string([]rune(part)[0])))
		if len(letters) == 2 {
			break
		}
	}
	if len(letters) == 0 {
		return "--"
	}
	return strings.Join(letters, "")
}

type databaseSimulator struct {
	repository *databaseRepository
	mu         sync.RWMutex
	clients    map[chan []byte]struct{}
	random     *rand.Rand
}

func newDatabaseSimulator(repository *databaseRepository) *databaseSimulator {
	return &databaseSimulator{repository: repository, clients: make(map[chan []byte]struct{}), random: rand.New(rand.NewPCG(9042, 108))}
}

func (s *databaseSimulator) subscribe() chan []byte {
	client := make(chan []byte, 8)
	s.mu.Lock()
	s.clients[client] = struct{}{}
	s.mu.Unlock()
	return client
}

func (s *databaseSimulator) unsubscribe(client chan []byte) {
	s.mu.Lock()
	delete(s.clients, client)
	s.mu.Unlock()
}

func (s *databaseSimulator) publish(payload []byte) {
	s.mu.RLock()
	clients := make([]chan []byte, 0, len(s.clients))
	for client := range s.clients {
		clients = append(clients, client)
	}
	s.mu.RUnlock()
	for _, client := range clients {
		select {
		case client <- payload:
		default:
		}
	}
}

func (s *databaseSimulator) tick(ctx context.Context) {
	items, err := s.repository.snapshot(ctx)
	if err != nil {
		log.Printf("read fleet for update: %v", err)
		return
	}
	batchSize := min(databaseSimulationBatchSize, len(items))
	updates := make([]vehicle, 0, batchSize)
	for range batchSize {
		item := items[s.random.IntN(len(items))]
		s.advance(&item)
		if err := s.repository.recordPosition(ctx, item); err != nil {
			log.Printf("store position for %s: %v", item.Code, err)
			continue
		}
		updates = append(updates, item)
	}
	if len(updates) == 0 {
		return
	}
	payload, err := json.Marshal(fleetEvent{Type: "vehicle-updates", Vehicles: updates})
	if err == nil {
		s.publish(payload)
	}
}

func (s *databaseSimulator) advance(item *vehicle) {
	if item.Status == "offline" && s.random.IntN(5) == 0 {
		item.Status = "stopped"
	} else if item.Status == "stopped" && s.random.IntN(4) == 0 {
		item.Status = "moving"
	} else if item.Status == "moving" && s.random.IntN(12) == 0 {
		item.Status = "stopped"
	}
	if item.Status != "moving" {
		item.SpeedKph = 0
		item.LastUpdate = time.Now().Format("15:04:05")
		return
	}
	item.SpeedKph = 25 + s.random.IntN(78)
	item.HeadingDeg = (item.HeadingDeg + s.random.IntN(31) - 15 + 360) % 360
	radians := float64(item.HeadingDeg) * math.Pi / 180
	item.Lat += math.Cos(radians) * 0.00035
	item.Lng += math.Sin(radians) * 0.00035
	item.LastUpdate = time.Now().Format("15:04:05")
}

func (s *databaseSimulator) run(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func writeDatabaseJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func main() {
	if os.Getenv("DEMO_MODE") == "true" {
		log.Print("starting in demo mode; PostgreSQL is disabled")
		legacyInMemoryMain()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repository, err := newDatabaseRepository(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer repository.pool.Close()

	seeded, err := repository.ensureDemoFleet(ctx)
	if err != nil {
		log.Fatal("seed demo fleet: ", err)
	}
	if seeded {
		log.Printf("seeded %d demo vehicles", databaseDemoFleetSize)
	}
	sim := newDatabaseSimulator(repository)
	go sim.run(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", withCORS(func(w http.ResponseWriter, r *http.Request) {
		if err := repository.pool.Ping(r.Context()); err != nil {
			writeDatabaseJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "database unavailable"})
			return
		}
		writeDatabaseJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}))
	mux.HandleFunc("GET /api/fleet", withCORS(func(w http.ResponseWriter, r *http.Request) {
		items, err := repository.snapshot(r.Context())
		if err != nil {
			writeDatabaseJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not load fleet"})
			return
		}
		writeDatabaseJSON(w, http.StatusOK, items)
	}))
	mux.HandleFunc("POST /api/fleet/seed", withCORS(func(w http.ResponseWriter, r *http.Request) {
		seeded, err := repository.ensureDemoFleet(r.Context())
		if err != nil {
			writeDatabaseJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not seed fleet"})
			return
		}
		items, err := repository.snapshot(r.Context())
		if err != nil {
			writeDatabaseJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not load fleet"})
			return
		}
		writeDatabaseJSON(w, http.StatusOK, map[string]any{"seeded": seeded, "vehicles": items})
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
	log.Printf("fleet API listening on http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
