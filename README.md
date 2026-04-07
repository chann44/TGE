# Arrakis

Arrakis is a self-hosted, open-source supply chain security platform.

Our goal is to make open source much safer and make package registries more approachable and trustworthy for every team and community that depends on them.

## 🚀 What it does

- Scans your repo dependencies (direct + transitive)
- Detects known vulnerabilities (CVE / OSV)
- Flags risky patterns:
  - new/untrusted packages
  - suspicious install scripts
  - dependency confusion risks
- Tracks dependency changes over time
- Generates simple risk reports

## ⚙️ How it works

1. Connect your GitHub repo
2. Extract dependency manifests (npm, pip, go, etc.)
3. Build dependency graph
4. Run scanners + rule engine
5. Output findings with risk score

## 🧱 Tech stack

- Go (backend)
- Postgres (metadata)
- Redis (jobs / queues)
- Docker (self-hosted deployment)

## 🏗️ Project structure

```text
apps/        # api, worker, web
domain/      # core models (dependency, scan, findings)
services/    # parsing, scanning, risk engine
adapters/    # github, osv, registries
deployments/ # docker / k8s
```

## 🐳 Run locally (dev dependencies only)

```bash
git clone git@github.com:chann44/TGE.git
cd arrakis

cp .env.example .env
docker compose -f deployments/dev.compose.yml up -d
```

API: `http://localhost:8080`

## 🚢 Self-host with published images

```bash
git clone git@github.com:chann44/TGE.git
cd arrakis

cp .env.example .env
docker compose -f deployments/selfhost.compose.yml up -d
```

Self-host compose uses Traefik and serves web + API on one domain (`/` and `/api`).

Set these required values in `.env` before startup:

```bash
TRAEFIK_ACME_EMAIL=ops@example.com
DOMAIN_RECORD_TYPE=A
DOMAIN_RECORD_VALUE=<your-server-public-ip>
```

Then open `http://<server-ip>/settings` and add your domain in the dashboard.
The app will show DNS records you need to set:

- TXT ownership record: `_tge-challenge.<your-domain>`
- Traffic record: `A` (or `CNAME` if configured)

After DNS propagation, click Verify in the dashboard. Traefik will publish HTTPS routes automatically.

Infrastructure requirements:

- Open inbound ports `80` and `443` on your host/cloud firewall
- Ensure your DNS points to `DOMAIN_RECORD_VALUE`

Traefik terminates TLS automatically with Let's Encrypt.

Default images used by compose:

```bash
chann44/tge-backend:latest
chann44/tge-web:latest
```

To publish updated images to Docker Hub:

```bash
docker login
make docker-build
make docker-push
```

Automated publish is also configured via GitHub Actions in `.github/workflows/docker-publish.yml`.
Push a tag like `v1.0.0` to publish multi-arch images to Docker Hub.

To use another registry/image name, override:

```bash
export TGE_BACKEND_IMAGE=your-registry/your-namespace/tge-backend:latest
export TGE_WEB_IMAGE=your-registry/your-namespace/tge-web:latest
docker compose -f deployments/selfhost.compose.yml up -d
```

For system-health log streaming, set these `.env` values:

```bash
CLICKHOUSE_HOST=localhost
CLICKHOUSE_PORT=9000
CLICKHOUSE_USER=default
CLICKHOUSE_PASSWORD=clickhouse
CLICKHOUSE_DATABASE=default
```

## 📌 Roadmap

- GitHub App integration
- CI integration (fail on high risk)
- Auto PR fixes for dependency updates
- Advanced heuristic rules engine
- Dashboard + alerts

## ⚠️ Status

Early stage project - APIs and schema may change.

## 🤝 Contributing

PRs and feedback welcome.

## 📄 License

MIT
