# Liz AI Agent — Makefile
# Targets: backend Go + app de escritorio nativa (Fyne) + release v0.1.0
#
# Fase 10 — Release v0.1.0
# Filosofía: "si no está en GitHub, no existe."

.PHONY: build build-headless run dev test test-short vet fmt lint clean clean-all \
        install uninstall tidy cross-compile package-deb package-rpm package-tarball \
        package docker docker-build docker-run docker-push release release-notes \
        dist help headless desktop all version

# ══════════════════════════════════════════════════
# Variables
# ══════════════════════════════════════════════════

BINARY_NAME=liz
BINARY_HEADLESS=liz-server
GO=$(shell which go 2>/dev/null || echo "$(HOME)/go-local/go/bin/go")
GOFLAGS=-v
MAIN_PATH=./cmd/liz
BUILD_DIR=./bin
DIST_DIR=./dist

# Versión (extraída de cmd/liz/main.go)
VERSION=$(shell grep -m1 '^const version' $(MAIN_PATH)/main.go | sed -E 's/.*"([^"]+)".*/\1/')
TAG=v$(VERSION)

# LDFLAGS comunes
LDFLAGS=-s -w -X main.version=$(VERSION)

# Docker
DOCKER_IMAGE=ghcr.io/caos1codex-hash/liz-ai-agent
DOCKER_TAG=$(DOCKER_IMAGE):$(VERSION)
DOCKER_LATEST=$(DOCKER_IMAGE):latest

# ══════════════════════════════════════════════════
# Targets principales
# ══════════════════════════════════════════════════

## version: Muestra la versión actual
version:
        @echo "Liz $(VERSION) (tag: $(TAG))"

## build: Compila el binario desktop de Liz con GUI Fyne (requiere deps OpenGL)
build:
        @echo "Compilando Liz desktop (GUI Fyne) v$(VERSION)..."
        @mkdir -p $(BUILD_DIR)
        CGO_ENABLED=1 $(GO) build $(GOFLAGS) -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
        @echo "✓ Binario creado: $(BUILD_DIR)/$(BINARY_NAME)"
        @$(BUILD_DIR)/$(BINARY_NAME) --version 2>/dev/null || true

## build-headless: Compila binario servidor headless (sin GUI, estático, portable)
build-headless:
        @echo "Compilando Liz headless (estático, cross-compilable) v$(VERSION)..."
        @mkdir -p $(BUILD_DIR)
        CGO_ENABLED=0 $(GO) build $(GOFLAGS) -tags headless -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_HEADLESS) $(MAIN_PATH)
        @echo "✓ Binario creado: $(BUILD_DIR)/$(BINARY_HEADLESS)"
        @$(BUILD_DIR)/$(BINARY_HEADLESS) --version 2>/dev/null || true

## run: Compila y ejecuta Liz con GUI nativa
run: build
        @echo "Iniciando Liz (modo desktop nativo)..."
        $(BUILD_DIR)/$(BINARY_NAME)

## dev: Ejecuta con go run (sin compilar binario) — modo desktop nativo
dev:
        $(GO) run $(MAIN_PATH)

## headless: Ejecuta Liz en modo servidor (sin GUI, para servidores/Docker)
headless: build-headless
        @echo "Iniciando Liz (modo headless, sin GUI)..."
        $(BUILD_DIR)/$(BINARY_HEADLESS) --headless

## desktop: Igual que run — alias explícito para modo desktop
desktop: run

# ══════════════════════════════════════════════════
# Testing y calidad
# ══════════════════════════════════════════════════

## test: Ejecuta todos los tests (con tag headless para evitar deps OpenGL)
test:
        CGO_ENABLED=0 $(GO) test -tags headless -v -cover ./...

## test-short: Tests sin los que requieren red/compilación lenta
test-short:
        CGO_ENABLED=0 $(GO) test -tags headless -short ./...

## test-stable: Solo tests de paquetes estables (no requieren OpenGL ni mocks rotos)
test-stable:
        CGO_ENABLED=0 $(GO) test -tags headless -short -count=1 \
                ./internal/nucleo/config/... \
                ./internal/nucleo/permisos/... \
                ./internal/nucleo/memoria/... \
                ./internal/nucleo/orquestador/... \
                ./internal/nucleo/herramientas/ \
                ./internal/nucleo/herramientas/registro/... \
                ./internal/nucleo/herramientas/integradas/...

## vet: Ejecuta go vet
vet:
        CGO_ENABLED=0 $(GO) vet -tags headless ./...

## fmt: Formatea el código con gofmt
fmt:
        $(GO) fmt ./...
        @echo "✓ Código formateado."

## lint: Ejecuta vet + fmt check
lint: vet
        @unformatted=$$(gofmt -l .); if [ -n "$$unformatted" ]; then \
                echo "✗ Archivos no formateados:"; echo "$$unformatted"; exit 1; \
        fi
        @echo "✓ Lint passed."

# ══════════════════════════════════════════════════
# Cross-compilation y packaging (Fase 10)
# ══════════════════════════════════════════════════

## cross-compile: Compila binarios para todas las plataformas (desktop + headless x5)
cross-compile:
        @echo "Cross-compiling Liz $(TAG) para todas las plataformas..."
        @chmod +x scripts/build-release.sh
        ./scripts/build-release.sh $(TAG)

## package-deb: Crea paquete .deb para Debian/Ubuntu
package-deb: build
        @echo "Creando paquete DEB..."
        @chmod +x scripts/package-deb.sh
        ./scripts/package-deb.sh $(TAG)

## package-rpm: Crea paquete .rpm para Fedora/RHEL
package-rpm: build
        @echo "Creando paquete RPM..."
        @chmod +x scripts/package-rpm.sh
        ./scripts/package-rpm.sh $(TAG)

## package-tarball: Crea tarball genérico portable
package-tarball: cross-compile
        @echo "Creando tarball genérico..."
        @chmod +x scripts/package-tarball.sh
        ./scripts/package-tarball.sh $(TAG)

## package: Crea todos los paquetes (DEB + RPM + tarball + checksums)
package: package-deb package-rpm package-tarball
        @echo "✓ Todos los paquetes creados en $(DIST_DIR)/"
        @ls -lh $(DIST_DIR)/*.deb $(DIST_DIR)/*.rpm $(DIST_DIR)/*.tar.gz 2>/dev/null || true

## dist: Alias de cross-compile + package
dist: package
        @echo "✓ Distribuibles completos en $(DIST_DIR)/"

# ══════════════════════════════════════════════════
# Docker (Fase 10)
# ══════════════════════════════════════════════════

## docker-build: Construye imagen Docker multi-arch (headless)
docker-build:
        @echo "Construyendo imagen Docker $(DOCKER_TAG)..."
        docker build \
                --build-arg VERSION=$(VERSION) \
                -t $(DOCKER_TAG) \
                -t $(DOCKER_LATEST) \
                -f docker/Dockerfile \
                .
        @echo "✓ Imagen creada: $(DOCKER_TAG)"

## docker-build-multiarch: Construye imagen multi-arch (linux/amd64 + linux/arm64) vía buildx
docker-build-multiarch:
        @echo "Construyendo imagen Docker multi-arch..."
        docker buildx build \
                --platform linux/amd64,linux/arm64 \
                --build-arg VERSION=$(VERSION) \
                -t $(DOCKER_TAG) \
                -t $(DOCKER_LATEST) \
                -f docker/Dockerfile \
                --push \
                .
        @echo "✓ Imagen multi-arch publicada: $(DOCKER_TAG)"

## docker-run: Ejecuta Liz en contenedor Docker (modo headless)
docker-run:
        @echo "Iniciando Liz en Docker (modo headless)..."
        @mkdir -p $(HOME)/.liz-docker
        docker run --rm -it \
                -p 3000:3000 \
                -v $(HOME)/.liz-docker:/home/liz/.liz \
                -e NVIDIA_API_KEY=$(NVIDIA_API_KEY) \
                $(DOCKER_LATEST)

## docker-compose-up: Levanta Liz con docker-compose
docker-compose-up:
        docker compose -f docker/docker-compose.yml up -d
        @echo "✓ Liz corriendo en http://localhost:3000"
        @echo "  Logs: docker compose -f docker/docker-compose.yml logs -f"

## docker-compose-down: Detiene Liz
docker-compose-down:
        docker compose -f docker/docker-compose.yml down

## docker-push: Publica imagen en ghcr.io (requiere login previo)
docker-push: docker-build
        @echo "Publicando imagen en $(DOCKER_IMAGE)..."
        docker push $(DOCKER_TAG)
        docker push $(DOCKER_LATEST)
        @echo "✓ Imagen publicada"

## docker: Alias de docker-build
docker: docker-build

# ══════════════════════════════════════════════════
# Release (Fase 10)
# ══════════════════════════════════════════════════

## release-notes: Genera notas de release en stdout
release-notes:
        @echo "# Liz $(TAG) — Fase 10"
        @echo ""
        @echo "## Novedades"
        @echo ""
        @echo "- Primer release oficial de Liz AI Agent"
        @echo "- Binarios para 5 plataformas (linux amd64/arm64, darwin amd64/arm64, windows amd64)"
        @echo "- Paquetes nativos: DEB, RPM, AUR PKGBUILD, tarball"
        @echo "- Imagen Docker multi-arch en ghcr.io"
        @echo "- Instalador automático multi-distro"
        @echo ""
        @echo "## Instalación"
        @echo ""
        @echo "\`\`\`bash"
        @echo "curl -fsSL https://github.com/caos1codex-hash/liz-ai-agent/releases/latest/download/install.sh | bash"
        @echo "\`\`\`"
        @echo ""
        @echo "## Docker"
        @echo ""
        @echo "\`\`\`bash"
        @echo "docker run -d -p 3000:3000 -v liz-data:/home/liz/.liz \\"
        @echo "  -e NVIDIA_API_KEY=\$$NVIDIA_API_KEY \\"
        @echo "  ghcr.io/caos1codex-hash/liz-ai-agent:latest"
        @echo "\`\`\`"

## release-tag: Crea el tag de release y lo sube a GitHub (triggera workflow)
release-tag:
        @echo "Creando tag $(TAG)..."
        git tag -a $(TAG) -m "Liz $(TAG) — Fase 10: Release v0.1.0"
        git push origin $(TAG)
        @echo "✓ Tag $(TAG) creado y pusheado"
        @echo "  El workflow de release se ejecutará automáticamente en GitHub Actions"
        @echo "  Ver: https://github.com/caos1codex-hash/liz-ai-agent/actions"

## release: Prepara y publica el release completo (dist + tag)
release: dist release-tag
        @echo ""
        @echo "✓ Release $(TAG) preparado y tag pusheado"
        @echo "  GitHub Actions construirá y publicará todo automáticamente."
        @echo ""
        @echo "Assets locales en $(DIST_DIR)/:"
        @ls -lh $(DIST_DIR)/ 2>/dev/null || true

# ══════════════════════════════════════════════════
# Limpieza
# ══════════════════════════════════════════════════

## clean: Elimina binarios y archivos temporales
clean:
        rm -rf $(BUILD_DIR) $(DIST_DIR)
        $(GO) clean
        @echo "✓ Limpieza completada."

## clean-all: Limpieza profunda (incluye caché de builds Fyne y Go)
clean-all: clean
        rm -rf $(HOME)/.cache/fyne 2>/dev/null || true
        $(GO) clean -cache -testcache -modcache 2>/dev/null || true
        @echo "✓ Limpieza profunda completada."

# ══════════════════════════════════════════════════
# Utilidades
# ══════════════════════════════════════════════════

## tidy: Descarga y ordena dependencias Go
tidy:
        $(GO) mod tidy

## install: Instala Liz globalmente (~/go/bin/liz)
install: build
        @mkdir -p $(GOPATH)/bin
        cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)
        @echo "✓ Liz instalada en $(GOPATH)/bin/$(BINARY_NAME)"

## install-system: Instala en /usr/local/bin (requiere sudo)
install-system: build
        @sudo install -m 0755 $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
        @echo "✓ Liz instalada en /usr/local/bin/$(BINARY_NAME)"

## uninstall: Desinstala Liz del sistema (usa script oficial)
uninstall:
        @chmod +x scripts/uninstall.sh
        ./scripts/uninstall.sh --purge

## all: Igual que build (ya no hay frontend separado)
all: build
        @echo "✓ Build completo: $(BUILD_DIR)/$(BINARY_NAME) (backend + GUI nativa)"

## help: Muestra esta ayuda
help:
        @echo "Liz AI Agent v$(VERSION) — Fase 10 (Release v0.1.0)"
        @echo ""
        @echo "Targets disponibles:"
        @sed -n 's/^##//p' $(MAKEFILE_LIST) | sed -e 's/^/  /'
        @echo ""
        @echo "Notas:"
        @echo "  - Por defecto Liz arranca con GUI nativa (Fyne + OpenGL)."
        @echo "  - Usa 'make headless' para modo servidor puro (sin GUI, estático)."
        @echo "  - Usa 'make cross-compile' para binarios de todas las plataformas."
        @echo "  - Usa 'make release' para preparar y publicar un release completo."
        @echo "  - Dependencias Linux desktop: libGL, libX11, libXrandr, libXcursor,"
        @echo "    libXi, libXinerama, libXxf86vm, libwayland, libxkbcommon, libEGL."
        @echo ""
        @echo "Filosofía: 'si no está en GitHub, no existe.'"
