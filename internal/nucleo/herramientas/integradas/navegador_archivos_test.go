package integradas

import (
        "context"
        "os"
        "path/filepath"
        "strings"
        "testing"
)

func TestNavegador_Basico(t *testing.T) {
        n := NewNavegadorArchivos()
        if n.Nombre() != "navegador_archivos" {
                t.Errorf("Nombre = %q", n.Nombre())
        }
        if err := n.Validar(); err != nil {
                t.Errorf("Validar falló: %v", err)
        }
        if len(n.Parametros()) < 4 {
                t.Errorf("muy pocos parámetros: %d", len(n.Parametros()))
        }
}

func TestNavegador_Listar(t *testing.T) {
        // Crear dir temporal con archivos
        tmp := t.TempDir()
        os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("1"), 0644)
        os.WriteFile(filepath.Join(tmp, "b.go"), []byte("2"), 0644)
        os.Mkdir(filepath.Join(tmp, "subdir"), 0755)
        os.WriteFile(filepath.Join(tmp, ".oculto"), []byte("3"), 0644)

        n := NewNavegadorArchivos()
        res, _ := n.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "listar",
                "ruta":      tmp,
        })
        if !res.Exito {
                t.Fatalf("listar falló: %+v", res)
        }
        datos := res.Datos.(ResultadoNavegador)
        // Esperamos: a.txt, b.go, subdir (no .oculto)
        if len(datos.Entradas) != 3 {
                t.Errorf("len(Entradas) = %d, esperaba 3", len(datos.Entradas))
        }
        // Directorios primero
        if datos.Entradas[0].Tipo != "directorio" {
                t.Errorf("primera entrada debería ser directorio, es %s", datos.Entradas[0].Tipo)
        }
}

func TestNavegador_ListarConPatron(t *testing.T) {
        tmp := t.TempDir()
        os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("1"), 0644)
        os.WriteFile(filepath.Join(tmp, "b.go"), []byte("2"), 0644)
        os.WriteFile(filepath.Join(tmp, "c.txt"), []byte("3"), 0644)

        n := NewNavegadorArchivos()
        res, _ := n.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "listar",
                "ruta":      tmp,
                "patron":    "*.txt",
        })
        if !res.Exito {
                t.Fatalf("falló: %+v", res)
        }
        datos := res.Datos.(ResultadoNavegador)
        if len(datos.Entradas) != 2 {
                t.Errorf("len = %d, esperaba 2", len(datos.Entradas))
        }
        for _, e := range datos.Entradas {
                if !strings.HasSuffix(e.Nombre, ".txt") {
                        t.Errorf("entrada no .txt: %s", e.Nombre)
                }
        }
}

func TestNavegador_ListarConExtensiones(t *testing.T) {
        tmp := t.TempDir()
        os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("1"), 0644)
        os.WriteFile(filepath.Join(tmp, "b.go"), []byte("2"), 0644)
        os.WriteFile(filepath.Join(tmp, "c.py"), []byte("3"), 0644)

        n := NewNavegadorArchivos()
        res, _ := n.Ejecutar(context.Background(), map[string]interface{}{
                "operacion":   "listar",
                "ruta":        tmp,
                "extensiones": []string{"go", "py"},
        })
        if !res.Exito {
                t.Fatalf("falló: %+v", res)
        }
        datos := res.Datos.(ResultadoNavegador)
        if len(datos.Entradas) != 2 {
                t.Errorf("len = %d, esperaba 2", len(datos.Entradas))
        }
}

func TestNavegador_ListarIncluirOcultos(t *testing.T) {
        tmp := t.TempDir()
        os.WriteFile(filepath.Join(tmp, ".secret"), []byte("1"), 0644)
        os.WriteFile(filepath.Join(tmp, "visible"), []byte("2"), 0644)

        n := NewNavegadorArchivos()
        // Sin incluir ocultos
        res, _ := n.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "listar",
                "ruta":      tmp,
        })
        datos := res.Datos.(ResultadoNavegador)
        if len(datos.Entradas) != 1 || datos.Entradas[0].Nombre != "visible" {
                t.Errorf("sin ocultos: %+v", datos.Entradas)
        }

        // Con incluir ocultos
        res, _ = n.Ejecutar(context.Background(), map[string]interface{}{
                "operacion":         "listar",
                "ruta":              tmp,
                "incluir_ocultos":   true,
        })
        datos = res.Datos.(ResultadoNavegador)
        if len(datos.Entradas) != 2 {
                t.Errorf("con ocultos: %+v", datos.Entradas)
        }
}

func TestNavegador_Stat(t *testing.T) {
        tmp := t.TempDir()
        ruta := filepath.Join(tmp, "archivo.txt")
        os.WriteFile(ruta, []byte("contenido de prueba"), 0644)

        n := NewNavegadorArchivos()
        res, _ := n.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "stat",
                "ruta":      ruta,
        })
        if !res.Exito {
                t.Fatalf("stat falló: %+v", res)
        }
        datos := res.Datos.(ResultadoNavegador)
        if datos.Stat == nil {
                t.Fatal("Stat nil")
        }
        if datos.Stat.Nombre != "archivo.txt" {
                t.Errorf("Nombre = %q", datos.Stat.Nombre)
        }
        if datos.Stat.Tamano != int64(len("contenido de prueba")) {
                t.Errorf("Tamano = %d, esperaba %d", datos.Stat.Tamano, len("contenido de prueba"))
        }
        if datos.Stat.Tipo != "archivo" {
                t.Errorf("Tipo = %q", datos.Stat.Tipo)
        }
}

func TestNavegador_StatDirectorio(t *testing.T) {
        tmp := t.TempDir()
        n := NewNavegadorArchivos()
        res, _ := n.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "stat",
                "ruta":      tmp,
        })
        if !res.Exito {
                t.Fatalf("falló: %+v", res)
        }
        datos := res.Datos.(ResultadoNavegador)
        if !datos.Stat.EsDir || datos.Stat.Tipo != "directorio" {
                t.Errorf("debería ser directorio: %+v", datos.Stat)
        }
}

func TestNavegador_StatNoExiste(t *testing.T) {
        n := NewNavegadorArchivos()
        res, _ := n.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "stat",
                "ruta":      "/ruta/que/no/existe/seguramente/2025",
        })
        if res.Exito {
                t.Error("debería fallar para ruta inexistente")
        }
}

func TestNavegador_Existe(t *testing.T) {
        tmp := t.TempDir()
        ruta := filepath.Join(tmp, "existe.txt")
        os.WriteFile(ruta, []byte("1"), 0644)

        n := NewNavegadorArchivos()
        // Existe
        res, _ := n.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "existe",
                "ruta":      ruta,
        })
        if !res.Exito {
                t.Fatal("existe falló")
        }
        datos := res.Datos.(ResultadoNavegador)
        if !datos.Existe {
                t.Error("Existe debería ser true")
        }

        // No existe
        res, _ = n.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "existe",
                "ruta":      filepath.Join(tmp, "no_existe"),
        })
        datos = res.Datos.(ResultadoNavegador)
        if datos.Existe {
                t.Error("Existe debería ser false")
        }
}

func TestNavegador_Arbol(t *testing.T) {
        tmp := t.TempDir()
        // Estructura:
        // tmp/
        //   a.txt
        //   subdir/
        //     b.go
        //     subsub/
        //       c.txt
        os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("1"), 0644)
        os.Mkdir(filepath.Join(tmp, "subdir"), 0755)
        os.WriteFile(filepath.Join(tmp, "subdir", "b.go"), []byte("2"), 0644)
        os.Mkdir(filepath.Join(tmp, "subdir", "subsub"), 0755)
        os.WriteFile(filepath.Join(tmp, "subdir", "subsub", "c.txt"), []byte("3"), 0644)

        n := NewNavegadorArchivos()
        // Profundidad 1 — solo hijos directos
        res, _ := n.Ejecutar(context.Background(), map[string]interface{}{
                "operacion":       "arbol",
                "ruta":            tmp,
                "profundidad_max": 1,
        })
        if !res.Exito {
                t.Fatalf("arbol falló: %+v", res)
        }
        datos := res.Datos.(ResultadoNavegador)
        // Esperamos: tmp, a.txt, subdir (3 entradas)
        if len(datos.Entradas) != 3 {
                t.Errorf("profundidad 1: len = %d, esperaba 3. Entradas: %+v",
                        len(datos.Entradas), datos.Entradas)
        }

        // Profundidad 3 — todo (incluyendo la raíz)
        // Raíz + a.txt + subdir + b.go + subsub + c.txt = 6
        res, _ = n.Ejecutar(context.Background(), map[string]interface{}{
                "operacion":       "arbol",
                "ruta":            tmp,
                "profundidad_max": 3,
        })
        datos = res.Datos.(ResultadoNavegador)
        if len(datos.Entradas) != 6 {
                t.Errorf("profundidad 3: len = %d, esperaba 6. Entradas: %+v",
                        len(datos.Entradas), datos.Entradas)
        }
}

func TestNavegador_ArbolConFiltro(t *testing.T) {
        tmp := t.TempDir()
        os.WriteFile(filepath.Join(tmp, "a.go"), []byte("1"), 0644)
        os.WriteFile(filepath.Join(tmp, "b.txt"), []byte("2"), 0644)
        os.Mkdir(filepath.Join(tmp, "sub"), 0755)
        os.WriteFile(filepath.Join(tmp, "sub", "c.go"), []byte("3"), 0644)

        n := NewNavegadorArchivos()
        res, _ := n.Ejecutar(context.Background(), map[string]interface{}{
                "operacion":       "arbol",
                "ruta":            tmp,
                "profundidad_max": 3,
                "extensiones":     []string{"go"},
        })
        if !res.Exito {
                t.Fatalf("falló: %+v", res)
        }
        datos := res.Datos.(ResultadoNavegador)
        // Esperamos: tmp, sub (dirs pasan siempre) + a.go, c.go
        nombres := []string{}
        for _, e := range datos.Entradas {
                nombres = append(nombres, e.Nombre)
        }
        // Debe contener a.go y c.go, no b.txt
        for _, n := range []string{"a.go", "c.go"} {
                encontrado := false
                for _, x := range nombres {
                        if x == n {
                                encontrado = true
                                break
                        }
                }
                if !encontrado {
                        t.Errorf("falta %s en %v", n, nombres)
                }
        }
        for _, x := range nombres {
                if x == "b.txt" {
                        t.Errorf("b.txt no debería estar: %v", nombres)
                }
        }
}

func TestNavegador_OperacionInvalida(t *testing.T) {
        n := NewNavegadorArchivos()
        res, _ := n.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "invalida",
                "ruta":      "/tmp",
        })
        if res.Exito {
                t.Error("debería fallar para operacion inválida")
        }
}

func TestNavegador_OperacionInvalidaOption(t *testing.T) {
        // operacion no está en Opciones
        n := NewNavegadorArchivos()
        res, _ := n.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "borrar", // no está en opciones
                "ruta":      "/tmp",
        })
        if res.Exito {
                t.Error("debería fallar para operacion fuera de opciones")
        }
}

func TestNavegador_Limite(t *testing.T) {
        tmp := t.TempDir()
        // Crear 20 archivos
        for i := 0; i < 20; i++ {
                os.WriteFile(filepath.Join(tmp, string(rune('a'+i%26))+string(rune('a'+(i+1)%26))+".txt"),
                        []byte("1"), 0644)
        }

        n := NewNavegadorArchivos()
        res, _ := n.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "listar",
                "ruta":      tmp,
                "limite":    5,
        })
        if !res.Exito {
                t.Fatalf("falló: %+v", res)
        }
        datos := res.Datos.(ResultadoNavegador)
        if !datos.Truncado {
                t.Error("debería estar truncado")
        }
        if len(datos.Entradas) > 5 {
                t.Errorf("len = %d, esperaba <= 5", len(datos.Entradas))
        }
}

func TestNavegador_SinRuta(t *testing.T) {
        n := NewNavegadorArchivos()
        res, _ := n.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "listar",
        })
        if res.Exito {
                t.Error("debería fallar sin ruta")
        }
}
