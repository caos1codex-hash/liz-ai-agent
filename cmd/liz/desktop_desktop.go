//go:build !headless

// Archivo: desktop_desktop.go
// Build tag: !headless (default — binario con GUI Fyne completa)
//
// Este archivo SÓLO se compila cuando NO se usa -tags headless.
// Importa el paquete internal/desktop que arrastra dependencias de
// Fyne v2 + OpenGL + GLFW + Wayland (CGO obligatorio).
//
// Para compilar el binario desktop completo:
//
//	go build -o liz ./cmd/liz
//
// Requiere libGL, libX11, libXrandr, libXcursor, libXi, libXinerama,
// libXxf86vm, libwayland, libxkbcommon, libEGL (dev headers).

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/caos1codex-hash/liz-ai-agent/internal/desktop"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/config"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/logger"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/servidor"
)

// lanzarModoVisual arranca el servidor HTTP en background y abre la GUI
// nativa Fyne. Bloquea hasta que el usuario cierra la ventana.
//
// Si el backend no responde tras 10s, la GUI se abre igualmente y mostrará
// un estado de "reconectando" hasta que el servidor esté disponible.
func lanzarModoVisual(srv *servidor.Servidor, gestorCfg *config.Gestor, log *logger.Logger) {
	// Iniciar servidor HTTP en background (no bloquea)
	errChan := make(chan error, 1)
	go func() {
		if err := srv.Iniciar(); err != nil {
			errChan <- err
		}
	}()

	// Esperar a que el backend esté listo (timeout 10s)
	baseURL := fmt.Sprintf("http://%s:%d", gestorCfg.ObtenerHost(), gestorCfg.ObtenerPuerto())
	log.Info("Esperando backend listo en %s…", baseURL)
	if err := desktop.EsperarBackend(baseURL, 10*time.Second); err != nil {
		log.Warn("Backend no responde tras 10s: %v (la GUI intentará conectar igualmente)", err)
	} else {
		log.Info("Backend listo. Arrancando GUI nativa (Fase 8 desktop)…")
	}

	// Lanzar GUI nativa (bloquea hasta que se cierre la ventana)
	desktopApp := desktop.NuevaApp(desktop.AppOpciones{
		BaseURL: baseURL,
	})
	go func() {
		if err := <-errChan; err != nil {
			log.Error("Servidor HTTP terminó con error: %v", err)
		}
	}()
	desktopApp.Ejecutar()

	// Al cerrar la GUI, el proceso termina.
	log.Info("GUI cerrada. Cerrando Liz…")
	_ = os.Exit
}
