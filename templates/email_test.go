package templates_test

import (
	"regexp"
	"os"
	"testing"

	"github.com/malpou/pantry-expiration-notifier/i18n"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplate_UsesOnlyValidTranslationKeys(t *testing.T) {
	// Read the template source file
	content, err := os.ReadFile("email.templ")
	require.NoError(t, err, "failed to read email.templ")

	// Find all translations.Get("key") calls
	re := regexp.MustCompile(`translations\.Get\("([^"]+)"\)`)
	matches := re.FindAllStringSubmatch(string(content), -1)

	require.NotEmpty(t, matches, "no translation keys found in template")

	// Verify each key exists in RequiredKeys
	for _, match := range matches {
		key := match[1]
		assert.Contains(t, i18n.RequiredKeys, key, "template uses unknown translation key: %s", key)
	}
}
