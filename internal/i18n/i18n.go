// Package i18n localizes the messages the server writes for people: health
// flags, console replies, job summaries and reviewer status. The English
// format string is the key, so untranslated messages fall back to English.
// Prompts sent to agents stay in English on purpose.
package i18n

import (
	"fmt"
	"sync/atomic"
)

var current atomic.Value

func init() {
	current.Store("en")
}

// SetLanguage switches the language for every later message ("en" or "zh").
func SetLanguage(language string) {
	if _, ok := catalogs[language]; ok || language == "en" {
		current.Store(language)
	}
}

// Language returns the active language.
func Language() string {
	return current.Load().(string)
}

var catalogs = map[string]map[string]string{
	"zh": zh,
}

// T translates format and applies fmt.Sprintf when args are given.
func T(format string, args ...any) string {
	if translated, ok := catalogs[Language()][format]; ok {
		format = translated
	}
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}
