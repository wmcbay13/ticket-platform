#!/usr/bin/env bash
source "$(dirname "$0")/common.sh"
mkdir -p .local/bin .local/downloads
case "$(uname -m)" in x86_64) arch=amd64;; aarch64) arch=arm64;; *) echo 'Unsupported architecture'; exit 1;; esac
install_checked() {
  local name="$1" url="$2" checksum_url="$3"
  curl -fsSL --retry 3 "$url" -o ".local/downloads/$name"
  curl -fsSL --retry 3 "$checksum_url" -o ".local/downloads/$name.sha256"
  local checksum
  checksum="$(awk '{print $1}' ".local/downloads/$name.sha256")"
  printf '%s  %s\n' "$checksum" ".local/downloads/$name" | sha256sum -c -
}
if ! command -v kubectl >/dev/null; then
  url="https://dl.k8s.io/release/v${KUBERNETES_VERSION}/bin/linux/$arch/kubectl"
  install_checked kubectl "$url" "$url.sha256"
  install -m 755 .local/downloads/kubectl .local/bin/kubectl
fi
if ! command -v kind >/dev/null; then
  url="https://github.com/kubernetes-sigs/kind/releases/download/v${KIND_VERSION}/kind-linux-$arch"
  install_checked kind "$url" "$url.sha256sum"
  install -m 755 .local/downloads/kind .local/bin/kind
fi
if ! command -v helm >/dev/null; then
  file="helm-v${HELM_VERSION}-linux-$arch.tar.gz"
  install_checked "$file" "https://get.helm.sh/$file" "https://get.helm.sh/$file.sha256sum"
  tar -xzf ".local/downloads/$file" -C .local/downloads
  install -m 755 ".local/downloads/linux-$arch/helm" .local/bin/helm
fi
python3 -c 'import yaml' || { echo 'Install Python PyYAML (6.0.3), for example in a virtualenv, then rerun.'; exit 1; }
for tool in docker kind kubectl helm go npm python3 curl jq; do command -v "$tool" >/dev/null || { echo "Missing $tool"; exit 1; }; done
echo 'Tools available. Downloads and Helm caches stay under .local/.'
