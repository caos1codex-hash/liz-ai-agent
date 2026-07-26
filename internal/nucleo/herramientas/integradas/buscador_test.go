package integradas

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuscador_Basico(t *testing.T) {
	b := NewBuscador()
	if b.Nombre() != "buscador" {
		t.Errorf("Nombre = %q", b.Nombre())
	}
	if err := b.Validar(); err != nil {
		t.Errorf("Validar: %v", err)
	}
	if len(b.Parametros()) < 5 {
		t.Errorf("muy pocos parámetros: %d", len(b.Parametros()))
	}
}

func crearArbolPrueba(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	// estructura:
	// tmp/
	//   main.go       (package main)
	//   utils.go      (helper)
	//   README.md     (docs)
	//   subdir/
	//     helper.go   (Helper func)
	//     test.txt
	os.WriteFile(filepath.Join(tmp, "main.go"),
		[]byte("package main\n\nfunc main() {\n\tHelper()\n}\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "utils.go"),
		[]byte("package main\n\n// Helper function\nfunc Helper() {}\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "README.md"),
		[]byte("# Proyecto\n\nUsa Helper() para cosas.\n"), 0644)
	os.Mkdir(filepath.Join(tmp, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmp, "subdir", "helper.go"),
		[]byte("package subdir\n\n// Helper additional\nfunc Helper() {}\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "subdir", "test.txt"),
		[]byte("test line\nhelper here\n"), 0644)
	return tmp
}

func TestBuscador_ArchivosPorPatron(t *testing.T) {
	tmp := crearArbolPrueba(t)
	b := NewBuscador()

	res, _ := b.Ejecutar(context.Background(), map[string]interface{}{
		"operacion": "archivos",
		"ruta":      tmp,
		"patron":    "*.go",
	})
	if !res.Exito {
		t.Fatalf("falló: %+v", res)
	}
	datos := res.Datos.(ResultadoBuscador)
	if datos.TotalArchivos != 3 {
		t.Errorf("TotalArchivos = %d, esperaba 3 (main, utils, subdir/helper)", datos.TotalArchivos)
	}
	// Verificar que todos son .go
	for _, a := range datos.Archivos {
		if !strings.HasSuffix(a.Nombre, ".go") {
			t.Errorf("archivo sin extensión .go: %s", a.Nombre)
		}
	}
}

func TestBuscador_ArchivosPorExtension(t *testing.T) {
	tmp := crearArbolPrueba(t)
	b := NewBuscador()

	res, _ := b.Ejecutar(context.Background(), map[string]interface{}{
		"operacion":   "archivos",
		"ruta":        tmp,
		"extensiones": []string{"go", "md"},
	})
	if !res.Exito {
		t.Fatalf("falló: %+v", res)
	}
	datos := res.Datos.(ResultadoBuscador)
	if datos.TotalArchivos != 4 {
		t.Errorf("TotalArchivos = %d, esperaba 4", datos.TotalArchivos)
	}
}

func TestBuscador_ArchivosPorTamano(t *testing.T) {
	tmp := crearArbolPrueba(t)
	b := NewBuscador()

	// tamano_min alto: ningún archivo pasa
	res, _ := b.Ejecutar(context.Background(), map[string]interface{}{
		"operacion":  "archivos",
		"ruta":       tmp,
		"tamano_min": 1000000,
	})
	datos := res.Datos.(ResultadoBuscador)
	if datos.TotalArchivos != 0 {
		t.Errorf("esperaba 0 archivos con tamano_min 1MB, obtuve %d", datos.TotalArchivos)
	}

	// tamano_min bajo: todos pasan
	res, _ = b.Ejecutar(context.Background(), map[string]interface{}{
		"operacion":  "archivos",
		"ruta":       tmp,
		"tamano_min": 1,
	})
	datos = res.Datos.(ResultadoBuscador)
	if datos.TotalArchivos != 5 {
		t.Errorf("esperaba 5 archivos con tamano_min 1, obtuve %d", datos.TotalArchivos)
	}
}

func TestBuscador_ContenidoSimple(t *testing.T) {
	tmp := crearArbolPrueba(t)
	b := NewBuscador()

	res, _ := b.Ejecutar(context.Background(), map[string]interface{}{
		"operacion":    "contenido",
		"ruta":         tmp,
		"texto":        "Helper",
		"ignorar_case": false,
	})
	if !res.Exito {
		t.Fatalf("falló: %+v", res)
	}
	datos := res.Datos.(ResultadoBuscador)
	// "Helper" aparece en:
	// - main.go (1 vez: Helper())
	// - utils.go (2 veces: comentario + firma)
	// - README.md (1 vez: Helper())
	// - subdir/helper.go (2 veces: comentario + firma)
	// Total: 6 matches
	if datos.TotalMatches == 0 {
		t.Errorf("esperaba al menos 1 match, obtuve 0")
	}
	// Verificar que todos los matches contienen "Helper"
	for _, m := range datos.Matches {
		if !strings.Contains(m.Contenido, "Helper") {
			t.Errorf("match no contiene 'Helper': %+v", m)
		}
	}
}

func TestBuscador_ContenidoCaseInsensitive(t *testing.T) {
	tmp := crearArbolPrueba(t)
	b := NewBuscador()

	// Case insensitive: "helper" debería encontrar "Helper"
	res, _ := b.Ejecutar(context.Background(), map[string]interface{}{
		"operacion":    "contenido",
		"ruta":         tmp,
		"texto":        "helper",
		"ignorar_case": true,
	})
	datos := res.Datos.(ResultadoBuscador)
	if datos.TotalMatches == 0 {
		t.Errorf("case insensitive debería encontrar matches")
	}
}

func TestBuscador_ContenidoCaseSensitive(t *testing.T) {
	tmp := crearArbolPrueba(t)
	b := NewBuscador()

	// Case sensitive: "helper" (minúsculas) NO debería encontrar "Helper"
	res, _ := b.Ejecutar(context.Background(), map[string]interface{}{
		"operacion":    "contenido",
		"ruta":         tmp,
		"texto":        "helper",
		"ignorar_case": false,
	})
	datos := res.Datos.(ResultadoBuscador)
	// "helper" en minúsculas solo aparece en subdir/test.txt ("helper here")
	if datos.TotalMatches != 1 {
		t.Errorf("case sensitive: esperaba 1 match, obtuve %d", datos.TotalMatches)
	}
}

func TestBuscador_ContenidoRegex(t *testing.T) {
	tmp := crearArbolPrueba(t)
	b := NewBuscador()

	// Regex: "func\s+\w+" encuentra declaraciones de funciones
	res, _ := b.Ejecutar(context.Background(), map[string]interface{}{
		"operacion":    "contenido",
		"ruta":         tmp,
		"texto":        "func\\s+\\w+",
		"regex":        true,
		"ignorar_case": false,
		"extensiones":  []string{"go"},
	})
	if !res.Exito {
		t.Fatalf("falló: %+v", res)
	}
	datos := res.Datos.(ResultadoBuscador)
	// main.go: func main()
	// utils.go: func Helper()
	// subdir/helper.go: func Helper()
	if datos.TotalMatches != 3 {
		t.Errorf("esperaba 3 matches (funciones), obtuve %d", datos.TotalMatches)
	}
}

func TestBuscador_Combinado(t *testing.T) {
	tmp := crearArbolPrueba(t)
	b := NewBuscador()

	// Buscar "Helper" solo en archivos .go
	res, _ := b.Ejecutar(context.Background(), map[string]interface{}{
		"operacion":   "combinado",
		"ruta":        tmp,
		"texto":       "Helper",
		"extensiones": []string{"go"},
	})
	if !res.Exito {
		t.Fatalf("falló: %+v", res)
	}
	datos := res.Datos.(ResultadoBuscador)
	// main.go: 1 (Helper())
	// utils.go: 2 (comentario + firma)
	// subdir/helper.go: 2 (comentario + firma)
	// Total: 5
	if datos.TotalMatches != 5 {
		t.Errorf("esperaba 5 matches, obtuve %d", datos.TotalMatches)
	}
	// Ningún match en README.md
	for _, m := range datos.Matches {
		if strings.HasSuffix(m.Archivo, ".md") {
			t.Errorf("combinado no debería incluir .md: %s", m.Archivo)
		}
	}
}

func TestBuscador_Contexto(t *testing.T) {
	tmp := crearArbolPrueba(t)
	b := NewBuscador()

	res, _ := b.Ejecutar(context.Background(), map[string]interface{}{
		"operacion":       "contenido",
		"ruta":            tmp,
		"texto":           "func main()",
		"contexto_lineas": 2,
		"extensiones":     []string{"go"},
	})
	if !res.Exito {
		t.Fatalf("falló: %+v", res)
	}
	datos := res.Datos.(ResultadoBuscador)
	if datos.TotalMatches == 0 {
		t.Fatal("no encontró 'func main()'")
	}
	m := datos.Matches[0]
	if len(m.Contexto) == 0 {
		t.Errorf("Contexto vacío cuando contexto_lineas=2")
	}
}

func TestBuscador_ModificadoDesde(t *testing.T) {
	tmp := crearArbolPrueba(t)
	b := NewBuscador()

	// modificado_desde en el futuro: ningún archivo pasa
	res, _ := b.Ejecutar(context.Background(), map[string]interface{}{
		"operacion":        "archivos",
		"ruta":             tmp,
		"modificado_desde": time.Now().Add(1 * time.Hour).Format(time.RFC3339),
	})
	datos := res.Datos.(ResultadoBuscador)
	if datos.TotalArchivos != 0 {
		t.Errorf("futuro: esperaba 0, obtuve %d", datos.TotalArchivos)
	}

	// modificado_desde en el pasado: todos pasan
	res, _ = b.Ejecutar(context.Background(), map[string]interface{}{
		"operacion":        "archivos",
		"ruta":             tmp,
		"modificado_desde": "1h",
	})
	datos = res.Datos.(ResultadoBuscador)
	if datos.TotalArchivos == 0 {
		t.Errorf("pasado: esperaba >0, obtuve 0")
	}
}

func TestBuscador_Limite(t *testing.T) {
	tmp := t.TempDir()
	// Crear 50 archivos
	for i := 0; i < 50; i++ {
		os.WriteFile(filepath.Join(tmp, string(rune('a'+i%26))+string(rune('a'+(i+1)%26))+".txt"),
			[]byte("test"), 0644)
	}
	b := NewBuscador()

	res, _ := b.Ejecutar(context.Background(), map[string]interface{}{
		"operacion": "archivos",
		"ruta":      tmp,
		"limite":    10,
	})
	if !res.Exito {
		t.Fatalf("falló: %+v", res)
	}
	datos := res.Datos.(ResultadoBuscador)
	if !datos.Truncado {
		t.Error("debería estar truncado")
	}
	if datos.TotalArchivos > 10 {
		t.Errorf("TotalArchivos = %d, esperaba <= 10", datos.TotalArchivos)
	}
}

func TestBuscador_OperacionInvalida(t *testing.T) {
	b := NewBuscador()
	res, _ := b.Ejecutar(context.Background(), map[string]interface{}{
		"operacion": "invalida",
		"ruta":      "/tmp",
	})
	if res.Exito {
		t.Error("debería fallar para operacion inválida")
	}
}

func TestBuscador_ContenidoSinTexto(t *testing.T) {
	b := NewBuscador()
	res, _ := b.Ejecutar(context.Background(), map[string]interface{}{
		"operacion": "contenido",
		"ruta":      "/tmp",
	})
	if res.Exito {
		t.Error("debería fallar sin texto para operacion contenido")
	}
}

func TestBuscador_SinResultados(t *testing.T) {
	tmp := crearArbolPrueba(t)
	b := NewBuscador()

	res, _ := b.Ejecutar(context.Background(), map[string]interface{}{
		"operacion": "contenido",
		"ruta":      tmp,
		"texto":     "zzz_no_existe_zzz",
	})
	if !res.Exito {
		t.Fatalf("falló: %+v", res)
	}
	datos := res.Datos.(ResultadoBuscador)
	if datos.TotalMatches != 0 {
		t.Errorf("esperaba 0 matches, obtuve %d", datos.TotalMatches)
	}
}

func TestBuscador_BinarioSaltado(t *testing.T) {
	tmp := t.TempDir()
	// Crear un .png con "texto" dentro (no se debería buscar)
	os.WriteFile(filepath.Join(tmp, "imagen.png"), []byte("func main()"), 0644)
	os.WriteFile(filepath.Join(tmp, "code.go"), []byte("func main()"), 0644)

	b := NewBuscador()
	res, _ := b.Ejecutar(context.Background(), map[string]interface{}{
		"operacion": "contenido",
		"ruta":      tmp,
		"texto":     "func main",
	})
	if !res.Exito {
		t.Fatalf("falló: %+v", res)
	}
	datos := res.Datos.(ResultadoBuscador)
	// Solo .go debería tener match, no .png
	if datos.TotalMatches != 1 {
		t.Errorf("esperaba 1 match (solo .go), obtuve %d", datos.TotalMatches)
	}
	if datos.TotalMatches > 0 && !strings.HasSuffix(datos.Matches[0].Archivo, ".go") {
		t.Errorf("match en archivo no-.go: %s", datos.Matches[0].Archivo)
	}
}

func TestParsearFechaRelativa(t *testing.T) {
	casos := []string{"hoy", "1h", "24h", "7d", time.Now().Format(time.RFC3339)}
	for _, c := range casos {
		tm, err := parsearFechaRelativa(c)
		if err != nil {
			t.Errorf("parsearFechaRelativa(%q) error: %v", c, err)
		}
		if tm.IsZero() {
			t.Errorf("parsearFechaRelativa(%q) retornó zero time", c)
		}
	}

	// Inválido
	if _, err := parsearFechaRelativa("invalido"); err == nil {
		t.Error("esperaba error para 'invalido'")
	}
}
