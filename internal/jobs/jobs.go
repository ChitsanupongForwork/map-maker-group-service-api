// Package jobs คืองานเบื้องหลังที่ไม่มีใครเรียกผ่าน HTTP
// ตรรกะอยู่ในฟังก์ชันของฐานข้อมูลทั้งหมด (migrations/001) ที่นี่แค่ตั้งเวลาเรียก
package jobs

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Run(ctx context.Context, pool *pgxpool.Pool) {
	offline := time.NewTicker(time.Minute)
	defer offline.Stop()
	prune := time.NewTicker(24 * time.Hour)
	defer prune.Stop()

	call(ctx, pool, "mark_stale_devices_offline", "marked %d vehicles offline")
	call(ctx, pool, "prune_position_events", "pruned %d old positions")

	for {
		select {
		case <-ctx.Done():
			return
		case <-offline.C:
			// ไม่มีอุปกรณ์ไหนบอกเองว่า "ฉันออฟไลน์แล้ว" ต้องคอยกวาดหา (สเปกหัวข้อ 6.8)
			call(ctx, pool, "mark_stale_devices_offline", "marked %d vehicles offline")
		case <-prune.C:
			call(ctx, pool, "prune_position_events", "pruned %d old positions")
		}
	}
}

// call เรียกฟังก์ชันในฐานข้อมูลที่คืนจำนวนแถวที่แตะ แล้ว log เฉพาะตอนมีอะไรเกิดขึ้นจริง
// ชื่อฟังก์ชันมาจากโค้ดในไฟล์นี้เท่านั้น ไม่ได้มาจาก input ของผู้ใช้
func call(ctx context.Context, pool *pgxpool.Pool, function, message string) {
	var count int
	if err := pool.QueryRow(ctx, "SELECT "+function+"()").Scan(&count); err != nil {
		if ctx.Err() == nil {
			log.Printf("jobs: %s: %v", function, err)
		}
		return
	}
	if count > 0 {
		log.Printf("jobs: "+message, count)
	}
}
