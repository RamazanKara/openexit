package assist

import "strings"

func ReviewHeader() string {
	return "> AI-assisted draft.\n> Source of truth: structured OpenExit inventory and assessment.\n> Review required before operational use.\n"
}

func EnsureReviewHeader(text string) string {
	text = strings.TrimSpace(text)
	header := strings.TrimSpace(ReviewHeader())
	if strings.HasPrefix(text, header) {
		return text
	}
	if text == "" {
		return ReviewHeader()
	}
	return ReviewHeader() + "\n" + text
}
