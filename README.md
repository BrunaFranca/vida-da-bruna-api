# vida-da-bruna-api

[![ci](https://github.com/BrunaFranca/vida-da-bruna-api/actions/workflows/ci.yml/badge.svg)](https://github.com/BrunaFranca/vida-da-bruna-api/actions/workflows/ci.yml)

API em Go do app pessoal **Vida da Bruna**, construída como um lab de SRE: Postgres, Docker, OpenTelemetry, Prometheus, Grafana e, depois, Kubernetes com Helm.

> Em construção. Plano em [docs/adr/0001-lab-sre-api-go.md](docs/adr/0001-lab-sre-api-go.md).

```bash
docker compose up --build
curl localhost:8080/readyz
```
