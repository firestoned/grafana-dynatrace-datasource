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

Grafana plugins are distributed as a zip of the built `dist/` directory, named after the plugin id:

```bash
npm run build           # frontend → dist/
mage -v buildAll        # backend  → dist/gpx_dynatrace_*
npm run package         # → firestoned-dynatrace-datasource-<version>.zip
```

The zip contains a single top-level directory `firestoned-dynatrace-datasource/` — the layout Grafana expects under its plugins directory.

## Installing the plugin

Releases are **unsigned** — this plugin targets OSS / on-prem deployments. Grafana refuses to load unsigned plugins by default, so every install method below has two parts:

1. Allow the unsigned plugin to load (one-time per Grafana instance).
2. Get the plugin zip onto disk.

### Step 1 — Allow the unsigned plugin to load

The plugin id is `firestoned-dynatrace-datasource`. Apply **one** of the following — whichever matches how Grafana is deployed.

**Environment variable** (Docker, systemd, Kubernetes — most setups):

```
GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=firestoned-dynatrace-datasource
```

Comma-separate to allow multiple:

```
GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=firestoned-dynatrace-datasource,firestoned-splunk-datasource
```

**`grafana.ini`** (deb/rpm package installs — `/etc/grafana/grafana.ini`):

```ini
[plugins]
allow_loading_unsigned_plugins = firestoned-dynatrace-datasource
```

**Docker Compose**:

```yaml
services:
  grafana:
    image: grafana/grafana-oss:11.1.0
    environment:
      - GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=firestoned-dynatrace-datasource
```

**Kubernetes — Helm `grafana/grafana` chart** (`values.yaml`):

```yaml
env:
  GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS: firestoned-dynatrace-datasource
```

**systemd unit override** (running `grafana-server` directly):

```bash
sudo systemctl edit grafana-server
```

Add:

```ini
[Service]
Environment="GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=firestoned-dynatrace-datasource"
```

Then `sudo systemctl daemon-reload && sudo systemctl restart grafana-server`.

**Verifying it took effect** — after Grafana restarts, the log should contain a line like:

```
logger=plugin.loader level=warn msg="Permitting unsigned plugin. This is not recommended" pluginID=firestoned-dynatrace-datasource
```

(Default log path: `/var/log/grafana/grafana.log`.) If you don't see that line, the env var or `grafana.ini` setting isn't being read by the running process.

### Step 2 — Install the plugin zip

Three methods. Pick whichever matches your environment.

#### Manual install (any Grafana, including Docker)

Unzip into Grafana's plugins directory and restart:

```bash
unzip firestoned-dynatrace-datasource-<version>.zip -d /var/lib/grafana/plugins/
systemctl restart grafana-server
```

Default plugin path is `/var/lib/grafana/plugins` (deb/rpm) or `/opt/homebrew/var/lib/grafana/plugins` (Homebrew). Override with `GF_PATHS_PLUGINS`.

#### `grafana-cli` from a URL

Host the zip somewhere reachable (GitHub Releases, S3, internal mirror):

```bash
grafana-cli --pluginUrl https://example.com/firestoned-dynatrace-datasource-<version>.zip \
  plugins install firestoned-dynatrace-datasource
```

#### Grafana Docker image (`GF_INSTALL_PLUGINS`)

The official `grafana/grafana` image installs plugins on startup from `GF_INSTALL_PLUGINS` — use the `url;id` form for a custom zip, paired with the allowlist env var from Step 1:

```yaml
environment:
  - GF_INSTALL_PLUGINS=https://example.com/firestoned-dynatrace-datasource-<version>.zip;firestoned-dynatrace-datasource
  - GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=firestoned-dynatrace-datasource
```

After install, restart Grafana and confirm the plugin appears under **Administration → Plugins**.

## Roadmap

- Metric autocomplete in the query editor (via a `CallResource` handler hitting `/api/v2/metrics`).
- Template-variable support (`metricFindQuery`).
- Query type switch: metrics / problems / events / logs / DQL.
- Unit auto-detection from Dynatrace metric metadata.
- Alerting query validation.

## License

Apache-2.0. See [LICENSE](LICENSE).
