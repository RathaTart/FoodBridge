package main

import (
	"fmt"
	"log"
	"os"

	"github.com/RathaTart/FoodBridge/databases"
	"github.com/RathaTart/FoodBridge/entities"
	"github.com/RathaTart/FoodBridge/server" // ใช้ AuthMiddleware
	"github.com/RathaTart/FoodBridge/server/routes"
	"github.com/RathaTart/FoodBridge/server/bootstrap"

	"github.com/RathaTart/FoodBridge/app"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	gmlog "github.com/labstack/gommon/log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

/* -------------------- ENV helpers -------------------- */

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("missing required env %s (check your .env / Render env vars)", k)
	}
	return v
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func buildDSN() string {
	// ถ้ากำหนด POSTGRES_DSN มา ก็ใช้ตรง ๆ
	if dsn := os.Getenv("POSTGRES_DSN"); dsn != "" {
		return dsn
	}
	// บังคับต้องมีค่า DB_* ครบ เพื่อกันพลาดไปต่อ localhost
	host := mustEnv("DB_HOST")
	port := mustEnv("DB_PORT")
	user := mustEnv("DB_USER")
	pass := mustEnv("DB_PASSWORD")
	name := mustEnv("DB_NAME")
	ssl := mustEnv("DB_SSLMODE")

	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=Asia/Bangkok",
		host, user, pass, name, port, ssl,
	)
}
//  go func() {
//      ticker := time.NewTicker(60 * time.Second)
//      defer ticker.Stop()
//      for range ticker.C {
// -        if err := postSvc.AutoCloseSweep(time.Now()); err != nil {
// +        if err := postSvc.AutoCloseSweep(time.Now().Unix()); err != nil {
//              log.Printf("[auto-close] sweep error: %v", err)
//          }
//      }
//  }()

/* -------------------- main -------------------- */

func main() {
	
	_ = godotenv.Load() // dev only (บน Render จะไม่ใช้ไฟล์ .env)

	// ----- Connect DB -----
	dsn := buildDSN()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect database: ", err)
	}

	// สร้าง ENUM ต่าง ๆ ก่อน migrate (idempotent)
	if err := databases.EnsurePostgresEnums(db); err != nil {
		log.Fatal("ensure enums failed: ", err)
	}

	// ----- AutoMigrate -----
	if err := db.AutoMigrate(
		&entities.User{},
		&entities.Post{},
		&entities.PostDetail{},
		&entities.Report{},
		&entities.Verification{},
		&entities.Booking{},
		&entities.PostLike{},
		&entities.PostComment{},
		&entities.Notification{},
	); err != nil {
		log.Fatal("auto-migrate failed: ", err)
	}

	stopWorkers := bootstrap.StartBackgroundWorkers(db)
	defer stopWorkers()

	// ----- Auto-close sweep -----
	go func() {
		log.Println("[auto-close] disabled: missing post package")
	}()
	// ----- Echo -----
	e := echo.New()
	e.Debug = true
	e.Logger.SetLevel(gmlog.DEBUG)
	e.HideBanner = true
	e.Use(middleware.Recover())
	e.Use(middleware.Logger())
	e.Use(middleware.CORS())

	// ===== Public =====
	e.GET("/", func(c echo.Context) error {
		return c.JSON(200, echo.Map{
			"name":    "FoodBridge API",
			"status":  "ok",
			"health":  "/health",
			"auth":    echo.Map{"register": "POST /auth/register", "login": "POST /auth/login (returns JWT)"},
			"version": "v1",
		})
	})

	e.GET("/health", func(c echo.Context) error {
		app := "ok"
		dbStatus := "ok"
		if sqlDB, err := db.DB(); err != nil {
			dbStatus = "error: get sql.DB failed"
		} else if err := sqlDB.Ping(); err != nil {
			dbStatus = "error: " + err.Error()
		}
		return c.JSON(200, echo.Map{"status": "ok", "app": app, "db": dbStatus})
	})

	// กลุ่ม Public: มี /auth/register, /auth/login ฯลฯ
	public := e.Group("")
	routes.RegisterUserRoutes(public, db)

	// ===== Protected (ต้องมี Bearer JWT) =====
	protected := e.Group("", server.AuthMiddleware())

	routes.RegisterUserProtectedRoutes(protected, db)
	routes.RegisterPostRoutes(protected, db)
	routes.RegisterReportRoutes(protected, db)
	routes.RegisterVerificationRoutes(protected, db)
	routes.RegisterBookingRoutes(protected, db)
	routes.RegisterHistoryRoutes(protected, app.Deps{DB: db})
	routes.RegisterUploadRoutes(protected)
	// routes.RegisterPostDetailRoutes(protected, db)
	routes.RegisterCommentRoutes(protected, db)
	routes.RegisterNotificationRoutes(protected, db)

	// Debug routes (เลือกใช้ช่วง dev)
	e.GET("/debug/db-meta", func(c echo.Context) error {
		type Tbl struct{ Schema, Name string }
		var tables []Tbl
		var curDB, curSchema string
		var enumVals []string

		db.Raw("select current_database()").Scan(&curDB)
		db.Raw("select current_schema()").Scan(&curSchema)
		db.Raw(`select table_schema, table_name
		        from information_schema.tables
		        where table_schema not in ('pg_catalog','information_schema')
		        order by 1,2`).Scan(&tables)
		db.Raw(`select enumlabel
		        from pg_enum e join pg_type t on t.oid=e.enumtypid
		        where t.typname='booking_status'
		        order by enumsortorder`).Scan(&enumVals)

		return c.JSON(200, echo.Map{
			"current_database": curDB,
			"current_schema":   curSchema,
			"tables":           tables,
			"booking_status":   enumVals,
		})
	})

	// ==== LIST ALL ROUTES + JSON endpoint ====
	e.GET("/__routes", func(c echo.Context) error {
		out := []echo.Map{}
		for _, r := range e.Routes() {
			out = append(out, echo.Map{"method": r.Method, "path": r.Path, "name": r.Name})
			e.Logger.Infof("%-6s  %-30s  -> %s", r.Method, r.Path, r.Name)
		}
		return c.JSON(200, out)
	})

	// ----- Start -----
	port := getenv("PORT", "1323") // บน Render แนะนำตั้ง PORT เป็น env var
	e.Logger.Infof("Starting server on :%s", port)
	e.Logger.Fatal(e.Start(":" + port))
}
