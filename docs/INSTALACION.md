# Instalación de Liz AI Agent

> **Fase 10 — Release v0.1.0**
>
> Esta guía documenta todas las formas de instalar Liz: instalador automático,
> binarios precompilados, paquetes nativos (DEB/RPM/AUR), Docker, y compilación
> desde el código fuente.

## Tabla de contenidos

- [Instalación rápida (una línea)](#instalación-rápida-una-línea)
- [Requisitos previos](#requisitos-previos)
- [Binarios precompilados](#binarios-precompilados)
- [Paquetes nativos](#paquetes-nativos)
  - [DEB (Debian/Ubuntu)](#deb-debianubuntu)
  - [RPM (Fedora/RHEL)](#rpm-fedorarhel)
  - [AUR (Arch Linux)](#aur-arch-linux)
- [Docker](#docker)
- [Compilación desde el código fuente](#compilación-desde-el-código-fuente)
- [Configuración inicial](#configuración-inicial)
- [Verificación de la instalación](#verificación-de-la-instalación)
- [Desinstalación](#desinstalación)
- [Solución de problemas](#solución-de-problemas)

---

## Instalación rápida (una línea)

La forma más sencilla — el instalador detecta la distro, descarga el binario
adecuado, instala las dependencias del sistema y crea la configuración inicial:

```bash
curl -fsSL https://github.com/caos1codex-hash/liz-ai-agent/releases/latest/download/install.sh | bash
```

Opciones del instalador:

```bash
# Modo servidor puro (sin GUI, sin dependencias OpenGL)
curl -fsSL .../install.sh | bash -s -- --headless

# Compilar desde el código fuente en vez de descargar binario
curl -fsSL .../install.sh | bash -s -- --from-source

# Prefijo de instalación personalizado
curl -fsSL .../install.sh | bash -s -- --prefix /opt/liz

# Versión específica
curl -fsSL .../install.sh | bash -s -- --version v0.10.0

# Sin instalar dependencias del sistema
curl -fsSL .../install.sh | bash -s -- --no-deps

# Verbose output para depuración
curl -fsSL .../install.sh | bash -s -- --verbose
```

---

## Requisitos previos

### Mínimos (modo headless / servidor)

- Linux x86_64 o ARM64, macOS Intel o Apple Silicon
- glibc 2.31+ (Linux) o macOS 10.15+
- 64MB RAM mínimo, 256MB recomendado
- Conexión a internet para descargar NVIDIA API

### Completos (modo desktop con GUI Fyne)

Además de los mínimos:

**Linux (Debian/Ubuntu):**
```bash
sudo apt install libgl1-mesa-glx libx11-6 libxrandr2 libxinerama1 \
     libxcursor1 libxi6 libxxf86vm1 libwayland-client0 libxkbcommon0 \
     libegl1 libglx0
```

**Linux (Fedora/RHEL):**
```bash
sudo dnf install mesa-libGL libX11 libXrandr libXinerama libXcursor \
     libXi libXxf86vm wayland-client libxkbcommon libEGL libglvnd-glx
```

**Linux (Arch):**
```bash
sudo pacman -S mesa libx11 libxrandr libxinerama libxcursor libxi \
              libxxf86vm wayland libxkbcommon
```

**macOS:** No requiere dependencias adicionales para el binario headless.
La GUI Fyne solo está disponible para Linux en v0.1.0.

---

## Binarios precompilados

Descarga directa desde [GitHub Releases](https://github.com/caos1codex-hash/liz-ai-agent/releases):

| Plataforma | Binario | Modo |
|------------|---------|------|
| Linux x86_64 | `liz-linux-amd64` | Desktop con GUI Fyne |
| Linux x86_64 | `liz-server-linux-amd64` | Headless (estático) |
| Linux ARM64 | `liz-server-linux-arm64` | Headless (estático) |
| macOS Intel | `liz-server-darwin-amd64` | Headless |
| macOS Apple Silicon | `liz-server-darwin-arm64` | Headless |

Instalación manual:

```bash
# Descargar (reemplazar VERSION por la deseada, ej: v0.10.0)
VERSION=v0.10.0
curl -fsSL -o liz https://github.com/caos1codex-hash/liz-ai-agent/releases/download/$VERSION/liz-linux-amd64

# Hacer ejecutable
chmod +x liz

# Instalar en /usr/local/bin
sudo install -m 0755 liz /usr/local/bin/liz

# Crear directorio de configuración
mkdir -p ~/.liz
curl -fsSL -o ~/.liz/config.yaml \
  https://raw.githubusercontent.com/caos1codex-hash/liz-ai-agent/$VERSION/configs/liz.yaml.example

# Verificar
liz --version
```

---

## Paquetes nativos

### DEB (Debian/Ubuntu)

```bash
# Descargar el .deb
curl -fsSL -o liz.deb \
  https://github.com/caos1codex-hash/liz-ai-agent/releases/download/v0.10.0/liz_0.10.0_amd64.deb

# Instalar (resuelve dependencias automáticamente)
sudo apt install ./liz.deb
# o en distros antiguas: sudo dpkg -i liz.deb && sudo apt-get install -f

# Verificar
liz --version
```

El paquete DEB:
- Instala el binario en `/usr/local/bin/liz`
- Crea entrada de menú en `/usr/share/applications/liz.desktop`
- Copia config de ejemplo a `/etc/liz/config.yaml.example`
- Documentación en `/usr/share/doc/liz/`
- Script `postinst` que guía al usuario
- Script `prerm` que detiene procesos en ejecución

### RPM (Fedora/RHEL)

```bash
# Descargar el .rpm
curl -fsSL -o liz.rpm \
  https://github.com/caos1codex-hash/liz-ai-agent/releases/download/v0.10.0/liz-0.10.0-1.x86_64.rpm

# Instalar
sudo dnf install ./liz.rpm
# o en RHEL/CentOS 7: sudo yum localinstall liz.rpm

# Verificar
liz --version
```

### AUR (Arch Linux)

Si Liz está publicado en AUR:

```bash
# Instalar desde AUR (usando yay)
yay -S liz-ai-agent

# O compilar manualmente desde el PKGBUILD
git clone https://github.com/caos1codex-hash/liz-ai-agent.git
cd liz-ai-agent/packaging/
makepkg -si
```

Si no está en AUR todavía, usa el PKGBUILD incluido en el repo:

```bash
git clone https://github.com/caos1codex-hash/liz-ai-agent.git
cd liz-ai-agent/packaging/
makepkg -si
```

---

## Docker

Imagen oficial multi-arch en [ghcr.io](https://github.com/caos1codex-hash/liz-ai-agent/pkgs/container/liz-ai-agent):

```bash
# Pull de la imagen (multi-arch: detecta automáticamente amd64 o arm64)
docker pull ghcr.io/caos1codex-hash/liz-ai-agent:latest

# Ejecutar en modo detached con volumen persistente
docker run -d --name liz \
  -p 3000:3000 \
  -v liz-data:/home/liz/.liz \
  -e NVIDIA_API_KEY=$NVIDIA_API_KEY \
  -e TZ=America/Asuncion \
  --restart unless-stopped \
  ghcr.io/caos1codex-hash/liz-ai-agent:latest

# Verificar
curl http://localhost:3000/api/v1/salud

# Logs
docker logs -f liz

# Detener
docker stop liz && docker rm liz
```

### Docker Compose

```bash
# Descargar docker-compose.yml
curl -fsSL https://raw.githubusercontent.com/caos1codex-hash/liz-ai-agent/main/docker/docker-compose.yml \
  -o docker-compose.yml

# Configurar API key
export NVIDIA_API_KEY=tu_api_key

# Levantar
docker compose up -d

# Ver logs
docker compose logs -f liz

# Detener
docker compose down
```

### Imagen desde el código fuente

```bash
git clone https://github.com/caos1codex-hash/liz-ai-agent.git
cd liz-ai-agent
docker build -f docker/Dockerfile -t liz-local .
docker run --rm -it -p 3000:3000 liz-local
```

---

## Compilación desde el código fuente

### Requisitos

- **Go 1.22+** — [https://go.dev/dl/](https://go.dev/dl/)
- **Git**
- **CGO + OpenGL dev headers** (solo para binario desktop con GUI)

### Build rápido

```bash
git clone https://github.com/caos1codex-hash/liz-ai-agent.git
cd liz-ai-agent

# Binario desktop con GUI Fyne (requiere deps OpenGL)
make build

# Binario headless (sin deps OpenGL, estático, portable)
make build-headless

# Instalar en ~/go/bin
make install

# O instalar en /usr/local/bin (requiere sudo)
make install-system
```

### Build multi-plataforma

```bash
# Cross-compile para todas las plataformas (5 binarios + tarballs + checksums)
make cross-compile

# Crear paquetes DEB + RPM + tarball
make package
```

### Sin Make (usando go directamente)

```bash
# Headless estático
CGO_ENABLED=0 go build -tags headless -ldflags="-s -w" -o liz-server ./cmd/liz

# Desktop con GUI (requiere deps OpenGL instaladas)
CGO_ENABLED=1 go build -o liz ./cmd/liz
```

### Docker como entorno de build (recomendado para macOS/Windows)

Si desarrollas en macOS o Windows y quieres compilar el binario desktop para
Linux sin instalar todas las dependencias nativas:

```bash
# Construir imagen de desarrollo
docker build -f docker/Dockerfile.dev -t liz-dev .

# Compilar dentro del contenedor con el código fuente montado
docker run --rm -it -v $(pwd):/src -v ~/go:/go liz-dev bash -c \
  "cd /src && make build"
```

---

## Configuración inicial

Tras la instalación, edita `~/.liz/config.yaml` con tu API key de NVIDIA:

```yaml
nombre: Liz
version: "0.1.0"

servidor:
  host: "127.0.0.1"   # 0.0.0.0 para acceso externo / Docker
  puerto: 3000

ia:
  provider: "nvidia"
  api_key: "nvapi-tu_api_key_aqui"   # https://build.nvidia.com/
  modelo_default: "meta/llama-3.1-70b-instruct"

permisos:
  conceder_todos_al_iniciar: true   # D-006: Permisos Una Vez
```

Obtén tu API key gratuita en [https://build.nvidia.com/](https://build.nvidia.com/)
(cuenta con créditos gratuitos iniciales).

---

## Verificación de la instalación

```bash
# Versión instalada
liz --version
# Salida esperada: liz version 0.10.0

# Verificar checksums (si descargaste binarios manualmente)
sha256sum -c checksums-v0.10.0.txt

# Arrancar Liz en modo desktop (con GUI)
liz

# Arrancar en modo servidor headless
liz --headless
# o si instalaste el binario headless:
liz-server --headless

# Verificar que el servidor responde
curl http://localhost:3000/api/v1/salud

# Listar herramientas disponibles
curl http://localhost:3000/api/v1/herramientas
```

---

## Desinstalación

```bash
# Usar el script oficial (recomendado)
./scripts/uninstall.sh

# Desinstalación completa (elimina también ~/.liz/ con config, memoria, herramientas auto-creadas)
./scripts/uninstall.sh --purge

# Desinstalación manual
sudo rm -f /usr/local/bin/liz /usr/local/bin/liz-server
rm -f ~/.local/share/applications/liz.desktop
rm -rf ~/.liz   # opcional, elimina todos los datos
```

Si instalaste vía DEB:

```bash
sudo apt remove liz
sudo apt purge liz   # elimina también configs del sistema
```

Si instalaste vía RPM:

```bash
sudo dnf remove liz
```

Si instalaste vía Docker:

```bash
docker stop liz && docker rm liz
docker volume rm liz-data   # opcional, elimina datos persistentes
docker rmi ghcr.io/caos1codex-hash/liz-ai-agent:latest
```

---

## Solución de problemas

### `liz: error while loading shared libraries: libGL.so.1`

Faltan las dependencias de runtime OpenGL. Instálalas:

```bash
# Debian/Ubuntu
sudo apt install libgl1-mesa-glx libegl1 libglx0

# Fedora
sudo dnf install mesa-libGL libEGL libglvnd-glx

# Arch
sudo pacman -S mesa
```

O usa el binario headless: `liz-server --headless`

### `liz: cannot execute binary file: Exec format error`

Descargaste un binario para una arquitectura incorrecta. Verifica tu
arquitectura:

```bash
uname -m
# x86_64  → usar liz-linux-amd64
# aarch64 → usar liz-server-linux-arm64
```

### `connection refused` al hacer curl a localhost:3000

El servidor no está corriendo o usa otro puerto. Verifica:

```bash
# ¿Está corriendo el proceso?
pgrep -af liz

# ¿Qué puerto está escuchando?
ss -tlnp | grep liz

# ¿Está el puerto en la config?
cat ~/.liz/config.yaml | grep puerto
```

### `API NVIDIA no inicializada` en los logs

Falta la API key o es inválida. Edita `~/.liz/config.yaml`:

```yaml
ia:
  api_key: "nvapi-tu_api_key_aqui"
```

O pásala como variable de entorno (modo Docker):

```bash
docker run -e NVIDIA_API_KEY=nvapi-... ghcr.io/caos1codex-hash/liz-ai-agent:latest
```

### Docker: `permission denied` al hacer docker run

Agrega tu usuario al grupo docker:

```bash
sudo usermod -aG docker $USER
newgrp docker
```

### Build desktop falla con `Package gl was not found`

Faltan las dependencias de desarrollo OpenGL. Instálalas (ver [Requisitos previos](#requisitos-previos))
o compila solo el binario headless: `make build-headless`.

---

> **Filosofía**: "Si no está en GitHub, no existe."
> Si encuentras un bug, repórtalo en [Issues](https://github.com/caos1codex-hash/liz-ai-agent/issues).
