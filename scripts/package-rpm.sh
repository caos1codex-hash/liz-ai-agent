#!/usr/bin/env bash
# ============================================================================
# Liz AI Agent — Empaquetador RPM para Fedora/RHEL/CentOS
# ============================================================================
# Fase 10 — Release v0.1.0
#
# Crea un paquete .rpm instalable en Fedora/RHEL/CentOS/Rocky/Alma.
#
# Uso:
#   ./scripts/package-rpm.sh                  # usa versión de main.go
#   ./scripts/package-rpm.sh v0.1.0           # versión específica
#
# Output:
#   dist/liz-<version>-1.x86_64.rpm
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
RPM_ROOT="${OUT_DIR}/rpm-root"
RPM_FILE="${OUT_DIR}/liz-${VERSION}-1.x86_64.rpm"

info()    { printf "\033[1;34m▸\033[0m %s\n" "$*"; }
success() { printf "\033[1;32m✓\033[0m %s\n" "$*"; }
die()     { printf "\033[1;31m✗\033[0m %s\n" "$*" >&2; exit 1; }

# Verificar dependencias
if ! command -v rpmbuild >/dev/null 2>&1; then
    die "rpmbuild no está instalado (dnf install rpm-build / apt-get install rpm)"
fi

# Verificar binario desktop
BIN="${OUT_DIR}/liz-linux-amd64"
if [[ ! -f "$BIN" ]]; then
    info "Binario desktop no encontrado. Compilando..."
    mkdir -p "$OUT_DIR"
    CGO_ENABLED=1 go build -ldflags="-s -w -X main.version=${VERSION}" -o "$BIN" ./cmd/liz
fi

info "=== Empaquetando RPM v${VERSION} ==="

# Preparar árbol RPM
rm -rf "$RPM_ROOT"
mkdir -p "$RPM_ROOT"/{BUILD,RPMS,SOURCES,SPECS,SRPMS}
mkdir -p "$RPM_ROOT/SOURCES/liz-${VERSION}"

# Copiar archivos al SOURCES
install -m 0755 "$BIN" "$RPM_ROOT/SOURCES/liz-${VERSION}/liz"
cp configs/liz.yaml.example "$RPM_ROOT/SOURCES/liz-${VERSION}/" 2>/dev/null || true
cp README.md "$RPM_ROOT/SOURCES/liz-${VERSION}/" 2>/dev/null || true
cp CHANGELOG.md "$RPM_ROOT/SOURCES/liz-${VERSION}/" 2>/dev/null || true
cp LICENSE "$RPM_ROOT/SOURCES/liz-${VERSION}/" 2>/dev/null || true

# Crear .desktop entry ANTES del tarball para que se incluya
cat > "$RPM_ROOT/SOURCES/liz-${VERSION}/liz.desktop" <<EOF
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

# Crear tarball fuente (incluye el .desktop)
tar -C "$RPM_ROOT/SOURCES" -czf "$RPM_ROOT/SOURCES/liz-${VERSION}.tar.gz" "liz-${VERSION}"

# --- SPEC file ---
cat > "$RPM_ROOT/SPECS/liz.spec" <<EOF
Name:       liz
Version:    ${VERSION}
Release:    1%{?dist}
Summary:    Liz — AI Agent autónomo para Linux
License:    MIT
URL:        https://github.com/caos1codex-hash/liz-ai-agent
Source0:    %{name}-%{version}.tar.gz
BuildArch:  x86_64
Requires:   mesa-libGL libX11 libXrandr libXinerama libXcursor libXi libXxf86vm wayland-client libxkbcommon libEGL libglvnd-glx

%description
Liz es un agente de IA autónomo para Linux que controla completamente el
sistema operativo mediante lenguaje natural. Combina las capacidades de
Claude Code/Cursor/Aider con Google Assistant.

Features:
- Control total del sistema (procesos, archivos, monitor)
- Multi-modelo NVIDIA con selección inteligente
- Auto-creación de herramientas que no tiene
- GUI nativa Fyne (sin navegador, sin Electron)
- 7 herramientas integradas + auto-creadas
- Memoria conversacional con sesiones y hechos

%prep
%setup -q

%build
# Binario ya precompilado — solo empaquetado

%install
mkdir -p %{buildroot}/usr/local/bin
mkdir -p %{buildroot}/usr/share/applications
mkdir -p %{buildroot}/etc/liz
install -m 0755 liz %{buildroot}/usr/local/bin/liz
install -m 0644 liz.desktop %{buildroot}/usr/share/applications/liz.desktop
install -m 0644 liz.yaml.example %{buildroot}/etc/liz/config.yaml.example || true

%files
%doc README.md CHANGELOG.md
%license LICENSE
/usr/local/bin/liz
/usr/share/applications/liz.desktop
/etc/liz/config.yaml.example

%post
echo ""
echo "Liz AI Agent se ha instalado en /usr/local/bin/liz"
echo "Config de ejemplo: /etc/liz/config.yaml.example"
echo "Copia a ~/.liz/config.yaml y edita con tu API key NVIDIA:"
echo "  mkdir -p ~/.liz && cp /etc/liz/config.yaml.example ~/.liz/config.yaml"
echo ""

%preun
pkill -x liz 2>/dev/null || true
pkill -x liz-server 2>/dev/null || true
exit 0

%changelog
* $(date '+%a %b %d %Y') caos1codex-hash <caos1codex-hash@users.noreply.github.com> - ${VERSION}-1
- Release ${VERSION} de Liz AI Agent
EOF

# --- Construir RPM ---
info "Construyendo RPM..."
rpmbuild --define "_topdir ${RPM_ROOT}" -bb "$RPM_ROOT/SPECS/liz.spec" 2>&1 | tail -5

# Copiar al destino final
find "$RPM_ROOT/RPMS" -name "*.rpm" -exec cp {} "$RPM_FILE" \;

if [[ -f "$RPM_FILE" ]]; then
    success "→ ${RPM_FILE} ($(du -h "$RPM_FILE" | cut -f1))"
    info "Instalar con: sudo dnf install ${RPM_FILE}"
else
    die "No se pudo crear el paquete RPM"
fi

# Limpiar
rm -rf "$RPM_ROOT"
