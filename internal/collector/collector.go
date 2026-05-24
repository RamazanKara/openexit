package collector

import (
	"context"

	"github.com/RamazanKara/openexit/internal/inventory"
)

type CollectRequest struct {
	ProjectDir string
	Project    string
	Source     string
	Options    map[string]string
}

type Collector interface {
	Name() string
	Collect(ctx context.Context, req CollectRequest) (*inventory.Inventory, error)
}
