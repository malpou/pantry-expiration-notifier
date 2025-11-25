package i18n_test

import (
	"os"
	"testing"

	"github.com/malpou/pantry-expiration-notifier/i18n"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var supportedLanguages = []string{"da", "en", "fr"}

func TestLoad_SupportedLanguages(t *testing.T) {
	// Change to project root so i18n folder is accessible
	require.NoError(t, os.Chdir(".."), "failed to change directory")

	logger := zap.NewNop()

	for _, lang := range supportedLanguages {
		t.Run(lang, func(t *testing.T) {
			translations, err := i18n.Load(lang, logger)
			require.NoError(t, err, "failed to load language %s", lang)

			// Verify all required keys are present and non-empty
			for _, key := range i18n.RequiredKeys {
				value := translations.Get(key)
				assert.NotEmpty(t, value, "required key %q is empty", key)
			}
		})
	}
}
