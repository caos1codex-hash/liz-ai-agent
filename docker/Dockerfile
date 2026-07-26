# ============================================================================
# Liz AI Agent — Dockerfile multi-stage multi-arch
# ============================================================================
# Fase 10 — Release v0.1.0
#
# Imagen Docker oficial de Liz en modo servidor (headless).
# Binario estático CGO_ENABLED=0, sin dependencias OpenGL.
#
# Multi-stage:
#   1. builder:    Go 1.22 alpine, compila binario headless
#   2. runtime:    distroless/static:nonroot (imagen mínima sin shell)
#
# Multi-arch soportada:
#   linux/amd64, linux/arm64 (vía docker buildx)
#
# Build:
#   docker build -t liz-ai-agent:latest .
#   docker build --platform linux/arm64 -t liz-ai-agent:arm64 .
#
# Run:
#   docker run -d --name liz \\
#     -p 3000:3000 \\
#     -v liz-config:/home/liz/.liz \\
#     -e NVIDIA_API_KEY=$NVIDIA_API_KEY \\
#     liz-ai-agent:latest
#
# Imagen final: ~10MB (distroless) o ~25MB (alpine)
# ============================================================================

# --- Stage 1: Builder ---
FROM golang:1.22-alpine AS builder

# Metadata del builder
LABEL stage="liz-builder"

# Dependencias de build: git (para go mod), ca-certificates (para HTTPS)
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

# Copiar primero go.mod/go.sum para cachear dependencias
COPY go.mod go.sum ./
RUN go mod download

# Copiar el resto del código
COPY . .

# Argumentos de build (inyectados por CI)
ARG VERSION=dev
ARG TARGETOS=linux
ARG TARGETARCH=amd64

# Build estático headless (sin CGO, sin OpenGL)
# -tags headless     → exclude paquete desktop/Fyne
# CGO_ENABLED=0      → binario 100% estático
# -trimpath          → rutas absolutas removidas (reproducible)
# -ldflags -s -w     → strip symbols + DWARF
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build \
        -tags headless \
        -trimpath \
        -ldflags="-s -w -X main.version=${VERSION} -extldflags=-static" \
        -o /out/liz-server \
        ./cmd/liz

# Verificar que el binario estático funciona
RUN /out/liz-server --version

# --- Stage 2: Runtime ---
# Distroless static: imagen mínima sin shell, sin package manager, sin libs.
# Apropiada para binarios estáticos Go. Tamaño: ~2MB base + binario.
FROM gcr.io/distroless/static-debian12:nonroot

# Metadata OCI
LABEL org.opencontainers.image.title="Liz AI Agent"
LABEL org.opencontainers.image.description="Agente de IA autónomo para Linux — servidor headless"
LABEL org.opencontainers.image.source="https://github.com/caos1codex-hash/liz-ai-agent"
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.version="${VERSION:-dev}"
LABEL org.opencontainers.image.authors="caos1codex-hash"

# Usuario nonroot (UID 65532) ya viene con la imagen distroless:nonroot
WORKDIR /home/liz

# Copiar binario desde builder
COPY --from=builder /out/liz-server /usr/local/bin/liz-server

# Volumen para configuración y datos persistentes
VOLUME ["/home/liz/.liz"]

# Puerto del servidor HTTP (default config: 8080; override via LIZ_PUERTO o config.yaml)
EXPOSE 8080

# Healthcheck HTTP real (issue #26): el binario hace GET /api/v1/health via Go
# stdlib. No requiere curl/wget (que no existen en distroless).
# Antes usabamos --version que solo verificaba que el binario arranque, no
# que el servidor HTTP respondiera.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/liz-server", "--health"]

# Punto de entrada: arranca el servidor headless
ENTRYPOINT ["/usr/local/bin/liz-server"]
CMD ["--headless"]
