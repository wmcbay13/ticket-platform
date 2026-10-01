#!/usr/bin/env bash
set -euo pipefail
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"
source versions.env
export PATH="$PROJECT_ROOT/.local/bin:$PATH"
export KUBECONFIG="$PROJECT_ROOT/.local/kubeconfig"
export HELM_CACHE_HOME="$PROJECT_ROOT/.local/helm/cache"
export HELM_CONFIG_HOME="$PROJECT_ROOT/.local/helm/config"
export HELM_DATA_HOME="$PROJECT_ROOT/.local/helm/data"
k() { kubectl --context kind-ticket-platform "$@"; }
require_cluster() { k cluster-info >/dev/null; }
