# AGENTS.md

Contexto para assistentes de IA neste repositório.

- **O que é:** a API em Go do app pessoal Vida da Bruna, e ao mesmo tempo um lab público de SRE da Bruna (DevOps/SRE). O app em si fica num repo privado separado (`vida-da-bruna`).
- **Repo público:** nunca colocar dados pessoais ou financeiros, nem código/config de empregador.
- **Plano e etapas:** `docs/adr/0001-lab-sre-api-go.md`. Atualizar o status lá ao concluir cada etapa.
- **Idioma:** comentários, commits e docs em português. Conventional Commits.
- **Antes de commit:** `go vet ./...` e os testes precisam passar. No Windows dela o Smart App Control bloqueia o binário de teste; rodar dentro do container (com o `db` do Compose no ar):
  `docker run --rm --network vida-da-bruna-api_default -v "${PWD}:/src" -v gomod:/go/pkg/mod -w /src -e TEST_DATABASE_URL="postgres://vida:vida-local@db:5432/vida?sslmode=disable" golang:1.27 go test -race ./...`
- **Rotas:** `GET/POST /v1/{col}`, `GET/PUT/PATCH/DELETE /v1/{col}/{id}` (PUT substitui, PATCH mescla raso, DELETE é idempotente). Migrations em `internal/migrations/sql/`, aplicadas na subida.
- **Tempo real:** `GET /v1/stream?w=<col>&w=<col>/<id>...` (SSE multiplexado: **é o que o app usa**, porque o navegador limita 6 conexões por origem; eventos `{"w": item, "v": dados}`), além de `GET /v1/{col}/stream` e `GET /v1/{col}/{id}/stream` (um item só). Trigger no Postgres faz `pg_notify("docs")`; `internal/aovivo` escuta com LISTEN (com reconexão e backoff) e avisa os streams. Funciona com várias réplicas. Streams ficam **fora do SLO** (conexão longa de propósito).
- **Rodar local:** `docker compose up --build -d` → API em http://localhost:8080/readyz, Grafana em http://localhost:3000 (sem login).
- **Observabilidade (pasta `observabilidade/`):** API → OTLP → Collector → Mimir (métricas), Tempo (traces), Loki (logs) → Grafana. Regras de SLO em `observabilidade/mimir/regras/anonymous/slo.yaml` (validar com promtool); dashboards em JSON em `observabilidade/grafana/dashboards/` (são código: editar o JSON, não a UI). Carga: `docker compose --profile carga run --rm k6`. Runbook: `docs/runbooks/banco-fora.md`.
- **Kubernetes (etapa 5):** `bash deploy/subir.sh` cria o cluster kind `vida` (API pelo chart `deploy/chart`, Postgres CloudNativePG `deploy/k8s/postgres.yaml`, kube-prometheus-stack, metrics-server) e liga à plataforma (`deploy/k8s/plataforma.yaml`: Services sem seletor + EndpointSlice com o IP do nó `obs`). API em http://localhost:8081. SLOs e dashboards vão para a plataforma com `bash deploy/enviar-observabilidade.sh` (mesmos arquivos de `observabilidade/`). Game days: `docs/runbooks/gameday-kubernetes.md`. A imagem roda com UID numérico 65532 (exigência do `runAsNonRoot`).
- **Cuidado no Windows:** no PowerShell, flags como `-config.file=...` quebram; use o Bash ou aspas.
- No Windows da Bruna: Go em `C:\Program Files\Go\bin`, Git em `C:\Program Files\Git\cmd`, gh em `C:\Program Files\GitHub CLI`.
