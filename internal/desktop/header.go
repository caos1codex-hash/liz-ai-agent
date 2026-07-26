package desktop

import (
	"context"
	"fmt"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// Header es la barra superior con:
//   - Logo + nombre Liz
//   - Status del backend (online/offline)
//   - Modelo NVIDIA en uso
//   - Métricas del pipeline (mensajes procesados, último uso)
//   - Toggle de tema (dark/light)
//
// Es el equivalente desktop de web/src/components/Header.tsx + MetricsPanel.tsx
// + hooks/useBackendHealth.ts + usePipelineMetricas.ts + useModelos.ts + useTheme.ts.
type Header struct {
	widget.BaseWidget

	cliente *ClienteBackend
	app     fyne.App

	mu             sync.RWMutex
	online         bool
	modeloMasUsado string
	msgProc        int64
	categorias     map[string]int
	ultimoUso      time.Time

	// Callbacks
	onToggleTema func()

	// UI
	titulo      *widget.Label
	statusDot   *StatusDot
	statusLbl   *widget.Label
	modeloLbl   *widget.Label
	metricasBtn *widget.Button
	temaBtn     *widget.Button
}

// HeaderOpciones configura el header.
type HeaderOpciones struct {
	Cliente      *ClienteBackend
	App          fyne.App
	OnToggleTema func()
}

// NuevoHeader construye el header.
func NuevoHeader(opt HeaderOpciones) *Header {
	h := &Header{
		cliente:      opt.Cliente,
		app:          opt.App,
		onToggleTema: opt.OnToggleTema,
	}
	h.ExtendBaseWidget(h)

	h.titulo = widget.NewLabel("Liz")
	h.titulo.TextStyle = fyne.TextStyle{Bold: true}
	h.titulo.Importance = widget.HighImportance

	h.statusDot = NewStatusDot()
	h.statusLbl = widget.NewLabel("conectando…")
	h.statusLbl.Importance = widget.LowImportance

	h.modeloLbl = widget.NewLabel("sin modelo")
	h.modeloLbl.Importance = widget.LowImportance
	h.modeloLbl.Truncation = fyne.TextTruncateEllipsis

	h.metricasBtn = widget.NewButtonWithIcon("", theme_iconInfo(), func() {
		h.mostrarMetricas()
	})
	h.metricasBtn.Importance = widget.LowImportance

	h.temaBtn = widget.NewButtonWithIcon("", theme_iconSun(), func() {
		if h.onToggleTema != nil {
			h.onToggleTema()
		}
	})
	h.temaBtn.Importance = widget.LowImportance

	return h
}

// CreateRenderer implementa fyne.Widget.
func (h *Header) CreateRenderer() fyne.WidgetRenderer {
	left := container.NewHBox(
		widget.NewIcon(theme_iconComputer()),
		h.titulo,
	)
	mid := container.NewHBox(
		h.statusDot,
		h.statusLbl,
		widget.NewSeparator(), // vertical separator
		widget.NewLabel("🧠"),
		h.modeloLbl,
	)
	right := container.NewHBox(
		h.metricasBtn,
		h.temaBtn,
	)
	content := container.NewBorder(nil, nil, left, right, mid)
	return widget.NewSimpleRenderer(content)
}

// Iniciar empieza el polling periódico al backend.
// Health cada 30s, métricas cada 60s, modelos una vez.
func (h *Header) Iniciar() {
	go h.pollHealth()
	go h.pollMetricas()
	go h.cargarModelos()
}

// pollHealth hace GET /api/v1/health cada 30s.
func (h *Header) pollHealth() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		h.checkHealthOnce()
		<-ticker.C
	}
}

func (h *Header) checkHealthOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := h.cliente.Health(ctx)
	fyne.Do(func() {
		h.mu.Lock()
		h.online = err == nil
		if h.online {
			h.statusDot.SetEstado(EstadoOnline)
			h.statusLbl.SetText("online")
		} else {
			h.statusDot.SetEstado(EstadoOffline)
			h.statusLbl.SetText("offline")
		}
		h.mu.Unlock()
	})
}

// pollMetricas hace GET /api/v1/chat cada 60s.
func (h *Header) pollMetricas() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		h.pollMetricasOnce()
		<-ticker.C
	}
}

func (h *Header) pollMetricasOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	est, err := h.cliente.EstadoChat(ctx)
	if err != nil {
		return
	}
	fyne.Do(func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.msgProc = est.MensajesProcesados
		h.modeloMasUsado = est.ModeloMasUsado
		h.categorias = est.Categorias
		if t, err := time.Parse(time.RFC3339, est.UltimoUso); err == nil {
			h.ultimoUso = t
		}
		if h.modeloMasUsado != "" {
			h.modeloLbl.SetText(h.modeloMasUsado)
		} else {
			h.modeloLbl.SetText("sin modelo")
		}
	})
}

// cargarModelos carga la lista de modelos NVIDIA una sola vez.
func (h *Header) cargarModelos() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	modelos, err := h.cliente.ListarModelos(ctx)
	if err != nil || len(modelos) == 0 {
		return
	}
	fyne.Do(func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.modeloMasUsado == "" && len(modelos) > 0 {
			h.modeloLbl.SetText(modelos[0].Nombre)
		}
	})
}

// mostrarMetricas abre un diálogo con las métricas detalladas.
func (h *Header) mostrarMetricas() {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var cats string
	if len(h.categorias) > 0 {
		for cat, n := range h.categorias {
			cats += fmt.Sprintf("  %s: %d\n", cat, n)
		}
	} else {
		cats = "  (sin datos)"
	}

	ultimo := "nunca"
	if !h.ultimoUso.IsZero() {
		ultimo = formatoRelativo(h.ultimoUso)
	}

	texto := fmt.Sprintf(
		"Mensajes procesados: %d\nÚltimo uso: %s\nModelo más usado: %s\n\nCategorías:\n%s",
		h.msgProc, ultimo, h.modeloMasUsado, cats,
	)

	if h.app != nil {
		for _, w := range h.app.Driver().AllWindows() {
			var p *widget.PopUp
			p = widget.NewModalPopUp(
				container.NewVBox(
					widget.NewLabelWithStyle("Métricas del Pipeline", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
					widget.NewLabel(texto),
					widget.NewButton("Cerrar", func() {
						if p != nil {
							p.Hide()
						}
					}),
				),
				w.Canvas(),
			)
			p.Show()
			break
		}
	}
}

// SetTemaIcon actualiza el icono del botón de tema según el modo actual.
func (h *Header) SetTemaIcon(modo ModoTema) {
	if modo == ModoOscuro {
		h.temaBtn.SetIcon(theme_iconSun())
	} else {
		h.temaBtn.SetIcon(theme_iconMoon())
	}
}

// Online devuelve el estado actual del backend.
func (h *Header) Online() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.online
}
