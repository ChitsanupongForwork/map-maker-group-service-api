package postgres

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// Hint แปล error ตอนต่อฐานข้อมูลเป็นวิธีแก้ที่อ่านรู้เรื่อง คืน "" ถ้าไม่รู้จัก error นี้
// error ของ pgx บอกว่าเกิดอะไร แต่ไม่บอกว่าต้องแก้ตรงไหนใน DATABASE_URL
func Hint(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrSchemaNotFound) {
		return "ต่อ database ได้แล้ว แต่ไม่มี schema ตาม DB_SCHEMA — เช็กว่า DATABASE_URL ชี้ไปที่ database เดียวกับที่รัน migration ลงไป และสะกด DB_SCHEMA ถูก"
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		message := strings.ToLower(pgErr.Message)
		switch {
		case pgErr.Code == "28P01":
			return "ชื่อผู้ใช้หรือรหัสผ่านใน DATABASE_URL ไม่ถูกต้อง"
		case pgErr.Code == "3D000":
			return "ไม่มี database ชื่อนี้ — ท้าย URL ต้องเป็นชื่อ database (เช่น /postgres) ไม่ใช่ชื่อ schema ส่วนชื่อ schema ให้ใส่ใน DB_SCHEMA"
		case pgErr.Code == "28000" && (strings.Contains(message, "ssl") || strings.Contains(message, "encryption")):
			return "server บังคับใช้ SSL — เปลี่ยนท้าย URL เป็น ?sslmode=require (ฐานข้อมูลบน Render ต้องใช้แบบนี้)"
		case pgErr.Code == "28000":
			return "server ไม่อนุญาตให้ user นี้ต่อจากเครื่องนี้ — ดู pg_hba.conf ของ server"
		case pgErr.Code == "42501":
			return "ต่อได้แต่ user นี้ไม่มีสิทธิ์ — GRANT สิทธิ์บน schema และตารางให้ user นี้"
		}
	}

	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "cannot parse"):
		return "อ่าน DATABASE_URL ไม่ออก — ถ้ารหัสผ่านมีอักขระ @ : / # ? ต้องแปลงก่อน เช่น @ เป็น %40, # เป็น %23"
	case strings.Contains(message, "refused tls") || strings.Contains(message, "does not support ssl"):
		return "server ไม่รองรับ SSL — Postgres ในเครื่องให้ใช้ ?sslmode=disable"
	case strings.Contains(message, "refused"):
		return "ไม่มีอะไรรอรับที่ host:port นี้ — Postgres ไม่ได้เปิดอยู่ หรือพอร์ตผิด (ในเครื่องปกติคือ 5432)"
	case strings.Contains(message, "no such host"):
		return "หา host ไม่เจอ — สะกดชื่อ host ผิด หรือไม่ได้ต่ออินเทอร์เน็ต"
	case strings.Contains(message, "timeout") || strings.Contains(message, "deadline exceeded"):
		return "ต่อไม่ทันเวลา — host หรือพอร์ตผิด, firewall บล็อก หรือฐานข้อมูลบน Render ถูกพักไว้"
	}
	return ""
}
