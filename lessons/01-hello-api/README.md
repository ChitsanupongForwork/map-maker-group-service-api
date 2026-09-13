# บทที่ 1: Hello API

## เป้าหมาย

- เข้าใจว่า HTTP server รอรับ request อย่างไร
- เข้าใจว่า route จับคู่ method และ path กับ handler อย่างไร
- ส่ง response ทั้งข้อความธรรมดาและ JSON

## เริ่ม server

เปิด terminal ที่โฟลเดอร์หลักของโปรเจกต์ แล้วรัน:

```powershell
go run ./lessons/01-hello-api
```

เมื่อเห็นข้อความว่า server ทำงานอยู่ ให้เปิดอีก terminal หนึ่งเพื่อทดลองเรียก API

```powershell
curl.exe http://localhost:8081/
curl.exe http://localhost:8081/hello
```

ผลลัพธ์ที่คาดหวัง:

```text
Go API is running!
```

```json
{"message":"Hello, Go API!"}
```

หยุด server ด้วย `Ctrl+C`

## ภาพรวมการทำงาน

```text
Client ส่ง GET /hello
        ↓
Server รับ request ที่พอร์ต 8081
        ↓
Router เลือก helloHandler
        ↓
Handler แปลง struct เป็น JSON
        ↓
Client ได้ {"message":"Hello, Go API!"}
```

## คำสำคัญ

- **Server**: โปรแกรมที่เปิดรอรับ request จาก client
- **Route**: กติกาว่า method และ path ใดจะไปทำงานที่ handler ใด
- **Handler**: function ที่อ่าน request และเขียน response
- **JSON**: รูปแบบข้อความสำหรับแลกเปลี่ยนข้อมูลระหว่างระบบ
- **HTTP status 200**: request สำเร็จ

## แบบฝึกหัด

1. เพิ่ม route `GET /about`
2. ให้ route ตอบ JSON รูปแบบนี้

```json
{"name":"ชื่อของคุณ","learning":"Go API"}
```

3. ทดลองเปิด `http://localhost:8081/about`

คำใบ้: สร้าง struct ใหม่และ handler ใหม่ แล้วลงทะเบียนด้วย `mux.HandleFunc`
