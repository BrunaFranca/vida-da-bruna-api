#!/usr/bin/env bash
# Sobe (ou atualiza) o cluster "vida": API + Postgres (CloudNativePG) + Prometheus, ligados à plataforma
# de observabilidade (cluster "obs", repo plataforma-observabilidade, que precisa estar no ar).
# Idempotente. Uso: bash deploy/subir.sh
set -euo pipefail
cd "$(dirname "$0")/.."

CNPG_CHART=${CNPG_CHART:-0.29.2}
KPS_CHART=${KPS_CHART:-93.6.1}
MS_CHART=${MS_CHART:-3.14.0}
TAG=${TAG:-$(git rev-parse --short HEAD)}

docker inspect obs-control-plane >/dev/null 2>&1 || { echo "Suba a plataforma antes (plataforma-observabilidade/scripts/subir.sh)"; exit 1; }
kind get clusters | grep -qx vida || kind create cluster --config deploy/kind/vida.yaml
kubectl config use-context kind-vida >/dev/null

helm repo add cnpg https://cloudnative-pg.github.io/charts >/dev/null 2>&1 || true
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts >/dev/null 2>&1 || true
helm repo add metrics-server https://kubernetes-sigs.github.io/metrics-server >/dev/null 2>&1 || true
helm repo update cnpg prometheus-community metrics-server >/dev/null

# 1. Ponte para a plataforma (IP do nó "obs" na rede kind)
IP_OBS=$(docker inspect -f '{{(index .NetworkSettings.Networks "kind").IPAddress}}' obs-control-plane)
kubectl create namespace vida --dry-run=client -o yaml | kubectl apply -f - >/dev/null
for ns in vida monitoramento; do
  kubectl create namespace "$ns" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  sed "s/\${IP_OBS}/$IP_OBS/" deploy/k8s/plataforma.yaml | kubectl -n "$ns" apply -f - >/dev/null
done

# 2. Infra do cluster: métricas de CPU (HPA), Prometheus, operador do Postgres
helm upgrade --install metrics-server metrics-server/metrics-server -n kube-system --version "$MS_CHART" \
  --set 'args={--kubelet-insecure-tls}' --wait
helm upgrade --install kps prometheus-community/kube-prometheus-stack -n monitoramento --version "$KPS_CHART" \
  -f deploy/valores/kube-prometheus-stack.yaml --wait --timeout 10m
helm upgrade --install cnpg cnpg/cloudnative-pg -n cnpg-system --create-namespace --version "$CNPG_CHART" --wait

# 3. Postgres (espera o primário e a réplica)
kubectl -n vida apply -f deploy/k8s/postgres.yaml
kubectl -n vida wait cluster.postgresql.cnpg.io/vida-db --for=condition=Ready --timeout=10m

# 4. API: build da imagem, carga no kind (sem registry) e deploy pelo chart
docker build -q -t "vida-da-bruna-api:$TAG" . >/dev/null
kind load docker-image "vida-da-bruna-api:$TAG" --name vida >/dev/null
helm upgrade --install api deploy/chart -n vida --set imagem.tag="$TAG" --wait --timeout 5m

echo "API no ar: http://localhost:8081/readyz  (imagem $TAG)"
