#!/usr/bin/env bash
# ============================================================================
# Liz AI Agent — Empaquetador DEB para Debian/Ubuntu
# ============================================================================
# Fase 10 — Release v0.1.0
#
# Crea un paquete .deb instalable en Debian/Ubuntu/Mint/Pop!_OS/etc.
#
# Uso:
#   ./scripts/package-deb.sh                  # usa versión de main.go
#   ./scripts/package-deb.sh v0.1.0           # versión específica
#
# Output:
#   dist/liz_<version>_amd64.deb
# ============================================================================

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

VERSION="${1:-}"
if [[ -z "$VERSION" ]]; then
    VERSION="$(grep -m1 '^const version' cmd/liz/main.go | sed -E 's/.*"([^"]+)".*/\1/')"
fi
VERSION="${VERSION#v}"
PKG_VER="${VERSION//-/_}"

OUT_DIR="${ROOT_DIR}/dist"
PKG_DIR="${OUT_DIR}/deb-staging"
DEB_FILE="${OUT_DIR}/liz_${PKG_VER}_amd64.deb"

info()    { printf "\033[1;34m▸\033[0m %s\n" "$*"; }
success() { printf "\033[1;32m✓\033[0m %s\n" "$*"; }
die()     { printf "\033[1;31m✗\033[0m %s\n" "$*" >&2; exit 1; }

# Verificar dependencias
for cmd in dpkg-deb fakeroot; do
    command -v "$cmd" >/dev/null 2>&1 || die "$cmd no está instalado (apt-get install dpkg-dev fakeroot)"
done

# Verificar que el binario desktop ya está construido
BIN="${OUT_DIR}/liz-linux-amd64"
if [[ ! -f "$BIN" ]]; then
    info "Binario desktop no encontrado. Compilando..."
    mkdir -p "$OUT_DIR"
    CGO_ENABLED=1 go build -ldflags="-s -w -X main.version=${VERSION}" -o "$BIN" ./cmd/liz
fi

info "=== Empaquetando DEB v${VERSION} ==="

# Limpiar staging
rm -rf "$PKG_DIR"
mkdir -p "$PKG_DIR/DEBIAN"
mkdir -p "$PKG_DIR/usr/local/bin"
mkdir -p "$PKG_DIR/usr/share/doc/liz"
mkdir -p "$PKG_DIR/usr/share/applications"
mkdir -p "$PKG_DIR/etc/liz"

# --- Copiar binario ---
install -m 0755 "$BIN" "$PKG_DIR/usr/local/bin/liz"

# --- Config de ejemplo ---
cp configs/liz.yaml.example "$PKG_DIR/etc/liz/config.yaml.example" 2>/dev/null || true

# --- Documentación ---
cp README.md "$PKG_DIR/usr/share/doc/liz/" 2>/dev/null || true
cp CHANGELOG.md "$PKG_DIR/usr/share/doc/liz/" 2>/dev/null || true
cp LICENSE "$PKG_DIR/usr/share/doc/liz/" 2>/dev/null || true

cat > "$PKG_DIR/usr/share/doc/liz/copyright" <<EOF
Liz AI Agent
Copyright (c) 2026 caos1codex-hash

Licencia: ver archivo LICENSE
EOF

# --- Entrada de menú ---
cat > "$PKG_DIR/usr/share/applications/liz.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=Liz AI Agent
Comment=Agente de IA autónomo para Linux
Exec=/usr/local/bin/liz
Icon=utilities-terminal
Terminal=false
Categories=Development;System;Utility;
StartupNotify=true
Keywords=ai;agent;llm;nvidia;
EOF

# --- Mantenimiento: scripts postinst, prerm ---
cat > "$PKG_DIR/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e
# Crear directorio de configuración en el home del usuario que ejecuta liz por primera vez
echo ""
echo "Liz AI Agent se ha instalado en /usr/local/bin/liz"
echo "Config de ejemplo: /etc/liz/config.yaml.example"
echo "Copia a ~/.liz/config.yaml y edita con tu API key NVIDIA:"
echo "  mkdir -p ~/.liz && cp /etc/liz/config.yaml.example ~/.liz/config.yaml"
echo ""
EOF
chmod 0755 "$PKG_DIR/DEBIAN/postinst"

cat > "$PKG_DIR/DEBIAN/prerm" <<'EOF'
#!/bin/sh
set -e
# Detener procesos si están corriendo
pkill -x liz 2>/dev/null || true
pkill -x liz-server 2>/dev/null || true
exit 0
EOF
chmod 0755 "$PKG_DIR/DEBIAN/prerm"

# --- control file ---
INSTALLED_SIZE="$(du -sk "$PKG_DIR" | cut -f1)"
cat > "$PKG_DIR/DEBIAN/control" <<EOF
Package: liz
Version: ${PKG_VER}
Section: devel
Priority: optional
Architecture: amd64
Installed-Size: ${INSTALLED_SIZE}
Depends: libgl1-mesa-glx, libx11-6, libxrandr2, libxinerama1, libxcursor1, libxi6, libxxf86vm1, libwayland-client0, libxkbcommon0, libegl1, libglx0
Maintainer: caos1codex-hash <caos1codex-hash@users.noreply.github.com>
Description: Liz — AI Agent autónomo para Linux
 Agente de IA que controla completamente tu Linux mediante lenguaje natural.
 Combina Claude Code, Cursor, Aider y Google Assistant en un único binario.
 .
 Features:
  * Control total del sistema (procesos, archivos, monitor)
  * Multi-modelo NVIDIA con selección inteligente
  * Auto-creación de herramientas que no tiene
  * GUI nativa Fyne (sin navegador, sin Electron)
  * 7 herramientas integradas + auto-creadas
  * Memoria conversacional con sesiones y hechos
Homepage: https://github.com/caos1codex-hash/liz-ai-agent
EOF

# --- Construir DEB ---
info "Construyendo paquete DEB..."
fakeroot dpkg-deb --build --root-owner-group "$PKG_DIR" "$DEB_FILE"

# Verificar
if [[ -f "$DEB_FILE" ]]; then
    success "→ ${DEB_FILE} ($(du -h "$DEB_FILE" | cut -f1))"
    info "Instalar con: sudo dpkg -i ${DEB_FILE}"
else
    die "No se pudo crear el paquete DEB"
fi

# Limpiar staging
rm -rf "$PKG_DIR"
