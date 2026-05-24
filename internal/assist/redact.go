package assist

import "github.com/RamazanKara/openexit/internal/inventory"

func Redact(input string) string {
	return inventory.RedactString(input)
}
