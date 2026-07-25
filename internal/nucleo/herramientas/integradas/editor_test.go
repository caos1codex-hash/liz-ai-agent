package integradas

import (
        "context"
        "io/ioutil"
        "os"
        "path/filepath"
        "strings"
        "testing"
)

func TestEditor_Basico(t *testing.T) {
        e := NewEditor()
        if e.Nombre() != "editor" {
                t.Errorf("Nombre = %q", e.Nombre())
        }
        if err := e.Validar(); err != nil {
                t.Errorf("Validar: %v", err)
        }
        if len(e.Parametros()) < 5 {
                t.Errorf("muy pocos parámetros: %d", len(e.Parametros()))
        }
}

func TestEditor_EscribirYLeer(t *testing.T) {
        tmp := t.TempDir()
        ruta := filepath.Join(tmp, "archivo.txt")
        e := NewEditor()

        // Escribir
        res, _ := e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion":  "escribir",
                "ruta":       ruta,
                "contenido":  "hola\nmundo\n",
        })
        if !res.Exito {
                t.Fatalf("escribir falló: %+v", res)
        }
        datos := res.Datos.(ResultadoEditor)
        if datos.Bytes != int64(len("hola\nmundo\n")) {
                t.Errorf("Bytes = %d, esperaba %d", datos.Bytes, len("hola\nmundo\n"))
        }

        // Leer
        res, _ = e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "leer",
                "ruta":      ruta,
        })
        if !res.Exito {
                t.Fatalf("leer falló: %+v", res)
        }
        datos = res.Datos.(ResultadoEditor)
        if datos.Contenido != "hola\nmundo\n" {
                t.Errorf("Contenido = %q", datos.Contenido)
        }
        if datos.Lineas != 3 { // 2 + trailing empty
                t.Errorf("Lineas = %d, esperaba 3", datos.Lineas)
        }
}

func TestEditor_LeerNoExiste(t *testing.T) {
        e := NewEditor()
        res, _ := e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "leer",
                "ruta":      "/ruta/inexistente/2025",
        })
        if res.Exito {
                t.Error("debería fallar para archivo inexistente")
        }
}

func TestEditor_LeerDirectorioError(t *testing.T) {
        tmp := t.TempDir()
        e := NewEditor()
        res, _ := e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "leer",
                "ruta":      tmp,
        })
        if res.Exito {
                t.Error("debería fallar al leer directorio con 'leer'")
        }
}

func TestEditor_EscribirCrearDirs(t *testing.T) {
        tmp := t.TempDir()
        ruta := filepath.Join(tmp, "subdir1", "subdir2", "archivo.txt")
        e := NewEditor()

        res, _ := e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion":  "escribir",
                "ruta":       ruta,
                "contenido":  "test",
                "crear_dirs": true,
        })
        if !res.Exito {
                t.Fatalf("falló: %+v", res)
        }
        if _, err := os.Stat(ruta); err != nil {
                t.Errorf("archivo no creado: %v", err)
        }
}

func TestEditor_EscribirSinCrearDirs(t *testing.T) {
        tmp := t.TempDir()
        ruta := filepath.Join(tmp, "no_existe", "archivo.txt")
        e := NewEditor()

        res, _ := e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion":  "escribir",
                "ruta":       ruta,
                "contenido":  "test",
                "crear_dirs": false,
        })
        if res.Exito {
                t.Error("debería fallar al escribir sin crear dirs")
        }
}

func TestEditor_Agregar(t *testing.T) {
        tmp := t.TempDir()
        ruta := filepath.Join(tmp, "log.txt")
        e := NewEditor()

        // Crear archivo
        e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "escribir",
                "ruta":      ruta,
                "contenido": "linea1",
        })

        // Agregar
        res, _ := e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "agregar",
                "ruta":      ruta,
                "contenido": "linea2",
        })
        if !res.Exito {
                t.Fatalf("agregar falló: %+v", res)
        }

        // Verificar
        res, _ = e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "leer",
                "ruta":      ruta,
        })
        datos := res.Datos.(ResultadoEditor)
        if datos.Contenido != "linea1\nlinea2" {
                t.Errorf("Contenido = %q, esperaba 'linea1\\nlinea2'", datos.Contenido)
        }
}

func TestEditor_AgregarArchivoNuevo(t *testing.T) {
        tmp := t.TempDir()
        ruta := filepath.Join(tmp, "nuevo.txt")
        e := NewEditor()

        res, _ := e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "agregar",
                "ruta":      ruta,
                "contenido": "primera línea",
        })
        if !res.Exito {
                t.Fatalf("falló: %+v", res)
        }
        if _, err := os.Stat(ruta); err != nil {
                t.Errorf("archivo no creado: %v", err)
        }
}

func TestEditor_Insertar(t *testing.T) {
        tmp := t.TempDir()
        ruta := filepath.Join(tmp, "archivo.txt")
        e := NewEditor()

        // Contenido inicial: A, B, C
        e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "escribir",
                "ruta":      ruta,
                "contenido": "A\nB\nC",
        })

        // Insertar X en línea 2
        res, _ := e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "insertar",
                "ruta":      ruta,
                "contenido": "X",
                "linea":     2,
        })
        if !res.Exito {
                t.Fatalf("insertar falló: %+v", res)
        }

        res, _ = e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "leer",
                "ruta":      ruta,
        })
        datos := res.Datos.(ResultadoEditor)
        esperado := "A\nX\nB\nC"
        if datos.Contenido != esperado {
                t.Errorf("Contenido = %q, esperaba %q", datos.Contenido, esperado)
        }
}

func TestEditor_ReemplazarLiteral(t *testing.T) {
        tmp := t.TempDir()
        ruta := filepath.Join(tmp, "archivo.txt")
        e := NewEditor()

        e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "escribir",
                "ruta":      ruta,
                "contenido": "foo bar foo baz foo",
        })

        // Reemplazar todas
        res, _ := e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion":      "reemplazar",
                "ruta":           ruta,
                "buscar":         "foo",
                "reemplazar_con": "XXX",
                "todas":          true,
        })
        if !res.Exito {
                t.Fatalf("falló: %+v", res)
        }
        datos := res.Datos.(ResultadoEditor)
        if datos.Reemplazos != 3 {
                t.Errorf("Reemplazos = %d, esperaba 3", datos.Reemplazos)
        }

        res, _ = e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "leer",
                "ruta":      ruta,
        })
        datos = res.Datos.(ResultadoEditor)
        if datos.Contenido != "XXX bar XXX baz XXX" {
                t.Errorf("Contenido = %q", datos.Contenido)
        }
}

func TestEditor_ReemplazarPrimeroSolo(t *testing.T) {
        tmp := t.TempDir()
        ruta := filepath.Join(tmp, "archivo.txt")
        e := NewEditor()

        e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "escribir",
                "ruta":      ruta,
                "contenido": "foo bar foo",
        })

        res, _ := e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion":      "reemplazar",
                "ruta":           ruta,
                "buscar":         "foo",
                "reemplazar_con": "X",
                "todas":          false,
        })
        if !res.Exito {
                t.Fatalf("falló: %+v", res)
        }
        if res.Datos.(ResultadoEditor).Reemplazos != 1 {
                t.Errorf("esperaba 1 reemplazo")
        }

        res, _ = e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "leer",
                "ruta":      ruta,
        })
        if res.Datos.(ResultadoEditor).Contenido != "X bar foo" {
                t.Errorf("Contenido = %q", res.Datos.(ResultadoEditor).Contenido)
        }
}

func TestEditor_ReemplazarRegex(t *testing.T) {
        tmp := t.TempDir()
        ruta := filepath.Join(tmp, "archivo.txt")
        e := NewEditor()

        e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "escribir",
                "ruta":      ruta,
                "contenido": "Hello World\nhello world",
        })

        // Case-insensitive regex
        res, _ := e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion":      "reemplazar",
                "ruta":           ruta,
                "buscar":         "(?i)hello",
                "reemplazar_con": "Hi",
                "regex":          true,
                "todas":          true,
        })
        if !res.Exito {
                t.Fatalf("falló: %+v", res)
        }
        if res.Datos.(ResultadoEditor).Reemplazos != 2 {
                t.Errorf("Reemplazos = %d, esperaba 2", res.Datos.(ResultadoEditor).Reemplazos)
        }

        res, _ = e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "leer",
                "ruta":      ruta,
        })
        esperado := "Hi World\nHi world"
        if res.Datos.(ResultadoEditor).Contenido != esperado {
                t.Errorf("Contenido = %q, esperaba %q", res.Datos.(ResultadoEditor).Contenido, esperado)
        }
}

func TestEditor_ParchearNoEncuentra(t *testing.T) {
        tmp := t.TempDir()
        ruta := filepath.Join(tmp, "archivo.txt")
        e := NewEditor()

        e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "escribir",
                "ruta":      ruta,
                "contenido": "abc",
        })

        // Parchear con patrón que no existe → debería fallar
        res, _ := e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion":      "parchear",
                "ruta":           ruta,
                "buscar":         "xyz",
                "reemplazar_con": "X",
        })
        if res.Exito {
                t.Error("parchear debería fallar cuando no encuentra el patrón")
        }
}

func TestEditor_Backup(t *testing.T) {
        tmp := t.TempDir()
        ruta := filepath.Join(tmp, "archivo.txt")
        e := NewEditor()

        // Crear original
        e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "escribir",
                "ruta":      ruta,
                "contenido": "original",
        })

        // Escribir con backup
        res, _ := e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "escribir",
                "ruta":      ruta,
                "contenido": "nuevo",
                "backup":    true,
        })
        if !res.Exito {
                t.Fatalf("falló: %+v", res)
        }
        backupPath := res.Datos.(ResultadoEditor).Backup
        if backupPath == "" {
                t.Fatal("Backup vacío")
        }
        if _, err := os.Stat(backupPath); err != nil {
                t.Errorf("backup no creado: %v", err)
        }
        // Verificar contenido del backup
        data, _ := ioutil.ReadFile(backupPath)
        if string(data) != "original" {
                t.Errorf("backup contenido = %q, esperaba 'original'", string(data))
        }
}

func TestEditor_EliminarArchivo(t *testing.T) {
        tmp := t.TempDir()
        ruta := filepath.Join(tmp, "archivo.txt")
        e := NewEditor()

        e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "escribir",
                "ruta":      ruta,
                "contenido": "test",
        })

        res, _ := e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "eliminar",
                "ruta":      ruta,
        })
        if !res.Exito {
                t.Fatalf("eliminar falló: %+v", res)
        }
        if _, err := os.Stat(ruta); !os.IsNotExist(err) {
                t.Errorf("archivo debería no existir: %v", err)
        }
}

func TestEditor_EliminarDirectorio(t *testing.T) {
        tmp := t.TempDir()
        ruta := filepath.Join(tmp, "subdir")
        e := NewEditor()

        os.Mkdir(ruta, 0755)
        os.WriteFile(filepath.Join(ruta, "a.txt"), []byte("1"), 0644)
        os.Mkdir(filepath.Join(ruta, "subsub"), 0755)
        os.WriteFile(filepath.Join(ruta, "subsub", "b.txt"), []byte("2"), 0644)

        res, _ := e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "eliminar",
                "ruta":      ruta,
        })
        if !res.Exito {
                t.Fatalf("falló: %+v", res)
        }
        if _, err := os.Stat(ruta); !os.IsNotExist(err) {
                t.Errorf("dir debería no existir: %v", err)
        }
}

func TestEditor_CrearDirectorio(t *testing.T) {
        tmp := t.TempDir()
        ruta := filepath.Join(tmp, "a", "b", "c")
        e := NewEditor()

        res, _ := e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "crear_directorio",
                "ruta":      ruta,
        })
        if !res.Exito {
                t.Fatalf("falló: %+v", res)
        }
        if info, err := os.Stat(ruta); err != nil || !info.IsDir() {
                t.Errorf("directorio no creado: %v", err)
        }
}

func TestEditor_Mover(t *testing.T) {
        tmp := t.TempDir()
        origen := filepath.Join(tmp, "origen.txt")
        destino := filepath.Join(tmp, "destino.txt")
        e := NewEditor()

        e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "escribir",
                "ruta":      origen,
                "contenido": "data",
        })

        res, _ := e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "mover",
                "ruta":      origen,
                "destino":   destino,
        })
        if !res.Exito {
                t.Fatalf("falló: %+v", res)
        }
        if _, err := os.Stat(origen); !os.IsNotExist(err) {
                t.Errorf("origen debería no existir: %v", err)
        }
        if _, err := os.Stat(destino); err != nil {
                t.Errorf("destino debería existir: %v", err)
        }
}

func TestEditor_Copiar(t *testing.T) {
        tmp := t.TempDir()
        origen := filepath.Join(tmp, "origen.txt")
        destino := filepath.Join(tmp, "destino.txt")
        e := NewEditor()

        e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "escribir",
                "ruta":      origen,
                "contenido": "data",
        })

        res, _ := e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "copiar",
                "ruta":      origen,
                "destino":   destino,
        })
        if !res.Exito {
                t.Fatalf("falló: %+v", res)
        }
        // Ambos existen
        if _, err := os.Stat(origen); err != nil {
                t.Errorf("origen debería seguir existiendo: %v", err)
        }
        if _, err := os.Stat(destino); err != nil {
                t.Errorf("destino debería existir: %v", err)
        }
}

func TestEditor_CopiarDirectorio(t *testing.T) {
        tmp := t.TempDir()
        origen := filepath.Join(tmp, "origen")
        destino := filepath.Join(tmp, "destino")
        e := NewEditor()

        os.Mkdir(origen, 0755)
        os.WriteFile(filepath.Join(origen, "a.txt"), []byte("1"), 0644)
        os.Mkdir(filepath.Join(origen, "sub"), 0755)
        os.WriteFile(filepath.Join(origen, "sub", "b.txt"), []byte("2"), 0644)

        res, _ := e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "copiar",
                "ruta":      origen,
                "destino":   destino,
        })
        if !res.Exito {
                t.Fatalf("falló: %+v", res)
        }
        // Verificar estructura
        if _, err := os.Stat(filepath.Join(destino, "a.txt")); err != nil {
                t.Errorf("destino/a.txt no copiado: %v", err)
        }
        if _, err := os.Stat(filepath.Join(destino, "sub", "b.txt")); err != nil {
                t.Errorf("destino/sub/b.txt no copiado: %v", err)
        }
}

func TestEditor_PermisoOctal(t *testing.T) {
        tmp := t.TempDir()
        ruta := filepath.Join(tmp, "ejecutable.sh")
        e := NewEditor()

        res, _ := e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "escribir",
                "ruta":      ruta,
                "contenido": "#!/bin/sh",
                "permiso":   "755",
        })
        if !res.Exito {
                t.Fatalf("falló: %+v", res)
        }
        info, _ := os.Stat(ruta)
        if info.Mode().String() != "-rwxr-xr-x" {
                t.Errorf("Permiso = %s, esperaba -rwxr-xr-x", info.Mode().String())
        }
}

func TestEditor_LeerTruncado(t *testing.T) {
        tmp := t.TempDir()
        ruta := filepath.Join(tmp, "largo.txt")
        e := NewEditor()

        // Crear archivo con muchas líneas
        contenido := strings.Repeat("linea\n", 1000)
        e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "escribir",
                "ruta":      ruta,
                "contenido": contenido,
        })

        res, _ := e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion":   "leer",
                "ruta":        ruta,
                "max_lineas":  10,
        })
        if !res.Exito {
                t.Fatalf("falló: %+v", res)
        }
        datos := res.Datos.(ResultadoEditor)
        if !datos.Truncado {
                t.Error("debería estar truncado")
        }
        if datos.Lineas > 10 {
                t.Errorf("Lineas = %d, esperaba <= 10", datos.Lineas)
        }
}

func TestEditor_OperacionInvalida(t *testing.T) {
        e := NewEditor()
        res, _ := e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "invalida",
                "ruta":      "/tmp",
        })
        if res.Exito {
                t.Error("debería fallar para operacion inválida")
        }
}

func TestEditor_SinRuta(t *testing.T) {
        e := NewEditor()
        res, _ := e.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "leer",
        })
        if res.Exito {
                t.Error("debería fallar sin ruta")
        }
}

func TestCopiarArchivo_PermisosPreservados(t *testing.T) {
        tmp := t.TempDir()
        origen := filepath.Join(tmp, "origen.txt")
        destino := filepath.Join(tmp, "destino.txt")

        os.WriteFile(origen, []byte("data"), 0600)
        if err := copiarArchivo(origen, destino); err != nil {
                t.Fatalf("copiarArchivo falló: %v", err)
        }
        info, _ := os.Stat(destino)
        if info.Mode().String() != "-rw-------" {
                t.Errorf("permisos no preservados: %s", info.Mode().String())
        }
}
