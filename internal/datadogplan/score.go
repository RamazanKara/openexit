package datadogplan

import (
	"fmt"
	"math"
)

func Score(inv *Inventory, conversions []Conversion, validation *ValidationReport) Readiness {
	collectionNumerator := 0
	for _, family := range inv.Catalog.Coverage {
		if coverageSatisfied(family.Status) {
			collectionNumerator++
		}
	}
	collectionDenominator := len(inv.Catalog.Coverage)
	collection := ratio(collectionNumerator, collectionDenominator, 1)

	statusCounts := map[string]int{}
	translationNumerator := 0
	for _, conversion := range conversions {
		statusCounts[conversion.Status]++
		translationNumerator += int(math.Round(conversionWeight(conversion.Status) * 2))
	}
	translationDenominator := len(conversions) * 2
	translation := ratio(translationNumerator, translationDenominator, 1)

	validationNumerator, validationDenominator := validationCounts(validation)
	validationValue := ratio(validationNumerator, validationDenominator, 1)
	raw := 100 * collection * (0.9*translation + 0.1*validationValue)
	score := int(math.Round(raw))
	criticalFailure := false
	if validation != nil {
		for _, check := range validation.Checks {
			if check.Critical && check.Status == "failed" {
				criticalFailure = true
				break
			}
		}
	}
	if criticalFailure && score > 49 {
		score = 49
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	level := "low"
	if score >= 80 {
		level = "high"
	} else if score >= 50 {
		level = "medium"
	}
	// Keep this as an empty JSON array when there are no deductions. The public
	// plan schema deliberately rejects null so consumers can iterate it safely.
	deductions := []ScoreDeduction{}
	if collection < 1 {
		deductions = append(deductions, ScoreDeduction{Code: "inventory.incomplete", Description: fmt.Sprintf("%d of %d catalog families completed", collectionNumerator, collectionDenominator), Points: int(math.Round(100 * (1 - collection)))})
	}
	for _, status := range []string{StatusApproximate, StatusManual, StatusUnsupported} {
		if statusCounts[status] > 0 {
			deductions = append(deductions, ScoreDeduction{Code: "conversion." + status, Description: fmt.Sprintf("%d resources have %s conversion status", statusCounts[status], status), Points: statusCounts[status]})
		}
	}
	if criticalFailure {
		deductions = append(deductions, ScoreDeduction{Code: "validation.critical", Description: "A critical validation check failed; the score is capped at 49", Points: 51})
	}
	return Readiness{
		Score:          score,
		Level:          level,
		Formula:        "round(100 × C × (0.9 × T + 0.1 × V)); exact=1.0, approximate=0.5, manual=0, unsupported=0",
		Collection:     ReadinessFactor{Value: collection, Numerator: collectionNumerator, Denominator: collectionDenominator, Description: "Successfully evaluated catalog families"},
		Translation:    ReadinessFactor{Value: translation, Numerator: translationNumerator, Denominator: translationDenominator, Description: "Resource conversion points; exact=2, approximate=1, manual/unsupported=0"},
		Validation:     ReadinessFactor{Value: validationValue, Numerator: validationNumerator, Denominator: validationDenominator, Description: "Required internal validation checks passed"},
		Deductions:     deductions,
		Interpretation: "Exit readiness measures deterministic inventory and translation coverage. It is not production readiness or cutover approval.",
	}
}

func conversionWeight(status string) float64 {
	switch status {
	case StatusExact:
		return 1
	case StatusApproximate:
		return 0.5
	default:
		return 0
	}
}

func validationCounts(report *ValidationReport) (int, int) {
	if report == nil {
		return 0, 0
	}
	passed := 0
	total := 0
	for _, check := range report.Checks {
		if !check.Critical {
			continue
		}
		total++
		if check.Status == "passed" {
			passed++
		}
	}
	return passed, total
}

func ratio(numerator, denominator int, empty float64) float64 {
	if denominator == 0 {
		return empty
	}
	return float64(numerator) / float64(denominator)
}
