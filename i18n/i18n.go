package i18n

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

type Translations map[string]string

func (t Translations) Get(key string) string {
	return t[key]
}

var RequiredKeys = []string{
	"email_subject",
	"email_title",
	"email_intro",
	"email_footer",
	"days_expired",
	"days_expiring_today",
	"days_one_remaining",
	"days_remaining",
}

func Load(lang string, logger *zap.Logger) (Translations, error) {
	// Construct path to translation file
	translationFile := filepath.Join("i18n", lang+".json")

	// Check if file exists
	if _, err := os.Stat(translationFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("unsupported language '%s': translation file not found at %s", lang, translationFile)
	}

	// Read translation file
	data, err := os.ReadFile(translationFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read translation file %s: %w", translationFile, err)
	}

	// Parse JSON
	var translations Translations
	if err := json.Unmarshal(data, &translations); err != nil {
		return nil, fmt.Errorf("failed to parse translation file %s: %w", translationFile, err)
	}

	// Validate all required keys are present
	var missingKeys []string
	for _, key := range RequiredKeys {
		if _, ok := translations[key]; !ok {
			missingKeys = append(missingKeys, key)
		}
	}

	if len(missingKeys) > 0 {
		return nil, fmt.Errorf("translation file %s is missing required keys: %s", translationFile, strings.Join(missingKeys, ", "))
	}

	logger.Info("Loaded translations", zap.String("language", lang), zap.Int("keys", len(translations)))
	return translations, nil
}
