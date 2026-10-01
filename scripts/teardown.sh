#!/usr/bin/env bash
source "$(dirname "$0")/common.sh"
kind delete cluster --name ticket-platform --kubeconfig "$KUBECONFIG"
echo 'Cluster removed. .local/data and credentials are retained. Remove them explicitly only if you want a fresh database.'
