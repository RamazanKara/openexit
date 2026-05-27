package datadog

import "testing"

func TestDatadogQueryTagsExtractsFilterAndGroupTags(t *testing.T) {
	tags := datadogQueryTags("sum(last_5m):avg:trace.http.request.errors{env:prod,!pod_name:*,region:$region} by {service,team}")
	for _, expected := range []string{"env", "pod_name", "region", "service", "team"} {
		if !hasAll(tags, expected) {
			t.Fatalf("expected tag %q in %v", expected, tags)
		}
	}
}
