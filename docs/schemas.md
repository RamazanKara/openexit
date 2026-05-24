# Schemas

OpenExit schemas live under `schemas/` and mirror the typed Go manifests. The CLI performs typed validation plus YAML/JSON parse checks in v0.1.

Inventory dashboards can include optional `dataSources` and `templateVariables` fields so assessment can flag Grafana mapping risk. SLOs can include optional `sli`, `burnRateMonitorIds`, and `dashboardRefs` fields. The top-level inventory `volumes` section records whether log and trace volume assumptions are known.
