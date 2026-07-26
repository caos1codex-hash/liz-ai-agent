//go:build headless

// Archivo: desktop_headless.go
// Build tag: headless — binario servidor puro SIN dependencias Fyne/OpenGL.
//
// Este archivo se compila SOLO cuando se pasa -tags headless al go build.
// No importa el paquete internal/desktop, lo que permite:
//
//   - CGO_ENABLED=0 → binario 100% estático
//   - Cross-compilación a cualquier GOOS/GOARCH sin toolchain C
//   - Imagen Docker mínima (sin libGL runtime)
//   - Builds reproducibles en CI sin instalar dev headers
//
// Uso típico (Fase 10 — release v0.1.0):
//
//	CGO_ENABLED=0 go build -tags headless -ldflags="-s -w" -o liz-server ./cmd/liz
//	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags headless -o liz-server-arm64 ./cmd/liz
//	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -tags headless -o liz-server-macos-arm64 ./cmd/liz
//
// El binario resultante arranca el servidor HTTP en primer plano y bloquea
// hasta recibir SIGINT/SIGTERM. Pensado para Docker, systemd, servidores.

package main

import (
	"os"

	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/config"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/logger"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/servidor"
)

// lanzarModoVisual en modo headless simplemente arranca el servidor HTTP
// en primer plano y bloquea hasta que el proceso recibe SIGINT/SIGTERM.
// No abre ninguna ventana; pensado para servidores y contenedores Docker.
func lanzarModoVisual(srv *servidor.Servidor, gestorCfg *config.Gestor, log *logger.Logger) {
	log.Info("Binario compilado con -tags headless: modo servidor puro (sin GUI)")
	log.Info("Escuchando en %s:%d", gestorCfg.ObtenerHost(), gestorCfg.ObtenerPuerto())
	if err := srv.Iniciar(); err != nil {
		log.Error("Error al iniciar servidor: %v", err)
		os.Exit(1)
	}
}
