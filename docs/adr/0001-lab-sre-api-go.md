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
  4. Tempo real (SSE) + `ApiStore` no app + import do backup do artifact (local, nunca versionado).
  5. Kubernetes local (Docker Desktop) com Helm chart: probes, HPA, PDB, requests/limits, kube-prometheus-stack.
- Forma de trabalho: o Claude escreve e explica; no fim, uma documentação caprichada **com um diagrama de arquitetura de todo o projeto** (pedido da Bruna).

## Consequências
- Nuvem continua para depois (ADR 0004 do app).
- Sem autenticação enquanto for só local (portas presas em 127.0.0.1).
