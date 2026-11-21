#!/usr/bin/env bash
# "Você constrói, você opera": o projeto envia as próprias regras de SLO e dashboards para a plataforma
# compartilhada, no tenant dele. Os arquivos são os mesmos do Compose (observabilidade/), só os uids
# das fontes de dados mudam (mimir → mimir-<tenant> etc.).
# Uso: bash deploy/enviar-observabilidade.sh   (com a plataforma no ar)
set -euo pipefail
cd "$(dirname "$0")/.."
TENANT=${TENANT:-vida-da-bruna}
GRAFANA=${GRAFANA:-http://localhost:3300}

# 1. Regras (recording + alertas de burn rate) → ruler do Mimir, pela rede dos clusters.
MSYS_NO_PATHCONV=1 docker run --rm --network kind -v "$(pwd -W 2>/dev/null || pwd)/observabilidade/mimir/regras/anonymous:/r:ro" \
  grafana/mimirtool:2.16.0 rules sync --address http://obs-control-plane:30909 --id "$TENANT" /r/slo.yaml

# 2. Dashboards → pasta do projeto no Grafana da plataforma.
pasta=$(curl -sf -X POST "$GRAFANA/api/folders" -H 'Content-Type: application/json' \
  -d "{\"uid\":\"$TENANT\",\"title\":\"$TENANT\"}" >/dev/null 2>&1; echo "$TENANT")
for f in observabilidade/grafana/dashboards/*.json; do
  node -e '
    const [arq, tenant, pasta] = process.argv.slice(1);
    const txt = require("fs").readFileSync(arq, "utf8")
      .replace(/"uid": "(mimir|loki|tempo)"/g, (_, s) => `"uid": "${s}-${tenant}"`)
      .replace(/"datasource": "Mimir"/g, `"datasource": "Mimir · ${tenant}"`);
    process.stdout.write(JSON.stringify({ dashboard: { ...JSON.parse(txt), id: null }, folderUid: pasta, overwrite: true }));
  ' "$f" "$TENANT" "$pasta" |
    curl -sf -X POST "$GRAFANA/api/dashboards/db" -H 'Content-Type: application/json' --data-binary @- >/dev/null
  echo "dashboard enviado: $f"
done
