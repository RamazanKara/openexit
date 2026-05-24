package evidence

import "time"

type Provenance struct {
	GeneratedBy string    `json:"generatedBy" yaml:"generatedBy"`
	GeneratedAt time.Time `json:"generatedAt" yaml:"generatedAt"`
	Source      string    `json:"source" yaml:"source"`
}
