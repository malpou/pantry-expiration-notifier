# Pantry Expiration Notifier

A lightweight service that monitors product expiration dates in a Google Sheets spreadsheet and sends automated email notifications to prevent food waste.

## Features

- **Google Sheets Integration**: Reads product data directly from Google Sheets
- **Smart Notifications**: Sends emails at configurable thresholds (90, 60, 30, 14, 7, 3, and 1 days before expiration)
- **Duplicate Prevention**: Tracks sent notifications to avoid duplicate alerts
- **SMTP Support**: Works with any SMTP email provider
- **Structured Logging**: Uses zap for JSON-formatted, production-ready logs
- **HTML Email Templates**: Clean, responsive email design with color-coded urgency levels

## How It Works

1. Reads product data from a Google Sheets spreadsheet
2. Calculates days until expiration for each product
3. Sends email notifications when products reach notification thresholds
4. Updates the spreadsheet to track which notifications have been sent
5. Prevents duplicate notifications for the same threshold

## Prerequisites

- Go 1.24 or later
- Google Cloud Service Account with Sheets API access
- SMTP server credentials

## Installation

```bash
go get github.com/malpou/pantry-expiration-notifier
```

Or clone and build:

```bash
git clone https://github.com/malpou/pantry-expiration-notifier.git
cd pantry-expiration-notifier
go build
```

## Configuration

Create a `.env` file in the project root with the following variables:

```bash
# Google Service Account credentials (JSON, can be base64 encoded)
GOOGLE_SA_CREDENTIALS={"type":"service_account",...}

# Google Sheets
SPREADSHEET_ID=1_6yFnH6UYMS8FmNp9IlG4KkXpLyU-I1DT-i7r3wrED4
SHEET_NAME=data

# Recipients (comma-separated)
RECIPIENT_EMAILS=alice@example.com, bob@example.com, charlie@example.com

# Notification thresholds in days before expiration (comma-separated)
NOTIFICATION_DAYS=90,60,30,14,7,3,1

# Language (ISO 639-1 code)
LANGUAGE=da

# SMTP Configuration
SMTP_HOST=smtp.fastmail.com
SMTP_PORT=587
SMTP_USERNAME=you@fastmail.com
SMTP_PASSWORD=app-specific-password
SMTP_FROM=Spisekammer <pantry@yourdomain.com>

# Optional: Custom SMTP headers (comma-separated)
SMTP_HEADERS=X-PM-Message-Stream: outbound
```

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `GOOGLE_SA_CREDENTIALS` | Yes | Google Service Account JSON credentials (can be base64 encoded) |
| `SPREADSHEET_ID` | Yes | Google Sheets spreadsheet ID |
| `SHEET_NAME` | No | Sheet name (default: "Sheet1") |
| `RECIPIENT_EMAILS` | Yes | Comma-separated list of email recipients |
| `NOTIFICATION_DAYS` | No | Comma-separated list of notification thresholds in days before expiration (default: "90,60,30,14,7,3,1"). Example: "30,7,1" for notifications at 30, 7, and 1 day before expiration |
| `LANGUAGE` | No | Language code using ISO 639-1 standard (default: "da"). Supported languages: "da" (Danish), "en" (English). The service will fail if an unsupported language is specified or if translation keys are missing |
| `SMTP_HOST` | Yes | SMTP server hostname |
| `SMTP_PORT` | No | SMTP server port (default: 587) |
| `SMTP_USERNAME` | Yes | SMTP authentication username |
| `SMTP_PASSWORD` | Yes | SMTP authentication password |
| `SMTP_FROM` | Yes | Email sender address (supports "Name <email>" format) |
| `SMTP_HEADERS` | No | Optional custom SMTP headers (format: "Header-Name: value, Another-Header: value"). Example: "X-PM-Message-Stream: outbound" for Postmark or "X-Priority: 1, X-Mailer: MyApp" for multiple headers |

## Google Sheets Setup

### Sheet Format

Your spreadsheet should have the following columns (A-F):

| Column A | Column B | Column C | Column D | Column E | Column F |
|----------|----------|----------|----------|----------|----------|
| Name | Packaging | Location | Quantity | Expiration | Sent Days |
| Tomato Sauce | Can | Pantry | 5 | 2025-12-31 | 90,60 |
| Pasta | Box | Kitchen Cabinet | 2 | 2025-06-15 | |

- **Column A (Name)**: Product name
- **Column B (Packaging)**: Packaging type (e.g., Can, Box, Jar)
- **Column C (Location)**: Storage location (e.g., Pantry, Fridge, Freezer)
- **Column D (Quantity)**: Number of items
- **Column E (Expiration)**: Expiration date in YYYY-MM-DD format
- **Column F (Sent Days)**: Comma-separated list of notification thresholds already sent (automatically updated)

### Google Cloud Setup

1. Create a Google Cloud project
2. Enable the Google Sheets API
3. Create a Service Account
4. Generate a JSON key for the Service Account
5. Share your spreadsheet with the Service Account email address (with Editor permissions)

## Usage

Run the service manually:

```bash
./pantry-expiration-notifier
```

Or schedule it with cron (daily at 9 AM):

```bash
0 9 * * * /path/to/pantry-expiration-notifier
```

### Deploying to Render

This service is designed to run as a cron job on Render:

1. Create a new Cron Job on Render
2. Connect your Git repository
3. Configure the following settings:

**Runtime:** Go

**Schedule:** `0 9 * * 0` (Every Sunday at 9 AM)

**Command:** `./app`

**Build Command:** `go install github.com/a-h/templ/cmd/templ@latest && templ generate && go build -o app .`

4. Add all required environment variables from the Configuration section above

The service will automatically run according to your schedule and send notifications for products approaching expiration.

## Notification Thresholds

By default, the service sends notifications at the following thresholds before expiration:

- 90 days
- 60 days
- 30 days
- 14 days
- 7 days
- 3 days
- 1 day

You can customize these thresholds using the `NOTIFICATION_DAYS` environment variable. For example:
- `NOTIFICATION_DAYS=30,7,1` - Only notify at 30, 7, and 1 day before expiration
- `NOTIFICATION_DAYS=14,7,3,1,0` - Notify at 14, 7, 3, 1 days before and on expiration day

Each threshold triggers only once per product to avoid spam.

## Internationalization (i18n)

The service supports multiple languages through translation files in the `i18n/` directory. Each language is identified by its ISO 639-1 code.

### Supported Languages

- **Danish** (`da`) - Default
- **English** (`en`)

### Adding New Languages

To add support for a new language:

1. Create a new JSON file in the `i18n/` directory named `{language-code}.json` (e.g., `de.json` for German)
2. Include all required translation keys:
   - `email_subject` - Email subject line (supports `%d` for product count)
   - `email_title` - Email heading
   - `email_intro` - Introductory paragraph
   - `email_footer` - Footer text
   - `days_expired` - Text for expired products
   - `days_expiring_today` - Text for products expiring today
   - `days_one_remaining` - Text for products with 1 day remaining
   - `days_remaining` - Text for products with multiple days remaining (supports `%d` for day count)
3. Set the `LANGUAGE` environment variable to your language code
4. The service will validate that all required keys are present and fail gracefully if any are missing

### Example Translation File

```json
{
  "email_subject": "🥫 %d items approaching expiration",
  "email_title": "🥫 Items Expiring Soon",
  "email_intro": "The following items in your pantry are approaching their expiration date:",
  "email_footer": "This email was automatically generated by your pantry system.",
  "days_expired": "Expired!",
  "days_expiring_today": "Expires today",
  "days_one_remaining": "1 day left",
  "days_remaining": "%d days left"
}
```

## Email Template

Emails are sent in HTML format with:
- Color-coded urgency levels (critical: red, warning: orange, ok: green)
- Product name, packaging, and quantity
- Days remaining until expiration
- Localized content based on the `LANGUAGE` setting

## Logging

The service uses structured logging with zap, outputting JSON-formatted logs suitable for production environments:

```json
{"level":"info","ts":1732454400,"msg":"Sent notification","products":5,"recipients":2}
{"level":"info","ts":1732454401,"msg":"Sheet updated successfully"}
```

## License

MIT License - see [LICENSE](LICENSE) file for details
