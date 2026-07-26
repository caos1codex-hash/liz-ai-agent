# Liz AI Agent — Makefile
# Targets: backend Go + frontend React

.PHONY: build run dev test clean install fmt vet lint help \
        web-install web-dev web-build web-clean web-preview all

# Variables Go
BINARY_NAME=liz
GO=$(shell which go 2>/dev/null || echo "$(HOME)/go-local/go/bin/go")
GOFLAGS=-v
MAIN_PATH=./cmd/liz
BUILD_DIR=./bin

# Variables frontend
WEB_DIR=./web
NPM=$(shell which npm 2>/dev/null)

# Go paths
GOMOD=$(shell head -1 go.mod | awk '{print $$2}')

# ══════════════════════════════════════════════════
# Targets principales
# ══════════════════════════════════════════════════

## build: Compila el binario de Liz
build:
	@echo "Compilando Liz..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Binario creado: $(BUILD_DIR)/$(BINARY_NAME)"
	@$(BUILD_DIR)/$(BINARY_NAME) --version 2>/dev/null || true

## run: Compila y ejecuta Liz
run: build
	@echo "Iniciando Liz..."
	$(BUILD_DIR)/$(BINARY_NAME)

## dev: Ejecuta con go run (sin compilar binario)
dev:
	$(GO) run $(MAIN_PATH)

# ══════════════════════════════════════════════════
# Frontend (web/)
# ══════════════════════════════════════════════════

## web-install: Instala dependencias del frontend (npm install)
web-install:
	@echo "Instalando dependencias del frontend..."
	cd $(WEB_DIR) && $(NPM) install

## web-dev: Levanta Vite dev server (:5173) con proxy a :3000
web-dev:
	@echo "Levantando frontend (Vite dev server)..."
	@echo "  Asegúrate de que el backend esté corriendo en :3000 (make dev en otra terminal)"
	cd $(WEB_DIR) && $(NPM) run dev

## web-build: Build producción del frontend → web/dist/
web-build:
	@echo "Build del frontend..."
	cd $(WEB_DIR) && $(NPM) run build
	@echo "Build completado: $(WEB_DIR)/dist/"

## web-preview: Sirve el build de producción localmente
web-preview:
	cd $(WEB_DIR) && $(NPM) run preview

## web-clean: Limpia node_modules, dist y caches de Vite
web-clean:
	rm -rf $(WEB_DIR)/node_modules $(WEB_DIR)/dist $(WEB_DIR)/.vite
	@echo "Frontend limpiado."

## web-typecheck: Solo typecheck (sin emitir archivos)
web-typecheck:
	cd $(WEB_DIR) && $(NPM) run typecheck

# ══════════════════════════════════════════════════
# Testing y calidad
# ══════════════════════════════════════════════════

## test: Ejecuta todos los tests
test:
	$(GO) test ./... -v -cover

## vet: Ejecuta go vet
vet:
	$(GO) vet ./...

## fmt: Formatea el código con gofmt
fmt:
	$(GO) fmt ./...
	@echo "Código formateado."

## lint: Ejecuta vet + fmt check
lint: vet fmt
	@echo "Lint passed."

# ══════════════════════════════════════════════════
# Limpieza
# ══════════════════════════════════════════════════

## clean: Elimina binarios y archivos temporales (backend)
clean:
	rm -rf $(BUILD_DIR)
	$(GO) clean
	@echo "Limpieza completada."

## clean-all: Limpia backend + frontend
clean-all: clean web-clean
	@echo "Limpieza completa (backend + frontend)."

# ══════════════════════════════════════════════════
# Utilidades
# ══════════════════════════════════════════════════

## tidy: Descarga y ordena dependencias Go
tidy:
	$(GO) mod tidy

## install: Instala Liz globalmente (~/go/bin/liz)
install: build
	cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)
	@echo "Liz instalada en $(GOPATH)/bin/$(BINARY_NAME)"

## all: Build backend + frontend
all: build web-build
	@echo "Build completo: backend ($(BUILD_DIR)/$(BINARY_NAME)) + frontend ($(WEB_DIR)/dist/)"

## help: Muestra esta ayuda
help:
	@echo "Liz AI Agent — Targets disponibles:"
	@echo ""
	@echo "Backend (Go):"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | grep -v "web-" | column -t -s ':' | sed -e 's/^/ /'
	@echo ""
	@echo "Frontend (web/):"
	@sed -n 's/^##web-/  web-/p' $(MAKEFILE_LIST) | sed 's/^##//' | column -t -s ':' | sed -e 's/^/ /'
