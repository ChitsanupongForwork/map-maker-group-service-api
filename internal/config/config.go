// Package config อ่านค่าตั้งค่าทั้งหมดจาก environment — ที่อื่นห้ามเรียก os.Getenv เอง
package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port           string
	DatabaseURL    string
	DBSchema       string
	AllowedOrigins []string

	// DemoMode = ไม่ต่อฐานข้อมูล ใช้ข้อมูลจำลองในหน่วยความจำแทน
	// DATABASE_URL ว่างก็เข้าโหมดนี้เอง ยังไม่มีฐานข้อมูลก็ยังรัน server ได้
	DemoMode      bool
	DemoFleetSize int

	// Simulate = ให้ ingest.Simulator ขยับรถในฐานข้อมูลจริง ใช้ตอนมี DB แต่ยังไม่มีอุปกรณ์จริง
	Simulate bool
}

// รวมโดเมน preview ของ Vercel ไว้ด้วย ไม่งั้นทดสอบบน branch อื่นไม่ได้ (สเปกหัวข้อ 9)
const defaultOrigins = "http://localhost:3000,https://map-maker-group.vercel.app,https://map-maker-group-*.vercel.app"

// PowerShell บางเวอร์ชันเขียนไฟล์ .env พร้อม BOM ไว้หน้าบรรทัดแรก
var utf8BOM = string([]byte{0xEF, 0xBB, 0xBF})

func Load() Config {
	loadDotEnv(".env")

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	return Config{
		Port:           getenv("PORT", "8080"),
		DatabaseURL:    databaseURL,
		DBSchema:       getenv("DB_SCHEMA", "map-maker-db-new"),
		AllowedOrigins: splitList(getenv("FRONTEND_ORIGINS", defaultOrigins)),
		DemoMode:       databaseURL == "" || os.Getenv("DEMO_MODE") == "true",
		DemoFleetSize:  getInt("DEMO_FLEET_SIZE", 107),
		Simulate:       os.Getenv("SIMULATE") == "true",
	}
}

func getenv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func getInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func splitList(raw string) []string {
	items := make([]string, 0, 4)
	for _, item := range strings.Split(raw, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

// loadDotEnv อ่านไฟล์ .env ถ้ามี — ค่าที่ตั้งไว้ใน terminal แล้วชนะเสมอ
// ไม่ใช้ library เพราะต้องการแค่ KEY=VALUE บรรทัดละคู่
func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), utf8BOM))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}
}
