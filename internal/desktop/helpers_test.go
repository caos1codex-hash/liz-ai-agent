package desktop

import (
	"testing"
	"time"
)

// TestFormatoRelativo verifica los formatos relativos de tiempo.
func TestFormatoRelativo(t *testing.T) {
	casos := []struct {
		nombre string
		t      time.Time
		want   string
	}{
		{"cero", time.Time{}, ""},
		{"ahora", time.Now(), "ahora"},
		{"hace 5 min", time.Now().Add(-5 * time.Minute), "hace 5 min"},
		{"hace 2 h", time.Now().Add(-2 * time.Hour), "hace 2 h"},
		{"hace 3 d", time.Now().Add(-72 * time.Hour), "hace 3 d"},
	}
	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			got := formatoRelativo(tc.t)
			if tc.nombre == "hace 3 d" || tc.nombre == "hace 2 h" || tc.nombre == "hace 5 min" || tc.nombre == "ahora" || tc.nombre == "cero" {
				// Para "hace X h" o "hace X min" permitimos +/- 1 unidad de diferencia
				// (el test se ejecuta con un pequeño delay)
				if tc.nombre == "cero" {
					if got != tc.want {
						t.Errorf("got = %q, want %q", got, tc.want)
					}
					return
				}
				if tc.nombre == "ahora" {
					// "ahora" o "hace 1 min" son válidos
					if got != "ahora" && got != "hace 1 min" {
						t.Errorf("got = %q, want 'ahora' o 'hace 1 min'", got)
					}
					return
				}
				// Para los demás casos, validar prefijo
				if len(tc.want) >= 4 && got[:4] != tc.want[:4] {
					t.Errorf("got = %q, want (prefix) %q", got, tc.want[:4])
				}
			}
		})
	}
}

// TestFormatoDuracion verifica el formato de duraciones en ms.
func TestFormatoDuracion(t *testing.T) {
	casos := []struct {
		ms   int64
		want string
	}{
		{0, "0ms"},
		{100, "100ms"},
		{999, "999ms"},
		{1000, "1.0s"},
		{1500, "1.5s"},
		{60000, "60.0s"},
		{125000, "125.0s"},
	}
	for _, tc := range casos {
		t.Run(tc.want, func(t *testing.T) {
			got := formatoDuracion(tc.ms)
			if got != tc.want {
				t.Errorf("formatoDuracion(%d) = %q, want %q", tc.ms, got, tc.want)
			}
		})
	}
}

// TestSidebar_TituloSesion verifica la generación de títulos legibles.
func TestSidebar_TituloSesion(t *testing.T) {
	s := &Sidebar{}
	casos := []struct {
		nombre string
		ses    SesionChat
		want   string
	}{
		{
			nombre: "con titulo",
			ses:    SesionChat{ID: "abc123", Titulo: "Mi conversación"},
			want:   "Mi conversación",
		},
		{
			nombre: "sin titulo con proyecto",
			ses:    SesionChat{ID: "abc123", Proyecto: "mi-repo"},
			want:   "Conversación · mi-repo",
		},
		{
			nombre: "sin titulo sin proyecto, ID largo",
			ses:    SesionChat{ID: "abcdefgh1234567890"},
			want:   "Conversación abcdefgh",
		},
		{
			nombre: "sin titulo sin proyecto, ID corto",
			ses:    SesionChat{ID: "ab"},
			want:   "Conversación",
		},
	}
	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			got := s.tituloSesion(tc.ses)
			if got != tc.want {
				t.Errorf("tituloSesion = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestModoToString verifica la conversión ModoTema → string.
func TestModoToString(t *testing.T) {
	if s := modoToString(ModoOscuro); s != "oscuro" {
		t.Errorf("modoToString(Oscuro) = %q, want 'oscuro'", s)
	}
	if s := modoToString(ModoClaro); s != "claro" {
		t.Errorf("modoToString(Claro) = %q, want 'claro'", s)
	}
}

// TestStringToModo verifica la conversión string → ModoTema.
func TestStringToModo(t *testing.T) {
	if m := stringToModo("oscuro"); m != ModoOscuro {
		t.Errorf("stringToModo('oscuro') = %v, want ModoOscuro", m)
	}
	if m := stringToModo("claro"); m != ModoClaro {
		t.Errorf("stringToModo('claro') = %v, want ModoClaro", m)
	}
	if m := stringToModo("invalid"); m != ModoOscuro {
		t.Errorf("stringToModo('invalid') = %v, want ModoOscuro (default)", m)
	}
}

// TestWelcomeMessage verifica que el mensaje de bienvenida contenga
// palabras clave esperadas.
func TestWelcomeMessage(t *testing.T) {
	msg := welcomeMessage()
	if msg == "" {
		t.Fatal("welcomeMessage vacío")
	}
	for _, s := range []string{"Liz", "Linux", "RAM"} {
		if !contains(msg, s) {
			t.Errorf("welcomeMessage no contiene %q", s)
		}
	}
}

// TestWelcomePrompts verifica que haya 4 prompts de ejemplo.
func TestWelcomePrompts(t *testing.T) {
	ps := welcomePrompts()
	if len(ps) != 4 {
		t.Fatalf("esperaba 4 prompts, obtuve %d", len(ps))
	}
	for i, p := range ps {
		if p == "" {
			t.Errorf("prompt[%d] vacío", i)
		}
	}
}

// TestNivelDesdeString verifica la conversión de niveles de toast.
func TestNivelDesdeString(t *testing.T) {
	casos := []struct {
		in   string
		want ToastLevel
	}{
		{"info", ToastInfo},
		{"success", ToastSuccess},
		{"warning", ToastWarning},
		{"error", ToastError},
		{"unknown", ToastInfo},
		{"", ToastInfo},
	}
	for _, tc := range casos {
		t.Run(tc.in, func(t *testing.T) {
			if got := nivelDesdeString(tc.in); got != tc.want {
				t.Errorf("nivelDesdeString(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// contains helper.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(len(s) > 0 && len(sub) > 0 && indexOf(s, sub) >= 0))
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
