package desktop

import (
	"context"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// ProjectSelector permite elegir qué proyecto indexado usar como contexto
// para los mensajes al pipeline.
//
// Es el equivalente desktop de web/src/components/ProjectSelector.tsx + useProyectos.ts.
type ProjectSelector struct {
	widget.BaseWidget

	cliente *ClienteBackend

	mu          sync.RWMutex
	proyectos   []ProyectoContexto
	seleccionado string

	// UI
	selectW *widget.Select
	label   *widget.Label

	// Callback
	onCambio func(proyecto string)
}

// ProjectSelectorOpciones configura el selector.
type ProjectSelectorOpciones struct {
	Cliente   *ClienteBackend
	OnCambio  func(proyecto string)
}

// NuevoProjectSelector crea el selector de proyecto.
func NuevoProjectSelector(opt ProjectSelectorOpciones) *ProjectSelector {
	s := &ProjectSelector{
		cliente:  opt.Cliente,
		onCambio: opt.OnCambio,
	}
	s.ExtendBaseWidget(s)

	s.label = widget.NewLabel("Proyecto:")
	s.label.Importance = widget.LowImportance

	s.selectW = widget.NewSelect([]string{"(ninguno)"}, func(sel string) {
		s.mu.Lock()
		if sel == "(ninguno)" {
			s.seleccionado = ""
		} else {
			s.seleccionado = sel
		}
		cambio := s.seleccionado
		s.mu.Unlock()
		if s.onCambio != nil {
			s.onCambio(cambio)
		}
	})
	s.selectW.SetSelectedIndex(0)

	return s
}

// CreateRenderer implementa fyne.Widget.
func (s *ProjectSelector) CreateRenderer() fyne.WidgetRenderer {
	content := container.NewHBox(
		widget.NewIcon(theme_iconStorage()),
		s.label,
		s.selectW,
	)
	return widget.NewSimpleRenderer(content)
}

// Recargar vuelve a pedir la lista de proyectos al backend.
func (s *ProjectSelector) Recargar() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		proyectos, err := s.cliente.ListarProyectos(ctx)
		fyne.Do(func() {
			if err != nil {
				return
			}
			s.mu.Lock()
			s.proyectos = proyectos
			s.mu.Unlock()
			s.reconstruirOpciones()
		})
	}()
}

// reconstruirOpciones actualiza las opciones del widget.Select.
func (s *ProjectSelector) reconstruirOpciones() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	opciones := []string{"(ninguno)"}
	for _, p := range s.proyectos {
		opciones = append(opciones, p.Nombre)
	}
	s.selectW.Options = opciones
	// Mantener selección si sigue siendo válida
	if s.seleccionado != "" {
		encontrado := false
		for _, p := range s.proyectos {
			if p.Nombre == s.seleccionado {
				encontrado = true
				break
			}
		}
		if !encontrado {
			s.selectW.SetSelectedIndex(0)
		} else {
			s.selectW.SetSelected(s.seleccionado)
		}
	} else {
		s.selectW.SetSelectedIndex(0)
	}
	s.selectW.Refresh()
}

// Seleccionado devuelve el nombre del proyecto actualmente seleccionado.
func (s *ProjectSelector) Seleccionado() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.seleccionado
}

// SetSeleccionado permite forzar la selección desde fuera.
func (s *ProjectSelector) SetSeleccionado(nombre string) {
	s.mu.Lock()
	s.seleccionado = nombre
	s.mu.Unlock()
	s.reconstruirOpciones()
}
