package bootstrap

import (
	"log"
	"os"
	"time"

	"gorm.io/gorm"

	"github.com/RathaTart/FoodBridge/internal/jobs"
	"github.com/RathaTart/FoodBridge/pkg/booking"
	"github.com/RathaTart/FoodBridge/pkg/notification"
)

func StartBackgroundWorkers(db *gorm.DB) (stop func()) {
	repo := booking.NewGormRepo(db)

	loc, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		log.Printf("load tz failed, fallback to local: %v", err)
		loc = time.Local
	}

	// Notification publisher for worker-triggered events (expire, queue promote, etc.)
	notifRepo := notification.NewRepository(db)
	pub := notification.NewPublisher(notifRepo)

	// Secrets from env if available
	qrSecret := []byte(os.Getenv("QR_SECRET"))
	if len(qrSecret) == 0 {
		qrSecret = []byte("CHANGE_ME_IN_ENV")
	}

	svc := booking.NewService(repo, booking.Config{
		DayTZ:      loc,
		QRSecret:   qrSecret,
		QRTokenTTL: 30 * time.Minute,
		HoldTTL:    30 * time.Minute,
	}, pub)

	stopExpiry := jobs.NewBookingExpiryWorker(svc, time.Minute).Start()

	return func() {
		if stopExpiry != nil { stopExpiry() }
	}
}
