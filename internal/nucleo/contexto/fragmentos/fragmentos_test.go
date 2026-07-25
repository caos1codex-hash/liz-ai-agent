package fragmentos

import (
        "encoding/json"
        "os"
        "path/filepath"
        "strings"
        "testing"
)

func crearAlmacenTest(t *testing.T) (*Almacen, string) {
        t.Helper()
        tmpDir := t.TempDir()
        
        almacen, err := NuevoAlmacen(tmpDir, "proyecto_test")
        if err != nil {
                t.Fatalf("error creando almacén: %v", err)
        }
        
        return almacen, tmpDir
}

func TestAgregar_Basico(t *testing.T) {
        a, _ := crearAlmacenTest(t)

        contenido := `package main

func main() {}
`

        id, err := a.Agregar("main.go", contenido, "completo", "go", 1, 3)
        if err != nil {
                t.Fatalf("Agregar() error: %v", err)
        }

        if id == "" {
                t.Error("ID no debería estar vacío")
        }

        // Verificar que se persistió
        frag, err := a.Obtener(id)
        if err != nil {
                t.Fatalf("Obtener() error: %v", err)
        }

        if frag.Ruta != "main.go" {
                t.Errorf("ruta esperada 'main.go', obtuve '%s'", frag.Ruta)
        }
        if frag.Tipo != "completo" {
                t.Errorf("tipo esperado 'completo', obtuve '%s'", frag.Tipo)
        }
        if frag.Contenido != contenido {
                t.Error("contenido no coincide")
        }
        if frag.LineaIni != 1 || frag.LineaFin != 3 {
                t.Errorf("líneas incorrectas: %d-%d", frag.LineaIni, frag.LineaFin)
        }
}

func TestAgregar_NoDuplica(t *testing.T) {
        a, _ := crearAlmacenTest(t)

        contenido := "mismo contenido"

        id1, _ := a.Agregar("file.go", contenido, "completo", "go", 1, 1)
        id2, _ := a.Agregar("file.go", contenido, "completo", "go", 1, 1)

        if id1 != id2 {
                t.Error("mismo contenido debería generar mismo ID")
        }

        if a.Total() != 1 {
                t.Errorf("debería haber solo 1 fragmento, hay %d", a.Total())
        }
}

func TestObtener_NoExistente(t *testing.T) {
        a, _ := crearAlmacenTest(t)

        _, err := a.Obtener("no_existe_id")
        if err == nil {
                t.Error("debería retornar error para ID inexistente")
        }
}

func TestObtenerPorRuta(t *testing.T) {
        a, _ := crearAlmacenTest(t)

        a.Agregar("a.go", "contenido a", "completo", "go", 1, 1)
        a.Agregar("b.go", "contenido b", "completo", "go", 1, 1)
        a.Agregar("a.go", "contenido a2", "funcion", "go", 3, 5)

        frags, err := a.ObtenerPorRuta("a.go")
        if err != nil {
                t.Fatalf("ObtenerPorRuta() error: %v", err)
        }

        if len(frags) != 2 {
                t.Errorf("se esperaban 2 fragmentos para 'a.go', obtuve %d", len(frags))
        }

        // Verificar orden por línea de inicio
        if frags[0].LineaIni > frags[1].LineaIni {
                t.Error("fragmentos deberían estar ordenados por línea de inicio")
        }
}

func TestAgregarArchivoCompleto(t *testing.T) {
        a, _ := crearAlmacenTest(t)

        contenido := "línea1\nlínea2\nlínea3\n"

        id, err := a.AgregarArchivoCompleto("test.go", contenido, "go")
        if err != nil {
                t.Fatalf("AgregarArchivoCompleto() error: %v", err)
        }

        frag, _ := a.Obtener(id)
        if frag.Tipo != "completo" {
                t.Errorf("tipo debería ser 'completo', obtuve '%s'", frag.Tipo)
        }
        if frag.LineaIni != 1 || frag.LineaFin != 3 {
                t.Errorf("líneas incorrectas: %d-%d", frag.LineaIni, frag.LineaFin)
        }
}

func TestAgregarDesdeArchivo(t *testing.T) {
        a, _ := crearAlmacenTest(t)

        // Crear archivo temporal
        ruta := filepath.Join(t.TempDir(), "test.go")
        contenido := `package test

func Hola() string {
        return "hola"
}
`
        os.WriteFile(ruta, []byte(contenido), 0644)

        ids, err := a.AgregarDesdeArchivo("test.go", ruta)
        if err != nil {
                t.Fatalf("AgregarDesdeArchivo() error: %v", err)
        }

        if len(ids) == 0 {
                t.Error("debería haber generado al menos un fragmento")
        }
}

func TestListar(t *testing.T) {
        a, _ := crearAlmacenTest(t)

        a.Agregar("a.go", "contenido a", "completo", "go", 1, 5)
        a.Agregar("b.py", "contenido b", "completo", "python", 1, 3)

        lista, err := a.Listar()
        if err != nil {
                t.Fatalf("Listar() error: %v", err)
        }

        if len(lista) != 2 {
                t.Errorf("se esperaban 2 fragmentos, obtuve %d", len(lista))
        }

        // La lista no debería incluir el contenido completo
        for _, frag := range lista {
                if frag.Contenido != "" {
                        t.Error("la lista no debería incluir contenido completo")
                }
        }
}

func TestTotal(t *testing.T) {
        a, _ := crearAlmacenTest(t)

        if a.Total() != 0 {
                t.Errorf("almacén vacío debería tener 0 fragmentos, tiene %d", a.Total())
        }

        a.Agregar("a.go", "cont1", "completo", "go", 1, 1)
        a.Agregar("b.go", "cont2", "completo", "go", 1, 1)
        a.Agregar("c.go", "cont3", "completo", "go", 1, 1)

        if a.Total() != 3 {
                t.Errorf("se esperaban 3 fragmentos, tiene %d", a.Total())
        }
}

func TestEliminarPorRuta(t *testing.T) {
        a, _ := crearAlmacenTest(t)

        a.Agregar("a.go", "cont a", "completo", "go", 1, 1)
        a.Agregar("b.go", "cont b", "completo", "go", 1, 1)
        a.Agregar("a.go", "cont a2", "funcion", "go", 2, 5)

        eliminados, err := a.EliminarPorRuta("a.go")
        if err != nil {
                t.Fatalf("EliminarPorRuta() error: %v", err)
        }

        if len(eliminados) != 2 {
                t.Errorf("se esperaban 2 eliminados, obtuve %d", len(eliminados))
        }

        if a.Total() != 1 {
                t.Errorf("debería quedar 1 fragmento, hay %d", a.Total())
        }
}

func TestDirectorio(t *testing.T) {
        a, tmpDir := crearAlmacenTest(t)

        esperado := filepath.Join(tmpDir, "proyecto_test", ".liz", "archivos")
        if a.Directorio() != esperado {
                t.Errorf("directorio incorrecto: %s", a.Directorio())
        }
}

func TestGenerarResumen(t *testing.T) {
        tests := []struct{
                contenido string
                tipo     string
        }{
                {"package main\n\nfunc HolaMundo() {}", "funcion"},
                {"# Comentario\nvar x = 1", "var"},
                {"solo texto plano", "completo"},
        }

        for _, tt := range tests {
                resultado := generarResumen(tt.contenido, tt.tipo)
                if resultado == "" {
                        t.Errorf("generarResumen(%q, %q) no debería estar vacío", tt.contenido, tt.tipo)
                }
        }
}

func TestDetectarLenguajeExt(t *testing.T) {
        tests := []struct{
                ext      string
                esperado string
        }{
        {".go", "go"},
        {".py", "python"},
        {".ts", "typescript"},
        {".yaml", "yaml"},
        {".json", "json"},
        {".md", "markdown"},
        {".html", "html"},
        {".rs", "rust"},
        {".toml", "toml"},
        {".sh", "shell"},
        {".unknown", ""},
        }

        for _, tt := range tests {
                resultado := detectarLenguajeExt(tt.ext)
                if resultado != tt.esperado {
                t.Errorf("detectarLenguajeExt(%q) = %q, esperado %q",
                                tt.ext, resultado, tt.esperado)
                }
        }
}

func TestPersistencia_FormatoJSON(t *testing.T) {
        a, _ := crearAlmacenTest(t)

        id, _ := a.Agregar("test.go", "package test\n", "completo", "go", 1, 1)

        // Verificar que el archivo JSON es válido
        rutaFrag := filepath.Join(a.Directorio(), id+".json")
        datos, err := os.ReadFile(rutaFrag)
        if err != nil {
                t.Fatalf("error leyendo archivo de fragmento: %v", err)
        }

        var frag Fragmento
        if err := json.Unmarshal(datos, &frag); err != nil {
                t.Fatalf("JSON inválido: %v", err)
        }

        if frag.ID != id {
                t.Error("ID en archivo no coincide")
        }
        if frag.Timestamp == "" {
                t.Error("timestamp no debería estar vacío")
        }
}
// ============================================================================
// Tests de fragmentadores inteligentes (Fase 3)
// ============================================================================

func TestFragmentarGo_FuncionesSimples(t *testing.T) {
        // Bug #2: el código viejo nunca fragmentaba funciones que terminaban en "{"
        contenido := `package main

import "fmt"

func main() {
        fmt.Println("hola")
}

func Saludar(nombre string) string {
        return "hola " + nombre
}

type Usuario struct {
        Nombre string
        Edad   int
}
`
        frags := fragmentarGo(contenido)
        if len(frags) < 3 {
                t.Fatalf("debería generar al menos 3 fragmentos (import, main, Saludar, type), got %d", len(frags))
        }

        // Verificar que cada función es su propio fragmento
        tipos := make(map[string]int)
        for _, f := range frags {
                tipos[f.tipo]++
        }
        if tipos["funcion"] < 2 {
                t.Errorf("debería haber al menos 2 fragmentos de tipo 'funcion', got %d", tipos["funcion"])
        }
        if tipos["estructura"] < 1 {
                t.Errorf("debería haber al menos 1 fragmento de tipo 'estructura', got %d", tipos["estructura"])
        }
}

func TestFragmentarGo_FuncionMultilinea(t *testing.T) {
        // Verifica que las funciones con firma multi-línea se fragmentan correctamente
        contenido := `package main

func FuncionConFirmaLarga(
        a int,
        b string,
        c float64,
) (int, error) {
        return a, nil
}
`
        frags := fragmentarGo(contenido)
        if len(frags) < 1 {
                t.Fatalf("debería generar al menos 1 fragmento, got %d", len(frags))
        }
        if frags[0].tipo != "funcion" {
                t.Errorf("primer fragmento debería ser 'funcion', got %s", frags[0].tipo)
        }
        // El fragmento debe contener toda la función incluyendo el cuerpo
        if !strings.Contains(frags[0].contenido, "return a, nil") {
                t.Error("el fragmento de función debería contener el cuerpo completo")
        }
}

func TestFragmentarGo_LineasValidas(t *testing.T) {
        contenido := `package main

func A() {}

func B() {}
`
        frags := fragmentarGo(contenido)
        if len(frags) < 2 {
                t.Fatalf("debería generar al menos 2 fragmentos, got %d", len(frags))
        }

        // Verificar que lineaIni <= lineaFin y todas son >= 1
        for _, f := range frags {
                if f.lineaIni < 1 {
                        t.Errorf("lineaIni debe ser >= 1, got %d", f.lineaIni)
                }
                if f.lineaFin < f.lineaIni {
                        t.Errorf("lineaFin (%d) debe ser >= lineaIni (%d)", f.lineaFin, f.lineaIni)
                }
        }
}

func TestFragmentarPython(t *testing.T) {
        contenido := `import os
import sys

class Utilidades:
    def __init__(self):
        self.x = 1

    def saludar(self):
        return "hola"

def main():
    print("main")

def _privado():
    pass
`
        frags := fragmentarPython(contenido)
        // La clase contiene a sus métodos (la fragmentación top-level no separa
        // métodos del cuerpo de la clase). Así esperamos: 1 clase + 2 funciones.
        if len(frags) < 3 {
                t.Fatalf("debería generar al menos 3 fragmentos (1 clase + 2 funciones), got %d", len(frags))
        }

        // Verificar que se detectan clases y funciones
        hayClase := false
        hayFunc := false
        for _, f := range frags {
                if f.tipo == "estructura" {
                        hayClase = true
                }
                if f.tipo == "funcion" {
                        hayFunc = true
                }
        }
        if !hayClase {
                t.Error("debería detectar al menos una clase (estructura)")
        }
        if !hayFunc {
                t.Error("debería detectar al menos una función")
        }
}

func TestFragmentarJS(t *testing.T) {
        contenido := `import { foo } from './foo.js';

function saludar() {
  return "hola";
}

const arrow = () => 42;

class MiClase {
  constructor() {
    this.x = 1;
  }
}

export default function main() {
  console.log("main");
}
`
        frags := fragmentarJS(contenido)
        if len(frags) < 3 {
                t.Fatalf("debería generar al menos 3 fragmentos (function, class, export default), got %d", len(frags))
        }

        // Verificar que class se detecta como estructura
        hayEstructura := false
        for _, f := range frags {
                if f.tipo == "estructura" {
                        hayEstructura = true
                        break
                }
        }
        if !hayEstructura {
                t.Error("debería detectar la class como estructura")
        }
}

func TestFragmentarTS(t *testing.T) {
        contenido := `interface Usuario {
  nombre: string;
  edad: number;
}

type Resultado = string | number;

function procesar(u: Usuario): Resultado {
  return u.nombre;
}
`
        frags := fragmentarJS(contenido)
        if len(frags) < 2 {
                t.Fatalf("debería generar al menos 2 fragmentos, got %d", len(frags))
        }
}

func TestFragmentarRust(t *testing.T) {
        contenido := `pub struct Punto {
    x: f64,
    y: f64,
}

impl Punto {
    pub fn new(x: f64, y: f64) -> Self {
        Self { x, y }
    }
}

pub fn distancia(a: &Punto, b: &Punto) -> f64 {
    ((a.x - b.x).powi(2) + (a.y - b.y).powi(2)).sqrt()
}
`
        frags := fragmentarRust(contenido)
        if len(frags) < 3 {
                t.Fatalf("debería generar al menos 3 fragmentos (struct, impl, fn), got %d", len(frags))
        }
}

func TestFragmentarJava(t *testing.T) {
        contenido := `public class Usuario {
    private String nombre;
    private int edad;

    public Usuario(String nombre, int edad) {
        this.nombre = nombre;
        this.edad = edad;
    }

    public String getNombre() {
        return nombre;
    }
}
`
        frags := fragmentarJava(contenido)
        if len(frags) < 1 {
                t.Fatalf("debería generar al menos 1 fragmento (class), got %d", len(frags))
        }
        if frags[0].tipo != "estructura" {
                t.Errorf("primer fragmento debería ser 'estructura', got %s", frags[0].tipo)
        }
}

func TestFragmentarC(t *testing.T) {
        contenido := `#include <stdio.h>

struct punto {
    int x;
    int y;
};

int suma(int a, int b) {
    return a + b;
}

int main() {
    printf("hola");
    return 0;
}
`
        frags := fragmentarC(contenido)
        if len(frags) < 2 {
                t.Fatalf("debería generar al menos 2 fragmentos (struct, funciones), got %d", len(frags))
        }
}

func TestContarLlaves_IgnoraStrings(t *testing.T) {
        abre, cierra := contarLlaves(`fmt.Println("hello { world }")`)
        if abre != 0 || cierra != 0 {
                t.Errorf("no debería contar llaves dentro de strings: abre=%d cierra=%d", abre, cierra)
        }
}

func TestContarLlaves_IgnoraComentarios(t *testing.T) {
        abre, cierra := contarLlaves(`// comentario con { y }`)
        if abre != 0 || cierra != 0 {
                t.Errorf("no debería contar llaves en comentarios: abre=%d cierra=%d", abre, cierra)
        }
}

func TestContarLlaves_CuentaReales(t *testing.T) {
        // `func foo() { ... }` con un map anidado: 2 abre + 2 cierra reales
        abre, cierra := contarLlaves(`func foo() { x := map[string]int{1: 2} }`)
        if abre != 2 {
                t.Errorf("debería contar 2 abre-llaves, got %d", abre)
        }
        if cierra != 2 {
                t.Errorf("debería contar 2 cierra-llaves, got %d", cierra)
        }
}

// ============================================================================
// Tests del índice en memoria (ruta → []id)
// ============================================================================

func TestIndiceRuta_OptimizacionConsultas(t *testing.T) {
        // Después de agregar fragmentos, ObtenerPorRuta debe usar el índice
        a, _ := crearAlmacenTest(t)

        contenido := `package main

func A() {}
func B() {}
func C() {}
`
        ruta := filepath.Join(t.TempDir(), "test.go")
        os.WriteFile(ruta, []byte(contenido), 0644)

        ids, _ := a.AgregarDesdeArchivo("test.go", ruta)
        if len(ids) == 0 {
                t.Fatal("debería haber generado fragmentos")
        }

        // ObtenerPorRuta debe retornar todos los fragmentos en orden
        frags, err := a.ObtenerPorRuta("test.go")
        if err != nil {
                t.Fatalf("ObtenerPorRuta() error: %v", err)
        }
        if len(frags) != len(ids) {
                t.Errorf("debería retornar %d fragmentos, got %d", len(ids), len(frags))
        }

        // Verificar orden por lineaIni
        for i := 1; i < len(frags); i++ {
                if frags[i].LineaIni < frags[i-1].LineaIni {
                        t.Error("los fragmentos deben estar ordenados por lineaIni")
                        break
                }
        }
}

func TestEliminarPorRuta_ActualizaIndice(t *testing.T) {
        a, _ := crearAlmacenTest(t)

        ruta := filepath.Join(t.TempDir(), "test.go")
        contenido := `package main
func A() {}
func B() {}
`
        os.WriteFile(ruta, []byte(contenido), 0644)

        a.AgregarDesdeArchivo("test.go", ruta)

        // Antes de eliminar: ObtenerPorRuta retorna N fragmentos
        frags, _ := a.ObtenerPorRuta("test.go")
        if len(frags) == 0 {
                t.Fatal("debería tener fragmentos antes de eliminar")
        }

        // Eliminar
        eliminados, err := a.EliminarPorRuta("test.go")
        if err != nil {
                t.Fatalf("EliminarPorRuta() error: %v", err)
        }
        if len(eliminados) == 0 {
                t.Error("debería haber eliminado al menos un fragmento")
        }

        // Después de eliminar: ObtenerPorRuta retorna 0 fragmentos
        frags, _ = a.ObtenerPorRuta("test.go")
        if len(frags) != 0 {
                t.Errorf("debería retornar 0 fragmentos después de eliminar, got %d", len(frags))
        }

        // Total también debe ser 0
        if a.Total() != 0 {
                t.Errorf("Total() debería ser 0 después de eliminar, got %d", a.Total())
        }
}

func TestRecargarIndiceDesdeDisco(t *testing.T) {
        // Si se crea un nuevo Almacen apuntando al mismo directorio, debe cargar
        // el índice desde los archivos existentes en disco.
        tmpDir := t.TempDir()
        a1, _ := NuevoAlmacen(tmpDir, "proyecto_test")

        ruta := filepath.Join(t.TempDir(), "test.go")
        contenido := `package main
func A() {}
`
        os.WriteFile(ruta, []byte(contenido), 0644)
        a1.AgregarDesdeArchivo("test.go", ruta)

        // Crear segundo almacén apuntando al mismo dir
        a2, err := NuevoAlmacen(tmpDir, "proyecto_test")
        if err != nil {
                t.Fatalf("NuevoAlmacen() error: %v", err)
        }

        // Debe poder encontrar los fragmentos del primer almacén
        frags, err := a2.ObtenerPorRuta("test.go")
        if err != nil {
                t.Fatalf("ObtenerPorRuta() en segundo almacén: %v", err)
        }
        if len(frags) == 0 {
                t.Error("el segundo almacén debería cargar el índice desde disco")
        }
}
