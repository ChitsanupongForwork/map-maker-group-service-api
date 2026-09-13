// Package postgres เปิด connection pool ไปยัง PostgreSQL
package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrSchemaNotFound = ต่อ database ได้ แต่ข้างในไม่มี schema ตามที่ตั้งไว้ใน DB_SCHEMA
var ErrSchemaNotFound = errors.New("schema not found")

// Open ต่อฐานข้อมูล แล้วเช็กสองอย่างก่อนคืน pool
//
//  1. ต่อได้จริง (ping) — รหัสผิด host ผิด หรือ Postgres ไม่ได้เปิด จะรู้ตั้งแต่ server เริ่ม
//  2. มี schema อยู่จริง — ถ้าไม่เช็ก search_path จะหล่นไปที่ public เงียบ ๆ
//     แล้วไปพังตอน request แรกด้วย "relation does not exist" ซึ่งไม่บอกว่าผิดที่ไหน
func Open(ctx context.Context, databaseURL, schema string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}

	// ตารางทั้งหมดอยู่ใน schema เดียว — ตั้ง search_path ที่นี่ที่เดียว
	// SQL ในฟีเจอร์จะได้เขียน FROM vehicles เฉย ๆ ไม่ต้องใส่ชื่อ schema ทุกบรรทัด
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = quoteIdentifier(schema) + ",public"
	cfg.MaxConns = 8
	cfg.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}

	// ฐานข้อมูลบน Render ที่หลับอยู่อาจตื่นช้า ให้เวลา 10 วินาที
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(checkCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect %s: %w", describe(cfg), err)
	}
	if err := checkSchema(checkCtx, pool, schema); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func checkSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	var database string
	var exists bool
	var available []string
	err := pool.QueryRow(ctx, `
SELECT current_database(),
       EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = $1),
       COALESCE(array_agg(nspname::text ORDER BY nspname)
                FILTER (WHERE nspname NOT LIKE 'pg\_%' AND nspname <> 'information_schema'), '{}')
FROM pg_namespace`, schema).Scan(&database, &exists, &available)
	if err != nil {
		return fmt.Errorf("check schema: %w", err)
	}
	if !exists {
		// บอกรายชื่อ schema ที่มีอยู่ไปด้วย — ส่วนใหญ่คือต่อผิด database หรือสะกดชื่อผิด
		return fmt.Errorf("%w: %q is not in database %q (schemas there: %s)",
			ErrSchemaNotFound, schema, database, strings.Join(available, ", "))
	}
	return nil
}

// Describe คืนปลายทางของ DATABASE_URL แบบไม่มีรหัสผ่าน เช่น postgres@127.0.0.1:5432/postgres
// พิมพ์ลง log หรือหน้าจอได้ปลอดภัย
func Describe(databaseURL string) string {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return "(DATABASE_URL could not be parsed)"
	}
	return describe(cfg)
}

func describe(cfg *pgxpool.Config) string {
	c := cfg.ConnConfig
	return fmt.Sprintf("%s@%s:%d/%s", c.User, c.Host, c.Port, c.Database)
}

// ชื่อ schema มีขีด (map-maker-db-new) ต้องห่อด้วย "..." ไม่งั้น Postgres อ่านเป็นเครื่องหมายลบ
func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
