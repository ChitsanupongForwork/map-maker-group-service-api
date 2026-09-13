# โครงสร้างโปรเจกต์ Go

> คู่มือเลือกโครงสร้าง + ผลประเมินสำหรับ `map-maker-group-service-api`
> โดยวางคู่กับ front-end `map-maker-group` ที่เป็นคนเรียก API นี้

---

## สรุปก่อน

| คำถาม | คำตอบ |
|---|---|
| โปรเจกต์นี้ควรใช้โครงสร้างไหน | **ระดับ 4 แบบเบา** — `cmd/api` + `internal/` จัด package ตามฟีเจอร์ ไฟล์แบนในแต่ละฟีเจอร์ |
| คะแนน | 36 / 40 ชนะอันดับสอง (ระดับ 3) อยู่ 9 คะแนน — ดูหัวข้อ 10 |
| ฟีเจอร์ฝั่ง backend มีกี่ตัว | 3 ตัว: `fleet`, `history`, `ingest` (+ `jobs` งานเบื้องหลัง) |
| ต้องทำเลยไหม | **ไม่** ทำตอนเริ่มขั้นที่ 3 ของ `API-REQUIREMENTS.md` หัวข้อ 10 (ตอนเขียน `GET /api/fleet`) |
| Hexagonal ล่ะ | ไม่คุ้ม ตรรกะส่วนใหญ่อยู่ใน SQL แล้ว ดูหัวข้อ 10 |
| กฎที่ห้ามพลาด | ฟีเจอร์ **ห้าม import กันเอง** — คุยกันผ่านฐานข้อมูลเท่านั้น |

---

## สารบัญ

**ภาค 1 — คู่มือ**
1. [กฎ 3 ข้อที่ Go บังคับ](#1-กฎ-3-ข้อที่-go-บังคับ)
2. [โครงสร้าง 5 ระดับ](#2-โครงสร้าง-5-ระดับ)
3. [ต่างกันที่ลูกศรเส้นเดียว](#3-ต่างกันที่ลูกศรเส้นเดียว)
4. [7 กฎที่สำคัญกว่าชื่อโฟลเดอร์](#4-7-กฎที่สำคัญกว่าชื่อโฟลเดอร์)
5. [กับดักที่เจอบ่อย](#5-กับดักที่เจอบ่อย)

**ภาค 2 — วางคู่กับ map-maker-group**

6. [front-end จัดโครงสร้างไว้แบบไหน](#6-front-end-จัดโครงสร้างไว้แบบไหน)
7. [แปลกติกา front-end เป็นภาษา Go](#7-แปลกติกา-front-end-เป็นภาษา-go)
8. [ฟีเจอร์ front-end 8 ตัว → backend 3 ตัว](#8-ฟีเจอร์-front-end-8-ตัว--backend-3-ตัว)

**ภาค 3 — ผลประเมิน**

9. [ข้อเท็จจริงที่ใช้ประเมิน](#9-ข้อเท็จจริงที่ใช้ประเมิน)
10. [ตารางคะแนน](#10-ตารางคะแนน)
11. [โครงสร้างเป้าหมาย](#11-โครงสร้างเป้าหมาย)
12. [กฎ import](#12-กฎ-import)
13. [โค้ดตัวอย่างจุดสำคัญ](#13-โค้ดตัวอย่างจุดสำคัญ)
14. [ทำทีละขั้นตาม API-REQUIREMENTS](#14-ทำทีละขั้นตาม-api-requirements)
15. [สิ่งที่ต้องแก้ตามเมื่อย้ายโครงสร้าง](#15-สิ่งที่ต้องแก้ตามเมื่อย้ายโครงสร้าง)
16. [เมื่อไหร่ต้องประเมินใหม่](#16-เมื่อไหร่ต้องประเมินใหม่)

---

# ภาค 1 — คู่มือ

## 1. กฎ 3 ข้อที่ Go บังคับ

โครงสร้างทุกแบบในเอกสารนี้เกิดจากการรับมือกับ 3 ข้อนี้ เข้าใจ 3 ข้อนี้ก่อน แล้วเรื่องโฟลเดอร์จะเป็นเรื่องง่าย

### 1.1 หนึ่งโฟลเดอร์ = หนึ่ง package

ไฟล์ทุกไฟล์ในโฟลเดอร์เดียวกันอยู่ใน package เดียวกัน และ**มองเห็นกันหมด** แม้แต่ชื่อที่ขึ้นต้นด้วยตัวพิมพ์เล็ก

> **แตกไฟล์ไม่ได้สร้างขอบเขตอะไรเลย แตกโฟลเดอร์ต่างหากที่สร้าง**

### 1.2 import วนกลับไม่ได้ — เป็น compile error

```text
package fleet   import "history"
package history import "fleet"
→ import cycle not allowed   (build ไม่ผ่าน)
```

ไม่ใช่คำเตือน build ไม่ผ่านจริง ๆ ข้อนี้ทำให้ต้องคิดเรื่อง **ทิศทางของการพึ่งพา** ตั้งแต่วันแรก

### 1.3 `internal/` คือ private ที่คอมไพเลอร์บังคับ

โค้ดใต้ `internal/` ถูก import ได้เฉพาะจากโค้ดที่อยู่ใต้โฟลเดอร์แม่ของ `internal/` เท่านั้น
โปรเจกต์อื่นดึงไปใช้ไม่ได้ เป็นกลไก encapsulation ระดับ package อันเดียวที่ Go มี และได้มาฟรี

> **ข้อควรรู้:** repo `golang-standards/project-layout` **ไม่ใช่มาตรฐานทางการ** ของทีม Go
> เป็นของที่ชุมชนรวบรวมไว้ และโฟลเดอร์ `pkg/` ในนั้นก็ยังเถียงกันไม่จบ อย่าลอกทั้งก้อน

---

## 2. โครงสร้าง 5 ระดับ

เลข 1–5 **ไม่ใช่คะแนนความดี** แต่เป็นลำดับที่โค้ดเบสโตขึ้นจริง
โปรเจกต์ส่วนใหญ่เริ่มที่ 1 แล้วค่อยขยับตามปัญหาที่เจอจริง

### ระดับ 1 — แบน ไฟล์เดียว package เดียว

```text
fleet-api/
├── go.mod
├── main.go          // HTTP + routing + startup
└── db.go            // SQL + type ข้อมูล
```

| ดีตรงไหน | พังตอนไหน |
|---|---|
| ไม่มีทางเกิด import cycle | ไม่มีใครตอบได้ว่าแก้ตรงนี้แล้วกระทบอะไรบ้าง |
| ย้ายฟังก์ชันไปมาไม่ต้องแก้ import | เทสยาก handler เรียก DB ตรง ๆ |
| `go run .` จบ | มีสองโหมด (จริง/จำลอง) ต้องก๊อปโค้ดทั้งชุด |

**ใช้เมื่อ:** CLI เล็ก ๆ, prototype, service ไม่เกิน ~1,500 บรรทัดที่รู้ว่าจะไม่โต

### ระดับ 2 — แบน แต่แยกไฟล์ตามหน้าที่

```text
fleet-api/
├── main.go          // อ่าน env + ประกอบของ
├── handler.go       // request → response เท่านั้น
├── service.go       // กฎธุรกิจ ไม่รู้จัก HTTP ไม่รู้จัก SQL
├── store.go         // SQL เท่านั้น
└── vehicle.go       // type ของโดเมน
```

| ดีตรงไหน | พังตอนไหน |
|---|---|
| ได้ประโยชน์ส่วนใหญ่ของการแบ่งชั้นโดยไม่ต้องสู้กับ import | ขอบเขตอยู่ในหัวคนเขียน คอมไพเลอร์ไม่ช่วยบังคับ |
| วันที่จะขยับขึ้นไป แค่ลากไฟล์เข้าโฟลเดอร์ | มีคนที่สองเข้ามา วินัยนี้หายเร็วมาก |

**ใช้เมื่อ:** ทำคนเดียว ~1,000–3,000 บรรทัด **ขั้นที่เหมาะที่สุดสำหรับฝึกคิดเรื่องแบ่งหน้าที่**

### ระดับ 3 — แบ่งชั้นตามชนิดของโค้ด

```text
fleet-api/
├── cmd/api/main.go
└── internal/
    ├── config/
    ├── http/            // handler ของทุกฟีเจอร์รวมกันที่นี่
    ├── service/         // กฎธุรกิจของทุกฟีเจอร์
    └── storage/         // SQL ของทุกฟีเจอร์
```

| ดีตรงไหน | พังตอนไหน |
|---|---|
| handler ยิง SQL ตรง ๆ ไม่ได้แล้ว เพราะไม่ได้ import | ฟีเจอร์เยอะขึ้น `http/` กลายเป็นกองไฟล์ 40 อัน |
| เทส service ได้โดยไม่มี DB | แก้ฟีเจอร์เดียวต้องเปิด 3 โฟลเดอร์ |
| คนใหม่เดาถูกว่าโค้ดอยู่ไหน | ชวนให้สร้าง `models/` ที่ทุกอย่าง import |

**ใช้เมื่อ:** REST service ที่มีโดเมนเดียว ~3,000–20,000 บรรทัด

### ระดับ 4 — แบ่งตามฟีเจอร์

```text
fleet-api/
├── cmd/api/main.go
└── internal/
    ├── fleet/           // ฟีเจอร์ครบทุกชั้นในโฟลเดอร์เดียว
    │   ├── handler.go
    │   ├── store.go
    │   └── vehicle.go
    ├── history/
    └── platform/        // ของกลางที่จำเป็นจริง ๆ เท่านั้น
```

| ดีตรงไหน | พังตอนไหน |
|---|---|
| แก้ฟีเจอร์เดียวจบในโฟลเดอร์เดียว | ฟีเจอร์ A อยากได้ของ B และ B อยากได้ของ A → import cycle |
| ลบฟีเจอร์ได้จริงเพราะอยู่ที่เดียว | `platform/` โตจนกลายเป็น `utils/` ได้ง่าย |
| ยกฟีเจอร์ออกไปเป็น service แยกได้ง่าย | ต้องมีกฎว่าฟีเจอร์คุยกันทางไหน |

**ใช้เมื่อ:** มีหลายเรื่องที่แยกจากกันชัด
**แบบเบา:** ถ้าฟีเจอร์ยังน้อย ให้ไฟล์ในแต่ละฟีเจอร์แบนไว้ ไม่ต้องมีโฟลเดอร์ย่อยข้างใน

### ระดับ 5 — Hexagonal / Ports & Adapters

```text
internal/
├── domain/fleet/        // ไม่ import อะไรนอกจาก stdlib
│   ├── vehicle.go
│   ├── service.go
│   └── ports.go         // interface ที่โดเมนต้องการ
├── adapters/
│   ├── postgres/        // ทำตาม port
│   ├── memory/          // ทำตาม port เดียวกัน
│   └── httpapi/
└── app/wire.go
```

| ดีตรงไหน | พังตอนไหน |
|---|---|
| เปลี่ยน infra ได้โดยไม่แตะโดเมน | CRUD ธรรมดากลายเป็น 4 ไฟล์ 3 interface |
| เทสโดเมนได้เร็วมาก | ตามโค้ดยาก ทุกการเรียกผ่าน interface |
| กฎธุรกิจอ่านออกโดยไม่รู้จัก framework | โดเมนบาง ๆ จะไม่ได้อะไรกลับมาเลย |

**ใช้เมื่อ:** กฎธุรกิจซับซ้อนกว่าการอ่านเขียนฐานข้อมูลจริง ๆ — **อย่าเริ่มที่นี่**

### แล้ว microservices ล่ะ

คนละแกนกัน ห้าระดับข้างบนคือ **จัดโค้ดยังไงใน repo เดียว**
monolith กับ microservices คือ **deploy เป็นกี่ก้อน**
ระดับ 4 ที่ทำดี ๆ แยกเป็น microservices ได้ง่าย ส่วนระดับ 1 ที่แตกเป็น microservices จะได้ปัญหาเดิมคูณจำนวน service

---

## 3. ต่างกันที่ลูกศรเส้นเดียว

ระดับ 3 กับระดับ 5 มีกล่องเหมือนกัน ต่างกันแค่ **ใครเป็นเจ้าของ interface** และลูกศรล่างสุดชี้ไปทางไหน

```text
  ระดับ 3 — แบ่งชั้น                 ระดับ 5 — Hexagonal

  ┌──────────┐                       ┌──────────┐
  │ handler  │                       │ handler  │
  └────┬─────┘                       └────┬─────┘
       │ เรียก                             │ เรียก
       ▼                                  ▼
  ┌──────────┐                       ┌──────────────────────┐
  │ service  │                       │ service              │
  └────┬─────┘                       │ ┌ type Store ──────┐ │
       │ import แล้วเรียกตรง ๆ          │ └──────────────────┘ │
       ▼                             └──────────▲───────────┘
  ┌──────────┐                                  │ ทำตามสัญญา
  │ postgres │                       ┌──────────┴───────────┐
  └──────────┘                       │ postgres             │
                                     └──────────────────────┘
```

- **ซ้าย:** service ต้อง import postgres → เทส service ไม่ได้ถ้าไม่มีฐานข้อมูล
- **ขวา:** service ประกาศ interface เอง ไม่รู้จัก postgres เลย → ใส่ตัวปลอมตอนเทสได้

> ข้อดีของ Go: ได้ลูกศรแบบขวาโดย **ไม่ต้องย้ายไปใช้โครงสร้างระดับ 5 ทั้ง repo**
> แค่ประกาศ interface ในไฟล์ของคนที่เรียกใช้ (กฎข้อ 4.1)

---

## 4. 7 กฎที่สำคัญกว่าชื่อโฟลเดอร์

โฟลเดอร์สวยแต่ทำกฎพวกนี้ผิด ได้โค้ดที่แย่กว่าไฟล์เดียวที่ทำถูก

### 4.1 ประกาศ interface ฝั่งคนใช้ ไม่ใช่ฝั่งคนทำ

ต่างจาก Java / C# มากที่สุด ฝั่ง postgres **ไม่ต้องประกาศ** ว่าทำตาม interface ไหน มี method ครบก็ถือว่าทำตามแล้ว

```go
// internal/fleet/store.go — คนใช้เขียนสัญญา และเขียนแค่ที่ตัวเองใช้
type Store interface {
    Snapshot(ctx context.Context) (Snapshot, error)
}
```

### 4.2 รับ interface, คืน struct

```go
func NewHandler(store Store) *Handler   // รับ interface → สลับของได้
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore   // คืน struct จริง
```

### 4.3 ประกอบของทั้งหมดใน `main()`

ไม่ต้องใช้ DI framework ให้ `main()` เป็นที่เดียวที่รู้ว่าของจริงคืออะไร ที่เหลือรับผ่าน constructor

### 4.4 `context.Context` เป็นพารามิเตอร์ตัวแรกเสมอ

ทุกฟังก์ชันที่ทำ I/O รับ `ctx` ตัวแรกแล้วส่งต่อลงไป
สำคัญมากกับ SSE — ผู้ใช้ปิดแท็บเมื่อไหร่ `r.Context()` จะถูกยกเลิก และเราต้องเลิกส่งทันที
**อย่าเก็บ `ctx` ไว้ใน struct**

### 4.5 ห่อ error ด้วย `%w` อย่า log ทุกชั้น

```go
if err != nil {
    return fmt.Errorf("fleet snapshot: %w", err)   // ห่อแล้วส่งขึ้น
}
```

ชั้นบนสุด (handler) เป็นคนตัดสินใจว่าจะ log และตอบ status อะไร

### 4.6 ชื่อ package คือส่วนหนึ่งของชื่อ

คนเรียกเห็น `fleet.Snapshot` ไม่ใช่ `Snapshot` เฉย ๆ
ดังนั้น `fleet.FleetSnapshot` คือชื่อซ้ำ ใช้ `fleet.Snapshot` พอ
ชื่อ package สั้น เอกพจน์ ตัวเล็กล้วน ไม่มี `_` ไม่มี camelCase

### 4.7 วางเทสไว้ข้างโค้ด

`store_test.go` อยู่โฟลเดอร์เดียวกับ `store.go` ไม่ต้องมีโฟลเดอร์ `tests/`
ตั้ง package เป็น `fleet_test` ถ้าอยากบังคับให้เทสผ่าน API สาธารณะเท่านั้น

---

## 5. กับดักที่เจอบ่อย

| กับดัก | ทำไมพัง | ทำแทน |
|---|---|---|
| package ชื่อ `utils`, `common`, `helpers` | ชื่อไม่บอกว่าข้างในทำอะไร ทุกคนโยนของลงไปจนทุก package import มัน | ตั้งชื่อตามงาน เช่น `httpx`, `retry` |
| package `models` เก็บ struct ทุกตัว | ทุกส่วนผูกกันผ่านก้อนกลาง | type อยู่กับฟีเจอร์ที่เป็นเจ้าของ |
| สร้าง interface ทั้งที่มีตัวทำตัวเดียว | ความซับซ้อนฟรี ๆ | เขียน struct ก่อน ดึง interface ออกเมื่อมีตัวที่สอง |
| ลอกโครงสร้าง Kubernetes มาใส่ service 4 endpoint | แก้ปัญหาที่ยังไม่มี | เริ่มเล็ก ขยับตามปัญหาจริง |
| `main()` คนละชุดสำหรับแต่ละโหมด | โค้ด HTTP ซ้ำสองชุดแล้วค่อย ๆ เพี้ยนจากกัน | `main()` เดียว สลับเฉพาะชิ้นที่ต่างกันจริง |

> ข้อสุดท้ายเกิดขึ้นจริงในโค้ดเวอร์ชันก่อนของ repo นี้ — `main()` เช็ก `DEMO_MODE` แล้วโดดไป
> `legacyInMemoryMain()` ที่ตั้ง router และเขียน SSE ซ้ำอีกชุด (ดู `git show HEAD:db_service.go`)

---

# ภาค 2 — วางคู่กับ map-maker-group

## 6. front-end จัดโครงสร้างไว้แบบไหน

`map-maker-group` เป็น Next.js 16 ที่จัดแบบ **ระดับ 4 — แบ่งตามฟีเจอร์** และบังคับกฎด้วย `eslint-plugin-boundaries`

```text
src/
├── app/            // เส้นทางของหน้า — ประกอบฟีเจอร์เข้าด้วยกัน
├── features/       // 8 ฟีเจอร์ รวม ~4,700 บรรทัด
│   ├── fleet/              13 ไฟล์   ~1,180 บรรทัด
│   ├── fleet-map/          10 ไฟล์     ~980 บรรทัด
│   ├── vehicle-history/     8 ไฟล์     ~970 บรรทัด
│   ├── fleet-export/        9 ไฟล์     ~580 บรรทัด
│   ├── vehicle-detail/      5 ไฟล์     ~390 บรรทัด
│   ├── vehicle-list/        4 ไฟล์     ~260 บรรทัด
│   ├── app-shell/           5 ไฟล์     ~220 บรรทัด
│   └── launchpad/           3 ไฟล์     ~140 บรรทัด
├── shared/         // ui, hooks, lib ที่ไม่รู้จักฟีเจอร์ไหน
├── server/         // ของฝั่งเซิร์ฟเวอร์ที่ไม่ผูกฟีเจอร์
└── config/
```

กฎที่ `eslint.config.mjs` บังคับ:

| จาก | import ได้ |
|---|---|
| `app` | feature, shared, server, config |
| `feature` | feature (ผ่าน index เท่านั้น), shared, server, config |
| `shared` | shared, config |
| `server` | server, config |
| `config` | config |

และ **ประตูของแต่ละฟีเจอร์มีแค่สองบาน**: `@/features/<name>` กับ `@/features/<name>/server`
ห้ามเจาะเข้าไปถึงไฟล์ข้างใน

> คอมเมนต์บนสุดของ `eslint.config.mjs` เขียนไว้ว่า *"กฎที่บังคับด้วยเครื่องมือไม่ได้ = กฎที่ไม่มีอยู่จริง"*
> ข่าวดีคือฝั่ง Go ได้เครื่องมือนี้มาจากคอมไพเลอร์ ไม่ต้องติดตั้งอะไรเพิ่ม — ดูหัวข้อถัดไป

---

## 7. แปลกติกา front-end เป็นภาษา Go

แนวคิดเดียวกันเกือบทั้งหมด ต่างกันที่ **TypeScript ต้องพึ่ง ESLint แต่ Go ได้จากคอมไพเลอร์**

| front-end (`map-maker-group`) | backend Go | ใครบังคับ |
|---|---|---|
| `src/features/<name>/` | `internal/<name>/` = หนึ่ง package | คอมไพเลอร์ |
| `index.ts` — ประตูเดียวของฟีเจอร์ | ชื่อขึ้นต้น**ตัวพิมพ์ใหญ่** = ออกนอก package ได้ ตัวเล็ก = อยู่ข้างในเท่านั้น | คอมไพเลอร์ |
| `no-restricted-imports` ห้ามเจาะไส้ใน | ไฟล์แบนใน package เดียว ไม่มีไส้ในให้เจาะ | ไม่ต้องบังคับ |
| `boundaries` `default: "disallow"` | `internal/` + import cycle เป็น compile error | คอมไพเลอร์ |
| `server/index.ts` แยกเพราะ `server-only` | ไม่ต้องมี backend ทั้งก้อนคือฝั่งเซิร์ฟเวอร์ | — |
| `src/shared/` | `internal/platform/` แต่ต้องตั้งชื่อ package ตามงาน (`httpx`, `postgres`) | code review |
| `src/server/` | `internal/platform/postgres/` | code review |
| `src/config/` | `internal/config/` | — |
| `src/app/` — ประกอบฟีเจอร์เป็นหน้า | `cmd/api/main.go` — ประกอบฟีเจอร์เป็น server | — |
| `types.ts` | `vehicle.go` — struct + `json:"..."` tag | เทส |
| `lib/status.ts` ตรรกะล้วน | ฟังก์ชันล้วน + `_test.go` ข้าง ๆ | เทส |
| `lib/mock-fleet.ts` ข้อมูลจำลอง | `memstore.go` ใช้ตอน `DEMO_MODE` และตอนเทส | — |
| `schema.ts` (zod) ตรวจ input | ตรวจ `from` / `to` ใน handler ด้วย `strconv` | เทส |

### จุดเดียวที่ Go ต้องเข้มกว่า front-end

front-end **ยอมให้ฟีเจอร์ import ฟีเจอร์อื่นได้** (ผ่าน index)
TypeScript ยอมให้ import วนกันได้ แม้จะเสี่ยงบั๊กตอนโหลด แต่ Go ไม่ยอมเลย

**กฎของ backend: ฟีเจอร์ห้าม import กันเอง** ถ้าต้องใช้ข้อมูลของกัน ให้ใช้ฐานข้อมูลเป็นตัวกลาง
ซึ่ง backend นี้ทำแบบนั้นได้เป็นธรรมชาติอยู่แล้ว (หัวข้อ 12)

### ชื่อ type — แปลงตามกฎข้อ 4.6

| TypeScript | Go |
|---|---|
| `FleetSnapshot` | `fleet.Snapshot` |
| `FleetOption` | `fleet.Option` |
| `Vehicle` | `fleet.Vehicle` |
| `VehicleStatus` | `fleet.Status` |
| `DataStatus` | **ไม่มีใน Go โดยตั้งใจ** — front-end คำนวณเอง (`API-REQUIREMENTS.md` หัวข้อ 2) |

---

## 8. ฟีเจอร์ front-end 8 ตัว → backend 3 ตัว

**อย่าลอกรายชื่อโฟลเดอร์ของ front-end มาทั้งหมด** ลอกแค่หลักการ
เพราะ front-end แบ่งตาม **สิ่งที่ผู้ใช้เห็น** แต่ backend แบ่งตาม **ข้อมูลที่อ่านเขียน**
และ `API-REQUIREMENTS.md` หัวข้อ 1 บอกชัดว่าหน้าอื่นใช้ข้อมูลชุดเดียวกับ `/map`

| ฟีเจอร์ front-end | ใช้ข้อมูลจาก backend | package Go |
|---|---|---|
| `fleet` | `GET /api/fleet`, `GET /api/fleet/stream` | `fleet` |
| `fleet-map` | ข้อมูลเดียวกับ `fleet` | `fleet` |
| `vehicle-list` | ข้อมูลเดียวกับ `fleet` | `fleet` |
| `vehicle-detail` | ข้อมูลเดียวกับ `fleet` (+ `speed-history` ภายหลัง) | `fleet` / `history` |
| `vehicle-history` | `GET /api/vehicles/{id}/history` | `history` |
| `fleet-export` | Next.js ทำเองที่ `/api/fleet/export` | — |
| `app-shell` | ไม่มี endpoint ของตัวเอง | — |
| `launchpad` | ไม่มี endpoint ของตัวเอง | — |
| — | รับตำแหน่งรถเข้า + simulator | `ingest` |
| — | ตั้ง offline + ลบข้อมูลเก่า | `jobs` |

ผลคือ **3 ฟีเจอร์ + งานเบื้องหลัง 1 ตัว** แต่ละตัวอ่านเขียนคนละแบบชัดเจน:

| package | ตารางหลัก | จังหวะ | ตรรกะที่ไม่ใช่ SQL |
|---|---|---|---|
| `fleet` | อ่าน view `fleet_snapshot` | อ่านบ่อยมาก ทุก 3 วินาที | หา diff เพื่อส่ง patch, heartbeat, ส่งก้อนเต็มตอนต่อใหม่ |
| `history` | อ่าน `position_events` | นาน ๆ ครั้ง แต่ก้อนใหญ่ | ระยะทาง, นาทีวิ่ง/จอด, max/avg, ไทม์ไลน์ events, ลดจุดให้ไม่เกิน 5,000 |
| `ingest` | เขียน `position_events` + `vehicle_live_state` | ต่อเนื่อง | ทรานแซกชันสองตาราง |
| `jobs` | เรียก `mark_stale_devices_offline()`, `prune_position_events()` | ทุก 1 นาที / รายวัน | ไม่มี |

---

# ภาค 3 — ผลประเมิน

## 9. ข้อเท็จจริงที่ใช้ประเมิน

| เรื่อง | ค่าจริง | ที่มา |
|---|---|---|
| โค้ด Go ตอนนี้ | `main.go` hello world 18 บรรทัด, โค้ดเก่าถูกลบ (ยังไม่ commit) | `git status` |
| กำลังเรียน | `lessons/01-hello-api` | repo นี้ |
| คนทำ | 1 คน ดูแลทั้ง front-end และ backend | — |
| endpoint ในสเปก | 4 ตัว (+ `speed-history` ภายหลัง) + ช่องทางรับข้อมูลเข้า | `API-REQUIREMENTS.md` หัวข้อ 1, 7.1, 10 |
| ฐานข้อมูล | 6 ตาราง, 1 view, 2 function | `migrations/001_map_history_core.sql` |
| ตรรกะที่ต้องเทสโดยไม่มี DB | สรุป history, ไทม์ไลน์ events, ลดจุด, diff ของ SSE | `API-REQUIREMENTS.md` หัวข้อ 4, 5 |
| ต้องสลับโหมด | `DEMO_MODE=true` บน Render ตอนนี้ | `render.yaml` |
| deploy | web service ตัวเดียวบน Render | `render.yaml` |
| front-end | ระดับ 4 บังคับด้วย ESLint, 8 ฟีเจอร์ | `map-maker-group/eslint.config.mjs` |
| อนาคตที่มีแนวโน้ม | `tenants`, `gps_devices`, `vehicle_assignments`, `trips`, outbox, partition | `git show main:migrations/001_fleet_realtime_postgres.sql` |
| ขนาดโค้ดเมื่อทำครบสเปก | **~2,000–3,500 บรรทัด** (ประมาณการ รวมเทส) | ประเมินจากสเปก |

---

## 10. ตารางคะแนน

แต่ละช่องให้ 1–5 (5 = เข้ากับโปรเจกต์นี้ที่สุด)
**คะแนนเป็นการตัดสินเชิงคุณภาพ ไม่ใช่การวัด** — เหตุผลของแต่ละแถวอยู่ใต้ตาราง

| เกณฑ์ | ระดับ 1 | ระดับ 2 | ระดับ 3 | **ระดับ 4 เบา** | ระดับ 5 |
|---|:-:|:-:|:-:|:-:|:-:|
| A. เหมาะกับขนาด ~2–3.5k บรรทัด | 2 | 3 | 5 | **5** | 3 |
| B. เหมาะกับคนเดียวที่กำลังเรียน Go | 5 | 5 | 3 | **4** | 1 |
| C. เทสตรรกะ history โดยไม่มี Postgres | 2 | 3 | 4 | **4** | 5 |
| D. สลับ `DEMO_MODE` ↔ Postgres | 2 | 3 | 4 | **4** | 5 |
| E. ตรงกับวิธีคิดของ front-end | 1 | 2 | 2 | **5** | 3 |
| F. แยก 3 ฟีเจอร์ไม่ให้พันกัน | 1 | 2 | 3 | **5** | 4 |
| G. โตไปถึง schema เต็มบน `main` โดยไม่รื้อ | 1 | 2 | 3 | **5** | 4 |
| H. ต้นทุนต่อการเพิ่ม 1 endpoint (ต่ำ = คะแนนสูง) | 5 | 5 | 3 | **4** | 1 |
| **รวม (เต็ม 40)** | **19** | **25** | **27** | **36** | **26** |

### เหตุผลของแต่ละเกณฑ์

- **A. ขนาด** — ระดับ 1–2 เริ่มอึดอัดเมื่อเกิน ~1,500–3,000 บรรทัด ระดับ 5 มีโค้ดเชื่อมต่อ (interface, adapter, wiring) เยอะเมื่อเทียบกับตรรกะจริง
- **B. ผู้เรียน** — ระดับ 4 ได้ 4 ไม่ใช่ 3 เพราะคุณใช้โครงสร้างแบบนี้อยู่แล้วใน front-end และเปิดโฟลเดอร์เดียวก็เห็นทั้งฟีเจอร์ ระดับ 5 ต้องเข้าใจ port/adapter ก่อนเขียน endpoint แรกได้
- **C. เทส** — ตรรกะของ history เป็นฟังก์ชันล้วนได้ทั้งหมด (รับ `[]Point` คืนผลสรุป) ระดับ 3–4 เทสได้ดีพอ ๆ กับระดับ 5 ถ้าเขียนเป็นฟังก์ชันล้วน ระดับ 5 ได้เต็มเพราะโครงสร้างบังคับให้เป็นแบบนั้น
- **D. สลับโหมด** — ต้องการ interface แค่ตัวเดียวต่อฟีเจอร์ (`Store`) ระดับ 3–4 ทำได้สบาย โค้ดเวอร์ชันก่อนพิสูจน์แล้วว่าระดับ 1 ทำให้ `main()` ซ้ำสองชุด
- **E. front-end** — front-end แบ่งตามฟีเจอร์ ระดับ 3 แบ่งตามชนิดโค้ด ต้องสลับวิธีคิดทุกครั้งที่ข้าม repo ระดับ 5 จัดตามโดเมนบางส่วนจึงได้ 3
- **F. แยกฟีเจอร์** — 3 ฟีเจอร์อ่านเขียนคนละตาราง คนละจังหวะ (หัวข้อ 8) ระดับ 3 เอาของทั้งสามมาปนกันในทุกชั้น
- **G. การโต** — schema เต็มบน `main` มี `tenants`, `gps_devices`, `trips` เพิ่มเข้ามา ระดับ 4 = เพิ่มโฟลเดอร์ใหม่ ระดับ 3 = เพิ่มไฟล์ในทุกชั้นจน `http/` บวม
- **H. ต้นทุน** — ลองนึกถึงการเพิ่ม `GET /api/vehicles/{id}/speed-history` (หัวข้อ 7.1 ในสเปก):
  ระดับ 4 แตะ 2 ไฟล์ใน `history/` · ระดับ 3 แตะ 3 โฟลเดอร์ · ระดับ 5 แตะ port + adapter + domain + wiring

### ถ้าไม่นับ front-end ผลเปลี่ยนไหม

ตัดเกณฑ์ E ออก: ระดับ 4 = **31**, ระดับ 3 = 25, ระดับ 2 = 23, ระดับ 5 = 23, ระดับ 1 = 18

**ระดับ 4 ยังนำอยู่** แปลว่าคำตอบไม่ได้มาจากความอยากให้เหมือน front-end อย่างเดียว
แต่มาจากการที่ backend นี้มี 3 ฟีเจอร์ที่ลักษณะต่างกันชัด — front-end ทำให้ช่องห่างกว้างขึ้นและลดต้นทุนการเรียนรู้

### ทำไมไม่ใช่ระดับ 5

ตรรกะธุรกิจที่หนักที่สุดของระบบนี้ **อยู่ใน SQL แล้ว** —
การคิด `status` อยู่ใน view `fleet_snapshot`, การกันข้อมูลเก่าเขียนทับอยู่ใน `WHERE EXCLUDED.recorded_at > s.recorded_at`,
การตั้ง offline และลบข้อมูลเก่าเป็น function ในฐานข้อมูล
ถ้าทำ Hexagonal จะได้ชั้น adapter ห่อ SQL ที่ไม่มีกฎอะไรให้ปกป้อง

---

## 11. โครงสร้างเป้าหมาย

```text
map-maker-group-service-api/
├── cmd/
│   └── api/
│       └── main.go              // อ่าน config → เปิด pool → ประกอบฟีเจอร์ → ListenAndServe
│
├── internal/
│   ├── config/
│   │   └── config.go            // DATABASE_URL, DB_SCHEMA, FRONTEND_ORIGINS, PORT, DEMO_MODE — อ่าน env ที่นี่ที่เดียว
│   │
│   ├── platform/                // ≈ src/shared + src/server ของ front-end
│   │   ├── postgres/
│   │   │   └── pool.go          // pgxpool + search_path "map-maker-db-new"
│   │   └── httpx/
│   │       ├── json.go          // WriteJSON, WriteError
│   │       └── cors.go          // allow-list + โดเมน preview ของ Vercel (สเปกหัวข้อ 9)
│   │
│   ├── fleet/                   // ≈ features/fleet (+ fleet-map, vehicle-list, vehicle-detail)
│   │   ├── vehicle.go           // Vehicle, Snapshot, Option, Status — สัญญากับ types.ts
│   │   ├── store.go             // interface Store + PostgresStore (อ่าน view fleet_snapshot)
│   │   ├── memstore.go          // ≈ lib/mock-fleet.ts — ใช้ตอน DEMO_MODE และเทส
│   │   ├── diff.go              // เทียบ snapshot เก่า/ใหม่ → patch เฉพาะฟิลด์ที่เปลี่ยน
│   │   ├── hub.go               // อ่าน store ทุก 3 วิ, กระจาย patch ให้ทุก client
│   │   ├── handler.go           // GET /api/fleet, GET /api/fleet/stream
│   │   ├── diff_test.go
│   │   └── handler_test.go      // ใช้ memstore ไม่ต้องมี DB
│   │
│   ├── history/                 // ≈ features/vehicle-history
│   │   ├── point.go             // Point, Event, Result
│   │   ├── summarize.go         // ระยะทาง, นาทีวิ่ง/จอด, max/avg — ฟังก์ชันล้วน
│   │   ├── events.go            // start / stop / moving / end จาก ignition_on — ฟังก์ชันล้วน
│   │   ├── downsample.go        // ลดจุดให้ไม่เกิน 5,000 — ฟังก์ชันล้วน
│   │   ├── store.go             // interface Store + PostgresStore
│   │   ├── memstore.go          // ≈ lib/mock-history.ts
│   │   ├── handler.go           // GET /api/vehicles/{id}/history
│   │   ├── summarize_test.go
│   │   └── events_test.go
│   │
│   ├── ingest/                  // ไม่มีคู่ใน front-end — ฝั่งเขียนข้อมูล
│   │   ├── store.go             // ทรานแซกชัน: INSERT position_events + UPSERT vehicle_live_state
│   │   └── simulator.go         // จำลองรถวิ่งเขียนผ่านเส้นทางจริง
│   │
│   └── jobs/
│       └── jobs.go              // ทุก 1 นาที mark_stale_devices_offline(), รายวัน prune_position_events()
│
├── migrations/                  // มีอยู่แล้ว
├── lessons/                     // แบบฝึกหัด แต่ละบทเป็น main ของตัวเอง ไม่มีใคร import
├── API-REQUIREMENTS.md
├── GO-STRUCTURE.md
├── go.mod                       // module fleet-monitor-server
└── render.yaml
```

### ทำไม `memstore.go` อยู่ในฟีเจอร์ ไม่ใช่ใน `ingest`

front-end วาง `mock-fleet.ts` ไว้ใน `features/fleet/lib/` เพราะข้อมูลจำลองเป็นเรื่องของฟีเจอร์นั้น
ถ้าให้ `ingest/simulator.go` เขียนลง `fleet.MemStore` ตอน `DEMO_MODE` จะเกิด `ingest` → `fleet` ข้ามฟีเจอร์ทันที

**แยกหน้าที่ให้ชัด:**
- `fleet/memstore.go` ขยับตำแหน่งรถเองในหน่วยความจำ — ใช้เมื่อ **ไม่มี** ฐานข้อมูล
- `ingest/simulator.go` เขียนผ่านทรานแซกชันจริง — ใช้เมื่อ **มี** ฐานข้อมูลแต่ยังไม่มีอุปกรณ์จริง

---

## 12. กฎ import

เขียนแบบเดียวกับ `eslint.config.mjs` ของ front-end เพื่อให้เทียบกันได้

| จาก | import ได้ | ห้าม |
|---|---|---|
| `cmd/api` | ทุกอย่างใน `internal/` | — |
| `internal/fleet` | `platform/*`, `config` | `history`, `ingest`, `jobs` |
| `internal/history` | `platform/*`, `config` | `fleet`, `ingest`, `jobs` |
| `internal/ingest` | `platform/*`, `config` | `fleet`, `history`, `jobs` |
| `internal/jobs` | `platform/*`, `config` | ทุกฟีเจอร์ |
| `internal/platform/*` | stdlib, `pgx` | **ทุกอย่างใน `internal/` ที่ไม่ใช่ platform** |
| `internal/config` | stdlib | ทุกอย่าง |

### ฟีเจอร์คุยกันผ่านฐานข้อมูล

```text
        ┌──────────┐   INSERT + UPSERT    ┌────────────────────────┐
        │  ingest  │ ───────────────────▶ │  vehicle_live_state    │
        └──────────┘                      │  position_events       │
                                          └───────┬────────┬───────┘
                                  อ่านทุก 3 วิ     │        │  อ่านตามช่วงเวลา
                                                  ▼        ▼
                                          ┌──────────┐  ┌──────────┐
                                          │  fleet   │  │ history  │
                                          └──────────┘  └──────────┘

        ไม่มีลูกศร import ระหว่างกล่องสามกล่องนี้เลย
```

`fleet/hub.go` อ่าน snapshot ทั้งกองทุก 3 วินาทีด้วย **goroutine ตัวเดียว** (ไม่ใช่ตัวละ client)
แล้วเทียบกับรอบก่อน ส่งเฉพาะฟิลด์ที่เปลี่ยนตามสเปกหัวข้อ 4

**ข้อดี:** ไม่พลาดกรณีไหนเลย — รถออกจากอุโมงค์, job ตั้ง offline, ข้อมูลเข้าช้า ทุกอย่างสะท้อนใน snapshot
**ข้อเสีย:** ช้าได้สูงสุด 3 วินาที และอ่าน 1,000 แถวทุก 3 วินาทีแม้ไม่มีอะไรเปลี่ยน — รับได้สำหรับขนาดนี้
**อัปเกรดเมื่อจำเป็น:** `LISTEN / NOTIFY` ของ Postgres (เวอร์ชันบน `main` เคยทำไว้ใน `002_fleet_change_notifications.sql`)

### วิธีเช็กว่ากฎยังไม่พัง

```powershell
go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./internal/...
```

ดูว่าบรรทัดของ `internal/fleet` มีคำว่า `internal/history` หรือ `internal/ingest` โผล่มาไหม
ถ้าโผล่ = เส้นแบ่งฟีเจอร์ผิด ไม่ใช่ปัญหาของ import

---

## 13. โค้ดตัวอย่างจุดสำคัญ

### 13.1 สัญญากับ front-end — `internal/fleet/vehicle.go`

```go
package fleet

// Vehicle ต้องตรงกับ src/features/fleet/types.ts ของ map-maker-group ทุกฟิลด์
type Vehicle struct {
	ID              string  `json:"id"`
	Plate           string  `json:"plate"`
	Label           string  `json:"label"`
	Make            string  `json:"make"`
	Model           string  `json:"model"`
	Status          Status  `json:"status"`
	SpeedKph        int     `json:"speedKph"`
	HeadingDeg      int     `json:"headingDeg"`
	Lat             float64 `json:"lat"`
	Lng             float64 `json:"lng"`
	LastUpdate      int64   `json:"lastUpdate"` // epoch ms: recordedAt.UnixMilli()
	DriverID        string  `json:"driverId"`
	DriverName      string  `json:"driverName"`
	GroupID         string  `json:"groupId"`
	AreaID          string  `json:"areaId"`
	Address         string  `json:"address"`
	FuelPct         int     `json:"fuelPct"`
	OdometerKm      int     `json:"odometerKm"`
	EngineHours     int     `json:"engineHours"`
	TodayDistanceKm float64 `json:"todayDistanceKm"`
	SpeedHistory    []int   `json:"speedHistory"` // ต้องเป็น []int{} ห้ามเป็น nil
}

type Status string

const (
	StatusRunning  Status = "running"
	StatusEngineOn Status = "engine-on"
	StatusParking  Status = "parking"
	StatusOffline  Status = "offline"
)

type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type Snapshot struct {
	GeneratedAt int64     `json:"generatedAt"`
	Groups      []Option  `json:"groups"`
	Areas       []Option  `json:"areas"`
	Drivers     []Option  `json:"drivers"`
	Vehicles    []Vehicle `json:"vehicles"`
}
```

> **กับดักของ Go ที่ชนกับสเปกหัวข้อ 9 ("อย่าส่ง `null`") ตรง ๆ**
>
> | Go | JSON ที่ได้ |
> |---|---|
> | `var s []int` (nil) | `null` ❌ |
> | `s := []int{}` | `[]` ✅ |
>
> ทุก slice ที่ออกไปหา front-end ต้องสร้างด้วย `[]T{}` หรือ `make([]T, 0)`
> เขียนเทสหนึ่งตัวที่ marshal `Snapshot` แล้วเช็กว่าไม่มีคำว่า `null` — กันพังได้ทั้งระบบ

> **กับดักตัวที่สอง: NULL จากฐานข้อมูล**
>
> view `fleet_snapshot` ใน `migrations/001_map_history_core.sql` ใส่ `COALESCE` ให้ `address`, `fuel_pct`, `driver_name` แล้ว
> แต่ **`group_id`, `area_id`, `driver_id` ยังไม่มี** — ทั้งสามเป็น NULL ได้ (`ON DELETE SET NULL`)
> ถ้า scan NULL เข้า `string` ตรง ๆ pgx จะคืน error และ `GET /api/fleet` พังทั้งก้อนเพราะรถคันเดียว
>
> เลือกทางใดทางหนึ่ง:
> - แก้ view ให้เป็น `COALESCE(v.group_id, '') AS group_id` ← ง่ายสุด
> - หรือ scan เข้า `*string` แล้วแปลงเป็น `""` ใน Go
>
> แต่ต้องรู้ว่าทั้งสองทางแค่ทำให้ไม่พัง — สเปกหัวข้อ 11 บอกว่า `groupId` ต้องมีอยู่จริงใน `groups`
> ค่า `""` จะทำให้รถคันนั้นหลุดจากตัวกรองกลุ่มบนหน้าเว็บ

### 13.2 สัญญาฝั่งข้อมูล — `internal/fleet/store.go`

```go
package fleet

import "context"

// Store ประกาศที่นี่ เพราะ fleet เป็นคนใช้ (กฎข้อ 4.1)
// ทั้ง PostgresStore และ MemStore มี method นี้ ก็ถือว่าทำตามแล้วโดยอัตโนมัติ
type Store interface {
	Snapshot(ctx context.Context) (Snapshot, error)
}
```

### 13.3 ฟีเจอร์ลงทะเบียนเส้นทางของตัวเอง — `internal/fleet/handler.go`

```go
package fleet

import (
	"net/http"

	"fleet-monitor-server/internal/platform/httpx"
)

type Handler struct {
	store Store
	hub   *Hub
}

func NewHandler(store Store, hub *Hub) *Handler {
	return &Handler{store: store, hub: hub}
}

// Register ≈ การที่ src/app ของ front-end ดึงฟีเจอร์มาวางในหน้า
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/fleet", h.snapshot)
	mux.HandleFunc("GET /api/fleet/stream", h.stream)
}

func (h *Handler) snapshot(w http.ResponseWriter, r *http.Request) {
	snap, err := h.store.Snapshot(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not load fleet")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, snap)
}
```

### 13.4 ประกอบทุกอย่างที่เดียว — `cmd/api/main.go`

```go
package main

import (
	"context"
	"log"
	"net/http"

	"fleet-monitor-server/internal/config"
	"fleet-monitor-server/internal/fleet"
	"fleet-monitor-server/internal/history"
	"fleet-monitor-server/internal/jobs"
	"fleet-monitor-server/internal/platform/httpx"
	"fleet-monitor-server/internal/platform/postgres"
)

func main() {
	ctx := context.Background()
	cfg := config.Load()

	var fleetStore fleet.Store
	var historyStore history.Store

	if cfg.DemoMode {
		fleetStore = fleet.NewMemStore(1000)
		historyStore = history.NewMemStore()
	} else {
		pool, err := postgres.Open(ctx, cfg.DatabaseURL, cfg.DBSchema)
		if err != nil {
			log.Fatal(err)
		}
		defer pool.Close()

		fleetStore = fleet.NewPostgresStore(pool)
		historyStore = history.NewPostgresStore(pool)
		go jobs.Run(ctx, pool)
	}

	// ไม่ว่าโหมดไหน ส่วนข้างล่างนี้มีชุดเดียว — ต่างจากโค้ดเวอร์ชันก่อนที่มีสองชุด
	hub := fleet.NewHub(fleetStore)
	go hub.Run(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	fleet.NewHandler(fleetStore, hub).Register(mux)
	history.NewHandler(historyStore).Register(mux)

	handler := httpx.CORS(cfg.AllowedOrigins, mux)
	log.Printf("listening on :%s (demo=%v)", cfg.Port, cfg.DemoMode)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, handler))
}
```

### 13.5 ตรรกะล้วนเทสได้โดยไม่มีอะไรเลย — `internal/history/summarize_test.go`

```go
package history

import "testing"

func TestSummarizeStoppedTime(t *testing.T) {
	points := []Point{
		{Timestamp: 0, SpeedKph: 40},
		{Timestamp: 60_000, SpeedKph: 0},   // จอดตั้งแต่นาทีที่ 1
		{Timestamp: 180_000, SpeedKph: 30}, // ออกตัวนาทีที่ 3
	}

	got := Summarize(points)

	if got.StoppedMinutes != 2 {
		t.Fatalf("StoppedMinutes = %d, want 2", got.StoppedMinutes)
	}
}
```

ไม่มี Postgres, ไม่มี HTTP, รันเสร็จในหลักมิลลิวินาที — นี่คือผลตอบแทนของการแยก `summarize.go` ออกจาก `store.go`

---

## 14. ทำทีละขั้นตาม API-REQUIREMENTS

**อย่าสร้างโครงสร้างในหัวข้อ 11 ทั้งหมดในวันเดียว** ให้โฟลเดอร์เกิดตอนที่มีโค้ดจะใส่จริง
ตารางนี้ผูกกับลำดับในหัวข้อ 10 ของ `API-REQUIREMENTS.md`

| ขั้น | งานในสเปก | สร้างอะไรเพิ่ม | อยู่ระดับไหน |
|:-:|---|---|---|
| — | ตอนนี้: เรียน `lessons/` | ไม่ต้องแตะอะไร | 1 |
| 1 | `GET /healthz` + deploy ได้ | ย้าย `main.go` → `cmd/api/main.go`, สร้าง `internal/config` | 1 (แต่อยู่ในที่ถูกแล้ว) |
| 2 | สร้างตาราง | `internal/platform/postgres/pool.go` | 1 |
| 3 | `GET /api/fleet` — **ขั้นที่สำคัญที่สุด** | `internal/fleet/` (`vehicle.go`, `store.go`, `handler.go`), `internal/platform/httpx/`, `memstore.go` ถ้า Render ยังไม่มี DB | **4 ที่มีฟีเจอร์เดียว** |
| 4 | รับข้อมูลเข้า | `internal/ingest/` | 4 — ฟีเจอร์ที่ 2 |
| 5 | `GET /api/fleet/stream` | `fleet/hub.go`, `fleet/diff.go`, `diff_test.go` | 4 |
| 6 | `GET /api/vehicles/{id}/history` | `internal/history/` ทั้งก้อน | 4 — ฟีเจอร์ที่ 3 |
| 7 | งานเบื้องหลัง | `internal/jobs/` | 4 — ครบ |

> **ทำไมกระโดดจาก 1 ไป 4 ที่ขั้น 3 ได้โดยไม่ขัดกับคำแนะนำ "อย่าเริ่มที่ระดับสูง"**
> ตอนมีฟีเจอร์เดียว ระดับ 3 กับระดับ 4 หน้าตาเหมือนกันเป๊ะ (`cmd/api` + `internal/fleet`)
> ความต่างเกิดตอนเพิ่มฟีเจอร์ที่สองในขั้น 4 — ถึงตอนนั้นคุณจะมีโค้ดจริงให้ตัดสินใจแล้ว

---

## 15. สิ่งที่ต้องแก้ตามเมื่อย้ายโครงสร้าง

- [ ] `render.yaml` — `buildCommand: go build -o app .` ต้องเปลี่ยนเป็น `go build -o app ./cmd/api`
      ถ้าลืม Render จะ build ไฟล์ `main.go` ที่รากซึ่งไม่มีแล้ว
- [ ] `README.md` — เปลี่ยน `go run .` เป็น `go run ./cmd/api`
- [ ] อ่าน `PORT` จาก env — `main.go` ตอนนี้ fix ไว้ที่ `:8080` แต่ `.env.example` ตั้ง `PORT=8081`
      และ `lessons/01-hello-api` ก็ใช้ `:8081` อยู่ รันพร้อมกันจะชนพอร์ต
- [ ] `.gitignore` — เพิ่ม `tmp/` เพราะ `lessons/01-hello-api/tmp/` มี `main.exe` และ `build-errors.log` ที่ยังไม่ถูกกันไว้
- [ ] `FRONTEND_ORIGINS` ใน `render.yaml` มีแค่โดเมนหลัก สเปกหัวข้อ 9 บอกว่าต้องรองรับโดเมน preview ของ Vercel ด้วย
- [ ] รู้ไว้: `go build ./...` และ `go test ./...` จะคอมไพล์ `lessons/` ด้วย ถ้าบทเรียนไหนตั้งใจเขียนไม่เสร็จ คำสั่งนี้จะพัง
      ใช้ `go test ./internal/...` แทนตอนเช็กโค้ดจริง

---

## 16. เมื่อไหร่ต้องประเมินใหม่

โครงสร้างนี้ถูกต้องสำหรับสเปกปัจจุบัน ถ้าเจอสัญญาณข้างล่าง ให้กลับมาดูตารางคะแนนอีกรอบ

| สัญญาณ | ความหมาย | ทำอะไร |
|---|---|---|
| ฟีเจอร์หนึ่งต้อง import อีกฟีเจอร์ | เส้นแบ่งฟีเจอร์ผิด | ย้ายของที่ใช้ร่วมลงฐานข้อมูลหรือ `platform/` หรือรวมสองฟีเจอร์เป็นหนึ่ง |
| เพิ่ม `tenants`, `gps_devices`, `trips` จาก schema บน `main` | ฟีเจอร์เพิ่มเป็น 5–6 ตัว | ยังเป็นระดับ 4 เพิ่มโฟลเดอร์ + middleware ดึง tenant ใน `platform/httpx` |
| รับข้อมูลจากอุปกรณ์จริง (TCP / MQTT) ปริมาณสูง | `ingest` scale คนละแบบกับ API | เพิ่ม `cmd/ingest/main.go` เป็น binary ที่สอง ใช้ `internal/ingest` เดิม |
| ตรรกะ history หรือ trips ซับซ้อนมาก (คิดเงิน, geofence) | มีกฎธุรกิจที่ต้องปกป้องจริง | ยก **เฉพาะ package นั้น** เป็นแบบระดับ 5 ไม่ต้องทำทั้ง repo |
| ไฟล์ใน `fleet/` เกิน ~15 ไฟล์ | ฟีเจอร์ใหญ่เกินหนึ่งเรื่อง | แตกเป็น `fleet/` กับ `fleetstream/` หรือแยกโฟลเดอร์ย่อย |
| `platform/` มีของที่รู้จักคำว่า vehicle | ของเฉพาะฟีเจอร์หลุดไปอยู่ของกลาง | ย้ายกลับเข้าฟีเจอร์ |

---

## ภาคผนวก — ที่มาของข้อมูล

| เรื่อง | ไฟล์ |
|---|---|
| สเปก endpoint, ตาราง, ลำดับงาน | `API-REQUIREMENTS.md` |
| schema ปัจจุบัน | `migrations/001_map_history_core.sql` |
| schema เวอร์ชันเต็ม | `git show main:migrations/001_fleet_realtime_postgres.sql` |
| โค้ด Go เวอร์ชันก่อน (ระดับ 1) | `git show HEAD:db_service.go`, `git show HEAD:main.go` |
| การ deploy | `render.yaml` |
| โครงสร้างและกฎของ front-end | `map-maker-group/eslint.config.mjs` |
| ประตูของฟีเจอร์ fleet | `map-maker-group/src/features/fleet/index.ts` |
| type ที่ต้องตรงกัน | `map-maker-group/src/features/fleet/types.ts` |
| จุดที่ front-end จะเปลี่ยนมาเรียก API นี้ | `map-maker-group/src/features/fleet/server/queries.ts` |
