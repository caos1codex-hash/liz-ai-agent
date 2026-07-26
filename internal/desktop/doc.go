// Package desktop implementa la interfaz gráfica nativa de Liz.
//
// Sustituye al frontend web (React + Vite) por una aplicación de escritorio
// 100% nativa construida con Fyne v2. No usa navegador, no usa WebView,
// no usa HTML/CSS/JS — solo widgets nativos pintados vía OpenGL/GLFW.
//
// Arquitectura:
//
//	cmd/liz/main.go  ──┐
//	                   ├─→ servidor HTTP (puerto 3000, Fases 1-7)
//	                   └─→ desktop.App (Fyne, goroutine)
//	                          │
//	                          ├─→ sidebar   (CRUD sesiones, selección)
//	                          ├─→ header    (status, modelo, métricas, theme)
//	                          ├─→ chat      (lista mensajes + input + SSE)
//	                          └─→ toasts    (notificaciones efímeras)
//
// La UI se comunica con el backend por HTTP + SSE sobre localhost:3000,
// usando el mismo contrato de endpoints que el frontend web original.
//
// Filosofía: "nativo nativo". Un solo binario `liz` arranca el servidor
// y abre la ventana. Sin dependencias de navegador, sin Chromium embebido,
// sin Tauri/Wails. Puro Go + OpenGL.
package desktop
