# Runbook: banco de dados fora do ar

**Alertas:** `DisponibilidadeBudgetQueimandoRapido` (critical) e `DisponibilidadeBudgetQueimandoDevagar` (warning)
**SLO afetado:** disponibilidade (99,5% das requisições `/v1` sem 5xx, em 30 dias)

## Sintomas
- Dashboard **API · SLOs**: burn rate de 5m e 1h acima da linha de 14,4x.
- Dashboard **API · RED**: "Erros 5xx" perto de 100%; latência sobe para ~3 s (o timeout da requisição).
- `/readyz` responde **503**; `/livez` continua **200** (a API está viva, só não está pronta).

## Diagnóstico (do sintoma à causa)
1. **RED → trace:** no painel de latência, clique num *exemplar* (o ponto) → abre o trace no Tempo. O span do Postgres aparece com erro de conexão.
2. **Trace → log:** no trace, "Logs for this span" → Loki mostra `erro no banco` com o mesmo `trace_id`.
3. **Confirme o banco:**
   ```bash
   docker compose ps db                  # Exited?
   docker compose logs db --tail 50      # por que caiu?
   ```

## Mitigação
```bash
docker compose start db
curl localhost:8080/readyz              # deve voltar a {"status":"ok"}
```
A API se reconecta sozinha (o pool do pgx tenta de novo); não precisa reiniciar a API.

## Verificação
- `/readyz` 200 e "Erros 5xx" volta a 0%.
- O alerta **critical** some em poucos minutos (a janela curta de 5m limpa).
- O **warning** pode continuar por até ~30 min (janela curta de 30m): é esperado, não é um novo problema.
- Anote no postmortem quanto do **error budget** o incidente consumiu (dashboard SLO → "Error budget restante").

## Simular este incidente (game day)
Com o ambiente no ar (`docker compose up -d`):
```bash
docker compose --profile carga run --rm k6 &   # tráfego normal (15 min)
# espere ~2 min para ter uma linha de base
docker compose stop db                          # o incidente
# acompanhe: Grafana → API · SLOs (o critical dispara em ~3 a 5 min)
docker compose start db                         # a mitigação
```

Resultado do primeiro game day (21/11/2026): o critical disparou com 100% de 5xx (burn rate 200x na janela de 5m) e se resolveu em ~6 min depois da volta do banco; o k6 seguinte rodou com 0% de falha e p95 de 13 ms.
