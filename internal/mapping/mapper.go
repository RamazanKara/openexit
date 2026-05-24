package mapping

import (
	"context"

	"github.com/RamazanKara/openexit/internal/assessment"
	"github.com/RamazanKara/openexit/internal/inventory"
)

type Mapper interface {
	Name() string
	Source() string
	Target() string
	Map(ctx context.Context, inv *inventory.Inventory, findings []assessment.Finding) (*MappingResult, error)
}

type MappingResult struct {
	TargetType       string         `json:"targetType" yaml:"targetType"`
	DashboardDrafts  []DashboardMap `json:"dashboardDrafts" yaml:"dashboardDrafts"`
	AlertRuleDrafts  []AlertRuleMap `json:"alertRuleDrafts" yaml:"alertRuleDrafts"`
	UnsupportedItems []Unsupported  `json:"unsupportedItems" yaml:"unsupportedItems"`
	ManualReview     []ManualReview `json:"manualReview" yaml:"manualReview"`
}

type DashboardMap struct {
	SourceID string `json:"sourceId" yaml:"sourceId"`
	Title    string `json:"title" yaml:"title"`
	Level    int    `json:"level" yaml:"level"`
	Path     string `json:"path" yaml:"path"`
}

type AlertRuleMap struct {
	SourceID     string `json:"sourceId" yaml:"sourceId"`
	Alert        string `json:"alert" yaml:"alert"`
	Path         string `json:"path" yaml:"path"`
	ReviewNeeded bool   `json:"reviewNeeded" yaml:"reviewNeeded"`
}

type Unsupported struct {
	SourceID string `json:"sourceId" yaml:"sourceId"`
	Type     string `json:"type" yaml:"type"`
	Reason   string `json:"reason" yaml:"reason"`
}

type ManualReview struct {
	SourceID string `json:"sourceId" yaml:"sourceId"`
	Reason   string `json:"reason" yaml:"reason"`
}
