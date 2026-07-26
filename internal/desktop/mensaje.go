package desktop

import (
        "fmt"
        "strings"
        "time"

        "fyne.io/fyne/v2"
        "fyne.io/fyne/v2/container"
        "fyne.io/fyne/v2/widget"
)

// MensajeBubble renderiza un mensaje del usuario o del asistente.
//
// Es el equivalente desktop de web/src/components/Message.tsx + Markdown.tsx.
// A diferencia del web original que usa react-markdown + react-syntax-highlighter,
// aquí usamos widget.RichText con markdown básico soportado por Fyne.
type MensajeBubble struct {
        widget.BaseWidget

        rol       string
        contenido string
        estado    string

        // Metadata
        modelo       string
        herramientas []string
        tokens       int
        duracionMs   int64
        pasos        int
        enviado      time.Time

        // UI
        content *widget.RichText
        meta    *widget.Label
}

// NewMensajeBubble crea una burbuja vacía.
func NewMensajeBubble() *MensajeBubble {
        m := &MensajeBubble{}
        m.ExtendBaseWidget(m)
        m.content = widget.NewRichTextWithText("")
        m.content.Wrapping = fyne.TextWrapWord
        m.meta = widget.NewLabel("")
        m.meta.Importance = widget.LowImportance
        m.meta.TextStyle = fyne.TextStyle{Italic: true}
        m.meta.Truncation = fyne.TextTruncateEllipsis
        return m
}

// SetMensaje actualiza el contenido mostrado.
func (m *MensajeBubble) SetMensaje(msg MensajeUI) {
        m.rol = msg.Rol
        m.contenido = msg.Contenido
        m.estado = msg.Estado
        m.modelo = msg.Modelo
        m.herramientas = msg.Herramientas
        m.tokens = msg.Tokens
        m.duracionMs = msg.DuracionMs
        m.pasos = msg.Pasos
        m.enviado = msg.Enviado
        m.Refresh()
}

// CreateRenderer implementa fyne.Widget.
func (m *MensajeBubble) CreateRenderer() fyne.WidgetRenderer {
        m.ExtendBaseWidget(m)
        return widget.NewSimpleRenderer(m.layout())
}

func (m *MensajeBubble) layout() fyne.CanvasObject {
        // Reconstruir al refrescar
        avatar := m.avatarIcon()
        header := container.NewHBox(
                avatar,
                widget.NewLabel(m.nombreRol()),
        )
        if m.estado == "enviando" {
                header.Add(widget.NewLabel("· enviando…"))
        } else if m.estado == "pensando" {
                header.Add(widget.NewLabel("· pensando…"))
        } else if m.estado == "streaming" {
                header.Add(widget.NewLabel("· escribiendo…"))
        }

        // Contenido
        if m.estado == "pensando" && m.contenido == "" {
                m.content.ParseMarkdown("**Liz está pensando…**")
        } else if m.contenido != "" {
                m.content.ParseMarkdown(m.contenido)
        } else {
                m.content.ParseMarkdown("")
        }

        // Metadata
        metaText := m.metadataText()
        if metaText != "" {
                m.meta.SetText(metaText)
                m.meta.Show()
        } else {
                m.meta.Hide()
        }

        body := container.NewVBox(
                m.content,
                m.meta,
        )

        // Alineación según rol
        var root fyne.CanvasObject
        if m.rol == "usuario" {
                // Mensaje usuario alineado a la derecha, con fondo primario
                root = container.NewBorder(nil, nil, nil, body, header)
        } else {
                // Asistente a la izquierda
                root = container.NewVBox(header, body)
        }
        return root
}

// avatarIcon devuelve el icono según el rol.
func (m *MensajeBubble) avatarIcon() *widget.Icon {
        if m.rol == "usuario" {
                return widget.NewIcon(theme_iconComputer())
        }
        return widget.NewIcon(theme_iconChat())
}

// nombreRol devuelve "Tú" o "Liz".
func (m *MensajeBubble) nombreRol() string {
        if m.rol == "usuario" {
                return "Tú"
        }
        return "Liz"
}

// metadataText construye el texto de badges (modelo, herramientas, tokens, etc.).
func (m *MensajeBubble) metadataText() string {
        if m.estado != "" {
                return "" // sin metadata mientras está en curso
        }
        var parts []string
        if m.modelo != "" {
                parts = append(parts, "🧠 "+m.modelo)
        }
        if len(m.herramientas) > 0 {
                parts = append(parts, "🔧 "+strings.Join(m.herramientas, ", "))
        }
        if m.pasos > 0 {
                parts = append(parts, fmt.Sprintf("📋 %d pasos", m.pasos))
        }
        if m.tokens > 0 {
                parts = append(parts, fmt.Sprintf("🎫 %d tokens", m.tokens))
        }
        if m.duracionMs > 0 {
                parts = append(parts, fmt.Sprintf("⏱ %s", formatoDuracion(m.duracionMs)))
        }
        return strings.Join(parts, "  ·  ")
}

// formatoDuracion convierte ms a "1.2s" o "350ms".
func formatoDuracion(ms int64) string {
        if ms < 1000 {
                return fmt.Sprintf("%dms", ms)
        }
        return fmt.Sprintf("%.1fs", float64(ms)/1000.0)
}
