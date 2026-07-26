# Liz AI Agent — Makefile
# Targets: backend Go + app de escritorio nativa (Fyne)

.PHONY: build run dev test clean install fmt vet lint help headless desktop all

# Variables Go
BINARY_NAME=liz
GO=$(shell which go 2>/dev/null || echo "$(HOME)/go-local/go/bin/go")
GOFLAGS=-v
MAIN_PATH=./cmd/liz
BUILD_DIR=./bin

# Go paths
GOMOD=$(shell head -1 go.mod | awk '{print $$2}')

# ══════════════════════════════════════════════════
# Targets principales
# ══════════════════════════════════════════════════

## build: Compila el binario de Liz (backend + GUI Fyne)
build:
	    @echo "Compilando Liz..."
	    @mkdir -p $(BUILD_DIR)
	    $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	    @echo "Binario creado: $(BUILD_DIR)/$(BINARY_NAME)"
	    @$(BUILD_DIR)/$(BINARY_NAME) --version 2>/dev/null || true

## run: Compila y ejecuta Liz con GUI nativa
run: build
	    @echo "Iniciando Liz (modo desktop nativo)..."
	    $(BUILD_DIR)/$(BINARY_NAME)

## dev: Ejecuta con go run (sin compilar binario) — modo desktop nativo
dev:
	    $(GO) run $(MAIN_PATH)

## headless: Ejecuta Liz en modo servidor (sin GUI, para servidores/Docker)
headless: build
	    @echo "Iniciando Liz (modo headless, sin GUI)..."
	    $(BUILD_DIR)/$(BINARY_NAME) --headless

## desktop: Igual que run — alias explícito para modo desktop
desktop: run

# ══════════════════════════════════════════════════
# Testing y calidad
# ══════════════════════════════════════════════════

## test: Ejecuta todos los tests
test:
	    $(GO) test ./... -v -cover

## test-short: Tests sin los que requieren red/compilación lenta
test-short:
	    $(GO) test ./... -short

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

## clean: Elimina binarios y archivos temporales
clean:
	    rm -rf $(BUILD_DIR)
	    $(GO) clean
	    @echo "Limpieza completada."

## clean-all: Limpieza profunda (incluye caché de builds Fyne)
clean-all: clean
	    rm -rf $(HOME)/.cache/fyne 2>/dev/null || true
	    @echo "Limpieza completa."

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
	    @echo "Liz instalada en $(GOPATH)/bin/$(BINARY_NAME)"

## all: Igual que build (ya no hay frontend separado)
all: build
	    @echo "Build completo: $(BUILD_DIR)/$(BINARY_NAME) (backend + GUI nativa)"

## help: Muestra esta ayuda
help:
	    @echo "Liz AI Agent — App de escritorio nativa (Fase 8 desktop)"
	    @echo ""
	    @echo "Targets disponibles:"
	    @sed -n 's/^##//p' $(MAKEFILE_LIST) | sed -e 's/^/  /'
	    @echo ""
	    @echo "Notas:"
	    @echo "  - Por defecto Liz arranca con GUI nativa (Fyne + OpenGL)."
	    @echo "  - Usa 'make headless' para modo servidor puro (sin GUI)."
	    @echo "  - Dependencias Linux: libGL, libX11, libXrandr, libXcursor, libXi,"
	    @echo "    libXinerama, libXxf86vm, libwayland, libxkbcommon, libEGL."
