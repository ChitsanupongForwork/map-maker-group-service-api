// Command dbcheck ตรวจการต่อฐานข้อมูลทีละขั้น ด้วย .env ชุดเดียวกับ server
//
//	go run ./cmd/dbcheck
//
// อ่านอย่างเดียว ไม่เขียนอะไรลงฐานข้อมูล
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"fleet-monitor-server/internal/config"
	"fleet-monitor-server/internal/platform/postgres"
)

// ของที่ server ต้องใช้ ทั้งหมดสร้างโดย migrations/001_map_history_core.sql
var (
	requiredRelations = []string{"vehicle_groups", "areas", "drivers", "vehicles", "position_events", "vehicle_live_state", "fleet_snapshot"}
	requiredFunctions = []string{"mark_stale_devices_offline", "prune_position_events"}
	requiredGrants    = []struct{ relation, privilege, usedBy string }{
		{"fleet_snapshot", "SELECT", "GET /api/fleet"},
		{"position_events", "SELECT", "GET /api/vehicles/{id}/history"},
		{"position_events", "INSERT", "POST /api/positions"},
		{"vehicle_live_state", "UPDATE", "POST /api/positions และงานตั้ง offline"},
		{"position_events", "DELETE", "งานลบข้อมูลเก่า"},
	}
)

func main() {
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		fail("DATABASE_URL ยังไม่ได้ตั้ง", "คัดลอก .env.example เป็น .env แล้วแก้ DATABASE_URL เป็นของจริง")
	}
	fmt.Printf("ปลายทาง  %s\nschema    %s\n\n", postgres.Describe(cfg.DatabaseURL), cfg.DBSchema)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// ขั้น 1–2: ต่อได้ และมี schema — postgres.Open เช็กให้ทั้งสองอย่าง
	pool, err := postgres.Open(ctx, cfg.DatabaseURL, cfg.DBSchema)
	if err != nil {
		fail(err.Error(), postgres.Hint(err))
	}
	defer pool.Close()

	var database, user, version string
	if err := pool.QueryRow(ctx, "SELECT current_database(), current_user, current_setting('server_version')").
		Scan(&database, &user, &version); err != nil {
		fail(err.Error(), postgres.Hint(err))
	}
	pass("ต่อได้ — database=%s user=%s PostgreSQL %s", database, user, version)
	pass("มี schema %q", cfg.DBSchema)

	problems := checkObjects(ctx, pool, cfg.DBSchema)
	if problems == 0 {
		problems += checkData(ctx, pool)
	}

	fmt.Println()
	if problems > 0 {
		fmt.Printf("เจอ %d เรื่องที่ต้องแก้ก่อนรัน server\n", problems)
		os.Exit(1)
	}
	fmt.Println("พร้อมแล้ว — รัน server ด้วย: go run ./cmd/api")
}

// checkObjects ขั้น 3–4: ตาราง view function และสิทธิ์ที่ server ใช้ครบไหม
func checkObjects(ctx context.Context, pool *pgxpool.Pool, schema string) (problems int) {
	var usage bool
	if err := pool.QueryRow(ctx, "SELECT has_schema_privilege($1::text, 'USAGE')", schema).Scan(&usage); err != nil {
		fail(err.Error(), postgres.Hint(err))
	}
	if !usage {
		warn("user นี้ไม่มีสิทธิ์ USAGE บน schema — GRANT USAGE ON SCHEMA \"%s\" TO <user>;", schema)
		return 1
	}

	found := map[string]bool{}
	for _, name := range requiredRelations {
		// ระบุ schema ตรง ๆ กันเจอตารางชื่อเดียวกันใน public แล้วเข้าใจผิดว่าครบ
		var exists bool
		if err := pool.QueryRow(ctx, "SELECT to_regclass(format('%I.%I', $1::text, $2::text)) IS NOT NULL", schema, name).
			Scan(&exists); err != nil {
			fail(err.Error(), postgres.Hint(err))
		}
		found[name] = exists
	}
	for _, name := range requiredFunctions {
		var exists bool
		if err := pool.QueryRow(ctx, `
SELECT EXISTS (SELECT 1 FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
               WHERE n.nspname = $1 AND p.proname = $2)`, schema, name).Scan(&exists); err != nil {
			fail(err.Error(), postgres.Hint(err))
		}
		found[name] = exists
	}

	missing := 0
	for _, name := range append(requiredRelations, requiredFunctions...) {
		if !found[name] {
			missing++
			warn("ไม่มี %s", name)
		}
	}
	if missing > 0 {
		warn("รัน migrations/001_map_history_core.sql ลงใน database นี้ก่อน")
		return missing
	}
	pass("ตาราง view และ function ครบ (%d ตาราง/view, %d function)", len(requiredRelations), len(requiredFunctions))

	for _, grant := range requiredGrants {
		var ok bool
		if err := pool.QueryRow(ctx, "SELECT has_table_privilege(format('%I.%I', $1::text, $2::text), $3::text)",
			schema, grant.relation, grant.privilege).Scan(&ok); err != nil {
			fail(err.Error(), postgres.Hint(err))
		}
		if !ok {
			problems++
			warn("ไม่มีสิทธิ์ %s บน %s (ใช้กับ %s)", grant.privilege, grant.relation, grant.usedBy)
		}
	}
	if problems == 0 {
		pass("สิทธิ์ครบสำหรับทุก endpoint")
	}
	return problems
}

// checkData ขั้น 5: มีข้อมูลให้หน้าเว็บแสดงหรือยัง — ไม่มีก็ไม่นับเป็นปัญหา แค่เตือน
func checkData(ctx context.Context, pool *pgxpool.Pool) (problems int) {
	var vehicles, liveStates, onMap, positions int64
	if err := pool.QueryRow(ctx, `
SELECT (SELECT count(*) FROM vehicles),
       (SELECT count(*) FROM vehicle_live_state),
       (SELECT count(*) FROM fleet_snapshot),
       (SELECT count(*) FROM position_events)`).Scan(&vehicles, &liveStates, &onMap, &positions); err != nil {
		fail(err.Error(), postgres.Hint(err))
	}
	pass("ข้อมูล — vehicles=%d live_state=%d บนแผนที่=%d position_events=%d", vehicles, liveStates, onMap, positions)

	if onMap == 0 {
		warn("หน้า /map จะว่าง — ใส่ข้อมูลด้วย migrations/002_seed_demo.sql หรือ 003_seed_fleet_1000.sql")
	} else if vehicles > onMap {
		warn("รถ %d คันยังไม่มีสถานะล่าสุด จะไม่โผล่บนแผนที่จนกว่าจะส่งตำแหน่งเข้ามา", vehicles-onMap)
	}
	if positions == 0 {
		warn("position_events ว่าง — หน้า /history จะไม่มีเส้นทางให้ดู")
	}
	return 0
}

func pass(format string, args ...any) { fmt.Printf("[ok] "+format+"\n", args...) }
func warn(format string, args ...any) { fmt.Printf("[!!] "+format+"\n", args...) }

func fail(message, hint string) {
	fmt.Printf("[xx] %s\n", message)
	if hint != "" {
		fmt.Printf("     วิธีแก้: %s\n", hint)
	}
	os.Exit(1)
}
