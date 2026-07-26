package desktop

import (
        "context"
        "fmt"
        "sync"
        "time"

        "fyne.io/fyne/v2"
        "fyne.io/fyne/v2/container"
        "fyne.io/fyne/v2/dialog"
        "fyne.io/fyne/v2/widget"
)

// Sidebar muestra la lista de conversaciones con CRUD completo:
// listar, crear, seleccionar y eliminar sesiones.
//
// Es el equivalente desktop del componente web/src/components/Sidebar.tsx
// + hooks/useSesiones.ts. Persistencia:
//   - Sesión activa en fyne.App.Preferences()
//   - Sesiones en backend (memoria/sesiones.go)
type Sidebar struct {
        widget.BaseWidget

        cliente *ClienteBackend
        app     fyne.App
        win     fyne.Window

        // Estado
        mu           sync.RWMutex
        sesiones     []SesionChat
        sesionActiva string
        usuarioID    string

        // Callbacks
        onSeleccion func(sesionID string)
        onCrear     func(sesion *SesionChat)

        // UI
        lista     *widget.List
        refreshB  *widget.Button
        estadoLbl *widget.Label
}

// SidebarOpciones configura el sidebar.
type SidebarOpciones struct {
        Cliente  *ClienteBackend
        App      fyne.App
        Window   fyne.Window
        Usuario  string
        OnSelect func(sesionID string)
        OnCrear  func(sesion *SesionChat)
}

// NuevoSidebar construye el sidebar de conversaciones.
func NuevoSidebar(opt SidebarOpciones) *Sidebar {
        if opt.Usuario == "" {
                opt.Usuario = "usuario_default"
        }
        s := &Sidebar{
                cliente:     opt.Cliente,
                app:         opt.App,
                win:         opt.Window,
                usuarioID:   opt.Usuario,
                onSeleccion: opt.OnSelect,
                onCrear:     opt.OnCrear,
        }
        s.ExtendBaseWidget(s)

        // Restaurar sesión activa de preferencias
        if s.app != nil {
                s.sesionActiva = s.app.Preferences().StringWithFallback("sesion_activa", "")
        }

        s.lista = widget.NewList(
                func() int {
                        s.mu.RLock()
                        defer s.mu.RUnlock()
                        return len(s.sesiones)
                },
                func() fyne.CanvasObject {
                        return container.NewBorder(
                                nil, nil, nil,
                                widget.NewButtonWithIcon("", theme_iconDelete(), func() {}),
                                container.NewVBox(
                                        widget.NewLabel("Título de conversación"),
                                        widget.NewLabel("hace 2 min"),
                                ),
                        )
                },
                func(i widget.ListItemID, obj fyne.CanvasObject) {
                        s.mu.RLock()
                        defer s.mu.RUnlock()
                        if i < 0 || i >= len(s.sesiones) {
                                return
                        }
                        ses := s.sesiones[i]
                        border := obj.(*fyne.Container)
                        info := border.Objects[0].(*fyne.Container)
                        titulo := info.Objects[0].(*widget.Label)
                        sub := info.Objects[1].(*widget.Label)

                        titulo.SetText(s.tituloSesion(ses))
                        titulo.TextStyle = fyne.TextStyle{Bold: ses.ID == s.sesionActiva}
                        titulo.Truncation = fyne.TextTruncateEllipsis

                        sub.SetText(formatoRelativo(ses.CreadaEn))
                        sub.Importance = widget.LowImportance

                        btnEliminar := border.Objects[1].(*widget.Button)
                        // Re-bind del handler con el ID correcto
                        btnEliminar.OnTapped = func() {
                                s.confirmarEliminar(ses.ID)
                        }
                },
        )
        s.lista.OnSelected = func(id widget.ListItemID) {
                s.mu.RLock()
                defer s.mu.RUnlock()
                if id < 0 || id >= len(s.sesiones) {
                        return
                }
                ses := s.sesiones[id]
                s.sesionActiva = ses.ID
                if s.app != nil {
                        s.app.Preferences().SetString("sesion_activa", ses.ID)
                }
                if s.onSeleccion != nil {
                        s.onSeleccion(ses.ID)
                }
                s.lista.Refresh()
        }

        s.estadoLbl = widget.NewLabel("Cargando...")
        s.estadoLbl.Importance = widget.LowImportance
        s.estadoLbl.Truncation = fyne.TextTruncateEllipsis

        s.refreshB = widget.NewButtonWithIcon("Refrescar", theme_iconRefresh(), func() {
                s.Recargar()
        })

        return s
}

// CreateRenderer implementa fyne.Widget.
func (s *Sidebar) CreateRenderer() fyne.WidgetRenderer {
        header := container.NewBorder(
                nil, nil,
                widget.NewIcon(theme_iconChat()),
                container.NewHBox(s.refreshB),
                widget.NewLabel("Conversaciones"),
        )

        nuevaBtn := widget.NewButtonWithIcon("Nueva conversación", theme_iconPlus(), func() {
                s.crearNueva()
        })

        content := container.NewBorder(
                header,
                container.NewVBox(nuevaBtn, s.estadoLbl),
                nil, nil,
                s.lista,
        )
        return widget.NewSimpleRenderer(content)
}

// Recargar vuelve a pedir las sesiones al backend.
func (s *Sidebar) Recargar() {
        go func() {
                ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
                defer cancel()

                sesiones, err := s.cliente.ListarSesiones(ctx, s.usuarioID)
                fyne.Do(func() {
                        if err != nil {
                                s.estadoLbl.SetText("Error: " + err.Error())
                                return
                        }
                        s.mu.Lock()
                        s.sesiones = sesiones
                        s.mu.Unlock()
                        s.estadoLbl.SetText(fmt.Sprintf("%d conversaciones", len(sesiones)))
                        s.lista.Refresh()
                })
        }()
}

// crearNueva pide al backend una nueva sesión y la selecciona.
func (s *Sidebar) crearNueva() {
        go func() {
                ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
                defer cancel()

                ses, err := s.cliente.CrearSesion(ctx, s.usuarioID, "")
                if err != nil {
                        fyne.Do(func() {
                                if s.win != nil {
                                        dialog.ShowError(err, s.win)
                                }
                        })
                        return
                }
                fyne.Do(func() {
                        s.mu.Lock()
                        // prepend
                        s.sesiones = append([]SesionChat{*ses}, s.sesiones...)
                        s.sesionActiva = ses.ID
                        s.mu.Unlock()
                        if s.app != nil {
                                s.app.Preferences().SetString("sesion_activa", ses.ID)
                        }
                        s.estadoLbl.SetText(fmt.Sprintf("%d conversaciones", len(s.sesiones)))
                        s.lista.Refresh()
                        if s.onCrear != nil {
                                s.onCrear(ses)
                        }
                        if s.onSeleccion != nil {
                                s.onSeleccion(ses.ID)
                        }
                })
        }()
}

// confirmarEliminar pide confirmación antes de cerrar la sesión.
func (s *Sidebar) confirmarEliminar(id string) {
        if s.win == nil {
                return
        }
        dialog.ShowConfirm("Eliminar conversación",
                "¿Cerrar y eliminar esta conversación? Esta acción no se puede deshacer.",
                func(ok bool) {
                        if !ok {
                                return
                        }
                        s.eliminar(id)
                }, s.win)
}

func (s *Sidebar) eliminar(id string) {
        go func() {
                ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
                defer cancel()
                err := s.cliente.CerrarSesion(ctx, id)
                fyne.Do(func() {
                        if err != nil {
                                dialog.ShowError(err, s.win)
                                return
                        }
                        s.mu.Lock()
                        defer s.mu.Unlock()
                        out := s.sesiones[:0]
                        for _, ses := range s.sesiones {
                                if ses.ID != id {
                                        out = append(out, ses)
                                }
                        }
                        s.sesiones = out
                        if s.sesionActiva == id {
                                s.sesionActiva = ""
                                if s.app != nil {
                                        s.app.Preferences().SetString("sesion_activa", "")
                                }
                                if s.onSeleccion != nil {
                                        s.onSeleccion("")
                                }
                        }
                        s.estadoLbl.SetText(fmt.Sprintf("%d conversaciones", len(s.sesiones)))
                        s.lista.Refresh()
                })
        }()
}

// SesionActiva devuelve el ID de la sesión actualmente seleccionada.
func (s *Sidebar) SesionActiva() string {
        s.mu.RLock()
        defer s.mu.RUnlock()
        return s.sesionActiva
}

// SetSesionActiva permite forzar la sesión activa desde fuera (por ejemplo
// cuando el backend crea una sesión implícitamente al enviar el primer mensaje).
func (s *Sidebar) SetSesionActiva(id string) {
        s.mu.Lock()
        s.sesionActiva = id
        s.mu.Unlock()
        if s.app != nil {
                s.app.Preferences().SetString("sesion_activa", id)
        }
        // Asegurar que exista en la lista (si no, recargar)
        go s.Recargar()
        fyne.Do(func() { s.lista.Refresh() })
}

// tituloSesion devuelve un título legible para una sesión.
func (s *Sidebar) tituloSesion(ses SesionChat) string {
        if ses.Titulo != "" {
                return ses.Titulo
        }
        if ses.Proyecto != "" {
                return "Conversación · " + ses.Proyecto
        }
        if len(ses.ID) >= 8 {
                return "Conversación " + ses.ID[:8]
        }
        return "Conversación"
}

// formatoRelativo devuelve "hace 2 min", "hace 1 h", "ayer", etc.
func formatoRelativo(t time.Time) string {
        if t.IsZero() {
                return ""
        }
        d := time.Since(t)
        switch {
        case d < time.Minute:
                return "ahora"
        case d < time.Hour:
                return fmt.Sprintf("hace %d min", int(d.Minutes()))
        case d < 24*time.Hour:
                return fmt.Sprintf("hace %d h", int(d.Hours()))
        case d < 7*24*time.Hour:
                return fmt.Sprintf("hace %d d", int(d.Hours()/24))
        }
        return t.Format("02/01/2006")
}
