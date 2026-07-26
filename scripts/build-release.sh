#!/usr/bin/env bash
# ============================================================================
# Liz AI Agent — Script de cross-compilation para releases
# ============================================================================
# Fase 10 — Release v0.1.0
#
# Compila binarios para todas las plataformas soportadas:
#
#   Desktop (con GUI Fyne — requiere CGO + OpenGL):
#     - linux/amd64   → liz-linux-amd64-<version>
#
#   Headless (sin GUI — CGO_ENABLED=0, estático, portable):
#     - linux/amd64   → liz-server-linux-amd64-<version>
#     - linux/arm64   → liz-server-linux-arm64-<version>
#     - darwin/amd64  → liz-server-darwin-amd64-<version>
#     - darwin/arm64  → liz-server-darwin-arm64-<version>
#     - windows/amd64 → liz-server-windows-amd64-<version>.exe
#
# Uso:
#   ./scripts/build-release.sh                  # usa versión de main.go
#   ./scripts/build-release.sh v0.1.0           # versión específica
#   ./scripts/build-release.sh --desktop-only   # solo binario desktop
#   ./scripts/build-release.sh --headless-only  # solo binarios headless
# ============================================================================

set -euo pipefail

# --- Configuración ---
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

OUT_DIR="${ROOT_DIR}/dist"
MAIN_PKG="./cmd/liz"

# Extraer versión del código fuente si no se especifica
VERSION="${1:-}"
if [[ -z "$VERSION" ]]; then
    VERSION="$(grep -m1 '^const version' cmd/liz/main.go | sed -E 's/.*"([^"]+)".*/\1/')"
    [[ -z "$VERSION" ]] && { echo "ERROR: no se pudo detectar versión"; exit 1; }
fi
VERSION="${VERSION#v}"
TAG="v${VERSION}"

# Flags de build: -s (strip symbols), -w (strip DWARF)
LDFLAGS="-s -w -X main.version=${VERSION}"

# Colores
info()    { printf "\033[1;34m▸\033[0m %s\n" "$*"; }
success() { printf "\033[1;32m✓\033[0m %s\n" "$*"; }
warn()    { printf "\033[1;33m!\033[0m %s\n" "$*" >&2; }
error()   { printf "\033[1;31m✗\033[0m %s\n" "$*" >&2; }

# Verificar que Go está instalado
if ! command -v go >/dev/null 2>&1; then
    error "Go no está instalado. Instálalo desde https://go.dev/dl/"
    exit 1
fi

info "=== Build de Liz ${TAG} ==="
info "Go: $(go version)"
info "Output: ${OUT_DIR}/"
echo

mkdir -p "$OUT_DIR"

# Array para registrar los binarios producidos
declare -a BUILT_BINS

# --- Helper: construir un target headless ---
build_headless() {
    local goos="$1"
    local goarch="$2"
    local suffix="$3"
    local output="${OUT_DIR}/liz-server-${suffix}"

    info "Compilando headless: GOOS=${goos} GOARCH=${goarch} CGO_ENABLED=0"
    GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
        go build -tags headless -ldflags="${LDFLAGS}" \
        -o "$output" "$MAIN_PKG"

    local size
    size="$(du -h "$output" | cut -f1)"
    success "→ ${output} (${size})"
    BUILT_BINS+=("$output")
}

# --- Helper: construir desktop (solo linux/amd64 nativo) ---
build_desktop() {
    local output="${OUT_DIR}/liz-linux-amd64"

    info "Compilando desktop (GUI Fyne): linux/amd64 CGO_ENABLED=1"
    if ! pkg-config --exists gl glfw3 2>/dev/null; then
        if ! (dpkg -s libgl1-mesa-dev xorg-dev >/dev/null 2>&1) && \
           ! (rpm -q mesa-libGL-devel libX11-devel >/dev/null 2>&1) && \
           ! (pacman -Q mesa libx11 >/dev/null 2>&1); then
            warn "Dependencias OpenGL dev no detectadas — el build puede fallar"
            warn "Instala: libgl1-mesa-dev xorg-dev libxrandr-dev libxinerama-dev \\"
            warn "         libxcursor-dev libxi-dev libxxf86vm-dev libwayland-dev \\"
            warn "         libxkbcommon-dev libegl-dev libglx-dev"
        fi
    fi

    CGO_ENABLED=1 go build -ldflags="${LDFLAGS}" -o "$output" "$MAIN_PKG"

    local size
    size="$(du -h "$output" | cut -f1)"
    success "→ ${output} (${size})"
    BUILT_BINS+=("$output")
}

# --- Helper: crear tarball para un binario ---
make_tarball() {
    local bin="$1"
    local tarball="${bin}-${TAG}.tar.gz"
    local staging; staging="$(mktemp -d)"
    trap 'rm -rf "$staging"' RETURN

    cp "$bin" "$staging/"
    cp "${ROOT_DIR}/configs/liz.yaml.example" "$staging/" 2>/dev/null || true
    cp "${ROOT_DIR}/scripts/install.sh" "$staging/" 2>/dev/null || true
    cp "${ROOT_DIR}/scripts/uninstall.sh" "$staging/" 2>/dev/null || true
    cp "${ROOT_DIR}/README.md" "$staging/" 2>/dev/null || true
    cp "${ROOT_DIR}/LICENSE" "$staging/" 2>/dev/null || true

    tar -C "$staging" -czf "$tarball" .
    local size; size="$(du -h "$tarball" | cut -f1)"
    success "→ ${tarball} (${size})"
}

# --- Helper: generar checksums ---
make_checksums() {
    info "Generando checksums SHA256..."
    local sums="${OUT_DIR}/checksums-${TAG}.txt"
    : > "$sums"
    for f in "$OUT_DIR"/*.tar.gz "$OUT_DIR"/liz-* "$OUT_DIR"/liz-server-*; do
        [[ -f "$f" ]] || continue
        [[ "$f" == *.tar.gz ]] || continue
        local basename; basename="$(basename "$f")"
        local sha; sha="$(sha256sum "$f" | cut -d' ' -f1)"
        echo "${sha}  ${basename}" >> "$sums"
    done
    success "→ ${sums}"
}

# ============================================================================
# MAIN
# ============================================================================

# Determinar modo
MODE="${2:-all}"
case "$MODE" in
    --desktop-only)  MODE="desktop" ;;
    --headless-only) MODE="headless" ;;
    *)               MODE="all" ;;
esac

# --- Build desktop (solo si estamos en linux/amd64 con deps OpenGL) ---
if [[ "$MODE" == "all" || "$MODE" == "desktop" ]]; then
    if [[ "$(uname -s)" == "Linux" ]] && [[ "$(uname -m)" == "x86_64" ]]; then
        build_desktop
    else
        warn "Build desktop omitido: requiere linux/amd64 nativo con deps OpenGL"
        warn "(en CI se ejecuta en ubuntu-latest con deps instaladas)"
    fi
fi

# --- Build headless para todas las plataformas ---
if [[ "$MODE" == "all" || "$MODE" == "headless" ]]; then
    build_headless linux   amd64 "linux-amd64"
    build_headless linux   arm64 "linux-arm64"
    build_headless darwin  amd64 "darwin-amd64"
    build_headless darwin  arm64 "darwin-arm64"
    build_headless windows amd64 "windows-amd64.exe"
fi

# --- Crear tarballs ---
echo
info "Empaquetando tarballs..."
for bin in "${BUILT_BINS[@]}"; do
    make_tarball "$bin"
done

# --- Checksums ---
make_checksums

# --- Resumen final ---
echo
info "=== Resumen ==="
for bin in "${BUILT_BINS[@]}"; do
    printf "  %-50s %s\n" "$(basename "$bin")" "$(du -h "$bin" | cut -f1)"
done
echo
echo "Tarballs y checksums en: ${OUT_DIR}/"
echo
success "Build completado para ${TAG}"
