# vida-da-bruna-api

[![ci](https://github.com/BrunaFranca/vida-da-bruna-api/actions/workflows/ci.yml/badge.svg)](https://github.com/BrunaFranca/vida-da-bruna-api/actions/workflows/ci.yml)

API em Go do app pessoal **Vida da Bruna**, construída como um lab de SRE: Postgres, Docker, OpenTelemetry, stack LGTM (Loki, Grafana, Tempo, Mimir), SLOs com alertas de burn rate e, depois, Kubernetes com Helm.

> Em construção. Plano e status em [docs/adr/0001-lab-sre-api-go.md](docs/adr/0001-lab-sre-api-go.md). Documentação completa (com diagrama de arquitetura) no fim do projeto.

```bash
docker compose up --build -d
curl localhost:8080/readyz          # API
# Grafana: http://localhost:3000 → pasta "Vida da Bruna"
docker compose --profile carga run --rm k6   # tráfego de teste
```

Game day de banco fora do ar: [docs/runbooks/banco-fora.md](docs/runbooks/banco-fora.md).
