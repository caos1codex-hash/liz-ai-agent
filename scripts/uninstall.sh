#!/usr/bin/env bash
# ============================================================================
# Liz AI Agent — Desinstalador
# ============================================================================
# Fase 10 — Release v0.1.0
#
# Uso:
#   ./scripts/uninstall.sh             # desinstalación estándar
#   ./scripts/uninstall.sh --purge     # también elimina ~/.liz/ (config, memoria, herramientas auto-creadas)
#   ./scripts/uninstall.sh --prefix /usr/local   # prefix personalizado
# ============================================================================

set -euo pipefail

PREFIX="/usr/local"
PURGE=false

info()    { printf "\033[1;34m▸\033[0m %s\n" "$*"; }
success() { printf "\033[1;32m✓\033[0m %s\n" "$*"; }
warn()    { printf "\033[1;33m!\033[0m %s\n" "$*" >&2; }
error()   { printf "\033[1;31m✗\033[0m %s\n" "$*" >&2; }
die()     { error "$*"; exit 1; }

while [[ $# -gt 0 ]]; do
    case "$1" in
        --purge) PURGE=true; shift ;;
        --prefix) PREFIX="$2"; shift 2 ;;
        -h|--help)
            sed -n '3,12p' "$0"
            exit 0
            ;;
        *) die "Argumento desconocido: $1" ;;
    esac
done

BINDIR="${PREFIX}/bin"
SUDO=""
if [[ "$EUID" -ne 0 ]] && [[ ! -w "$PREFIX" ]]; then
    SUDO="sudo"
fi

info "Desinstalando Liz AI Agent..."

# 1. Detener procesos en ejecución
if pgrep -x liz >/dev/null 2>&1 || pgrep -x liz-server >/dev/null 2>&1; then
    warn "Se encontraron procesos Liz en ejecución. Deteniéndolos..."
    pkill -x liz 2>/dev/null || true
    pkill -x liz-server 2>/dev/null || true
    sleep 1
    success "Procesos detenidos"
fi

# 2. Eliminar binarios
for bin in liz liz-server; do
    if [[ -f "${BINDIR}/${bin}" ]]; then
        $SUDO rm -f "${BINDIR}/${bin}"
        success "Eliminado: ${BINDIR}/${bin}"
    fi
done

# 3. Eliminar entrada de menú
desktop_file="${HOME}/.local/share/applications/liz.desktop"
if [[ -f "$desktop_file" ]]; then
    rm -f "$desktop_file"
    success "Eliminado: ${desktop_file}"
fi

# 4. Eliminar configuración y datos (solo con --purge)
if $PURGE; then
    config_dir="${HOME}/.liz"
    if [[ -d "$config_dir" ]]; then
        warn "Eliminando ${config_dir} (config, memoria, herramientas auto-creadas, índices)..."
        rm -rf "$config_dir"
        success "Eliminado: ${config_dir}"
    fi
else
    if [[ -d "${HOME}/.liz" ]]; then
        info "Configuración conservada en ${HOME}/.liz (usa --purge para eliminarla)"
    fi
fi

echo
success "Desinstalación completada."
if ! $PURGE && [[ -d "${HOME}/.liz" ]]; then
    echo "  Para eliminar también config/memoria:  ./scripts/uninstall.sh --purge"
fi
