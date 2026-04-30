# grafana-dynatrace-datasource

A [Grafana](https://grafana.com) data source plugin that queries the [Dynatrace v2 metrics API](https://docs.dynatrace.com/docs/dynatrace-api/environment-api/metric-v2) so you can render Dynatrace metrics on Grafana dashboards.

The plugin has a Go backend and a TypeScript/React frontend in one repo.

## Status

Early. Supports timeseries metric queries via `metricSelector` + optional `entitySelector` and `resolution`. Problems, logs, and DQL/Grail queries are not implemented yet — see [Roadmap](#roadmap).

## Requirements

- Grafana **10.4** or newer.
- A Dynatrace environment (SaaS or Managed).
- A Dynatrace API token with at minimum the **`metrics.read`** scope. Generate one under *Access Tokens* in the Dynatrace UI.

## Quickstart (local dev)

Clone, install, build, run:

```bash
git clone https://github.com/firestoned/grafana-dynatrace-datasource.git
cd grafana-dynatrace-datasource

# Frontend
npm install
npm run build           # one-shot build into ./dist
# or: npm run dev       # watch mode

# Backend
go mod download
go install github.com/magefile/mage@v1.15.0
mage -v buildAll        # builds gpx_dynatrace binaries for all platforms into ./dist

# Run Grafana with the plugin mounted
export DT_URL="https://abc12345.live.dynatrace.com"
export DT_TOKEN="dt0c01.XXXXXXXXXXXXXXXXXXXXXXXX..."
docker compose up
```

Open http://localhost:3000, log in as `admin` / `admin`. The Dynatrace data source is auto-provisioned from the env vars above. You can also configure it manually in **Connections → Data sources → Add data source → Dynatrace**.

## Project layout

```
.
├── src/                       # TypeScript frontend
│   ├── plugin.json            # Plugin manifest (id, type, backend: true)
│   ├── module.ts              # Entry — registers DataSource + editors
│   ├── datasource.ts          # DataSourceWithBackend subclass
│   ├── types.ts               # Query and config TypeScript types
│   └── components/
│       ├── ConfigEditor.tsx   # Env URL + API token fields
│       └── QueryEditor.tsx    # Metric selector / resolution / entity selector
│
├── pkg/                       # Go backend
│   ├── main.go                # datasource.Manage(...) entry
│   └── plugin/
│       ├── datasource.go      # QueryData + CheckHealth, Dynatrace HTTP client
│       └── datasource_test.go # Tests against an httptest fake
│
├── provisioning/datasources/  # Auto-config for local dev
├── docker-compose.yaml        # Local Grafana with plugin mounted
├── Magefile.go                # Backend build entry point
├── go.mod                     # Go module
└── package.json               # Frontend deps and scripts
```

## How it works

The plugin sets `"backend": true` in `plugin.json`, so when a user runs a query in a panel, Grafana proxies it from the browser to the Grafana server, which forwards it via gRPC to our Go binary (`gpx_dynatrace`). The Go process holds the decrypted API token and calls Dynatrace.

This matters because:

- The token never reaches the browser. It's stored in Grafana's `secureJsonData` (encrypted at rest) and only decrypted on the backend.
- We don't hit CORS — the request to Dynatrace is server-to-server.
- Alerting and unified-alerting work, which require a backend.

## Configuring a data source

Two fields:

| Field            | What                                                                                     |
| ---------------- | ---------------------------------------------------------------------------------------- |
| Environment URL  | `https://{env-id}.live.dynatrace.com` (SaaS) or `https://{host}/e/{env-id}` (Managed). |
| API Token        | Token with `metrics.read`. Other scopes added if/when more endpoints are supported.      |

**Save & Test** runs `CheckHealth`, which calls `GET /api/v2/metrics?pageSize=1` and reports auth failures distinctly.

## Writing queries

| Field           | What                                                                                                       | Example                                       |
| --------------- | ---------------------------------------------------------------------------------------------------------- | --------------------------------------------- |
| Metric selector | Required. A Dynatrace [metric selector](https://docs.dynatrace.com/docs/shortlink/api-metric-v2-selector). | `builtin:host.cpu.usage:avg`                  |
| Resolution      | Optional. `1m`, `5m`, `1h`, `Inf` (single value), or empty for the Dynatrace default.                       | `1m`                                          |
| Entity selector | Optional [entity selector](https://docs.dynatrace.com/docs/shortlink/api-entity-v2-selector).               | `type("HOST"),tag("env:prod")`                |

Time range is taken from the panel's time range and passed as `from` / `to`.

## Testing

```bash
npm run test:ci   # frontend Jest
go test ./...     # backend unit tests against an httptest fake server
```

## Building a release artifact

```bash
npm run build
mage -v buildAll
# Optional: sign the plugin so it can be loaded without
# GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS
npm run sign
```

The `dist/` directory is then a zip-able plugin folder you can drop into `/var/lib/grafana/plugins/`.

## Roadmap

- Metric autocomplete in the query editor (via a `CallResource` handler hitting `/api/v2/metrics`).
- Template-variable support (`metricFindQuery`).
- Query type switch: metrics / problems / events / logs / DQL.
- Unit auto-detection from Dynatrace metric metadata.
- Alerting query validation.

## License

Apache-2.0. See [LICENSE](LICENSE).
