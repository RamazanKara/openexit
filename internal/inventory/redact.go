package inventory

import "regexp"

var (
	keyValueSecretPattern = regexp.MustCompile(`(?i)(api[_-]?key|app[_-]?key|token|password|secret)(["'\s:=]+)([A-Za-z0-9_\-./+=]{8,})`)
	bearerSecretPattern   = regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9_\-./+=]{16,}`)
	datadogKeyPattern     = regexp.MustCompile(`(?i)dd[a-z0-9]{30,}`)
	privateKeyPattern     = regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`)
	secretPatterns        = []*regexp.Regexp{keyValueSecretPattern, bearerSecretPattern, datadogKeyPattern, privateKeyPattern}
)

func RedactString(input string) string {
	out := input
	out = keyValueSecretPattern.ReplaceAllString(out, "$1$2[REDACTED]")
	out = bearerSecretPattern.ReplaceAllString(out, "$1[REDACTED]")
	out = datadogKeyPattern.ReplaceAllString(out, "[REDACTED]")
	out = privateKeyPattern.ReplaceAllString(out, "[REDACTED PRIVATE KEY]")
	return out
}

func RedactBytes(input []byte) []byte {
	return []byte(RedactString(string(input)))
}

func ContainsSecret(input string) bool {
	for _, pattern := range secretPatterns {
		if pattern.MatchString(input) {
			return true
		}
	}
	return false
}
