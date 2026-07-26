#!/usr/bin/env bash
# ============================================================================
# Liz AI Agent — Instalador automático multi-distro
# ============================================================================
# Fase 10 — Release v0.1.0
#
# Uso:
#   curl -fsSL https://github.com/caos1codex-hash/liz-ai-agent/releases/latest/download/install.sh | bash
#   # o desde el repo:
#   ./scripts/install.sh
#
# Opciones:
#   --headless        Instalar solo el binario servidor (sin dependencias OpenGL)
#   --prefix PATH     Prefijo de instalación (default: /usr/local)
#   --version VERSION Versión a instalar (default: latest)
#   --no-deps         No instalar dependencias del sistema (asume que ya están)
#   --from-source     Compilar desde el código fuente (en vez de descargar binario)
#   --verbose         Output detallado
#   -h, --help        Mostrar esta ayuda
#
# Distro soportadas:
#   - Debian/Ubuntu (apt)
#   - Fedora/RHEL/CentOS (dnf/yum)
#   - Arch/Manjaro (pacman)
#   - openSUSE (zypper) — best effort
#
# Filosofía: "si no está en GitHub no existe" — todo se descarga del release
# oficial en https://github.com/caos1codex-hash/liz-ai-agent/releases
# ============================================================================

set -euo pipefail

# --- Configuración por defecto ---
REPO="caos1codex-hash/liz-ai-agent"
PREFIX="/usr/local"
VERSION="latest"
HEADLESS_ONLY=false
NO_DEPS=false
FROM_SOURCE=false
VERBOSE=false
BINDIR=""
CONFIGDIR="${HOME}/.liz"

# --- Helpers de output ---
info()    { printf "\033[1;34m▸\033[0m %s\n" "$*"; }
success() { printf "\033[1;32m✓\033[0m %s\n" "$*"; }
warn()    { printf "\033[1;33m!\033[0m %s\n" "$*" >&2; }
error()   { printf "\033[1;31m✗\033[0m %s\n" "$*" >&2; }
die()     { error "$*"; exit 1; }
debug()   { if $VERBOSE; then printf "\033[2m·\033[0m %s\n" "$*"; fi; }

# --- Parsear argumentos ---
while [[ $# -gt 0 ]]; do
    case "$1" in
        --headless)    HEADLESS_ONLY=true; shift ;;
        --prefix)      PREFIX="$2"; shift 2 ;;
        --version)     VERSION="$2"; shift 2 ;;
        --no-deps)     NO_DEPS=true; shift ;;
        --from-source) FROM_SOURCE=true; shift ;;
        --verbose|-v)  VERBOSE=true; shift ;;
        -h|--help)
            sed -n '3,30p' "$0"
            exit 0
            ;;
        *) die "Argumento desconocido: $1 (usa --help)" ;;
    esac
done

BINDIR="${PREFIX}/bin"

# --- Detección de plataforma ---
ARCH="$(uname -m)"
OS="$(uname -s)"
case "$OS" in
    Linux) PLATFORM="linux" ;;
    Darwin) PLATFORM="darwin" ;;
    *) die "Sistema operativo no soportado: $OS (solo Linux y macOS)" ;;
esac
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) die "Arquitectura no soportada: $ARCH (solo amd64 y arm64)" ;;
esac
info "Plataforma detectada: ${PLATFORM}/${ARCH}"

# --- Detección de distro Linux ---
detect_distro() {
    if [[ ! -f /etc/os-release ]]; then
        echo "unknown"
        return
    fi
    # shellcheck disable=SC1091
    . /etc/os-release
    case "${ID:-}"" / ""${ID_LIKE:-}" in
        *ubuntu*|*debian*)   echo "debian" ;;
        *fedora*|*rhel*|*centos*|*rocky*|*alma*) echo "fedora" ;;
        *arch*|*manjaro*|*garuda*) echo "arch" ;;
        *opensuse*|*suse*)   echo "opensuse" ;;
        *) echo "unknown" ;;
    esac
}

DISTRO="n/a"
if [[ "$PLATFORM" == "linux" ]]; then
    DISTRO="$(detect_distro)"
    info "Distro detectada: ${DISTRO}"
fi

# --- Comprobación de permisos sudo ---
need_sudo() {
    if [[ "$EUID" -ne 0 ]] && [[ ! -w "$PREFIX" ]]; then
        return 0
    fi
    return 1
}
SUDO=""
if need_sudo; then
    if ! command -v sudo >/dev/null 2>&1; then
        die "Se requiere sudo para instalar en ${PREFIX} pero no está disponible"
    fi
    SUDO="sudo"
    info "Se usará sudo para instalaciones en ${PREFIX}"
fi

# ============================================================================
# 1. INSTALAR DEPENDENCIAS DEL SISTEMA
# ============================================================================
install_deps() {
    if $NO_DEPS; then
        warn "--no-deps: saltando instalación de dependencias del sistema"
        return
    fi

    if $HEADLESS_ONLY; then
        info "Modo --headless: no se requieren dependencias OpenGL"
        return
    fi

    if [[ "$PLATFORM" != "linux" ]]; then
        info "Plataforma $PLATFORM: dependencias GUI no se gestionan automáticamente (ver README)"
        return
    fi

    info "Instalando dependencias de sistema para GUI Fyne (OpenGL/X11/Wayland)..."
    case "$DISTRO" in
        debian)
            $SUDO apt-get update -qq
            $SUDO apt-get install -y -qq \
                libgl1-mesa-dev xorg-dev libxrandr-dev libxinerama-dev \
                libxcursor-dev libxi-dev libxxf86vm-dev libwayland-dev \
                libxkbcommon-dev libegl-dev libglx-dev \
                ca-certificates curl
            ;;
        fedora)
            $SUDO dnf install -y -q \
                mesa-libGL-devel libXrandr-devel libXinerama-devel \
                libXcursor-devel libXi-devel libXxf86vm-devel wayland-devel \
                libxkbcommon-devel libX11-devel libXext-devel libXfixes-devel \
                libXdamage-devel mesa-libGLES-devel mesa-libEGL-devel \
                ca-certificates curl
            ;;
        arch)
            $SUDO pacman -S --noconfirm --needed \
                mesa libxrandr libxinerama libxcursor libxi libxxf86vm \
                wayland libxkbcommon xorg-server-devel \
                ca-certificates curl
            ;;
        opensuse)
            $SUDO zypper install -y \
                Mesa-libGL-devel libXrandr-devel libXinerama-devel \
                libXcursor-devel libXi-devel libXxf86vm-devel wayland-devel \
                libxkbcommon-devel xorg-x11-devel \
                ca-certificates curl
            ;;
        *)
            warn "Distro no reconocida. Instala manualmente las deps OpenGL (ver README)."
            warn "Lista: libGL, libX11, libXrandr, libXcursor, libXi, libXinerama,"
            warn "       libXxf86vm, libwayland, libxkbcommon, libEGL."
            ;;
    esac
    success "Dependencias instaladas"
}

# ============================================================================
# 2. DESCARGAR E INSTALAR BINARIO
# ============================================================================
resolve_version() {
    if [[ "$VERSION" == "latest" ]]; then
        local url="https://api.github.com/repos/${REPO}/releases/latest"
        debug "Resolviendo latest version desde: $url"
        local tag
        tag="$(curl -fsSL "$url" | grep -m1 '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')"
        [[ -z "$tag" ]] && die "No se pudo resolver la última versión desde GitHub"
        echo "$tag"
    else
        echo "$VERSION"
    fi
}

# Nombre del asset a descargar según plataforma y modo
asset_name() {
    local tag="$1"
    local v="${tag#v}"  # quitar 'v' inicial
    if $HEADLESS_ONLY; then
        echo "liz-server-${PLATFORM}-${ARCH}-${v}.tar.gz"
    else
        echo "liz-${PLATFORM}-${ARCH}-${v}.tar.gz"
    fi
}

download_and_install() {
    local tag
    tag="$(resolve_version)"
    info "Instalando Liz ${tag} para ${PLATFORM}/${ARCH}..."

    local asset
    asset="$(asset_name "$tag")"
    local url="https://github.com/${REPO}/releases/download/${tag}/${asset}"
    debug "Descargando: $url"

    local tmpdir
    tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' EXIT

    local archive="${tmpdir}/${asset}"
    if ! curl -fSL --progress-bar -o "$archive" "$url"; then
        error "No se pudo descargar $asset"
        if $HEADLESS_ONLY; then
            warn "Intenta sin --headless para usar el binario completo con GUI."
        else
            warn "Si tu plataforma no tiene binario precompilado, usa --from-source."
        fi
        die "Descarga fallida"
    fi
    success "Descarga completa: $asset"

    # Descomprimir
    tar -xzf "$archive" -C "$tmpdir"
    local binary_name="liz"
    $HEADLESS_ONLY && binary_name="liz-server"

    local extracted="${tmpdir}/${binary_name}"
    [[ ! -f "$extracted" ]] && extracted="$(find "$tmpdir" -name "${binary_name}" -type f | head -1)"
    [[ ! -f "$extracted" ]] && die "Binario no encontrado en el archive"

    # Instalar en $BINDIR
    $SUDO mkdir -p "$BINDIR"
    $SUDO install -m 0755 "$extracted" "${BINDIR}/${binary_name}"
    success "Binario instalado: ${BINDIR}/${binary_name}"

    # Verificar
    if "${BINDIR}/${binary_name}" --version >/dev/null 2>&1; then
        success "Verificación: $(${BINDIR}/${binary_name} --version)"
    else
        warn "El binario no responde a --version (¿arquitectura incorrecta?)"
    fi

    # Crear directorio de configuración
    mkdir -p "$CONFIGDIR"
    success "Directorio de configuración: ${CONFIGDIR}"

    # Copiar config de ejemplo si no existe
    if [[ ! -f "${CONFIGDIR}/config.yaml" ]]; then
        local cfg_url="https://raw.githubusercontent.com/${REPO}/${tag}/configs/liz.yaml.example"
        if curl -fsSL "$cfg_url" -o "${CONFIGDIR}/config.yaml" 2>/dev/null; then
            success "Config inicial: ${CONFIGDIR}/config.yaml (editar con tu API key NVIDIA)"
        else
            warn "No se pudo descargar config de ejemplo desde ${cfg_url}"
        fi
    fi
}

# ============================================================================
# 3. COMPILAR DESDE EL CÓDIGO FUENTE
# ============================================================================
build_from_source() {
    info "Compilando Liz desde el código fuente..."

    if ! command -v go >/dev/null 2>&1; then
        info "Go no encontrado. Instalando Go 1.22..."
        local go_version="go1.22.5"
        local go_tar="${go_version}.${PLATFORM}-${ARCH}.tar.gz"
        local go_url="https://go.dev/dl/${go_tar}"
        local tmpdir; tmpdir="$(mktemp -d)"
        trap 'rm -rf "$tmpdir"' EXIT
        curl -fsSL "$go_url" -o "${tmpdir}/${go_tar}"
        $SUDO rm -rf /usr/local/go 2>/dev/null || true
        $SUDO tar -C /usr/local -xzf "${tmpdir}/${go_tar}"
        export PATH="/usr/local/go/bin:${PATH}"
        success "Go instalado: $(go version)"
    fi

    local tmpdir; tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' EXIT

    info "Clonando repositorio..."
    git clone --depth 1 "https://github.com/${REPO}.git" "${tmpdir}/liz"
    cd "${tmpdir}/liz"

    local ldflags="-s -w -X main.version=$(git describe --tags --always 2>/dev/null || echo dev)"
    local binary_name="liz"
    local build_tags=""
    if $HEADLESS_ONLY; then
        binary_name="liz-server"
        build_tags="-tags headless"
        info "Compilando en modo headless (sin GUI, CGO_ENABLED=0, estático)..."
        CGO_ENABLED=0 go build -ldflags="${ldflags}" $build_tags -o "${binary_name}" ./cmd/liz
    else
        info "Compilando binario completo con GUI Fyne (requiere deps OpenGL)..."
        CGO_ENABLED=1 go build -ldflags="${ldflags}" $build_tags -o "${binary_name}" ./cmd/liz
    fi

    $SUDO mkdir -p "$BINDIR"
    $SUDO install -m 0755 "${binary_name}" "${BINDIR}/${binary_name}"
    success "Binario instalado: ${BINDIR}/${binary_name}"

    mkdir -p "$CONFIGDIR"
    if [[ ! -f "${CONFIGDIR}/config.yaml" ]]; then
        cp configs/liz.yaml.example "${CONFIGDIR}/config.yaml"
        success "Config inicial: ${CONFIGDIR}/config.yaml"
    fi
}

# ============================================================================
# 4. POST-INSTALACIÓN
# ============================================================================
post_install() {
    # Crear entrada de menú para Linux desktop
    if [[ "$PLATFORM" == "linux" ]] && ! $HEADLESS_ONLY; then
        local desktop_file="${HOME}/.local/share/applications/liz.desktop"
        mkdir -p "$(dirname "$desktop_file")"
        cat > "$desktop_file" <<EOF
[Desktop Entry]
Type=Application
Name=Liz AI Agent
Comment=Agente de IA autónomo para Linux
Exec=${BINDIR}/liz
Icon=utilities-terminal
Terminal=false
Categories=Development;System;Utility;
StartupNotify=true
Keywords=ai;agent;llm;nvidia;
EOF
        success "Entrada de menú creada: ${desktop_file}"
    fi

    # Mensaje final
    echo
    printf "\033[1;35m"
    cat <<'BANNER'
  ╔══════════════════════════════════════════════════════════════════════╗
  ║                                                                      ║
  ║   ██╗     ██╗██████╗ ██████╗  █████╗ ██████╗ ███████╗                ║
  ║   ██║     ██║██╔══██╗██╔══██╗██╔══██╗██╔══██╗██╔════╝                ║
  ║   ██║     ██║██████╔╝██████╔╝███████║██████╔╝███████╗                ║
  ║   ██║     ██║██╔═══╝ ██╔══██╗██╔══██║██╔══██╗╚════██║                ║
  ║   ███████╗██║██║     ██████╔╝██║  ██║██║  ██║███████║                ║
  ║   ╚══════╝╚═╝╚═╝     ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═╝╚══════╝                ║
  ║                                                                      ║
  ║              v0.1.0 — Instalación completada                         ║
  ╚══════════════════════════════════════════════════════════════════════╝
BANNER
    printf "\033[0m"
    echo
    if $HEADLESS_ONLY; then
        echo "  Ejecuta:  liz-server --headless"
        echo "  Config:   ${CONFIGDIR}/config.yaml"
        echo "  Logs:     ${CONFIGDIR}/liz.log"
    else
        echo "  Ejecuta:  liz              (modo desktop con GUI Fyne)"
        echo "            liz --headless   (modo servidor puro)"
        echo "  Config:   ${CONFIGDIR}/config.yaml"
    fi
    echo
    echo "  Próximos pasos:"
    echo "    1. Edita ${CONFIGDIR}/config.yaml y agrega tu API key de NVIDIA"
    echo "    2. Ejecuta 'liz' o 'liz-server --headless'"
    echo "    3. Lee la documentación: https://github.com/${REPO}#readme"
    echo
    success "¡Liz está lista para usarse!"
}

# ============================================================================
# MAIN
# ============================================================================
main() {
    info "=== Instalador de Liz AI Agent ==="
    debug "PREFIX=${PREFIX} VERSION=${VERSION} HEADLESS_ONLY=${HEADLESS_ONLY} FROM_SOURCE=${FROM_SOURCE}"

    install_deps

    if $FROM_SOURCE; then
        build_from_source
    else
        download_and_install
    fi

    post_install
}

main "$@"
