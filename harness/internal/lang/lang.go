// Package lang picks the language of user-facing messages: English by default,
// Spanish with ORQ_LANG=es (the original language of orq).
package lang

import (
	"os"
	"strings"
)

// ES reports whether the user asked for Spanish messages.
func ES() bool { return strings.HasPrefix(strings.ToLower(os.Getenv("ORQ_LANG")), "es") }

// T returns en, or es when ORQ_LANG=es.
func T(en, es string) string {
	if ES() {
		return es
	}
	return en
}
