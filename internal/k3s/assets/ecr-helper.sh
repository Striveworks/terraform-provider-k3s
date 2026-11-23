#!/bin/bash
set -euo pipefail

# Install or remove the kubelet ECR credential provider.
#
# Environment variables:
#   - ECR_HELPER_ACTION
#     install (default) or cleanup.
#   - ECR_CREDENTIAL_PROVIDER_URL
#   - ECR_CREDENTIAL_PROVIDER_CHECKSUM
#     sha256 checksum of the downloaded binary. Required for install.
#   - ECR_ROLE_ARN
#   - ECR_TOKEN_FILE
#     Required for install. Written into the kubelet credential provider config.

BIN_DIR=/usr/local/libexec
BIN_PATH=/usr/local/libexec/ecr-credential-provider
CONFIG_DIR=/etc/rancher/k3s
CONFIG_PATH=/etc/rancher/k3s/credential-provider-config.yaml
RULES_PATH=/etc/fapolicyd/rules.d/81-ecr-credential-provider.rules

yaml_quote() {
  local value=$1
  value=${value//\\/\\\\}
  value=${value//\"/\\\"}
  printf '"%s"' "$value"
}

cleanup() {
  rm -f "$BIN_PATH" "$CONFIG_PATH"
  if [ -f "$RULES_PATH" ]; then
    rm -f "$RULES_PATH"
    if [ -d /etc/fapolicyd/rules.d ]; then
      fagenrules --load
      systemctl try-restart fapolicyd
    fi
  fi
}

install() {
  : "${ECR_CREDENTIAL_PROVIDER_URL:?ECR_CREDENTIAL_PROVIDER_URL is required}"
  : "${ECR_CREDENTIAL_PROVIDER_CHECKSUM:?ECR_CREDENTIAL_PROVIDER_CHECKSUM is required}"
  : "${ECR_ROLE_ARN:?ECR_ROLE_ARN is required}"
  : "${ECR_TOKEN_FILE:?ECR_TOKEN_FILE is required}"

  mkdir -p "$BIN_DIR" "$CONFIG_DIR"
  curl -fsSL -o "$BIN_PATH" "$ECR_CREDENTIAL_PROVIDER_URL" || { rm -f "$BIN_PATH"; exit 1; }
  echo "${ECR_CREDENTIAL_PROVIDER_CHECKSUM}  ${BIN_PATH}" | sha256sum -c - || { rm -f "$BIN_PATH"; exit 1; }
  chown root:root "$BIN_PATH"
  chmod 0755 "$BIN_PATH"
  if command -v restorecon >/dev/null 2>&1; then
    restorecon -v "$BIN_PATH"
  fi

  cat > "$CONFIG_PATH" <<EOF
apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
  - name: ecr-credential-provider
    matchImages:
      - "*.dkr.ecr.*.amazonaws.com"
      - "*.dkr.ecr-fips.*.amazonaws.com"
      - "*.dkr.ecr.*.amazonaws.com.cn"
      - "*.dkr.ecr-fips.*.amazonaws.com.cn"
    defaultCacheDuration: 12h
    apiVersion: credentialprovider.kubelet.k8s.io/v1
    env:
      - name: AWS_ROLE_ARN
        value: $(yaml_quote "$ECR_ROLE_ARN")
      - name: AWS_WEB_IDENTITY_TOKEN_FILE
        value: $(yaml_quote "$ECR_TOKEN_FILE")
      - name: AWS_EC2_METADATA_DISABLED
        value: "true"
      - name: AWS_STS_REGIONAL_ENDPOINTS
        value: regional
EOF
  chmod 0644 "$CONFIG_PATH"
  chown root:root "$CONFIG_PATH"

  if [ -d /etc/fapolicyd/rules.d ]; then
    cat > "$RULES_PATH" <<EOF
allow perm=any all : path=${BIN_PATH}
EOF
    if getent group fapolicyd >/dev/null 2>&1; then
      chown root:fapolicyd "$RULES_PATH"
    else
      chown root:root "$RULES_PATH"
    fi
    chmod 0644 "$RULES_PATH"
    fagenrules --load
    systemctl try-restart fapolicyd
  fi
}

case "${ECR_HELPER_ACTION:-install}" in
  install) install ;;
  cleanup) cleanup ;;
  *)
    echo "unknown ECR_HELPER_ACTION: ${ECR_HELPER_ACTION}" >&2
    exit 1
    ;;
esac
