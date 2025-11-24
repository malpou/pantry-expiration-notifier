package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/malpou/pantry-expiration-notifier/templates"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

var notificationDays = []int{90, 60, 30, 14, 7, 3, 1}

type Config struct {
	SpreadsheetID   string
	SheetName       string
	RecipientEmails []string
	SMTP            SMTPConfig
	SACredentials   []byte
}

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

type Product struct {
	Row        int
	Name       string
	Packaging  string
	Quantity   int
	Expiration time.Time
	SentDays   []int
}

type ExpiringProduct struct {
	Product
	DaysUntil int
	Threshold int
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx := context.Background()

	srv, err := sheets.NewService(ctx, option.WithCredentialsJSON(cfg.SACredentials))
	if err != nil {
		log.Fatalf("Unable to create Sheets client: %v", err)
	}

	resp, err := srv.Spreadsheets.Values.Get(cfg.SpreadsheetID, cfg.SheetName+"!A:E").Do()
	if err != nil {
		log.Fatalf("Unable to read sheet: %v", err)
	}

	products := parseProducts(resp.Values)

	loc, err := time.LoadLocation("Europe/Copenhagen")
	if err != nil {
		log.Fatalf("Failed to load timezone: %v", err)
	}
	today := time.Now().In(loc).Truncate(24 * time.Hour)

	var expiring []ExpiringProduct
	var updates []sheets.ValueRange

	for _, p := range products {
		daysUntil := int(p.Expiration.Sub(today).Hours() / 24)

		for _, threshold := range notificationDays {
			if daysUntil <= threshold && !slices.Contains(p.SentDays, threshold) {
				expiring = append(expiring, ExpiringProduct{
					Product:   p,
					DaysUntil: daysUntil,
					Threshold: threshold,
				})

				p.SentDays = append(p.SentDays, threshold)
				updates = append(updates, sheets.ValueRange{
					Range:  fmt.Sprintf("%s!E%d", cfg.SheetName, p.Row),
					Values: [][]any{{formatSentDays(p.SentDays)}},
				})
				break
			}
		}
	}

	if len(expiring) == 0 {
		log.Println("No notifications to send")
		return
	}

	if err := sendEmail(cfg, expiring); err != nil {
		log.Fatalf("Failed to send email: %v", err)
	}
	log.Printf("Sent notification for %d products to %d recipients\n", len(expiring), len(cfg.RecipientEmails))

	batchReq := &sheets.BatchUpdateValuesRequest{
		ValueInputOption: "RAW",
		Data:             make([]*sheets.ValueRange, len(updates)),
	}
	for i := range updates {
		batchReq.Data[i] = &updates[i]
	}
	if _, err = srv.Spreadsheets.Values.BatchUpdate(cfg.SpreadsheetID, batchReq).Do(); err != nil {
		log.Fatalf("Unable to update sheet: %v", err)
	}
	log.Println("Sheet updated successfully")
}

func loadConfig() (Config, error) {
	port, err := strconv.Atoi(getEnv("SMTP_PORT", "587"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid SMTP_PORT: %w", err)
	}

	saCredsRaw := mustEnv("GOOGLE_SA_CREDENTIALS")
	var saCreds []byte
	if decoded, err := base64.StdEncoding.DecodeString(saCredsRaw); err == nil {
		saCreds = decoded
	} else {
		saCreds = []byte(saCredsRaw)
	}

	// Parse comma-separated emails
	emailsRaw := mustEnv("RECIPIENT_EMAILS")
	var emails []string
	for email := range strings.SplitSeq(emailsRaw, ",") {
		email = strings.TrimSpace(email)
		if email != "" {
			emails = append(emails, email)
		}
	}
	if len(emails) == 0 {
		return Config{}, fmt.Errorf("RECIPIENT_EMAILS must contain at least one email")
	}

	return Config{
		SpreadsheetID:   mustEnv("SPREADSHEET_ID"),
		SheetName:       getEnv("SHEET_NAME", "Sheet1"),
		RecipientEmails: emails,
		SACredentials:   saCreds,
		SMTP: SMTPConfig{
			Host:     mustEnv("SMTP_HOST"),
			Port:     port,
			Username: mustEnv("SMTP_USERNAME"),
			Password: mustEnv("SMTP_PASSWORD"),
			From:     mustEnv("SMTP_FROM"),
		},
	}, nil
}

func mustEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("Required environment variable %s is not set", key)
	}
	return val
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func sendEmail(cfg Config, products []ExpiringProduct) error {
	var body bytes.Buffer

	tplProducts := make([]templates.ExpiringProduct, len(products))
	for i, p := range products {
		tplProducts[i] = templates.ExpiringProduct{
			Name:      p.Name,
			Packaging: p.Packaging,
			Quantity:  p.Quantity,
			DaysUntil: p.DaysUntil,
			Threshold: p.Threshold,
		}
	}

	err := templates.ExpirationEmail(tplProducts).Render(context.Background(), &body)
	if err != nil {
		return fmt.Errorf("render template: %w", err)
	}

	subject := fmt.Sprintf("=?UTF-8?B?%s?=",
		base64.StdEncoding.EncodeToString(fmt.Appendf(nil, "🥫 %d varer nærmer sig udløb", len(products))))

	msg := buildMIMEMessage(cfg.SMTP.From, cfg.RecipientEmails, subject, body.String())

	auth := smtp.PlainAuth("", cfg.SMTP.Username, cfg.SMTP.Password, cfg.SMTP.Host)
	addr := fmt.Sprintf("%s:%d", cfg.SMTP.Host, cfg.SMTP.Port)

	return smtp.SendMail(addr, auth, cfg.SMTP.From, cfg.RecipientEmails, msg)
}

func buildMIMEMessage(from string, to []string, subject, htmlBody string) []byte {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("From: %s\r\n", from))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ", ")))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(htmlBody)
	return buf.Bytes()
}

func parseProducts(rows [][]any) []Product {
	var products []Product
	for i, row := range rows {
		if i == 0 || len(row) < 4 {
			continue
		}
		qty, _ := strconv.Atoi(fmt.Sprint(row[2]))
		exp, err := time.Parse("2006-01-02", fmt.Sprint(row[3]))
		if err != nil {
			continue
		}
		var sentDays []int
		if len(row) >= 5 && row[4] != nil {
			sentDays = parseSentDays(fmt.Sprint(row[4]))
		}
		products = append(products, Product{
			Row:        i + 1,
			Name:       fmt.Sprint(row[0]),
			Packaging:  fmt.Sprint(row[1]),
			Quantity:   qty,
			Expiration: exp,
			SentDays:   sentDays,
		})
	}
	return products
}

func parseSentDays(s string) []int {
	if s == "" {
		return nil
	}
	var days []int
	for part := range strings.SplitSeq(s, ",") {
		if d, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
			days = append(days, d)
		}
	}
	return days
}

func formatSentDays(days []int) string {
	var parts []string
	for _, d := range days {
		parts = append(parts, strconv.Itoa(d))
	}
	return strings.Join(parts, ",")
}
