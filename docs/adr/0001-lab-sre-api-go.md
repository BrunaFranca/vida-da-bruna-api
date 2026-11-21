# 0001: API em Go como lab público de SRE

**Data:** 21/11/2026 · **Status:** Aceita

## Contexto
O app Vida da Bruna roda hoje como Claude Artifact (repo privado `vida-da-bruna`). A fase 3 do roadmap prevê uma API própria em Go + Postgres. A Bruna também quer um projeto público que mostre o trabalho dela de DevOps/SRE.

## Decisão
- Repo **público e separado**: `vida-da-bruna-api`. Só código e infra, **nenhum dado pessoal** (o app continua privado).
- API em Go (biblioteca padrão + pgx), um "banco de documentos" que espelha a interface `Store` do app (tabela `docs`: coleção, id, JSONB), para o frontend trocar de banco sem mudar telas.
- Tudo local com **Docker Compose** (Docker Desktop + WSL2).
- Etapas:
  1. Esqueleto (`/livez`, `/readyz`), Dockerfile, Compose com Postgres, CI. **Concluída em 21/11/2026.**
  2. CRUD de documentos + migrations + testes. **Concluída em 21/11/2026.**
  3. Observabilidade: OpenTelemetry → Collector → Mimir, Tempo e Loki (stack LGTM) + Grafana; dashboards e SLO com burn rate como código; k6 + runbook de incidente. **Concluída em 21/11/2026.** (Mimir em vez de Prometheus: está no currículo dela e fecha o LGTM; o Prometheus entra na etapa 5, no Kubernetes.) SLOs definidos por ela: 99,5% sem 5xx e 95% abaixo de 300 ms, em 30 dias.
  4. Tempo real (SSE) + `ApiStore` no app + import do backup do artifact (local, nunca versionado). **Concluída em 21/11/2026.** Lição: o navegador limita 6 conexões HTTP/1.1 por origem, então o app usa uma conexão SSE multiplexada (`/v1/stream?w=...`), não uma por coleção.
  5. Kubernetes local com Helm chart: probes, HPA, PDB, requests/limits, kube-prometheus-stack. **Concluída em 21/11/2026.** Mudanças no caminho (decididas com a Bruna): **kind** (cluster como código, reproduzível no CI) em vez do Docker Desktop; Postgres com **CloudNativePG** (primário + réplica, failover); observabilidade num **cluster separado e compartilhado** (repo `plataforma-observabilidade`, multi-tenant) em vez de dentro deste cluster: o projeto só tem o Prometheus (remote write) e a API manda OTLP direto para o gateway da plataforma, com o tenant `vida-da-bruna`. Resultados em `docs/runbooks/gameday-kubernetes.md`.
- Forma de trabalho: o Claude escreve e explica; no fim, uma documentação caprichada **com um diagrama de arquitetura de todo o projeto** (pedido da Bruna).

## Consequências
- Nuvem continua para depois (ADR 0004 do app).
- Sem autenticação enquanto for só local (portas presas em 127.0.0.1).
