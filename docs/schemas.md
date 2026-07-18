# Schemas

OpenExit publishes Draft 7 JSON Schemas under `schemas/` and embeds them in release binaries.

## Datadog v0.1

| Schema | Generated document |
| --- | --- |
| `openexit.datadog-inventory.schema.json` | `.openexit/inventory/datadog.inventory.json` |
| `openexit.datadog-plan.schema.json` | `.openexit/plan/openexit.plan.json` |
| `openexit.datadog-validation.schema.json` | `.openexit/validation/validation.json` |
| `openexit.migration-bundle.schema.json` | `migration/manifest.json` |

The Datadog inventory uses `kind: DatadogInventory` and catalog version `datadog-observability/v1`. It records scan metadata, endpoint-level coverage, stable source references, dependencies, redacted specs, and evidence paths/digests.

The plan uses `kind: DatadogMigrationPlan`, target `grafana-lgtm`, and ruleset `datadog-grafana-lgtm/v1`. It records the deterministic plan ID, inventory digest, conversion summary, transparent readiness factors, and one conversion decision per source resource. Decisions include status, reason codes, semantic changes, component-level results, and output links.

The validation document records every critical or advisory internal check. Export is permitted only when critical checks pass.

The migration bundle manifest records OpenExit build metadata and every payload file’s relative path, size, SHA-256 digest, and source references where applicable. `SHA256SUMS` additionally covers the manifest.

## Legacy and Release Schemas

The earlier project, generic inventory, assessment, mapping, plan, validation, and evidence-bundle schemas remain embedded for the experimental multi-provider engine. `openexit.release-manifest.schema.json` remains the release-artifact contract used with `SHA256SUMS`.
