# Liz AI Agent — Makefile
# Target principal: build

.PHONY: build run test clean install fmt vet lint help

# Variables
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

## clean: Elimina binarios y archivos temporales
clean:
	rm -rf $(BUILD_DIR)
	$(GO) clean
	@echo "Limpieza completada."

# ══════════════════════════════════════════════════
# Utilidades
# ══════════════════════════════════════════════════

## tidy: Descarga y ordena dependencias
tidy:
	$(GO) mod tidy

## install: Instala Liz globalmente (~/go/bin/liz)
install: build
	cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)
	@echo "Liz instalada en $(GOPATH)/bin/$(BINARY_NAME)"

## help: Muestra esta ayuda
help:
	@echo "Liz AI Agent — Targets disponibles:"
	@echo ""
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
