package assessment

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/RamazanKara/openexit/internal/inventory"
)

const (
	APIVersion = "openexit.dev/v1alpha1"
	Kind       = "Assessment"
)

type Assessment struct {
	APIVersion string    `json:"apiVersion" yaml:"apiVersion"`
	Kind       string    `json:"kind" yaml:"kind"`
	Metadata   Metadata  `json:"metadata" yaml:"metadata"`
	Source     Endpoint  `json:"source" yaml:"source"`
	Target     Endpoint  `json:"target" yaml:"target"`
	Score      Score     `json:"score" yaml:"score"`
	Findings   []Finding `json:"findings" yaml:"findings"`
	Warnings   []string  `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

type Metadata struct {
	Project     string    `json:"project" yaml:"project"`
	GeneratedAt time.Time `json:"generatedAt" yaml:"generatedAt"`
}

type Endpoint struct {
	Type string `json:"type" yaml:"type"`
}

type Score struct {
	Value               int      `json:"value" yaml:"value"`
	Level               string   `json:"level" yaml:"level"`
	AutomationPotential string   `json:"automationPotential" yaml:"automationPotential"`
	ManualReview        string   `json:"manualReviewRequired" yaml:"manualReviewRequired"`
	Drivers             []string `json:"drivers" yaml:"drivers"`
}

type Finding struct {
	ID             string   `json:"id" yaml:"id"`
	Severity       string   `json:"severity" yaml:"severity"`
	Title          string   `json:"title" yaml:"title"`
	Description    string   `json:"description" yaml:"description"`
	AffectedAssets []string `json:"affectedAssets" yaml:"affectedAssets"`
	EvidenceRefs   []string `json:"evidenceRefs" yaml:"evidenceRefs"`
	Recommendation string   `json:"recommendation" yaml:"recommendation"`
}

type Analyzer interface {
	Name() string
	Analyze(ctx context.Context, inv *inventory.Inventory) ([]Finding, error)
}

func New(project, source, target string, generatedAt time.Time, findings []Finding, score Score) *Assessment {
	return &Assessment{
		APIVersion: APIVersion,
		Kind:       Kind,
		Metadata: Metadata{
			Project:     project,
			GeneratedAt: generatedAt.UTC(),
		},
		Source:   Endpoint{Type: source},
		Target:   Endpoint{Type: target},
		Score:    score,
		Findings: findings,
	}
}

func Validate(a *Assessment) error {
	var problems []string
	if a.APIVersion != APIVersion {
		problems = append(problems, fmt.Sprintf("apiVersion must be %q", APIVersion))
	}
	if a.Kind != Kind {
		problems = append(problems, fmt.Sprintf("kind must be %q", Kind))
	}
	if strings.TrimSpace(a.Metadata.Project) == "" {
		problems = append(problems, "metadata.project is required")
	}
	for _, finding := range a.Findings {
		if finding.ID == "" || finding.Severity == "" || finding.Title == "" {
			problems = append(problems, "each finding requires id, severity, and title")
		}
		if len(finding.AffectedAssets) == 0 {
			problems = append(problems, fmt.Sprintf("finding %q requires affectedAssets", finding.ID))
		}
		if len(finding.EvidenceRefs) == 0 {
			problems = append(problems, fmt.Sprintf("finding %q requires evidenceRefs", finding.ID))
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func SortFindings(findings []Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].ID == findings[j].ID {
			return strings.Join(findings[i].AffectedAssets, ",") < strings.Join(findings[j].AffectedAssets, ",")
		}
		return findings[i].ID < findings[j].ID
	})
}
