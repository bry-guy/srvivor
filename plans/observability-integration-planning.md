# Observability integration plan

Status: planning

## Goal

Build an integrated, low-noise observability loop for the homelab so the admin
can answer three questions quickly:

1. **What is available?** Homepage at `https://home.bry-guy.net/` should list every important app, admin surface, platform component, and infrastructure endpoint.
2. **Is it healthy?** Grafana at `https://admin.bry-guy.net/` should show health, errors, latency, and capacity across apps, k3s, the Pi witness, Caddy edges, and Proxmox-adjacent infrastructure.
3. **Is it being used?** Grafana should show usage trends for Castaway, Homepage, admin/status surfaces, platform APIs, and external status checks without turning every user action into noisy alerts.

This plan refactors the earlier high-signal Castaway observability plan into a
broader **observability integration** plan. The original Castaway metrics/logs
remain the application instrumentation baseline.

## Operating model

- **Homepage is the catalog / entrypoint.** It answers, “What exists and where do I go?”
- **Grafana is the observability UI.** It answers, “What is happening and why?”
- **Gatus is external uptime/status.** It answers, “Can the important endpoints be reached from the Pi witness?”
- **Prometheus is metrics.** It powers dashboards and alerts.
- **Loki is logs.** It powers incident drill-down and request/command traces.
- **OpenTelemetry is the app instrumentation path.** It is preferred for new app metrics/logs/traces when practical, but direct Prometheus metrics are fine for simple Go services.

Do not expose Prometheus, Loki, Alertmanager, Argo CD, or app-internal metrics as
new public routes. Use Grafana, Homepage links, Gatus, and `kubectl port-forward`
for direct debugging.

## Current state

### Already available

- `admin.bry-guy.net` tailnet-only HTTPS Caddy route to Grafana.
- `status.bry-guy.net` tailnet-only HTTPS Caddy route to Gatus on the Pi witness.
- `home.bry-guy.net` tailnet-only HTTPS Caddy route to Homepage on k3s.
- k3s observability stack:
  - Grafana
  - Prometheus / kube-prometheus-stack
  - Alertmanager
  - Loki
  - Promtail
  - OpenTelemetry Collector
  - postgres-exporter
- Kubernetes pod/deployment/node metrics.
- Kubernetes logs in Loki via Promtail.
- Platform PostgreSQL metrics via postgres-exporter.
- Gatus checks for core machines and witness qnetd.
- Homepage Kubernetes integration is deployed with a ClusterIP-only service.

### Important gaps

#### Homepage gaps

- Homepage is listed as an app, but its own usage is not visible in Grafana.
- Caddy edge metrics/logs for `home.bry-guy.net`, `admin.bry-guy.net`, and `status.bry-guy.net` are not scraped/centralized.
- Homepage app logs are only pod logs; there is no dashboard for requests, errors, or popular links.
- Homepage services are manually listed and can drift from actual homelab inventory.
- Some services lack links because they are intentionally private/debug-only.

#### Castaway gaps

- `castaway-web` has `/healthz` and probes, but no `/metrics`.
- Gin default logs are noisy and dominated by `/healthz`.
- No stable request metrics by route/method/status.
- No request latency histogram.
- No app-level DB health or DB pool saturation metrics.
- No Castaway dashboard.
- `castaway-discord-bot` has no `/healthz`, `/metrics`, Service, ServiceMonitor, command metrics, or bot dashboard.

#### Pi witness / status gaps

- Gatus metrics are enabled but not scraped by Prometheus.
- Gatus does not yet check `home.bry-guy.net`.
- Prometheus does not scrape Pi witness host metrics.
- qnetd/corosync has TCP up/down only; no host/process metric.
- Blocky is disabled, so DNS usage is not observable yet.

#### Edge / access gaps

- Caddy request logs for admin/home/status are not in Loki.
- Caddy Prometheus metrics are not scraped.
- There is no unified view of edge usage:
  - requests/min by host
  - 4xx/5xx by host
  - p95 edge latency by host
  - TLS cert expiry / reload success

#### Infrastructure gaps

- Proxmox health is mostly checked by existing scripts and Gatus TCP checks, not Grafana.
- VM-level homelab usage is visible indirectly from k3s node metrics but not clearly tied to homelab services.
- Pi witness and Caddy appliance are not represented as first-class dashboard rows.

## Design principles

- Optimize for **high-signal / low-noise**.
- Prefer metrics with stable low-cardinality labels.
- Do not label by user ID, Discord guild ID, participant name, raw URL path with IDs, error message, token, or request body.
- Prefer route templates:
  - `/instances/:instanceID/leaderboard`
  - `/activities/:activityID/occurrences`
- Prefer bounded result labels:
  - `ok`
  - `user_error`
  - `dependency_error`
  - `internal_error`
- Keep secrets out of logs and Homepage config.
- Homepage should link to admin surfaces, but not create new unauthenticated bypasses.
- Grafana dashboards should be few and task-oriented:
  - Homelab overview
  - Edge / entrypoints
  - Castaway overview
  - Witness / status overview
  - Platform capacity

## Desired Homepage catalog

Homepage should show every service that matters to operating or using the homelab.

### Start

- Home — `https://home.bry-guy.net/`
- Admin / Grafana — `https://admin.bry-guy.net/`
- Status / Gatus — `https://status.bry-guy.net/`

### Applications

- Castaway Web API
- Castaway Discord Bot
- Future app services as they are deployed

### Platform

- Argo CD — linked with note: use port-forward for direct UI if not exposed
- Grafana
- Prometheus — link to Grafana Explore, not direct Prometheus UI
- Loki — link to Grafana Explore, not direct Loki UI
- Alertmanager — link to Grafana alerting, not direct Alertmanager UI
- OpenTelemetry Collector
- PostgreSQL Exporter
- Shared PostgreSQL

### Infrastructure

- SFFPC Proxmox
- OptiPlex Proxmox
- Pi witness / qnetd / Gatus
- JetKVM witness
- Router
- Caddy appliance edge

### Machines

- `us-west-appliance-500`
- `us-west-service-300`
- `us-west-stateful-400`
- `us-west-agent-600`
- SFFPC Proxmox node
- OptiPlex Proxmox node
- Pi witness

### Homepage catalog maintenance rules

- Every new homelab service must define:
  - name
  - group
  - owner/source repo
  - URL or explicit “no direct URL” note
  - health source: Kubernetes, Gatus, Prometheus metric, or script
  - Grafana dashboard link when one exists
- Avoid Homepage widgets that require API keys until secret handling is designed.
- Prefer Kubernetes status integration for in-cluster services.
- Prefer Gatus status links for external/tailnet services.

## Metrics and logs by layer

### 1. Edge layer: Caddy for admin/home/status

Purpose: understand usage of the homelab entrypoints.

Collect:

- Caddy Prometheus metrics from the admin endpoint or a private metrics handler.
- Caddy JSON access logs for:
  - `home.bry-guy.net`
  - `admin.bry-guy.net`
  - `status.bry-guy.net`

Prometheus signals:

- requests/min by host and status class
- p95 request duration by host
- 4xx/5xx by host
- Caddy reload success
- Caddy TLS certificate expiry if exposed/available

Log fields:

- host
- method
- path template or raw path with query stripped
- status
- duration
- user agent family if easy
- remote tailnet IP, not treated as identity

Do not log request bodies, headers containing auth, cookies, or tokens.

Implementation options:

1. Scrape Caddy `localhost:2019/metrics` from k3s Prometheus through a tailnet/static target.
2. Or expose a Caddy metrics handler bound only to the Tailscale IP and allow only k3s node/pod source ranges.
3. Ship Caddy JSON logs to Loki using Promtail on the appliance host, or a lightweight systemd-journal scraper.

Recommended first step: scrape Caddy metrics, then add logs if metrics show useful traffic.

### 2. Homepage layer

Purpose: understand whether the homelab entrypoint is healthy and used.

Collect:

- Kubernetes deployment health for `homepage`.
- Caddy edge metrics for `host=home.bry-guy.net`.
- Homepage pod logs in Loki.
- Optional app-level metrics if Homepage exposes useful metrics later.

Dashboard panels:

- Homepage pod ready/restarts.
- Requests/min to `home.bry-guy.net` from Caddy metrics.
- p95 response time for `home.bry-guy.net`.
- 4xx/5xx for `home.bry-guy.net`.
- Top requested paths from Caddy logs, excluding static assets if possible.

Gatus:

- Add `home-page`: `https://home.bry-guy.net/api/healthcheck`, expect 200.

Homepage itself does not need a custom app dashboard unless edge metrics/logs show real usage or errors.

### 3. Castaway web

Purpose: understand API usage, latency, and failures.

Add Prometheus instrumentation directly to `castaway-web`:

- `GET /metrics`
- `castaway_web_http_requests_total{method,route,status_class}`
- `castaway_web_http_request_duration_seconds_bucket{method,route}`
- `castaway_web_http_in_flight_requests`
- DB pool metrics from `pgxpool.Stat()`:
  - acquired connections
  - idle connections
  - max connections
  - acquire count/duration if practical
- `castaway_web_db_up`

Logging:

- Replace Gin default logger with structured request logs.
- Suppress successful `/healthz`.
- Log one summary per non-health request with:
  - service
  - method
  - route
  - status
  - status_class
  - duration_ms
  - request_id
  - error_class when relevant

Deployment:

- Add ServiceMonitor.
- Add OTEL resource env vars for future tracing/log correlation:
  - `OTEL_SERVICE_NAME=castaway-web`
  - `OTEL_RESOURCE_ATTRIBUTES=deployment.environment=home-k3s,service.namespace=castaway`

### 4. Castaway Discord bot

Purpose: understand whether Discord commands are being used and whether failures are user, app, Discord, DNS, or API dependency related.

Add a small HTTP server on `:8080`:

- `GET /healthz`
- `GET /metrics`

Metrics:

- `castaway_bot_commands_total{group,command,result}`
- `castaway_bot_command_duration_seconds_bucket{group,command}`
- `castaway_bot_api_requests_total{method,route,status_class,result}`
- `castaway_bot_api_request_duration_seconds_bucket{method,route}`
- `castaway_bot_discord_gateway_connected`
- `castaway_bot_discord_gateway_reconnects_total` if discordgo exposes a reliable hook

Logs:

- One command completion summary per command:
  - service
  - group
  - command
  - result
  - duration_ms
  - dependency_error_class when relevant

Deployment:

- Add Service.
- Add readiness/liveness probes.
- Add ServiceMonitor.
- Add OTEL resource env vars.

### 5. Gatus / Pi witness / qnetd

Purpose: understand external/tailnet reachability and witness health.

Add Gatus endpoints:

- `home-page`: `https://home.bry-guy.net/api/healthcheck`
- `admin-grafana`: `https://admin.bry-guy.net/login`
- `status-page`: existing local Gatus status check
- `castaway-web-health`: HTTP `/healthz` when reachable from witness
- `castaway-discord-bot-health`: bot `/healthz` after bot endpoint exists
- keep existing Proxmox API TCP checks
- keep existing qnetd TCP check

Scrape Gatus metrics from Prometheus:

- Use static target or Kubernetes `Endpoints` + `ServiceMonitor` pointing to the Pi witness Tailscale IP.
- Keep labels stable: endpoint name, group, status.

Dashboard panels:

- endpoint up/down table
- success rate by group
- p95 check latency by endpoint
- current failing checks

qnetd/corosync:

- Start with Gatus TCP up/down.
- Optional later: node-exporter textfile metric from `systemctl is-active corosync-qnetd`.

Pi host:

- Add node-exporter if easy in the Pi witness OS config.
- Alert only on host down or disk almost full.

Blocky:

- No dashboard while disabled.
- When enabled, add:
  - Blocky `/metrics` scrape
  - queries/sec
  - blocked queries/sec
  - upstream error rate
  - p95 DNS response time
  - DNS health Gatus check

### 6. Platform / k3s / shared services

Purpose: understand capacity, scheduling health, and platform component health.

Already mostly covered by kube-prometheus-stack. Add curated dashboard rows:

- Cluster node readiness.
- CPU/memory by node.
- PVC fullness.
- Pod restarts by namespace.
- Pending pods.
- Argo CD application health and sync status.
- PostgreSQL `pg_up`, connections, scrape errors.
- OTel Collector receiver/exporter errors.
- Loki ingestion and error rate if chart metrics are available.

Homepage should link these components to Grafana panels, not direct internal UIs.

### 7. Proxmox and physical infrastructure

Purpose: understand whether physical/VM substrate is healthy enough for homelab services.

Minimum first pass:

- Keep Gatus Proxmox API TCP checks.
- Keep existing `mise run selfhost:proxmox:check <node>` operational checks.
- Add dashboard panels based on Gatus endpoint health for Proxmox and qnetd.

Optional later:

- Add a Proxmox exporter with API token and secrets management.
- Track node online status, VM status, storage fullness, backup freshness.
- Add Homepage Proxmox widgets only after token storage is designed.

Do not add Proxmox API tokens to Homepage config as plain ConfigMap data.

## Dashboard set

### A. Homelab overview

The default Grafana landing dashboard.

Rows:

1. **Entry points**
   - Homepage up
   - Grafana/admin up
   - Status/Gatus up
   - Caddy 5xx by host
2. **Applications**
   - Castaway web up
   - Castaway bot up
   - command success rate
   - web 5xx rate
3. **Platform**
   - k3s nodes ready
   - Argo CD unhealthy/out-of-sync apps
   - pod restarts last 24h
   - PVCs near full
4. **Stateful**
   - PostgreSQL up
   - PostgreSQL connections
   - postgres-exporter scrape errors
5. **Witness / substrate**
   - Gatus failing endpoints
   - qnetd reachable
   - Proxmox API checks

### B. Edge / entrypoints

Rows:

- requests/min by host
- p95 latency by host
- status class by host
- top paths for home/admin/status
- Caddy reload success
- Caddy recent error logs

This is where Homepage stats appear in Grafana.

### C. Castaway overview

Rows:

1. **Health**
   - web pod ready / bot pod ready
   - web DB up
   - bot gateway connected
   - restarts last 24h
2. **Usage**
   - web requests/min by route top 5
   - Discord commands/min by command top 10
   - successful vs failed commands
3. **Latency**
   - web p95 HTTP latency
   - bot p95 command latency
   - bot -> web API p95 latency
4. **Errors**
   - web 5xx/min
   - bot dependency/internal errors/min
   - recent error logs

### D. Witness / status overview

Rows:

- Gatus endpoint table
- failing endpoints
- endpoint success rates
- endpoint p95 latency
- witness qnetd check
- optional Pi host CPU/memory/disk

### E. Platform capacity

Rows:

- k3s node CPU/memory
- namespace CPU/memory
- pod restarts
- PVC fullness
- Prometheus/Loki/OTel health
- PostgreSQL health

## Alerting policy

Alerts should indicate actionable admin work, not ordinary user mistakes.

Initial alerts:

- `HomepageDown`: Gatus `home-page` failing for 5m or Homepage deployment unavailable.
- `AdminGrafanaDown`: Gatus `admin-grafana` failing for 5m.
- `CaddyHigh5xx`: any edge host 5xx rate above threshold for 10m with minimum traffic.
- `CastawayWebDown`: deployment unavailable or `/healthz` failing for 5m.
- `CastawayBotDown`: deployment unavailable or bot health failing for 5m.
- `CastawayWebHighErrorRate`: 5xx > 5% for 10m and volume > 5 requests/10m.
- `CastawayBotCommandFailures`: dependency/internal failures > 3 in 15m.
- `CastawayWebHighLatency`: p95 > 1s for 15m and volume > 5 requests/15m.
- `GatusEndpointFailing`: important endpoint down for 5m.
- `WitnessQnetdDown`: qnetd TCP check failing for 5m.
- `PostgresDown`: reuse existing `pg_up == 0`.
- `PVCAlmostFull`: reuse existing PVC alert.

Avoid alerts for:

- individual 4xx/user errors
- single Discord command failures
- low-volume latency spikes
- Blocky while disabled
- Proxmox storage warning states that are known maintenance conditions

## Homepage integration workflow for new services

Every new service should include a small “observability contract” in its deploy
PR:

1. Homepage entry:
   - group
   - name
   - URL or explicit no-URL note
   - description
2. Health signal:
   - Kubernetes readiness, Gatus endpoint, or metrics gauge
3. Usage signal:
   - requests, commands, jobs, events, or another service-specific unit
4. Latency signal, if request-like
5. Error signal
6. Grafana dashboard row or existing dashboard mapping
7. Alert policy: alert, dashboard-only, or ignore

This keeps Homepage and Grafana aligned instead of treating them as separate
manual lists.

## Implementation phases

### Phase 1 — Homepage catalog and edge visibility

- Ensure all current homelab services appear in Homepage.
- Add Gatus check for `home.bry-guy.net`.
- Scrape Caddy metrics into Prometheus.
- Add Edge / entrypoints dashboard with Homepage request stats.
- Optional: ship Caddy access logs to Loki.

Done when:

- Grafana shows request rate/latency/errors for `home.bry-guy.net`.
- Homepage lists all currently known apps/platform/infrastructure services.
- Gatus checks home/admin/status.

### Phase 2 — Castaway instrumentation

- Add `castaway-web` metrics and structured logs.
- Add `castaway-discord-bot` health/metrics and structured command logs.
- Add ServiceMonitors.
- Add Castaway overview dashboard.
- Add low-noise Castaway alerts.

Done when:

- Prometheus has `castaway_web_http_requests_total`.
- Prometheus has `castaway_bot_commands_total`.
- Grafana answers whether commands are succeeding and where latency lives.

### Phase 3 — Witness/status integration

- Scrape Gatus metrics.
- Add Gatus dashboard.
- Add qnetd alert based on Gatus.
- Optionally add Pi witness node-exporter.

Done when:

- Grafana shows Gatus endpoint status and history.
- The Pi witness/qnetd state is visible without SSH.

### Phase 4 — Platform dashboard polish

- Add curated Homelab overview dashboard.
- Add Argo CD app health row.
- Add OTel Collector error panels.
- Add PostgreSQL usage panels.
- Add links from Homepage entries to relevant Grafana dashboards/panels.

Done when:

- The Grafana home dashboard can answer “what is broken?” within one screen.
- Homepage entries point to the right operational view.

### Phase 5 — Optional Proxmox / Blocky deepening

- Add Proxmox exporter only after API token secret handling is designed.
- Add Blocky dashboard only after Blocky is enabled.
- Add Homepage widgets for these only when secrets are managed safely.

## Acceptance checks

From `https://home.bry-guy.net/`, within two clicks, the admin can reach or understand:

- Grafana/admin
- Gatus/status
- Castaway web and bot status
- Argo CD operational note/link
- Prometheus/Loki via Grafana Explore
- Proxmox nodes
- Pi witness/qnetd
- platform machines

From `https://admin.bry-guy.net/`, within two clicks, the admin can answer:

- Is Homepage up and being used?
- Are admin/status/home routes healthy over Caddy?
- Is Castaway up?
- Are Discord commands succeeding?
- Which Castaway route/command is slow?
- Are failures app-level, DB-level, Discord-level, DNS/network-level, or edge-level?
- Are k3s nodes and PVCs healthy?
- Is the Pi witness/status stack healthy?
- Are any important Gatus checks failing?

Concrete metric/log checks:

- Prometheus has Caddy request metrics for `home.bry-guy.net` or equivalent host-level edge metrics.
- Gatus has a `home-page` check.
- Prometheus has Gatus endpoint metrics.
- Prometheus has `castaway_web_http_requests_total`.
- Prometheus has `castaway_bot_commands_total`.
- Loki has structured Castaway web request logs without `/healthz` spam.
- Loki has Castaway bot command completion logs.
- Grafana has Homelab overview, Edge / entrypoints, Castaway overview, and Witness / status dashboards.

## Out of scope for the first pass

- Public internet exposure.
- Authentication beyond tailnet-only routing.
- Full distributed tracing everywhere.
- Proxmox API widgets/tokens in Homepage.
- Blocky dashboards while Blocky is disabled.
- User-level analytics or identity tracking.
