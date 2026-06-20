package fingerprint

import "regexp"

// Regexes are compiled once; order of application matters — see normalizeMessage.
var (
	reURL    = regexp.MustCompile(`(?i)https?://\S+`)
	reEmail  = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`)
	reUUID   = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	reHex    = regexp.MustCompile(`(?i)0x[0-9a-f]+`)
	reSingle = regexp.MustCompile(`'[^']*'`)
	reDouble = regexp.MustCompile(`"[^"]*"`)
	reNum    = regexp.MustCompile(`\b\d+\b`)
)

// normalizeMessage replaces variable parts with stable placeholders so that
// semantically identical messages (differing only in IDs, addresses, counts)
// produce the same fingerprint (§8.1).
func normalizeMessage(msg string) string {
	// URL before email — URLs may contain @-signs.
	msg = reURL.ReplaceAllString(msg, "<url>")
	msg = reEmail.ReplaceAllString(msg, "<email>")
	// UUID before hex — UUIDs contain hex digits but have a specific format.
	msg = reUUID.ReplaceAllString(msg, "<uuid>")
	msg = reHex.ReplaceAllString(msg, "<addr>")
	// Quoted strings before bare numbers — quotes may contain digits.
	msg = reSingle.ReplaceAllString(msg, "<str>")
	msg = reDouble.ReplaceAllString(msg, "<str>")
	msg = reNum.ReplaceAllString(msg, "N")
	return msg
}
