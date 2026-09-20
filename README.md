# FoodBridge | Food-sharing backend

A backend API for sharing free meals, with user accounts, food posts, bookings, comments, notifications, and reporting workflows.

**Stack:** Go · Echo · GORM · PostgreSQL · Docker

## Project at a glance

- **API and authentication:** public registration/login routes and JWT-protected application routes.
- **Application structure:** feature modules separate controllers, services, and repositories.
- **Data layer:** PostgreSQL models managed through GORM, with schema initialization at startup.
- **Development environment:** Docker Compose supplies local services.

For a code walkthrough, start with [`main.go`](main.go), then [`server/routes`](server/routes) and [`pkg/booking`](pkg/booking).

## Setup notes for the current code

The module requires **Go 1.24.0** and specifies the **Go 1.24.5 toolchain**. These supersede the older version listed in the original guide below.

1. Clone this repository and copy `.env.example` to `.env`.
2. Configure `POSTGRES_DSN`, or all of `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, and `DB_SSLMODE`, to match your local database. Configure authentication and any external integrations required by the features you use.
3. Start the local database with `docker compose up -d db`.
4. Download dependencies with `go mod download`, then run `go run ./main.go`.
5. Check `http://localhost:1323/health`, or the port you set through `PORT`. The current entry point defaults to **1323** and returns JSON with `status`, `app`, and `db` fields; check the database status as well as the HTTP response.

The original Thai documentation follows for additional context.

---

# 🍚 FoodBridge Backend

Backend สำหรับโปรเจกต์ **FoodBridge** (แจกข้าวฟรี)  
พัฒนาโดยใช้ภาษา **Go (Echo Framework)** + **Postgres (Docker)**  

---

## 🚀 โครงสร้างโปรเจกต์
```bash
FoodBridge/
├── app/ # Dependency Injection (Deps)
├── config/ # โหลดค่าการตั้งค่า (.env)
├── databases/ # การเชื่อมต่อฐานข้อมูล
├── dto/ # Data Transfer Object
├── entities/ # Struct ของ DB
├── pkg/ # business logic แยกตาม feature (auth, booking, post ...)
├── server/ # router และ middleware
├── .env.example # ตัวอย่างไฟล์ config
├── docker-compose.yml # docker run postgres + adminer
├── Dockerfile # build backend เป็น container
├── go.mod / go.sum # dependency ของ Go
└── main.go # entrypoint ของระบบ
```


---

## 🛠️ สิ่งที่ต้องติดตั้งก่อน

- [Go](https://go.dev/) (>= 1.21)  
- [Docker Desktop](https://www.docker.com/products/docker-desktop/)  
- [Git](https://git-scm.com/)  

---

## ⚙️ วิธีรันโปรเจกต์บนเครื่อง

### 1. Clone repo
```bash
git clone https://github.com/RathaTart/FoodBridge.git
cd FoodBridge
```

### 2. สร้างไฟล์ .env
คัดลอกจาก .env.example

### 3. รัน Database ด้วย Docker
docker compose up -d

### 4. ติดตั้ง dependency Go
go mod tidy

### 5. รันเซิร์ฟเวอร์
go run ./main.go

### 5. ทดสอบ Health Check
```bash
curl http://localhost:8080/health
คาดว่าจะได้:
HTTP/1.1 200 OK
ok
```



