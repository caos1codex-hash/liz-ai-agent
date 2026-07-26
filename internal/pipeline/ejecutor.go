package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Ejecutor ejecuta los pasos del plan, coordinando herramientas y auto-creación.
// Maneja dependencias entre pasos y recopila resultados.
type Ejecutor struct {
	catalogo   CatalogoCliente
	autoGestor AutoCreacionGestor
}

// nuevoEjecutor crea un Ejecutor con las dependencias inyectadas.
func nuevoEjecutor(cat CatalogoCliente, auto AutoCreacionGestor) *Ejecutor {
	return &Ejecutor{
		catalogo:   cat,
		autoGestor: auto,
	}
}

// EjecutarPlan ejecuta todos los pasos del plan en orden, respetando dependencias.
// Retorna los resultados de cada paso ejecutado.
func (e *Ejecutor) EjecutarPlan(ctx context.Context, plan *PlanEjecucion, callback func(*ChunkStream)) ([]ResultadoPaso, error) {
	resultados := make([]ResultadoPaso, 0, len(plan.Pasos))

	for _, paso := range plan.Pasos {
		// Verificar que las dependencias se completaron exitosamente
		if err := e.verificarDependencias(paso.DependeDe, resultados); err != nil {
			resultado := ResultadoPaso{
				PasoID:   paso.ID,
				Exito:    false,
				Error:    fmt.Sprintf("Dependencia no cumplida: %v", err),
				Duracion: 0,
			}
			resultados = append(resultados, resultado)
			if callback != nil {
				callback(nuevoChunkConDatos("error", fmt.Sprintf("Paso %d falló: dependencia no cumplida", paso.ID), resultado))
			}
			continue
		}

		resultado := e.ejecutarPaso(ctx, paso, plan, callback)
		resultados = append(resultados, resultado)
	}

	return resultados, nil
}

// ejecutarPaso ejecuta un paso individual del plan.
func (e *Ejecutor) ejecutarPaso(ctx context.Context, paso PasoTarea, plan *PlanEjecucion, callback func(*ChunkStream)) ResultadoPaso {
	inicio := time.Now()

	// Notificar inicio del paso
	if callback != nil {
		callback(nuevoChunkConDatos("estado", fmt.Sprintf("Ejecutando paso %d: %s", paso.ID, paso.Descripcion),
			map[string]interface{}{"paso_id": paso.ID, "herramienta": paso.Herramienta}))
	}

	var resultado ResultadoPaso
	resultado.PasoID = paso.ID

	switch {
	case paso.Herramienta == "__auto_creacion__":
		resultado = e.ejecutarAutoCreacion(ctx, paso, callback)
	case paso.Herramienta != "" && e.catalogo != nil:
		resultado = e.ejecutarHerramienta(ctx, paso, callback)
	case paso.RequiereLLM:
		resultado.PasoID = paso.ID
		resultado.Exito = true
		resultado.Duracion = time.Since(inicio)
		// Los pasos que solo requieren LLM se manejan en el Respondedor
	default:
		resultado.Exito = true
		resultado.Duracion = time.Since(inicio)
	}

	resultado.Duracion = time.Since(inicio)
	resultado.ToolUsada = paso.Herramienta

	return resultado
}

// ejecutarHerramienta ejecuta una herramienta del catálogo.
func (e *Ejecutor) ejecutarHerramienta(ctx context.Context, paso PasoTarea, callback func(*ChunkStream)) ResultadoPaso {
	inicio := time.Now()
	resultado := ResultadoPaso{PasoID: paso.ID}

	if e.catalogo == nil {
		resultado.Exito = false
		resultado.Error = "No hay catálogo de herramientas disponible"
		resultado.Duracion = time.Since(inicio)
		return resultado
	}

	// Configurar timeout si es necesario
	if paso.TimeoutSegundos > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(paso.TimeoutSegundos)*time.Second)
		defer cancel()
	}

	res, err := e.catalogo.Ejecutar(ctx, paso.Herramienta, paso.Parametros)
	if err != nil {
		resultado.Exito = false
		resultado.Error = fmt.Sprintf("Error ejecutando %s: %v", paso.Herramienta, err)
		resultado.Duracion = time.Since(inicio)
		return resultado
	}

	resultado.Exito = res.Exito
	resultado.Datos = res.Datos
	resultado.Error = res.Error
	resultado.Metadata = res.Metadata
	resultado.Duracion = time.Since(inicio)

	if callback != nil {
		tipo := "herramienta"
		if !res.Exito {
			tipo = "error"
		}
		datosResumen := map[string]interface{}{
			"paso_id":     paso.ID,
			"herramienta": paso.Herramienta,
			"exito":       res.Exito,
			"duracion_ms": resultado.Duracion.Milliseconds(),
		}
		callback(nuevoChunkConDatos(tipo, fmt.Sprintf("Herramienta %s: %s", paso.Herramienta, estadoExito(res.Exito)), datosResumen))
	}

	return resultado
}

// ejecutarAutoCreación gestiona la creación automática de herramientas.
func (e *Ejecutor) ejecutarAutoCreacion(ctx context.Context, paso PasoTarea, callback func(*ChunkStream)) ResultadoPaso {
	inicio := time.Now()
	resultado := ResultadoPaso{PasoID: paso.ID}

	if e.autoGestor == nil {
		resultado.Exito = false
		resultado.Error = "No hay gestor de auto-creación disponible"
		resultado.Duracion = time.Since(inicio)
		return resultado
	}

	if callback != nil {
		callback(nuevoChunk("estado", "Iniciando auto-creación de herramienta..."))
	}

	// Extraer descripción de los parámetros o del paso
	descripcion := paso.Descripcion
	if paso.Parametros != nil {
		if desc, ok := paso.Parametros["descripcion"].(string); ok {
			descripcion = desc
		}
	}

	// Ejecutar el flujo de auto-creación
	resAuto, err := e.autoGestor.Crear(ctx, descripcion)
	if err != nil {
		resultado.Exito = false
		resultado.Error = fmt.Sprintf("Error en auto-creación: %v", err)
		resultado.Duracion = time.Since(inicio)
		return resultado
	}

	resultado.Exito = resAuto.Exito
	resultado.Datos = resAuto.Datos
	resultado.Duracion = time.Since(inicio)

	if callback != nil {
		callback(nuevoChunk("estado", fmt.Sprintf("Herramienta auto-creada: %v", resAuto.Datos)))
	}

	return resultado
}

// verificarDependencias verifica que los pasos dependientes se hayan completado exitosamente.
func (e *Ejecutor) verificarDependencias(dependencias []int, resultados []ResultadoPaso) error {
	for _, depID := range dependencias {
		encontrado := false
		for _, res := range resultados {
			if res.PasoID == depID {
				if !res.Exito {
					return fmt.Errorf("paso %d falló", depID)
				}
				encontrado = true
				break
			}
		}
		if !encontrado {
			return fmt.Errorf("paso %d no se ha ejecutado aún", depID)
		}
	}
	return nil
}

// FormatearResultados genera un resumen legible de los resultados para el LLM.
func FormatearResultados(resultados []ResultadoPaso) string {
	if len(resultados) == 0 {
		return "No se ejecutaron herramientas."
	}

	var sb strings.Builder
	for _, r := range resultados {
		sb.WriteString(fmt.Sprintf("## Paso %d", r.PasoID))
		sb.WriteString(fmt.Sprintf("\n- Herramienta: %s", r.ToolUsada))
		sb.WriteString(fmt.Sprintf("\n- Resultado: %s", estadoExito(r.Exito)))
		if r.Error != "" {
			sb.WriteString(fmt.Sprintf("\n- Error: %s", r.Error))
		}
		if r.Datos != nil {
			datosJSON, err := json.Marshal(r.Datos)
			if err == nil {
				datosStr := string(datosJSON)
				if len(datosStr) > 2000 {
					datosStr = datosStr[:2000] + "... (truncado)"
				}
				sb.WriteString(fmt.Sprintf("\n- Datos: %s", datosStr))
			}
		}
		sb.WriteString(fmt.Sprintf("\n- Duración: %v", r.Duracion))
		sb.WriteString("\n")
	}
	return sb.String()
}

// estadoExito devuelve un texto de estado legible.
func estadoExito(exito bool) string {
	if exito {
		return "exitoso"
	}
	return "fallido"
}
