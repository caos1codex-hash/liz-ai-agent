package auto_creacion

import (
	"fmt"
	"regexp"
	"strings"
)

// ============================================================================
// Plantillas y utilidades para generar código Go de herramientas
// ============================================================================

// PlantillaProtocolo es la especificación del protocolo subprocess que se
// incluye en el prompt del LLM para que genere código compatible.
const PlantillaProtocolo = `// PROTOCOLO SUBPROCESS (Liz ↔ herramienta)
//
// Tu programa Go debe ser package main, usar SOLO stdlib, y:
//
// 1. Leer UNA línea JSON de stdin con esta forma:
//    {"operacion": "info|validar|ejecutar", "parametros": {...}, "timeout_ms": 5000}
//
// 2. Ejecutar según "operacion":
//    - "info"    → responder con datos = {nombre, descripcion, parametros}
//    - "validar" → responder con datos = {ok: true} si todo está operativo
//    - "ejecutar" → ejecutar la operación con "parametros" y responder con el resultado
//
// 3. Escribir UNA línea JSON a stdout con esta forma:
//    {"exito": true, "datos": <any>, "error": "", "metadata": {...}}
//
// 4. Si hay error, exito=false y error=mensaje. NUNCA panic sin recover.
//
// 5. Salir con código 0 siempre (incluso en error controlado).
//    Solo salir non-zero si hay panic irrecuperable (Liz lo captura igual).
`

// PlantillaEjemplo es un ejemplo mínimo que el LLM debe usar como referencia
// de estructura (no se incluye literalmente en la salida).
const PlantillaEjemplo = `package main

import (
        "bufio"
        "encoding/json"
        "fmt"
        "os"
)

type solicitud struct {
        Operacion  string                 ` + "`json:\"operacion\"`" + `
        Parametros map[string]interface{} ` + "`json:\"parametros\"`" + `
}

type respuesta struct {
        Exito    bool                   ` + "`json:\"exito\"`" + `
        Datos    interface{}            ` + "`json:\"datos,omitempty\"`" + `
        Error    string                 ` + "`json:\"error,omitempty\"`" + `
        Metadata map[string]interface{} ` + "`json:\"metadata,omitempty\"`" + `
}

type parametro struct {
        Nombre      string      ` + "`json:\"nombre\"`" + `
        Tipo        string      ` + "`json:\"tipo\"`" + `
        Requerido   bool        ` + "`json:\"requerido\"`" + `
        Default     interface{} ` + "`json:\"default,omitempty\"`" + `
        Descripcion string      ` + "`json:\"descripcion\"`" + `
}

type info struct {
        Nombre      string      ` + "`json:\"nombre\"`" + `
        Descripcion string      ` + "`json:\"descripcion\"`" + `
        Parametros  []parametro ` + "`json:\"parametros\"`" + `
}

func main() {
        defer func() {
                if r := recover(); r != nil {
                        out, _ := json.Marshal(respuesta{Exito: false, Error: fmt.Sprintf("panic: %v", r)})
                        fmt.Println(string(out))
                        os.Exit(0)
                }
        }()

        reader := bufio.NewReader(os.Stdin)
        linea, err := reader.ReadString('\n')
        if err != nil {
                out, _ := json.Marshal(respuesta{Exito: false, Error: "leyendo stdin: " + err.Error()})
                fmt.Println(string(out))
                return
        }

        var sol solicitud
        if err := json.Unmarshal([]byte(linea), &sol); err != nil {
                out, _ := json.Marshal(respuesta{Exito: false, Error: "JSON inválido: " + err.Error()})
                fmt.Println(string(out))
                return
        }

        var res respuesta
        switch sol.Operacion {
        case "info":
                res = manejarInfo()
        case "validar":
                res = manejarValidar()
        case "ejecutar":
                res = manejarEjecutar(sol.Parametros)
        default:
                res = respuesta{Exito: false, Error: "operación desconocida: " + sol.Operacion}
        }

        out, _ := json.Marshal(res)
        fmt.Println(string(out))
}

func manejarInfo() respuesta {
        return respuesta{
                Exito: true,
                Datos: info{
                        Nombre:      "ejemplo_herramienta",
                        Descripcion: "Herramienta de ejemplo",
                        Parametros: []parametro{
                                {Nombre: "mensaje", Tipo: "string", Requerido: true, Descripcion: "Mensaje a procesar"},
                        },
                },
        }
}

func manejarValidar() respuesta {
        return respuesta{Exito: true, Datos: map[string]bool{"ok": true}}
}

func manejarEjecutar(params map[string]interface{}) respuesta {
        msg, ok := params["mensaje"].(string)
        if !ok {
                return respuesta{Exito: false, Error: "parámetro 'mensaje' requerido"}
        }
        return respuesta{
                Exito: true,
                Datos: map[string]string{"resultado": "procesado: " + msg},
                Metadata: map[string]interface{}{
                        "longitud_entrada": len(msg),
                },
        }
}
`

// PlantillaPrompt genera el prompt completo que se envía al LLM para producir
// el código Go de una herramienta.
func PlantillaPrompt(spec SpecHerramienta) string {
	var paramsJSON strings.Builder
	paramsJSON.WriteString("[\n")
	for i, p := range spec.Parametros {
		paramsJSON.WriteString("    ")
		paramsJSON.WriteString(fmt.Sprintf("{nombre: %q, tipo: %q, requerido: %v, default: %v, descripcion: %q}",
			p.Nombre, p.Tipo, p.Requerido, p.Default, p.Descripcion))
		if i < len(spec.Parametros)-1 {
			paramsJSON.WriteString(",")
		}
		paramsJSON.WriteString("\n")
	}
	paramsJSON.WriteString("]")

	return fmt.Sprintf(`Eres un ingeniero Go senior. Genera un programa Go COMPLETO y COMPILABLE que implemente una herramienta para el agente Liz.

== ESPECIFICACIÓN DE LA HERRAMIENTA ==
Nombre: %s
Descripción: %s
Parámetros: %s
Categoría: %s

== REQUISITOS ESTRICTOS ==
1. package main, UN SOLO archivo, SIN go.mod (se compila con 'go build fuente.go').
2. SOLO paquetes de la stdlib (os, net/http, encoding/json, bufio, strconv, strings, fmt, time, etc.). NUNCA paquetes externos.
3. Seguir EXACTAMENTE el protocolo subprocess de abajo.
4. Usar defer+recover en main() para que NUNCA panique sin responder JSON.
5. La operacion="info" debe retornar nombre, descripción y parámetros que coincidan con la especificación.
6. La operacion="validar" debe verificar cualquier dependencia (archivos, permisos, red) y retornar {ok: true/false}.
7. La operacion="ejecutar" debe:
   - Validar que los parámetros requeridos estén presentes
   - Coerce tipos si es necesario (float64→int, etc., ya que JSON numéricos llegan como float64)
   - Ejecutar la lógica de la herramienta
   - Retornar {exito: true, datos: <resultado>, metadata: {...}} o {exito: false, error: "..."}
8. NUNCA usar panic en código normal; siempre retornar error como JSON.
9. Código limpio, comentarios en español, nombres en español o inglés (consistente).
10. Eficiente: si la operación lee archivos, usar bufio; si hace HTTP, usar timeouts.

%s

== EJEMPLO DE ESTRUCTURA (no copies literalmente, adapta a la spec) ==
%s

== OUTPUT ==
Devuelve EXCLUSIVAMENTE el código Go dentro de un bloque `+"```go ... ```"+`.
Sin explicaciones antes ni después. El código debe ser completo y funcional.`,
		spec.Nombre, spec.Descripcion, paramsJSON.String(), spec.Categoria,
		PlantillaProtocolo, PlantillaEjemplo)
}

// PlantillaPromptDeteccion genera el prompt para el Detector.
func PlantillaPromptDeteccion(descripcion string, catalogo []InfoCatalogo) string {
	var cat strings.Builder
	if len(catalogo) == 0 {
		cat.WriteString("(catálogo vacío)")
	} else {
		for _, c := range catalogo {
			cat.WriteString(fmt.Sprintf("  - %s: %s\n", c.Nombre, c.Descripcion))
		}
	}

	return fmt.Sprintf(`Eres un analista de sistemas. Analiza la siguiente petición del usuario y determina qué herramientas NUEVAS necesita el agente Liz para completarla.

== PETICIÓN DEL USUARIO ==
%s

== HERRAMIENTAS YA DISPONIBLES EN EL CATÁLOGO ==
%s

== REGLAS ==
1. Solo sugiere herramientas que NO existan ya en el catálogo.
2. Una herramienta es reutilizable: no la crees para una sola invocación puntual. Créala si la capacidad es genérica.
3. Si la petición se puede resolver con las herramientas existentes, retorna lista vacía.
4. Máximo 5 herramientas por detección (prioriza las más críticas).
5. Para cada herramienta, define: nombre (snake_case), descripción corta, parámetros con tipo y descripción, razón de necesidad, categoría.

== CATEGORÍAS VÁLIDAS ==
"archivos", "red", "sistema", "datos", "codigo", "procesos", "seguridad", "otro"

== TIPOS DE PARÁMETROS VÁLIDOS ==
"string", "int", "bool", "float", "array", "object"

== OUTPUT ==
Devuelve EXCLUSIVAMENTE un bloque `+"```json ... ```"+` con este formato:
{
  "faltantes": [
    {
      "nombre": "compresor_archivos",
      "descripcion": "Comprime archivos en formato zip o tar.gz",
      "categoria": "archivos",
      "razon": "El usuario necesita comprimir CSVs antes de enviarlos y no existe herramienta de compresión",
      "parametros": [
        {"nombre": "archivos", "tipo": "array", "items": "string", "requerido": true, "descripcion": "Lista de rutas a comprimir"},
        {"nombre": "formato", "tipo": "string", "requerido": false, "default": "zip", "descripcion": "Formato: zip o tar.gz"}
      ]
    }
  ],
  "razon": "Resumen del análisis"
}

Si no se necesitan herramientas nuevas, devuelve {"faltantes": [], "razon": "..."}.`,
		descripcion, cat.String())
}

// ============================================================================
// Post-procesamiento del código generado
// ============================================================================

// regexMarkdownGo extrae código Go dentro de bloques ```go ... ```
var regexMarkdownGo = regexp.MustCompile("(?s)```(?:go|golang)?\\s*\n(.*?)```")

// ExtraerFuenteGo limpia la salida del LLM y retorna solo el código Go.
//
//  1. Si hay bloques ```go ... ```, extrae el primero.
//  2. Si no, toma todo el texto que empiece con "package main".
//  3. Si tampoco, retorna el input tal cual (el Compilador reportará el error).
func ExtraerFuenteGo(raw string) string {
	// Intentar extraer de bloque markdown
	if matches := regexMarkdownGo.FindStringSubmatch(raw); len(matches) >= 2 {
		return strings.TrimSpace(matches[1])
	}

	// Buscar "package main" y tomar desde ahí
	idx := strings.Index(raw, "package main")
	if idx >= 0 {
		return strings.TrimSpace(raw[idx:])
	}

	// Último recurso: devolver tal cual
	return strings.TrimSpace(raw)
}

// ValidarFuenteGo hace comprobaciones básicas antes de compilar.
// Retorna error descriptivo si el fuente no cumple los requisitos mínimos.
//
// Es tolerante con comentarios iniciales: el "package main" puede estar
// precedido por un header de comentarios (ver InyectarHeader).
func ValidarFuenteGo(fuente string) error {
	if fuente == "" {
		return fmt.Errorf("fuente vacío")
	}
	// Buscar "package main" en cualquier parte del fuente (no solo al inicio,
	// porque InyectarHeader añade un comentario antes).
	if !strings.Contains(fuente, "package main") {
		return fmt.Errorf("el fuente debe declarar 'package main'")
	}
	if !strings.Contains(fuente, "func main()") {
		return fmt.Errorf("el fuente debe tener func main()")
	}
	return nil
}

// InyectarHeader agrega un comentario al inicio del fuente con metadata.
func InyectarHeader(fuente string, spec SpecHerramienta, modelo string) string {
	header := fmt.Sprintf(`// ============================================================================
// Herramienta auto-creada por Liz (Fase 6)
//   Nombre:    %s
//   Categoría: %s
//   Modelo:    %s
//   Descripción: %s
//   NO EDITAR A MANO — regenerar vía API /api/v1/herramientas/auto-creadas/%s/recargar
// ============================================================================

`,
		spec.Nombre, spec.Categoria, modelo, spec.Descripcion, spec.Nombre)

	// Remover cualquier "package main" existente y reemplazar con header + package main
	fuente = strings.TrimSpace(fuente)
	if strings.HasPrefix(fuente, "package main") {
		return header + fuente
	}
	// Si no empieza con package main, prependemos igual (ValidarFuenteGo fallará luego)
	return header + fuente
}
