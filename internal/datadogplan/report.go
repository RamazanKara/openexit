package datadogplan

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
)

type reportData struct {
	Inventory  *Inventory
	Plan       *MigrationPlan
	Validation *ValidationReport
}

func renderReport(stage string, inv *Inventory, plan *MigrationPlan, validation *ValidationReport) error {
	functions := template.FuncMap{
		"pct":         func(value float64) string { return fmt.Sprintf("%.0f%%", value*100) },
		"statusClass": func(value string) string { return "status-" + strings.ReplaceAll(value, "_", "-") },
		"join":        strings.Join,
	}
	tmpl, err := template.New("report").Funcs(functions).Parse(reportTemplate)
	if err != nil {
		return err
	}
	path := filepath.Join(stage, ReportRel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	execErr := tmpl.Execute(file, reportData{Inventory: inv, Plan: plan, Validation: validation})
	closeErr := file.Close()
	if execErr != nil {
		return execErr
	}
	if closeErr != nil {
		return closeErr
	}
	return renderBundleReadme(stage, inv, plan, validation)
}

func renderBundleReadme(stage string, inv *Inventory, plan *MigrationPlan, validation *ValidationReport) error {
	validationStatus := "not run"
	if validation != nil {
		validationStatus = validation.Status
	}
	text := fmt.Sprintf(`# OpenExit Datadog Migration Plan

This directory is a deterministic, read-only review bundle for a Datadog to Grafana LGTM, Prometheus, Alloy, and OpenTelemetry migration.

- Inventory digest: %s
- Plan ID: %s
- Exit readiness: %d/100 (%s)
- Validation: %s
- Resources: %d
- Exact: %d
- Approximate: %d
- Manual: %d
- Unsupported: %d

Open [index.html](index.html) for the complete migration report.

Generated target files are candidates only. OpenExit does not deploy them, mutate Datadog, or make a cutover decision.
`, inv.Metadata.SnapshotDigest, plan.Metadata.PlanID, plan.Readiness.Score, plan.Readiness.Level, validationStatus, plan.Summary.Total, plan.Summary.Exact, plan.Summary.Approximate, plan.Summary.Manual, plan.Summary.Unsupported)
	return WriteText(filepath.Join(stage, BundleReadmeRel), text)
}

const reportTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="color-scheme" content="light dark">
  <title>OpenExit Datadog Migration Plan</title>
  <style>
    :root { --bg:#f6f7fb; --panel:#fff; --text:#172033; --muted:#5f6b7a; --line:#dce1ea; --accent:#6d4aff; --exact:#16865b; --approx:#a36b00; --manual:#b64c20; --unsupported:#7b405e; --failed:#b42318; }
    @media (prefers-color-scheme: dark) { :root { --bg:#10131a; --panel:#181d27; --text:#edf1f7; --muted:#a9b2c2; --line:#303746; --accent:#a997ff; --exact:#51d49a; --approx:#f3bd55; --manual:#ff966c; --unsupported:#e29ab9; --failed:#ff7b72; } }
    * { box-sizing:border-box; }
    body { margin:0; background:var(--bg); color:var(--text); font:15px/1.5 ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif; }
    main { max-width:1240px; margin:0 auto; padding:40px 24px 80px; }
    h1,h2,h3 { line-height:1.2; margin-top:0; }
    h1 { font-size:clamp(30px,5vw,52px); letter-spacing:-.04em; max-width:900px; }
    h2 { margin-top:44px; font-size:25px; }
    a { color:var(--accent); }
    code { font:13px ui-monospace,SFMono-Regular,Consolas,monospace; overflow-wrap:anywhere; }
    .lede { color:var(--muted); max-width:820px; font-size:18px; }
    .grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(190px,1fr)); gap:14px; margin:24px 0; }
    .card { background:var(--panel); border:1px solid var(--line); border-radius:14px; padding:18px; box-shadow:0 8px 30px rgb(20 30 50 / 5%); }
    .metric { font-size:32px; font-weight:750; letter-spacing:-.03em; }
    .label,.muted { color:var(--muted); }
    .score { display:flex; align-items:center; gap:18px; }
    .score-ring { display:grid; place-items:center; width:112px; height:112px; border-radius:50%; border:12px solid var(--accent); font-size:28px; font-weight:800; flex:none; }
    .pill { display:inline-block; border:1px solid currentColor; border-radius:999px; padding:2px 9px; font-size:12px; font-weight:700; text-transform:uppercase; letter-spacing:.04em; }
    .status-exact,.status-passed,.status-complete,.status-empty,.status-not-available { color:var(--exact); }
    .status-approximate,.status-warning { color:var(--approx); }
    .status-manual,.status-partial,.status-permission-denied { color:var(--manual); }
    .status-unsupported { color:var(--unsupported); }
    .status-failed,.status-error { color:var(--failed); }
    table { width:100%; border-collapse:collapse; background:var(--panel); border:1px solid var(--line); border-radius:14px; overflow:hidden; display:block; overflow-x:auto; }
    th,td { padding:12px 14px; border-bottom:1px solid var(--line); text-align:left; vertical-align:top; }
    th { color:var(--muted); font-size:12px; text-transform:uppercase; letter-spacing:.06em; }
    tbody tr:last-child td { border-bottom:0; }
    details { background:var(--panel); border:1px solid var(--line); border-radius:12px; margin:10px 0; padding:12px 15px; }
    summary { cursor:pointer; font-weight:700; }
    ul { padding-left:20px; }
    .warning { border-left:4px solid var(--manual); }
    .resource-links a { display:block; }
    .nowrap { white-space:nowrap; }
    footer { margin-top:52px; color:var(--muted); border-top:1px solid var(--line); padding-top:20px; }
  </style>
</head>
<body>
<main>
  <header>
    <div class="pill">Read-only migration plan</div>
    <h1>Datadog → Grafana, Prometheus &amp; OpenTelemetry</h1>
    <p class="lede">A deterministic inventory and conversion report. Every candidate is linked to redacted source evidence and every semantic change is made explicit.</p>
  </header>

  <section class="grid">
    <div class="card score"><div class="score-ring">{{.Plan.Readiness.Score}}</div><div><div class="label">Exit readiness</div><div class="metric">{{.Plan.Readiness.Level}}</div></div></div>
    <div class="card"><div class="metric">{{.Plan.Summary.Total}}</div><div class="label">Source resources</div></div>
    <div class="card"><div class="metric status-exact">{{.Plan.Summary.Exact}}</div><div class="label">Exact</div></div>
    <div class="card"><div class="metric status-approximate">{{.Plan.Summary.Approximate}}</div><div class="label">Approximate</div></div>
    <div class="card"><div class="metric status-manual">{{.Plan.Summary.Manual}}</div><div class="label">Manual</div></div>
    <div class="card"><div class="metric status-unsupported">{{.Plan.Summary.Unsupported}}</div><div class="label">Unsupported</div></div>
  </section>

  <section class="card warning">
    <strong>This is not production readiness.</strong> {{.Plan.Readiness.Interpretation}} OpenExit performs no Datadog writes, deployment, or automatic cutover.
  </section>

  <h2>Readiness calculation</h2>
  <div class="grid">
    <div class="card"><div class="metric">{{pct .Plan.Readiness.Collection.Value}}</div><div class="label">Collection · {{.Plan.Readiness.Collection.Numerator}}/{{.Plan.Readiness.Collection.Denominator}}</div></div>
    <div class="card"><div class="metric">{{pct .Plan.Readiness.Translation.Value}}</div><div class="label">Translation · {{.Plan.Readiness.Translation.Numerator}}/{{.Plan.Readiness.Translation.Denominator}}</div></div>
    <div class="card"><div class="metric">{{pct .Plan.Readiness.Validation.Value}}</div><div class="label">Validation · {{.Plan.Readiness.Validation.Numerator}}/{{.Plan.Readiness.Validation.Denominator}}</div></div>
  </div>
  <p><code>{{.Plan.Readiness.Formula}}</code></p>
  {{if .Plan.Readiness.Deductions}}<ul>{{range .Plan.Readiness.Deductions}}<li><code>{{.Code}}</code> — {{.Description}}</li>{{end}}</ul>{{end}}

  <h2>Inventory coverage</h2>
  <p><a href="inventory/datadog.inventory.json">Machine-readable inventory</a> · Snapshot <code>{{.Inventory.Metadata.SnapshotDigest}}</code></p>
  <table><thead><tr><th>Family</th><th>Status</th><th>Resources</th><th>Notes</th></tr></thead><tbody>
  {{range .Inventory.Catalog.Coverage}}<tr><td>{{.Family}}</td><td><span class="pill {{statusClass .Status}}">{{.Status}}</span></td><td>{{.Count}}</td><td>{{.Message}}<details><summary>{{len .Endpoints}} endpoint check(s)</summary><ul>{{range .Endpoints}}<li><code>{{.Path}}</code> — <span class="pill {{statusClass .Status}}">{{.Status}}</span> · {{.Count}} resource(s) {{.Message}}</li>{{end}}</ul></details></td></tr>{{end}}
  </tbody></table>

  <h2>Conversion ledger</h2>
  <p><a href="plan/openexit.plan.json">Machine-readable migration plan</a> · Plan <code>{{.Plan.Metadata.PlanID}}</code></p>
  <table><thead><tr><th>Source</th><th>Status</th><th>Decision</th><th>Evidence &amp; outputs</th></tr></thead><tbody>
  {{range .Plan.Resources}}
    <tr id="{{.SourceRef}}">
      <td><strong>{{.SourceName}}</strong><br><code>{{.SourceRef}}</code><br><span class="muted">{{.SourceKind}}</span></td>
      <td class="nowrap"><span class="pill {{statusClass .Status}}">{{.Status}}</span></td>
      <td>{{.Summary}}<br><span class="muted"><code>{{join .ReasonCodes ", "}}</code></span>
        {{if .SemanticChanges}}{{range .SemanticChanges}}<details><summary>{{.Code}}</summary><p>{{.Description}}</p><p class="muted">Impact: {{.Impact}}</p></details>{{end}}{{end}}
        {{if .Components}}<details><summary>{{len .Components}} component result(s)</summary><ul>{{range .Components}}<li><span class="pill {{statusClass .Status}}">{{.Status}}</span> <code>{{.ID}}</code> · {{.Kind}}{{if .ReasonCodes}}<br><span class="muted"><code>{{join .ReasonCodes ", "}}</code></span>{{end}}{{if .SourceQuery}}<br><strong>Datadog:</strong> <code>{{.SourceQuery}}</code>{{end}}{{if .TargetQuery}}<br><strong>Candidate:</strong> <code>{{.TargetQuery}}</code>{{end}}{{if .Review}}<br>{{.Review}}{{end}}</li>{{end}}</ul></details>{{end}}
      </td>
      <td class="resource-links"><a href="{{.EvidencePath}}">Source evidence</a>{{if .SourceURL}}<a href="{{.SourceURL}}">Open in Datadog</a>{{end}}{{range .Outputs}}<a href="{{.Path}}">{{.Kind}}</a>{{end}}</td>
    </tr>
  {{end}}
  </tbody></table>

  <h2>Validation</h2>
  <p>Overall status: <span class="pill {{statusClass .Validation.Status}}">{{.Validation.Status}}</span> · <a href="validation/validation.json">Machine-readable validation</a></p>
  <table><thead><tr><th>Check</th><th>Status</th><th>Message</th></tr></thead><tbody>
  {{range .Validation.Checks}}<tr><td>{{.Name}}</td><td><span class="pill {{statusClass .Status}}">{{.Status}}</span></td><td>{{.Message}}</td></tr>{{end}}
  </tbody></table>

  <h2>Generated candidates</h2>
  <ul>
    <li><a href="generated/alloy/config.alloy">Grafana Alloy candidate</a></li>
    <li><a href="generated/opentelemetry/collector.yaml">OpenTelemetry Collector candidate</a></li>
  </ul>

  <footer>Generated deterministically by OpenExit {{.Inventory.Metadata.CollectorVersion}} from a read-only Datadog scan collected at {{.Inventory.Metadata.CollectedAt}}.</footer>
</main>
</body>
</html>
`
