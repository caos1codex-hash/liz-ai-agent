package desktop

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// MensajeUI representa un mensaje en la conversación (usuario o asistente).
type MensajeUI struct {
	Rol       string // "usuario" | "asistente"
	Contenido string
	// Estado del mensaje:
	//   ""              -> completo
	//   "enviando"      -> mensaje usuario optimista (esperando confirmación)
	//   "pensando"      -> asistente antes del primer chunk
	//   "streaming"     -> asistente recibiéndose
	Estado string

	// Metadata (cuando está completo)
	Modelo       string
	Herramientas []string
	Tokens       int
	DuracionMs   int64
	Pasos        int

	// Timestamp
	Enviado time.Time
}

// ChatWindow orquesta la conversación: MessageList + MessageInput + SSE.
//
// Es el equivalente desktop de web/src/components/ChatWindow.tsx + useChat.ts.
type ChatWindow struct {
	widget.BaseWidget

	cliente *ClienteBackend
	win     fyne.Window

	mu        sync.RWMutex
	mensajes  []MensajeUI
	sesionID  string
	proyecto  string
	enviando  bool               // lock para evitar envíos concurrentes
	ctxCancel context.CancelFunc // cancela el stream en curso

	// UI
	list  *widget.List
	input *widget.Entry
	sendB *widget.Button

	// Callbacks
	onSesionCreada func(id string)
	onToast        func(tipo, msg string)
}

// ChatWindowOpciones configura la ventana de chat.
type ChatWindowOpciones struct {
	Cliente        *ClienteBackend
	Window         fyne.Window
	OnSesionCreada func(sesionID string)
	OnToast        func(tipo, msg string)
}

// NuevoChatWindow construye la ventana de chat.
func NuevoChatWindow(opt ChatWindowOpciones) *ChatWindow {
	c := &ChatWindow{
		cliente:        opt.Cliente,
		win:            opt.Window,
		onSesionCreada: opt.OnSesionCreada,
		onToast:        opt.OnToast,
	}
	c.ExtendBaseWidget(c)

	c.list = widget.NewList(
		func() int {
			c.mu.RLock()
			defer c.mu.RUnlock()
			return len(c.mensajes)
		},
		func() fyne.CanvasObject {
			return NewMensajeBubble()
		},
		func(i widget.ListItemID, obj fyne.CanvasObject) {
			c.mu.RLock()
			defer c.mu.RUnlock()
			if i < 0 || i >= len(c.mensajes) {
				return
			}
			bubble := obj.(*MensajeBubble)
			bubble.SetMensaje(c.mensajes[i])
		},
	)

	c.input = widget.NewMultiLineEntry()
	c.input.SetPlaceHolder("Escribe un mensaje a Liz… (Enter para enviar, Shift+Enter para salto de línea)")
	c.input.Wrapping = fyne.TextWrapWord
	c.input.OnSubmitted = func(s string) {
		c.enviarMensaje(s)
	}

	c.sendB = widget.NewButtonWithIcon("Enviar", theme_iconSend(), func() {
		c.enviarMensaje(c.input.Text)
	})
	c.sendB.Importance = widget.HighImportance

	return c
}

// CreateRenderer implementa fyne.Widget.
func (c *ChatWindow) CreateRenderer() fyne.WidgetRenderer {
	inputBar := container.NewBorder(
		nil, nil, nil, c.sendB, c.input,
	)
	content := container.NewBorder(
		nil, inputBar, nil, nil,
		c.list,
	)
	return widget.NewSimpleRenderer(content)
}

// SetSesion cambia la sesión activa y carga sus mensajes.
func (c *ChatWindow) SetSesion(id string) {
	c.mu.Lock()
	c.sesionID = id
	c.mensajes = nil
	c.mu.Unlock()
	c.list.Refresh()

	if id == "" {
		c.mostrarWelcome()
		return
	}
	c.cargarMensajes(id)
}

// SetProyecto cambia el proyecto de contexto activo.
func (c *ChatWindow) SetProyecto(p string) {
	c.mu.Lock()
	c.proyecto = p
	c.mu.Unlock()
}

// mostrarWelcome inserta 4 prompts de ejemplo clicables como mensaje de bienvenida.
func (c *ChatWindow) mostrarWelcome() {
	c.mu.Lock()
	c.mensajes = []MensajeUI{{
		Rol:       "asistente",
		Contenido: welcomeMessage(),
		Estado:    "",
		Enviado:   time.Now(),
	}}
	c.mu.Unlock()
	c.list.Refresh()
}

// cargarMensajes pide al backend los mensajes de la sesión.
func (c *ChatWindow) cargarMensajes(id string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, msgs, err := c.cliente.ObtenerSesion(ctx, id)
		fyne.Do(func() {
			if err != nil {
				if c.onToast != nil {
					c.onToast("error", "No se pudo cargar la conversación: "+err.Error())
				}
				c.mostrarWelcome()
				return
			}
			c.mu.Lock()
			c.mensajes = c.mensajes[:0]
			for _, m := range msgs {
				c.mensajes = append(c.mensajes, MensajeUI{
					Rol:       m.Rol,
					Contenido: m.Contenido,
					Estado:    "",
					Enviado:   m.Timestamp,
				})
			}
			if len(c.mensajes) == 0 {
				c.mensajes = append(c.mensajes, MensajeUI{
					Rol:       "asistente",
					Contenido: welcomeMessage(),
					Enviado:   time.Now(),
				})
			}
			c.mu.Unlock()
			c.list.Refresh()
			c.scrollToEnd()
		})
	}()
}

// enviarMensaje hace el flujo completo:
//  1. Mensaje optimista del usuario
//  2. Placeholder "Liz está pensando…"
//  3. POST /api/v1/chat con stream=true
//  4. Acumula chunks SSE en el mensaje asistente
//  5. Marca completado al recibir chunk tipo="completado"
func (c *ChatWindow) enviarMensaje(texto string) {
	texto = strings.TrimSpace(texto)
	if texto == "" {
		return
	}
	c.mu.Lock()
	if c.enviando {
		c.mu.Unlock()
		return
	}
	c.enviando = true
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.enviando = false
		c.mu.Unlock()
	}()

	// Limpiar input
	fyne.Do(func() {
		c.input.SetText("")
	})

	// Mensaje usuario optimista
	c.mu.Lock()
	usuarioIdx := len(c.mensajes)
	c.mensajes = append(c.mensajes, MensajeUI{
		Rol:       "usuario",
		Contenido: texto,
		Estado:    "enviando",
		Enviado:   time.Now(),
	})
	// Placeholder asistente
	asistenteIdx := len(c.mensajes)
	c.mensajes = append(c.mensajes, MensajeUI{
		Rol:       "asistente",
		Contenido: "",
		Estado:    "pensando",
		Enviado:   time.Now(),
	})
	c.mu.Unlock()
	fyne.Do(func() {
		c.list.Refresh()
		c.scrollToEnd()
	})

	// Contexto cancelable
	ctx, cancel := context.WithCancel(context.Background())
	c.mu.Lock()
	c.ctxCancel = cancel
	sesionID := c.sesionID
	proyecto := c.proyecto
	c.mu.Unlock()
	defer cancel()

	sol := SolicitudChat{
		Mensaje:   texto,
		SesionID:  sesionID,
		Proyecto:  proyecto,
		UsuarioID: "usuario_default",
		Stream:    true,
	}

	ch, err := c.cliente.StreamChat(ctx, sol)
	if err != nil {
		fyne.Do(func() {
			c.mu.Lock()
			if asistenteIdx < len(c.mensajes) {
				c.mensajes[asistenteIdx].Estado = ""
				c.mensajes[asistenteIdx].Contenido = "Error: " + err.Error()
			}
			if usuarioIdx < len(c.mensajes) {
				c.mensajes[usuarioIdx].Estado = ""
			}
			c.mu.Unlock()
			c.list.Refresh()
			if c.onToast != nil {
				c.onToast("error", "No se pudo enviar el mensaje: "+err.Error())
			}
		})
		return
	}

	// Procesar chunks en background
	go func() {
		var (
			accumulado   strings.Builder
			herramientas []string
			modelo       string
			tokens       int
			duracionMs   int64
			pasos        int
			recibioTexto bool
		)

		for chunk := range ch {
			switch chunk.Tipo {
			case "estado":
				// "Iniciando pipeline..." — actualizar placeholder
				fyne.Do(func() {
					c.mu.Lock()
					if asistenteIdx < len(c.mensajes) {
						c.mensajes[asistenteIdx].Estado = "pensando"
						c.mensajes[asistenteIdx].Contenido = chunk.Contenido
					}
					c.mu.Unlock()
					c.list.Refresh()
				})

			case "herramienta":
				herramientas = append(herramientas, chunk.Contenido)
				fyne.Do(func() {
					c.mu.Lock()
					if asistenteIdx < len(c.mensajes) {
						c.mensajes[asistenteIdx].Herramientas = herramientas
						if !recibioTexto {
							c.mensajes[asistenteIdx].Contenido = "Usando " + chunk.Contenido + "…"
						}
					}
					c.mu.Unlock()
					c.list.Refresh()
				})

			case "pensamiento":
				// Información intermedia (no se acumula en texto final)
				fyne.Do(func() {
					c.mu.Lock()
					if asistenteIdx < len(c.mensajes) && !recibioTexto {
						c.mensajes[asistenteIdx].Contenido = chunk.Contenido
					}
					c.mu.Unlock()
					c.list.Refresh()
				})

			case "hecho":
				// Hecho de memoria aprendido — ignorar por ahora

			case "texto":
				recibioTexto = true
				accumulado.WriteString(chunk.Contenido)
				fyne.Do(func() {
					c.mu.Lock()
					if asistenteIdx < len(c.mensajes) {
						c.mensajes[asistenteIdx].Estado = "streaming"
						c.mensajes[asistenteIdx].Contenido = accumulado.String()
					}
					c.mu.Unlock()
					c.list.Refresh()
					c.scrollToEnd()
				})

			case "completado":
				if chunk.Modelo != "" {
					modelo = chunk.Modelo
				}
				if chunk.Tokens > 0 {
					tokens = chunk.Tokens
				}
				if chunk.DuracionMs > 0 {
					duracionMs = chunk.DuracionMs
				}
				if chunk.PasosEjecutados > 0 {
					pasos = chunk.PasosEjecutados
				}
				if chunk.SesionID != "" && sesionID == "" {
					// El backend creó una sesión nueva
					sesionID = chunk.SesionID
					if c.onSesionCreada != nil {
						fyne.Do(func() { c.onSesionCreada(sesionID) })
					}
				}
				fyne.Do(func() {
					c.mu.Lock()
					if asistenteIdx < len(c.mensajes) {
						c.mensajes[asistenteIdx].Estado = ""
						if acumulado := accumulado.String(); acumulado != "" {
							c.mensajes[asistenteIdx].Contenido = acumulado
						}
						c.mensajes[asistenteIdx].Modelo = modelo
						c.mensajes[asistenteIdx].Tokens = tokens
						c.mensajes[asistenteIdx].DuracionMs = duracionMs
						c.mensajes[asistenteIdx].Pasos = pasos
						c.mensajes[asistenteIdx].Herramientas = herramientas
					}
					if usuarioIdx < len(c.mensajes) {
						c.mensajes[usuarioIdx].Estado = ""
					}
					c.mu.Unlock()
					c.list.Refresh()
				})
				return

			case "error":
				fyne.Do(func() {
					c.mu.Lock()
					if asistenteIdx < len(c.mensajes) {
						c.mensajes[asistenteIdx].Estado = ""
						c.mensajes[asistenteIdx].Contenido = "Error: " + chunk.Contenido
					}
					if usuarioIdx < len(c.mensajes) {
						c.mensajes[usuarioIdx].Estado = ""
					}
					c.mu.Unlock()
					c.list.Refresh()
					if c.onToast != nil {
						c.onToast("error", chunk.Contenido)
					}
				})
				return
			}
		}

		// Stream cerrado sin chunk completado — marcar como finalizado
		fyne.Do(func() {
			c.mu.Lock()
			if asistenteIdx < len(c.mensajes) {
				c.mensajes[asistenteIdx].Estado = ""
			}
			if usuarioIdx < len(c.mensajes) {
				c.mensajes[usuarioIdx].Estado = ""
			}
			c.mu.Unlock()
			c.list.Refresh()
		})
	}()
}

// Detener cancela el stream en curso (botón "Detener").
func (c *ChatWindow) Detener() {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.ctxCancel != nil {
		c.ctxCancel()
	}
}

// EstaEnviando indica si hay un stream en curso.
func (c *ChatWindow) EstaEnviando() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enviando
}

// scrollToEnd mueve el scroll al final (auto-scroll durante streaming).
func (c *ChatWindow) scrollToEnd() {
	c.mu.RLock()
	n := len(c.mensajes)
	c.mu.RUnlock()
	if n > 0 {
		c.list.ScrollToBottom()
	}
}

// welcomeMessage devuelve el mensaje de bienvenida de Liz.
func welcomeMessage() string {
	return `Hola, soy **Liz**. Tu agente de IA para Linux con control total del sistema.

Puedo:
- Ejecutar comandos y administrar tu sistema
- Buscar y manipular archivos
- Escribir y editar código
- Instalar software
- **Programarme a mí misma** herramientas que no tengo

Prueba con:
- "Lista los procesos que consumen más RAM"
- "Busca todos los .log en /var/log y dime cuántos hay"
- "Instala htop"
- "Crea un servidor HTTP en Go"`
}

// welcomePrompts devuelve 4 prompts de ejemplo clicables.
func welcomePrompts() []string {
	return []string{
		"Lista los procesos que consumen más RAM",
		"Busca todos los .log en /var/log",
		"Instala htop",
		"Crea un servidor HTTP en Go con auth JWT",
	}
}

// SetInput permite setear texto en el input desde fuera (ej: click en prompt de bienvenida).
func (c *ChatWindow) SetInput(s string) {
	c.input.SetText(s)
	if c.win != nil {
		c.win.Canvas().Focus(c.input)
	}
}

// enviarPrompt rapido — para clicks en welcome state.
func (c *ChatWindow) EnviarPrompt(p string) {
	c.enviarMensaje(p)
}

// SesionActual devuelve el ID de sesión actual.
func (c *ChatWindow) SesionActual() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sesionID
}

// Mensajes devuelve una copia de los mensajes actuales (para tests).
func (c *ChatWindow) Mensajes() []MensajeUI {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]MensajeUI, len(c.mensajes))
	copy(out, c.mensajes)
	return out
}

// String para debug.
func (c *ChatWindow) String() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return fmt.Sprintf("ChatWindow{sesion=%q, mensajes=%d, enviando=%v}", c.sesionID, len(c.mensajes), c.enviando)
}
