# vida-da-bruna-api

[![ci](https://github.com/BrunaFranca/vida-da-bruna-api/actions/workflows/ci.yml/badge.svg)](https://github.com/BrunaFranca/vida-da-bruna-api/actions/workflows/ci.yml)

API em Go do meu app pessoal **Vida da Bruna** (agenda, tarefas, hábitos e finanças), construída como um **laboratório de SRE**: do código ao Kubernetes, com observabilidade, SLOs, alertas de burn rate e game days documentados.

> O app em si (React + TypeScript) fica num repo privado; a observabilidade, num cluster compartilhado do repo [plataforma-observabilidade](https://github.com/BrunaFranca/plataforma-observabilidade).

## Como funciona

```mermaid
flowchart TB
  subgraph U["Usuária"]
    B(["Bruna, no PC ou no celular"])
  end
  subgraph A["Aplicação"]
    APP["App Vida da Bruna<br/>React + TypeScript"]
    API["API<br/>Go · 2 a 5 réplicas"]
  end
  subgraph D["Dados"]
    PG[("Postgres principal")]
    PGR[("Postgres reserva")]
  end
  subgraph O["Observabilidade (plataforma compartilhada)"]
    GW["Gateway"]
    ST[("Mimir · Loki · Tempo")]
    GR["Grafana + alertas"]
  end

  B -- "1" --> APP
  APP -- "2" --> API
  API -- "3" --> PG
  PG -- "4" --> API
  API -- "5" --> APP
  PG -. "6" .-> PGR
  API -- "7" --> GW
  GW --> ST
  ST -- "8" --> GR
  GR -- "9" --> B
```

| Nº | De → Para | Como conversa | Por quê |
|---|---|---|---|
| 1 | Bruna → App | navegador / app instalado | ela anota tarefas, marca hábitos, lança gastos |
| 2 | App → API | HTTP (REST): `GET`, `PUT`, `PATCH`, `DELETE` | o app só desenha as telas; a API valida e grava |
| 3 | API → Postgres | SQL (pgx), dados em JSON | o banco guarda tudo de forma durável |
| 4 | Postgres → API | `LISTEN/NOTIFY` | o banco avisa na hora quando algo muda, para todas as réplicas |
| 5 | API → App | SSE: **uma** conexão aberta por aba | a tela atualiza sozinha, sem recarregar |
| 6 | Principal → Reserva | replicação do Postgres (CloudNativePG) | se o principal cair, a reserva assume sozinha |
| 7 | API → Gateway | OTLP com o tenant `vida-da-bruna` | traces, métricas e logs saem da API para a central |
| 8 | Armazenamento → Grafana | consultas PromQL, LogQL e TraceQL | painéis de saúde e metas (SLOs) |
| 9 | Grafana → Bruna | alertas de burn rate | avisa quando o app está gastando o "orçamento de erro" rápido demais |

## A jornada de um pedido

**A história:** Bruna está com o app aberto no computador e, pelo celular, marca a tarefa "Tocar piano" como feita.

```mermaid
sequenceDiagram
  autonumber
  actor Bruna
  participant Cel as App no celular
  participant API as API (réplica 1)
  participant DB as Postgres
  participant API2 as API (réplica 2)
  participant PC as App no PC
  participant OBS as Observabilidade

  Note over PC,API2: O PC já está "ouvindo" as mudanças<br/>por uma conexão aberta (SSE)
  Bruna->>Cel: toca em "feito"
  Cel->>API: PATCH /v1/eventos/piano {"feito": true}
  API->>DB: grava a mudança
  DB-->>API: ok
  API-->>Cel: 204 (pronto)
  Note over DB: o banco avisa todas as réplicas<br/>(LISTEN/NOTIFY)
  DB-->>API2: "eventos" mudou
  API2->>DB: lê a lista atualizada
  API2-->>PC: envia a lista nova
  PC-->>Bruna: a tarefa aparece riscada no PC,<br/>sem recarregar
  API-)OBS: trace do pedido, tempo gasto, status
  Note over OBS: se muitos pedidos falharem ou ficarem lentos,<br/>o alerta de SLO dispara
```

**Em resumo:** o celular pede, a API grava, o banco avisa, e todas as telas abertas se atualizam, mesmo conectadas a outra cópia da API. Ao mesmo tempo, cada pedido deixa rastros na observabilidade, que mostra a saúde do app e avisa quando algo sai das metas.

## Arquitetura técnica

Visão de engenharia: clusters, namespaces, objetos do Kubernetes, protocolos e portas.

```mermaid
flowchart LR
  subgraph HOST["PC · Docker Desktop (WSL2) · rede Docker kind"]
    direction LR

    subgraph VIDA["cluster kind: vida"]
      direction TB
      subgraph NSV["ns: vida"]
        SVC["Service api<br/>NodePort 30081 → :8080"]
        DEP["Deployment api<br/>2–5 pods · Go<br/>probes livez/readyz/startup<br/>preStop 5s · UID 65532 · FS ro"]
        HPA["HPA<br/>CPU 70% do request"]
        PDB["PDB<br/>minAvailable 1"]
        RW["Service vida-db-rw<br/>:5432 → primário"]
        subgraph CNPG["CloudNativePG Cluster vida-db"]
          P1[("vida-db-1<br/>Postgres 17 · primário")]
          P2[("vida-db-2<br/>réplica streaming")]
        end
        SEC["Secret vida-db-app<br/>uri gerada pelo operador"]
        PGW["Service plataforma-gateway :4317<br/>+ EndpointSlice → nó obs:30317"]
        PMW["Service plataforma-mimir :9009<br/>+ EndpointSlice → nó obs:30909"]
      end
      subgraph NSM["ns: monitoramento"]
        PROM["Prometheus (Operator)<br/>kube-state-metrics · node-exporter<br/>PodMonitor do CNPG"]
      end
      MS["metrics-server<br/>(ns kube-system)"]
      OP["operador CNPG<br/>(ns cnpg-system)"]
    end

    subgraph OBS["cluster kind: obs"]
      direction TB
      subgraph NSO["ns: observabilidade"]
        GWS["Service gateway<br/>NodePort 30317"]
        GW["OTel Collector (gateway)<br/>otlp receiver · include_metadata<br/>batch por tenant · headers_setter"]
        MI[("Mimir · StatefulSet<br/>distributor/ingester/querier<br/>ruler + alertmanager<br/>NodePort 30909")]
        LO[("Loki · StatefulSet<br/>auth_enabled · TSDB v13")]
        TE[("Tempo · StatefulSet<br/>multitenancy")]
        GF["Grafana<br/>NodePort 30300<br/>fontes por tenant"]
      end
    end
  end

  NAV(["Navegador<br/>App React"]) -- "HTTP REST + SSE<br/>localhost:8081" --> SVC
  SVC --> DEP
  HPA -. escala .-> DEP
  MS -. "métricas de CPU" .-> HPA
  PDB -. protege .-> DEP
  DEP -- "pgx · SQL + LISTEN docs" --> RW --> P1
  P1 -- "replicação física" --> P2
  OP -. "failover, Secret, Services" .-> CNPG
  SEC -. "env DATABASE_URL" .-> DEP
  DEP -- "OTLP gRPC<br/>X-Scope-OrgID: vida-da-bruna" --> PGW
  PROM -- "remote write<br/>X-Scope-OrgID: vida-da-bruna" --> PMW
  PGW == "rede kind" ==> GWS --> GW
  PMW == "rede kind" ==> MI
  GW -- "OTLP HTTP /otlp" --> MI
  GW -- "OTLP gRPC" --> TE
  GW -- "OTLP HTTP /otlp" --> LO
  GF -- "PromQL · LogQL · TraceQL<br/>(header do tenant)" --> MI & LO & TE
  DEV(["Bruna · SRE<br/>localhost:3300"]) --> GF
```

| Ligação | Protocolo / porta | Detalhe |
|---|---|---|
| Navegador → API | HTTP :8081 (NodePort 30081) | REST para escrita; **uma** conexão SSE (`/v1/stream?w=...`) por aba |
| API → Postgres | TCP :5432 via `vida-db-rw` | pool pgx; spans por query (otelpgx); `LISTEN docs` numa conexão dedicada com backoff |
| Postgres → API | `NOTIFY docs` | trigger em `docs` publica `{colecao, id}`; todas as réplicas recebem |
| Primário → réplica | replicação física (CNPG) | failover automático; `vida-db-rw` passa a apontar para o novo primário |
| API → plataforma | OTLP gRPC → `obs:30317` | `OTEL_EXPORTER_OTLP_HEADERS=X-Scope-OrgID=vida-da-bruna`; métricas a cada 10 s |
| Prometheus → Mimir | remote write → `obs:30909/api/v1/push` | métricas do cluster com `cluster="vida"` |
| Gateway → backends | OTLP (HTTP para Mimir/Loki, gRPC para Tempo) | o tenant segue no header; lotes nunca misturam tenants |
| Projeto → plataforma | API do ruler (`mimirtool rules sync`) e API do Grafana | SLOs e dashboards vivem neste repo ("você constrói, você opera") |
| Ponte entre clusters | Services sem seletor + EndpointSlice | apontam para o IP do nó `obs` na rede Docker `kind` |

**CI (GitHub Actions):** `go vet` · testes com `-race` contra Postgres real · golangci-lint · build da imagem · `promtool check rules` · validação do Collector · JSON dos dashboards · `helm lint` + kubeconform. A plataforma tem um job que sobe o cluster kind inteiro e prova o isolamento entre tenants.

## O que este projeto mostra

| Área | Onde |
|---|---|
| **API em Go** só com biblioteca padrão + pgx, desligamento gracioso, `livez` × `readyz` | `cmd/api`, `internal/httpapi` |
| **Banco:** migrations embutidas com advisory lock (réplicas não migram juntas), JSONB | `internal/migrations`, `internal/docs` |
| **Tempo real:** SSE multiplexado (navegadores limitam 6 conexões por origem) + LISTEN/NOTIFY com reconexão e backoff | `internal/aovivo`, `internal/httpapi/multi.go` |
| **OpenTelemetry:** traces (HTTP + cada query), métricas com rota, logs com `trace_id`, exemplars | `internal/telemetria` |
| **SLOs como código:** disponibilidade 99,5% e latência (95% < 300 ms), alertas de burn rate em múltiplas janelas | `observabilidade/mimir/regras` |
| **Dashboards como código:** RED e SLO (SLI, error budget, burn rate) | `observabilidade/grafana/dashboards` |
| **Kubernetes:** Helm chart com probes, HPA, PDB, `preStop`, não-root, FS somente leitura | `deploy/chart` |
| **Postgres no Kubernetes:** CloudNativePG com réplica e failover automático | `deploy/k8s/postgres.yaml` |
| **Observabilidade multi-tenant** num cluster separado ("você constrói, você opera") | `deploy/enviar-observabilidade.sh` |
| **CI:** vet, testes com `-race` contra Postgres real, golangci-lint, imagem, promtool, kubeconform | `.github/workflows/ci.yml` |
| **Game days** com resultados medidos | `docs/runbooks` |

## Resultados dos game days

| Experimento | Resultado |
|---|---|
| Banco fora do ar (Compose) | alerta **critical** de burn rate disparou com 100% de 5xx; resolveu sozinho ~6 min após a volta |
| Carga de ~255 req/s (Kubernetes) | HPA escalou de **2 para 5 réplicas** |
| Matar um pod da API | sem impacto visível |
| Derrubar o primário do Postgres | **failover automático** para a réplica |
| **Total no Kubernetes** | **76.392 requisições, 0,02% de erro, p95 ≈ 5 ms** |

## Como rodar

**Modo rápido (Docker Compose):** API, Postgres e o stack LGTM completo numa máquina só.
```bash
docker compose up --build -d                  # API :8080 · Grafana :3000
docker compose --profile carga run --rm k6    # tráfego de teste
```

**Modo Kubernetes (dois clusters kind):**
```bash
bash ../plataforma-observabilidade/scripts/subir.sh   # cluster "obs" (Grafana :3300)
bash deploy/subir.sh                                  # cluster "vida" (API :8081)
bash deploy/enviar-observabilidade.sh                 # SLOs e dashboards → plataforma
```

**Testes:** `go test ./...` (os de integração usam `TEST_DATABASE_URL`, ver `AGENTS.md`).

## Rotas

| Rota | Faz |
|---|---|
| `GET /livez` · `GET /readyz` | vivo? · pronto (banco ok)? |
| `GET/POST /v1/{col}` | lista · cria (id gerado) |
| `GET/PUT/PATCH/DELETE /v1/{col}/{id}` | lê · substitui · mescla · apaga (idempotente) |
| `GET /v1/stream?w=<col>&w=<col>/<id>` | tempo real (SSE), vários itens numa conexão |

## Decisões e documentação
- [ADR 0001: plano e etapas do lab](docs/adr/0001-lab-sre-api-go.md)
- [Runbook: banco fora do ar](docs/runbooks/banco-fora.md) · [Game day no Kubernetes](docs/runbooks/gameday-kubernetes.md)
- Próximo passo: levar para a nuvem (Cloud Run + Postgres gerenciado + Terraform), para o app usar a API no PC e no celular.
