#!/usr/bin/env bash
# ============================================================================
# Liz AI Agent — Empaquetador tarball genérico
# ============================================================================
# Fase 10 — Release v0.1.0
#
# Crea un tarball .tar.gz genérico con binario + configs + scripts + docs.
# Portable a cualquier distro Linux (y macOS con binario adecuado).
#
# Uso:
#   ./scripts/package-tarball.sh                  # usa versión de main.go
#   ./scripts/package-tarball.sh v0.1.0           # versión específica
# ============================================================================

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

VERSION="${1:-}"
if [[ -z "$VERSION" ]]; then
    VERSION="$(grep -m1 '^const version' cmd/liz/main.go | sed -E 's/.*"([^"]+)".*/\1/')"
fi
VERSION="${VERSION#v}"
TAG="v${VERSION}"

OUT_DIR="${ROOT_DIR}/dist"
STAGING="${OUT_DIR}/tarball-staging/liz-${TAG}"
TARBALL="${OUT_DIR}/liz-${TAG}.tar.gz"

info()    { printf "\033[1;34m▸\033[0m %s\n" "$*"; }
success() { printf "\033[1;32m✓\033[0m %s\n" "$*"; }
die()     { printf "\033[1;31m✗\033[0m %s\n" "$*" >&2; exit 1; }

info "=== Empaquetando tarball genérico ${TAG} ==="

# Limpiar staging
rm -rf "$STAGING" "$TARBALL"
mkdir -p "$STAGING"/{bin,configs,scripts,docs}

# --- Copiar binarios ---
for bin in liz-linux-amd64 liz-server-linux-amd64 liz-server-linux-arm64 liz-server-darwin-amd64 liz-server-darwin-arm64; do
    if [[ -f "${OUT_DIR}/${bin}" ]]; then
        cp "${OUT_DIR}/${bin}" "${STAGING}/bin/"
    fi
done

# --- Config de ejemplo ---
cp configs/liz.yaml.example "${STAGING}/configs/"

# --- Scripts ---
cp scripts/install.sh "${STAGING}/scripts/"
cp scripts/uninstall.sh "${STAGING}/scripts/"
[[ -f scripts/build-release.sh ]] && cp scripts/build-release.sh "${STAGING}/scripts/"

# --- Docs ---
cp README.md "${STAGING}/"
cp CHANGELOG.md "${STAGING}/"
cp LICENSE "${STAGING}/" 2>/dev/null || true
cp -r docs/ "${STAGING}/docs/" 2>/dev/null || true

# --- README específico del tarball ---
cat > "${STAGING}/INSTALL.txt" <<EOF
Liz AI Agent ${TAG} — Instalación rápida
========================================

1. Ejecutar instalador (recomendado):
   ./scripts/install.sh

2. O instalar manualmente:
   sudo mkdir -p /usr/local/bin
   sudo cp bin/liz-linux-amd64 /usr/local/bin/liz
   mkdir -p ~/.liz
   cp configs/liz.yaml.example ~/.liz/config.yaml
   # Editar ~/.liz/config.yaml con tu API key NVIDIA

3. Ejecutar:
   liz              # modo desktop con GUI Fyne
   liz --headless   # modo servidor puro (sin GUI)

Para desinstalar: ./scripts/uninstall.sh [--purge]

Documentación completa: https://github.com/caos1codex-hash/liz-ai-agent
EOF

# --- Crear tarball ---
info "Creando ${TARBALL}..."
tar -C "${OUT_DIR}/tarball-staging" -czf "$TARBALL" "liz-${TAG}"

if [[ -f "$TARBALL" ]]; then
    success "→ ${TARBALL} ($(du -h "$TARBALL" | cut -f1))"
    info "Instalar con:"
    info "  tar -xzf $(basename "$TARBALL")"
    info "  cd liz-${TAG} && ./scripts/install.sh"
else
    die "No se pudo crear el tarball"
fi

# Limpiar staging
rm -rf "${OUT_DIR}/tarball-staging"
