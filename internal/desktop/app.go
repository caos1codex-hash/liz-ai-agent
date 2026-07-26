package desktop

import (
	"context"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// App es la aplicación de escritorio nativa de Liz.
//
// Orquesta todos los componentes desktop:
//   - Sidebar (CRUD sesiones)
//   - Header (status, modelo, métricas, theme toggle)
//   - ChatWindow (mensajes + SSE streaming)
//   - ProjectSelector (proyecto de contexto)
//   - Toasts (notificaciones efímeras)
//
// Filosofía: "nativo nativo". Un solo binario `liz` arranca el servidor
// HTTP y abre la ventana. Sin navegador, sin WebView, sin Electron/Tauri.
type App struct {
	fyneApp fyne.App
	win     fyne.Window
	cliente *ClienteBackend

	mu     sync.RWMutex
	tema   ModoTema

	sidebar       *Sidebar
	header        *Header
	chat          *ChatWindow
	proyectos     *ProjectSelector
	toasts        *Toast
}

// AppOpciones configura la app desktop.
type AppOpciones struct {
	// BaseURL del backend Go (default: http://localhost:3000)
	BaseURL string
	// ModoTema inicial (default: ModoOscuro)
	Modo ModoTema
}

// NuevaApp construye la app de escritorio Liz.
func NuevaApp(opt AppOpciones) *App {
	if opt.BaseURL == "" {
		opt.BaseURL = "http://localhost:3000"
	}
	if opt.Modo != ModoOscuro && opt.Modo != ModoClaro {
		opt.Modo = ModoOscuro
	}

	a := &App{
		fyneApp: app.NewWithID("com.liz.ai-agent"),
		cliente: NuevoCliente(OpcionesCliente{BaseURL: opt.BaseURL}),
		tema:    opt.Modo,
	}

	// Configurar tema inicial
	a.fyneApp.Settings().SetTheme(NuevoTema(a.tema))

	// Crear componentes
	a.toasts = NewToast(nil) // se le asigna window luego

	a.sidebar = NuevoSidebar(SidebarOpciones{
		Cliente:  a.cliente,
		App:      a.fyneApp,
		OnSelect: func(sesionID string) {
			a.chat.SetSesion(sesionID)
		},
	})

	a.header = NuevoHeader(HeaderOpciones{
		Cliente:      a.cliente,
		App:          a.fyneApp,
		OnToggleTema: a.toggleTema,
	})
	a.header.SetTemaIcon(a.tema)

	a.chat = NuevoChatWindow(ChatWindowOpciones{
		Cliente:        a.cliente,
		OnSesionCreada: func(id string) {
			a.sidebar.SetSesionActiva(id)
		},
		OnToast: func(tipo, msg string) {
			a.toasts.ShowFromString(tipo, msg)
		},
	})

	a.proyectos = NuevoProjectSelector(ProjectSelectorOpciones{
		Cliente:  a.cliente,
		OnCambio: func(p string) {
			a.chat.SetProyecto(p)
		},
	})

	return a
}

// Ejecutar inicia la UI y bloquea hasta que se cierra la ventana.
func (a *App) Ejecutar() {
	a.win = a.fyneApp.NewWindow("Liz — AI Agent para Linux")
	a.win.SetOnClosed(func() {
		// Al cerrar la ventana, salir de la app (no quedarse colgado)
		a.fyneApp.Quit()
	})

	// Asignar ventana a toasts ahora que existe
	a.toasts.window = a.win
	// Y al chat para focus del input
	a.chat.win = a.win

	// Layout principal:
	//   ┌─────────────────────────────────────┐
	//   │ Header                              │
	//   ├──────────┬──────────────────────────┤
	//   │ Sidebar  │ ChatWindow               │
	//   │          │                          │
	//   │          │   [ProjectSelector ▾]    │
	//   │          │                          │
	//   │          │   ┌─────────────────┐    │
	//   │          │   │ mensajes...     │    │
	//   │          │   └─────────────────┘    │
	//   │          │   [input........] [Send] │
	//   └──────────┴──────────────────────────┘
	chatContainer := container.NewBorder(
		a.proyectos, nil, nil, nil,
		a.chat,
	)

	mainSplit := container.NewHSplit(
		a.sidebar,
		chatContainer,
	)
	mainSplit.Offset = 0.25 // sidebar 25% del ancho

	content := container.NewBorder(
		a.header, nil, nil, nil,
		mainSplit,
	)

	a.win.SetContent(content)
	a.win.Resize(fyne.NewSize(1200, 750))
	a.win.SetFixedSize(false)

	// Configurar atajos de teclado
	a.configurarAtajos()

	// Iniciar polling del header y carga inicial
	a.header.Iniciar()
	a.sidebar.Recargar()
	a.proyectos.Recargar()
	a.chat.SetSesion(a.sidebar.SesionActiva())

	// Esperar un poco al backend antes de mostrar (da tiempo a que arranque)
	go func() {
		time.Sleep(800 * time.Millisecond)
		fyne.Do(func() {
			a.win.Show()
		})
	}()

	// Bloquear hasta que se cierre
	a.fyneApp.Run()
}

// toggleTema alterna entre oscuro y claro.
func (a *App) toggleTema() {
	a.mu.Lock()
	if a.tema == ModoOscuro {
		a.tema = ModoClaro
	} else {
		a.tema = ModoOscuro
	}
	modo := a.tema
	a.mu.Unlock()

	a.fyneApp.Settings().SetTheme(NuevoTema(modo))
	a.header.SetTemaIcon(modo)

	// Persistir preferencia
	a.fyneApp.Preferences().SetString("tema", modoToString(modo))
}

// modoToString convierte ModoTema a string para persistencia.
func modoToString(m ModoTema) string {
	if m == ModoClaro {
		return "claro"
	}
	return "oscuro"
}

// stringToModo parsea un string a ModoTema.
func stringToModo(s string) ModoTema {
	if s == "claro" {
		return ModoClaro
	}
	return ModoOscuro
}

// configurarAtajos registra los atajos de teclado globales.
func (a *App) configurarAtajos() {
	// Ctrl+N — nueva conversación
	a.win.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyN,
		Modifier: fyne.KeyModifierControl,
	}, func(shortcut fyne.Shortcut) {
		// Trigger crear nueva conversación (no exponemos método público
		// pero podemos llamar al callback onCrear indirectamente).
		// Solución simple: hacer clic en el sidebar con un shortcut interno.
		// Por ahora solo refrescamos.
		a.sidebar.Recargar()
	})

	// Ctrl+K — enfocar input
	a.win.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyK,
		Modifier: fyne.KeyModifierControl,
	}, func(shortcut fyne.Shortcut) {
		a.win.Canvas().Focus(nil) // quitar focus
	})

	// Ctrl+R — refrescar
	a.win.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyR,
		Modifier: fyne.KeyModifierControl,
	}, func(shortcut fyne.Shortcut) {
		a.sidebar.Recargar()
		a.proyectos.Recargar()
	})
}

// EsperarBackend bloquea hasta que el backend responde o se agota el timeout.
// Útil para arrancar la UI justo después del servidor HTTP.
func EsperarBackend(baseURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	c := NuevoCliente(OpcionesCliente{BaseURL: baseURL, Timeout: 3 * time.Second})
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, err := c.Health(ctx)
		cancel()
		if err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return &backendNoListoError{url: baseURL, timeout: timeout}
}

type backendNoListoError struct {
	url     string
	timeout time.Duration
}

func (e *backendNoListoError) Error() string {
	return "backend no listo en " + e.url + " tras " + e.timeout.String()
}

// IsFullscreenAvailable indica si el driver soporta fullscreen (no mobile).
func IsFullscreenAvailable() bool {
	return true
}

// Texto para testing.
var _ = widget.NewLabel
