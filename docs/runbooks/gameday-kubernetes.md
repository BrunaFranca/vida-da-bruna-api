# Game day no Kubernetes (cluster kind "vida")

Pré-requisitos: plataforma no ar (`plataforma-observabilidade/scripts/subir.sh`) e `bash deploy/subir.sh`.
Acompanhe no Grafana da plataforma (http://localhost:3300 → pasta `vida-da-bruna`).

```bash
# carga contínua (60 iterações/s ≈ 255 requisições/s) por 5 min, de dentro da rede dos clusters
docker run --rm --network kind -v "$PWD/carga:/carga:ro" -e ALVO=http://vida-control-plane:30081 \
  -e TAXA=60 -e DURACAO=5m grafana/k6:1.1.0 run /carga/trafego.js
```

| # | Experimento | Comando | O que deve acontecer |
|---|---|---|---|
| 1 | Matar um pod da API | `kubectl -n vida delete pod <api-...>` | O Service tira o pod da rotação; o Deployment cria outro; quase nenhum erro. |
| 2 | Carga alta | (o próprio k6) | O HPA sobe de 2 para até 5 réplicas pela CPU. |
| 3 | Derrubar o primário do Postgres | `kubectl -n vida delete pod $(kubectl -n vida get cluster.postgresql.cnpg.io vida-db -o jsonpath='{.status.currentPrimary}')` | O CloudNativePG promove a réplica; `vida-db-rw` passa a apontar para ela; o antigo volta como réplica. |

## Resultado do primeiro game day (21/11/2026)
- **HPA:** 2 → 5 réplicas com a CPU a 224% do request; caiu para 84% depois de escalar.
- **Pod morto:** sem impacto visível.
- **Failover:** evento `Failing over from vida-db-1 to vida-db-2`; `vida-db-2` virou primário e `vida-db-1` voltou como réplica. O operador espera confirmar que o primário está doente antes de promover (não é instantâneo, e é de propósito: evita failover por um soluço).
- **Total:** 76.392 requisições, **17 com erro (0,02%)**, p95 ≈ 5 ms, máximo 2,06 s (durante o failover).
- **SLO de disponibilidade (99,5%):** atendido com folga durante os três experimentos.
