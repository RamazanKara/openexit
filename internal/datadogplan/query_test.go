package datadogplan

import "testing"

func FuzzDatadogQueries(f *testing.F) {
	for _, query := range []string{
		"", "avg:system.cpu.user{env:prod} by {host}",
		"sum(last_5m):avg:trace.http.request.errors{env:prod} > 10",
		"sum(last_5m):sum:requests{*}.as_count() >= 1",
		"avg:metric{env:prod*}", "anomalies(avg:metric{*}, 'basic', 2)",
	} {
		f.Add(query)
	}
	f.Fuzz(func(t *testing.T, query string) {
		for _, convert := range []func(string) queryConversion{convertDashboardMetricQuery, convertMonitorQuery} {
			result := convert(query)
			if result != convert(query) {
				t.Fatal("query conversion is not deterministic")
			}
			if result.ReasonCode == "" || result.Review == "" || (result.OK != (result.Expr != "")) {
				t.Fatalf("conversion must have a reason and only accepted queries may emit an expression: %+v", result)
			}
		}
	})
}
