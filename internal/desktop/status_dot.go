package desktop

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// EstadoStatus representa el estado del backend.
type EstadoStatus int

const (
	// EstadoChecking color ámbar (conectando).
	EstadoChecking EstadoStatus = iota
	// EstadoOnline color verde.
	EstadoOnline
	// EstadoOffline color rojo.
	EstadoOffline
)

// StatusDot es un punto de color que indica el estado del backend.
// Es el equivalente desktop de web/src/components/StatusDot.tsx.
type StatusDot struct {
	widget.BaseWidget
	estado EstadoStatus
}

// NewStatusDot crea un StatusDot en estado checking.
func NewStatusDot() *StatusDot {
	s := &StatusDot{estado: EstadoChecking}
	s.ExtendBaseWidget(s)
	return s
}

// SetEstado cambia el color del punto.
func (s *StatusDot) SetEstado(e EstadoStatus) {
	s.estado = e
	s.Refresh()
}

// colorParaEstado devuelve el color RGB correspondiente.
func (s *StatusDot) colorParaEstado() color.Color {
	switch s.estado {
	case EstadoOnline:
		return &color.NRGBA{R: 0x10, G: 0xb9, B: 0x81, A: 0xff}
	case EstadoOffline:
		return &color.NRGBA{R: 0xef, G: 0x44, B: 0x44, A: 0xff}
	default: // checking
		return &color.NRGBA{R: 0xf5, G: 0x9e, B: 0x0b, A: 0xff}
	}
}

// CreateRenderer implementa fyne.Widget.
func (s *StatusDot) CreateRenderer() fyne.WidgetRenderer {
	circ := canvas.NewCircle(s.colorParaEstado())
	circ.StrokeWidth = 0
	return widget.NewSimpleRenderer(circ)
}

// MinSize Override — punto pequeño fijo.
func (s *StatusDot) MinSize() fyne.Size {
	return fyne.NewSize(10, 10)
}
