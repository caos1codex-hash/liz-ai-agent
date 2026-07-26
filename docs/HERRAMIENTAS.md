# Guía para Extender Herramientas

## Arquitectura del Sistema de Herramientas

Liz tiene 7 herramientas integradas que implementan la interfaz estándar `Herramienta`.
El sistema soporta 3 tipos de herramientas:

1. **Integradas** — escritas en Go, compiladas con Liz
2. **Auto-creadas** — generadas por Liz, ejecutadas como subprocesos
3. **Futuras** — extensibles vía el protocolo subprocess

## Interfaz Estándar

Toda herramienta debe implementar:

```go
type Herramienta interface {
    Info() InfoHerramienta                    // Metadatos (nombre, descripción, parámetros)
    Validar(params map[string]interface{}) error // Validar parámetros de entrada
    Ejecutar(ctx context.Context, params map[string]interface{}) (Resultado, error)
}
```

### `InfoHerramienta`

```go
type InfoHerramienta struct {
    Nombre     string          // Identificador único
    Descripcion string          // Descripción para el LLM
    Categoria   TipoHerramienta // sistema, archivo, busqueda, codigo, etc.
    Parametros  []ParametroInfo // Lista de parámetros aceptados
}
```

### `ParametroInfo`

```go
type ParametroInfo struct {
    Nombre     string // Nombre del parámetro
    Tipo       string // "string", "int", "bool", "array"
    Requerido  bool   // Si es obligatorio
    Default    interface{}
    Descripcion string
}
```

### `Resultado`

```go
type Resultado struct {
    Exito    bool                   `json:"exito"`
    Datos    interface{}            `json:"datos,omitempty"`
    Error    string                 `json:"error,omitempty"`
    Metadata map[string]interface{} `json:"metadata,omitempty"`
}
```

## Agregar una Herramienta Integrada

### Paso 1: Crear el archivo

Crear un archivo en `internal/nucleo/herramientas/integradas/mi_herramienta.go`:

```go
package integradas

import (
    "context"
    "fmt"
    "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas"
)

type MiHerramienta struct{}

func (h *MiHerramienta) Info() herramientas.InfoHerramienta {
    return herramientas.InfoHerramienta{
        Nombre:     "mi_herramienta",
        Descripcion: "Descripción de lo que hace mi herramienta",
        Categoria:   herramientas.TipoSistema,
        Parametros: []herramientas.ParametroInfo{
            {
                Nombre:     "parametro1",
                Tipo:       "string",
                Requerido:  true,
                Descripcion: "Parámetro requerido de ejemplo",
            },
        },
    }
}

func (h *MiHerramienta) Validar(params map[string]interface{}) error {
    if herramientas.ObtenerString(params, "parametro1") == "" {
        return fmt.Errorf("parametro1 es requerido")
    }
    return nil
}

func (h *MiHerramienta) Ejecutar(ctx context.Context, params map[string]interface{}) (herramientas.Resultado, error) {
    param1 := herramientas.ObtenerString(params, "parametro1")
    
    // Lógica de la herramienta aquí
    resultado := fmt.Sprintf("Procesé: %s", param1)
    
    return herramientas.Resultado{
        Exito: true,
        Datos: resultado,
    }, nil
}
```

### Paso 2: Registrar en el catálogo

En `cmd/liz/main.go`, agregar al catálogo:

```go
catalogo.Registrar(&integradas.MiHerramienta{})
```

### Paso 3: Escribir tests

Crear `internal/nucleo/herramientas/integradas/mi_herramienta_test.go`:

```go
package integradas

import (
    "context"
    "testing"
)

func TestMiHerramienta_Info(t *testing.T) {
    h := &MiHerramienta{}
    info := h.Info()
    if info.Nombre != "mi_herramienta" {
        t.Errorf("esperaba 'mi_herramienta', got '%s'", info.Nombre)
    }
}

func TestMiHerramienta_Validar(t *testing.T) {
    h := &MiHerramienta{}
    
    // Sin parámetro requerido
    err := h.Validar(map[string]interface{}{})
    if err == nil {
        t.Fatal("esperaba error sin parametro1")
    }
    
    // Con parámetro
    err = h.Validar(map[string]interface{}{"parametro1": "valor"})
    if err != nil {
        t.Fatalf("no esperaba error: %v", err)
    }
}

func TestMiHerramienta_Ejecutar(t *testing.T) {
    h := &MiHerramienta{}
    result, err := h.Ejecutar(context.Background(), map[string]interface{}{
        "parametro1": "test",
    })
    if err != nil {
        t.Fatalf("no esperaba error: %v", err)
    }
    if !result.Exito {
        t.Error("esperaba éxito")
    }
}
```

## Herramientas Auto-Creadas (Subproceso)

Las herramientas auto-creadas usan un protocolo diferente: se comunican con Liz
por JSON sobre stdin/stdout. Esto las hace más robustas que Go plugins.

### Protocolo

```
REQUEST (Liz → herramienta, stdin):
  {"operacion": "info|validar|ejecutar", "parametros": {...}, "timeout_ms": 5000}

RESPONSE (herramienta → Liz, stdout):
  {"exito": true, "datos": <any>, "error": "", "metadata": {...}}
```

### Ejemplo de herramienta auto-creada

```go
package main

import (
    "encoding/json"
    "fmt"
    "os"
)

type Request struct {
    Operacion  string                 `json:"operacion"`
    Parametros map[string]interface{} `json:"parametros"`
    TimeoutMs  int                    `json:"timeout_ms"`
}

type Response struct {
    Exito    bool        `json:"exito"`
    Datos    interface{} `json:"datos"`
    Error    string      `json:"error"`
    Metadata interface{} `json:"metadata"`
}

func main() {
    var req Request
    decoder := json.NewDecoder(os.Stdin)
    if err := decoder.Decode(&req); err != nil {
        sendError(fmt.Sprintf("error decodificando: %v", err))
        return
    }
    
    switch req.Operacion {
    case "info":
        sendSuccess(map[string]interface{}{
            "nombre":      "mi_herramienta",
            "descripcion": "Herramienta de ejemplo",
        })
    case "validar":
        sendSuccess(nil)
    case "ejecutar":
        // Lógica aquí
        sendSuccess("resultado de la ejecución")
    default:
        sendError(fmt.Sprintf("operación desconocida: %s", req.Operacion))
    }
}

func sendSuccess(datos interface{}) {
    json.NewEncoder(os.Stdout).Encode(Response{Exito: true, Datos: datos})
}

func sendError(errMsg string) {
    json.NewEncoder(os.Stdout).Encode(Response{Exito: false, Error: errMsg})
}
```

## Helpers Disponibles

El paquete `herramientas` provee helpers para validar parámetros:

```go
herramientas.ObtenerString(params, "key")      // string, "" si falta
herramientas.ObtenerInt(params, "key")         // int, 0 si falta
herramientas.ObtenerBool(params, "key")        // bool, false si falta
herramientas.ObtenerArray(params, "key")       // []interface{}, nil si falta
herramientas.ObtenerStringDefault(params, "key", "default")
```

## Convenciones

1. **Nombre único** — cada herramienta debe tener un nombre único en el catálogo
2. **Solo stdlib** — las herramientas auto-creadas solo pueden usar la stdlib de Go
3. **Timeout** — toda herramienta debe respetar `ctx.Done()` para cancellation
4. **Sin side effects en Validar()** — `Validar()` nunca debe modificar estado
5. **Metadata** — incluir `duracion_ms` y `herramienta` en los resultados
6. **Compilar-time check** — usar `var _ herramientas.Herramienta = (*MiHerramienta)(nil)`

## Seguridad

- Las herramientas se ejecutan con los permisos del usuario que corre Liz
- La herramienta `terminal` detecta comandos peligrosos (`rm -rf /`, `mkfs`, `shutdown`)
- `Validar()` actúa como primera línea de defensa
- El timeout del context se propaga a las herramientas (SIGKILL al expirar)