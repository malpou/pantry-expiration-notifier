package main

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/joho/godotenv"
	"github.com/malpou/pantry-expiration-notifier/config"
	"github.com/malpou/pantry-expiration-notifier/data"
	"github.com/malpou/pantry-expiration-notifier/notification"
	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}
	defer logger.Sync()

	// Load .env file as fallback if environment variables aren't set
	_ = godotenv.Load()

	cfg, err := config.Load(logger)
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
	}

	ctx := context.Background()

	sheetClient, err := data.NewSheetClient(ctx, cfg.SACredentials, cfg.SpreadsheetID, cfg.SheetName)
	if err != nil {
		logger.Fatal("Unable to create Sheets client", zap.Error(err))
	}

	products, err := sheetClient.LoadProducts()
	if err != nil {
		logger.Fatal("Unable to read sheet", zap.Error(err))
	}

	loc, err := time.LoadLocation("Europe/Copenhagen")
	if err != nil {
		logger.Fatal("Failed to load timezone", zap.Error(err))
	}
	today := time.Now().In(loc).Truncate(24 * time.Hour)

	var expiring []data.ExpiringProduct
	var updates []data.SentDaysUpdate

	for _, p := range products {
		daysUntil := int(p.Expiration.Sub(today).Hours() / 24)

		for _, threshold := range cfg.NotificationDays {
			if daysUntil <= threshold && !slices.Contains(p.SentDays, threshold) {
				expiring = append(expiring, data.ExpiringProduct{
					Product:   p,
					DaysUntil: daysUntil,
					Threshold: threshold,
				})

				p.SentDays = append(p.SentDays, threshold)
				updates = append(updates, data.SentDaysUpdate{
					Row:      p.Row,
					SentDays: p.SentDays,
				})
				break
			}
		}
	}

	if len(expiring) == 0 {
		logger.Info("No notifications to send")
		return
	}

	if err := notification.SendEmail(cfg.SMTP, cfg.RecipientEmails, expiring, cfg.Translations); err != nil {
		logger.Fatal("Failed to send email", zap.Error(err))
	}
	logger.Info("Sent notification",
		zap.Int("products", len(expiring)),
		zap.Int("recipients", len(cfg.RecipientEmails)))

	if err := sheetClient.UpdateSentDays(updates); err != nil {
		logger.Fatal("Unable to update sheet", zap.Error(err))
	}
	logger.Info("Sheet updated successfully")
}
