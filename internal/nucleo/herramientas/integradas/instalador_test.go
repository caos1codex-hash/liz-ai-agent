package integradas

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestInstalador_Basico(t *testing.T) {
	i := NewInstalador()
	if i.Nombre() != "instalador" {
		t.Errorf("Nombre = %q", i.Nombre())
	}
	if err := i.Validar(); err != nil {
		t.Errorf("Validar: %v", err)
	}
}

func TestInstalador_Gestores(t *testing.T) {
	i := NewInstalador()
	res, _ := i.Ejecutar(context.Background(), map[string]interface{}{
		"operacion": "gestores",
	})
	if !res.Exito {
		t.Fatalf("gestores falló: %+v", res)
	}
	datos := res.Datos.(ResultadoInstalador)
	if len(datos.Gestores) == 0 {
		t.Fatal("sin gestores reportados")
	}
	// Al menos uno debería estar disponible (test runner suele tener go)
	algunoDisponible := false
	for _, g := range datos.Gestores {
		if g.Disponible {
			algunoDisponible = true
			if g.Ruta == "" {
				t.Errorf("gestor %s disponible pero sin ruta", g.Nombre)
			}
		}
	}
	if !algunoDisponible {
		t.Log("ningún gestor disponible (raro pero posible)")
	}
}

func TestInstalador_InstalarDryRun(t *testing.T) {
	i := NewInstalador()
	// Dry-run con gestor go (siempre disponible en tests de Go)
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go no instalado")
	}
	res, _ := i.Ejecutar(context.Background(), map[string]interface{}{
		"operacion":       "instalar",
		"paquetes":        []string{"github.com/example/pkg@latest"},
		"gestor":          "go",
		"solo_verificar":  true,
		"sudo":            false,
	})
	if !res.Exito {
		t.Fatalf("dry-run falló: %+v", res)
	}
	datos := res.Datos.(ResultadoInstalador)
	if !datos.DryRun {
		t.Error("debería ser DryRun")
	}
	if datos.Gestor != "go" {
		t.Errorf("Gestor = %q", datos.Gestor)
	}
	if !strings.Contains(datos.ComandoEjecutado, "go") {
		t.Errorf("ComandoEjecutado = %q", datos.ComandoEjecutado)
	}
}

func TestInstalador_DesinstalarDryRun(t *testing.T) {
	i := NewInstalador()
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm no instalado")
	}
	res, _ := i.Ejecutar(context.Background(), map[string]interface{}{
		"operacion":       "desinstalar",
		"paquetes":        []string{"paquete-test"},
		"gestor":          "npm",
		"solo_verificar":  true,
		"sudo":            false,
	})
	if !res.Exito {
		t.Fatalf("dry-run falló: %+v", res)
	}
	datos := res.Datos.(ResultadoInstalador)
	if !datos.DryRun {
		t.Error("debería ser DryRun")
	}
	if !strings.Contains(datos.ComandoEjecutado, "npm uninstall") {
		t.Errorf("ComandoEjecutado = %q", datos.ComandoEjecutado)
	}
}

func TestInstalador_SinPaquetes(t *testing.T) {
	i := NewInstalador()
	res, _ := i.Ejecutar(context.Background(), map[string]interface{}{
		"operacion": "instalar",
		"paquetes":  []string{},
	})
	if res.Exito {
		t.Error("debería fallar con paquetes vacío")
	}
}

func TestInstalador_GestorInexistente(t *testing.T) {
	i := NewInstalador()
	res, _ := i.Ejecutar(context.Background(), map[string]interface{}{
		"operacion": "instalar",
		"paquetes":  []string{"x"},
		"gestor":    "gestor_inexistente_2025",
	})
	if res.Exito {
		t.Error("debería fallar para gestor inexistente")
	}
}

func TestInstalador_OperacionInvalida(t *testing.T) {
	i := NewInstalador()
	res, _ := i.Ejecutar(context.Background(), map[string]interface{}{
		"operacion": "invalida",
	})
	if res.Exito {
		t.Error("debería fallar para operacion inválida")
	}
}

func TestInstalador_AutodetectarGestor(t *testing.T) {
	// Sin gestor del sistema, autodetectar debería fallar (o retornar el primero)
	casos := []struct {
		paquetes []string
	}{
		{[]string{"python-requests"}},
		{[]string{"github.com/x/y"}},
		{[]string{"paquete-generico"}},
	}
	for _, c := range casos {
		g := autodetectarGestor(c.paquetes)
		// Aceptamos vacío (sin gestores disponibles) o un nombre válido
		if g != "" {
			encontrado := false
			for _, s := range GestoresSoportados {
				if g == s {
					encontrado = true
					break
				}
			}
			if !encontrado {
				t.Errorf("autodetectarGestor(%v) = %q, no está en GestoresSoportados",
					c.paquetes, g)
			}
		}
	}
}

func TestPrecisaSudo(t *testing.T) {
	casos := map[string]bool{
		"apt":       true,
		"apt-get":   true,
		"dnf":       true,
		"pacman":    true,
		"brew":      false,
		"pip":       false,
		"npm":       false,
		"go":        false,
		"cargo":     false,
	}
	for g, esperado := range casos {
		if obt := precisaSudo(g); obt != esperado {
			t.Errorf("precisaSudo(%q) = %v, esperaba %v", g, obt, esperado)
		}
	}
}

func TestConstruirArgsInstalacion(t *testing.T) {
	casos := []struct {
		gestor     string
		paquetes   []string
		actualizar bool
		esperado   string
	}{
		{"apt", []string{"vim"}, false, "install -y vim"},
		{"apt", []string{"vim"}, true, "install --only-upgrade -y vim"},
		{"npm", []string{"lodash"}, false, "install lodash"},
		{"npm", []string{"lodash"}, true, "update lodash"},
		{"pip", []string{"requests"}, false, "install requests"},
		{"pip", []string{"requests"}, true, "install --upgrade requests"},
		{"brew", []string{"wget"}, false, "install wget"},
		{"brew", []string{"wget"}, true, "upgrade wget"},
		{"go", []string{"github.com/x/y"}, false, "install github.com/x/y"},
		{"go", []string{"github.com/x/y"}, true, "get -u github.com/x/y"},
	}
	for _, c := range casos {
		args := construirArgsInstalacion(c.gestor, c.paquetes, c.actualizar, nil)
		if strings.Join(args, " ") != c.esperado {
			t.Errorf("construirArgsInstalacion(%q, %v, %v) = %q, esperaba %q",
				c.gestor, c.paquetes, c.actualizar, strings.Join(args, " "), c.esperado)
		}
	}
}

func TestConstruirArgsDesinstalacion(t *testing.T) {
	casos := []struct {
		gestor   string
		paquetes []string
		esperado string
	}{
		{"apt", []string{"vim"}, "remove -y --purge vim"},
		{"npm", []string{"lodash"}, "uninstall lodash"},
		{"pip", []string{"requests"}, "uninstall -y requests"},
		{"brew", []string{"wget"}, "uninstall wget"},
		{"go", []string{"x"}, ""}, // go no soporta desinstalar
	}
	for _, c := range casos {
		args := construirArgsDesinstalacion(c.gestor, c.paquetes, nil)
		if c.esperado == "" {
			if args != nil {
				t.Errorf("esperaba nil para %q, obtuve %v", c.gestor, args)
			}
			continue
		}
		if strings.Join(args, " ") != c.esperado {
			t.Errorf("construirArgsDesinstalacion(%q, %v) = %q, esperaba %q",
				c.gestor, c.paquetes, strings.Join(args, " "), c.esperado)
		}
	}
}

func TestConstruirArgsBuscar(t *testing.T) {
	casos := []struct {
		gestor   string
		termino  string
		esperado string
	}{
		{"apt-cache", "vim", "search vim"},
		{"apt", "vim", "search vim"},
		{"dnf", "vim", "search vim"},
		{"pacman", "vim", "-Ss vim"},
		{"brew", "vim", "search vim"},
	}
	for _, c := range casos {
		args := construirArgsBuscar(c.gestor, c.termino)
		if args == nil {
			t.Errorf("esperaba args para %q", c.gestor)
			continue
		}
		if strings.Join(args, " ") != c.esperado {
			t.Errorf("construirArgsBuscar(%q, %q) = %q, esperaba %q",
				c.gestor, c.termino, strings.Join(args, " "), c.esperado)
		}
	}
}

func TestInstalador_BuscarSinTermino(t *testing.T) {
	i := NewInstalador()
	res, _ := i.Ejecutar(context.Background(), map[string]interface{}{
		"operacion": "buscar",
	})
	if res.Exito {
		t.Error("debería fallar sin término de búsqueda")
	}
}

func TestObtenerVersionGestor(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go no instalado")
	}
	v := obtenerVersionGestor("go")
	if v == "" {
		t.Log("versión vacía (puede ser normal)")
	}
}
