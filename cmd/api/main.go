package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fleet-monitor-server/internal/config"
	"fleet-monitor-server/internal/fleet"
	"fleet-monitor-server/internal/history"
	"fleet-monitor-server/internal/ingest"
	"fleet-monitor-server/internal/jobs"
	"fleet-monitor-server/internal/platform/httpx"
	"fleet-monitor-server/internal/platform/postgres"
)

func main() {
	// ctx ถูกยกเลิกเมื่อกด Ctrl+C หรือ Render ส่ง SIGTERM ตอน deploy ใหม่
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()
	mux := http.NewServeMux()

	var fleetStore fleet.Store
	var historyStore history.Store
	var ping func(context.Context) error
	mode := "postgres"

	// จุดเดียวที่รู้ว่าของจริงคืออะไร — ต่างกันแค่ตัว store ส่วน HTTP ข้างล่างมีชุดเดียว
	if cfg.DemoMode {
		mode = "demo"
		if cfg.DatabaseURL == "" {
			log.Print("DATABASE_URL is empty; running in demo mode")
		}
		fleetStore = fleet.NewMemStore(cfg.DemoFleetSize)
		historyStore = history.NewMemStore(cfg.DemoFleetSize)
		mux.HandleFunc("POST /api/positions", func(w http.ResponseWriter, _ *http.Request) {
			httpx.WriteError(w, http.StatusServiceUnavailable,
				"POST /api/positions needs a database: set DATABASE_URL and restart")
		})
	} else {
		pool, err := postgres.Open(ctx, cfg.DatabaseURL, cfg.DBSchema)
		if err != nil {
			if hint := postgres.Hint(err); hint != "" {
				log.Fatalf("%v\nวิธีแก้: %s\nตรวจทีละขั้นด้วย: go run ./cmd/dbcheck", err, hint)
			}
			log.Fatal(err)
		}
		defer pool.Close()
		ping = pool.Ping
		log.Printf("connected to %s (schema %s)", postgres.Describe(cfg.DatabaseURL), cfg.DBSchema)

		fleetStore = fleet.NewPostgresStore(pool)
		historyStore = history.NewPostgresStore(pool)

		ingestStore := ingest.NewStore(pool)
		ingest.NewHandler(ingestStore).Register(mux)
		if cfg.Simulate {
			go ingest.NewSimulator(ingestStore).Run(ctx)
		}
		go jobs.Run(ctx, pool)
	}

	hub := fleet.NewHub(fleetStore)
	go hub.Run(ctx)

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if ping != nil {
			if err := ping(r.Context()); err != nil {
				httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "database unavailable", "mode": mode})
				return
			}
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "mode": mode})
	})
	fleet.NewHandler(fleetStore, hub).Register(mux)
	history.NewHandler(historyStore).Register(mux)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpx.CORS(cfg.AllowedOrigins, mux),
		ReadHeaderTimeout: 10 * time.Second,
		// ไม่ตั้ง WriteTimeout เพราะ /api/fleet/stream ต้องเปิดค้างได้นาน
		// r.Context() ของทุก request สืบจาก ctx นี้ พอกด Ctrl+C สาย SSE จะปิดตามทันที
		BaseContext: func(net.Listener) context.Context { return ctx },
	}

	serverErr := make(chan error, 1)
	go func() { serverErr <- server.ListenAndServe() }()
	log.Printf("listening on http://localhost:%s (mode=%s, simulate=%v)", cfg.Port, mode, cfg.Simulate && !cfg.DemoMode)

	select {
	case err := <-serverErr:
		log.Fatal(err) // เช่น พอร์ตถูกใช้อยู่แล้ว
	case <-ctx.Done():
	}

	log.Print("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
