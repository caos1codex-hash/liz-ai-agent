package desktop

import (
        "sync"
        "time"

        "fyne.io/fyne/v2"
        "fyne.io/fyne/v2/container"
        "fyne.io/fyne/v2/widget"
)

// ToastLevel nivel de severidad.
type ToastLevel int

const (
        // ToastInfo azul.
        ToastInfo ToastLevel = iota
        // ToastSuccess verde.
        ToastSuccess
        // ToastWarning ámbar.
        ToastWarning
        // ToastError rojo.
        ToastError
)

// Toast notificación efímera que aparece sobre el content.
// Es el equivalente desktop de web/src/components/Toast.tsx.
type Toast struct {
        mu       sync.Mutex
        popups   []*widget.PopUp
        window   fyne.Window
        duration time.Duration
}

// NewToast crea un gestor de toasts para la ventana dada.
func NewToast(w fyne.Window) *Toast {
        return &Toast{
                window:   w,
                duration: 4 * time.Second,
        }
}

// Show muestra un toast con el nivel y mensaje dados.
func (t *Toast) Show(level ToastLevel, msg string) {
        if t == nil || t.window == nil {
                return
        }
        t.mu.Lock()
        defer t.mu.Unlock()

        label := widget.NewLabel(msg)
        label.Truncation = fyne.TextTruncateEllipsis
        label.Wrapping = fyne.TextWrapWord

        var icon *widget.Icon
        switch level {
        case ToastSuccess:
                icon = widget.NewIcon(theme_iconCheck())
        case ToastWarning:
                icon = widget.NewIcon(theme_iconWarning())
        case ToastError:
                icon = widget.NewIcon(theme_iconError())
        default:
                icon = widget.NewIcon(theme_iconInfo())
        }

        content := container.NewHBox(icon, label)

        popup := widget.NewPopUp(content, t.window.Canvas())
        popup.Show()

        // Auto-dismiss
        go func() {
                time.Sleep(t.duration)
                fyne.Do(func() {
                        popup.Hide()
                        t.mu.Lock()
                        t.popups = removePopUp(t.popups, popup)
                        t.mu.Unlock()
                })
        }()

        t.popups = append(t.popups, popup)
}

// Helper: remover un popup de la lista.
func removePopUp(list []*widget.PopUp, p *widget.PopUp) []*widget.PopUp {
        out := list[:0]
        for _, x := range list {
                if x != p {
                        out = append(out, x)
                }
        }
        return out
}

// Helper: convertir (tipo string, msg string) → (ToastLevel, string).
func nivelDesdeString(tipo string) ToastLevel {
        switch tipo {
        case "info":
                return ToastInfo
        case "success":
                return ToastSuccess
        case "warning":
                return ToastWarning
        case "error":
                return ToastError
        }
        return ToastInfo
}

// ShowFromString atajo para integrar con callbacks onToast(tipo, msg).
func (t *Toast) ShowFromString(tipo, msg string) {
        t.Show(nivelDesdeString(tipo), msg)
}

// SetDuration permite ajustar cuánto permanecen visibles los toasts.
func (t *Toast) SetDuration(d time.Duration) {
        t.mu.Lock()
        defer t.mu.Unlock()
        t.duration = d
}
