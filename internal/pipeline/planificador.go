package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Planificador descompone la tarea del usuario en pasos ejecutables.
// Determina qué herramientas son necesarias y en qué orden.
// Puede solicitar auto-creación de herramientas faltantes.
type Planificador struct {
	orquestador   OrquestadorCliente
	catalogo      CatalogoCliente
	autoGestor    AutoCreacionGestor
	contextoCoord ContextoCoordinador
}

// nuevoPlanificador crea un Planificador con todas las dependencias inyectadas.
func nuevoPlanificador(
	orch OrquestadorCliente,
	cat CatalogoCliente,
	auto AutoCreacionGestor,
	ctxCoord ContextoCoordinador,
) *Planificador {
	return &Planificador{
		orquestador:   orch,
		catalogo:      cat,
		autoGestor:    auto,
		contextoCoord: ctxCoord,
	}
}

// Planificar genera un plan de ejecución basado en el mensaje y la clasificación.
// Retorna un PlanEjecucion con los pasos necesarios.
func (p *Planificador) Planificar(ctx context.Context, mensaje string, clasif *ResultadoClasificacion, sesion *SesionInfo, historial []turnoHistorial) (*PlanEjecucion, error) {
	plan := &PlanEjecucion{
		ID:                generarUUID(),
		Categoria:         clasif.Categoria,
		PuedeParalelizar:  false,
		NecesitaAutoCrear: false,
	}

	switch clasif.Categoria {
	case CategoriaConversacion:
		return p.planificarConversacion(plan, mensaje)
	case CategoriaEjecucionComando:
		return p.planificarEjecucionComando(plan, mensaje)
	case CategoriaArchivos:
		return p.planificarConLLM(ctx, plan, mensaje, clasif, sesion, historial)
	case CategoriaProcesos:
		return p.planificarConLLM(ctx, plan, mensaje, clasif, sesion, historial)
	case CategoriaMonitorizacion:
		return p.planificarMonitorizacion(plan, mensaje)
	case CategoriaInstalacion:
		return p.planificarConLLM(ctx, plan, mensaje, clasif, sesion, historial)
	case CategoriaBusqueda:
		return p.planificarConLLM(ctx, plan, mensaje, clasif, sesion, historial)
	case CategoriaCodigo, CategoriaAnalisis:
		return p.planificarCodigo(ctx, plan, mensaje, clasif, sesion, historial)
	case CategoriaAutoCreacion:
		return p.planificarAutoCreacion(plan, mensaje)
	default:
		return p.planificarConversacion(plan, mensaje)
	}
}

// planificarConversacion genera un plan sin herramientas (solo LLM).
func (p *Planificador) planificarConversacion(plan *PlanEjecucion, mensaje string) (*PlanEjecucion, error) {
	plan.DescripcionGlobal = "Respuesta conversacional directa"
	plan.Pasos = []PasoTarea{
		{
			ID:          1,
			Descripcion: "Generar respuesta conversacional con LLM",
			RequiereLLM: true,
		},
	}
	plan.EstimacionPasos = 1
	return plan, nil
}

// planificarEjecucionComando planifica la ejecución de un comando shell.
func (p *Planificador) planificarEjecucionComando(plan *PlanEjecucion, mensaje string) (*PlanEjecucion, error) {
	comando := extraerComando(mensaje)
	if comando == "" {
		// Fallback: usar LLM para extraer el comando
		if p.orquestador != nil {
			// El paso 1 usa LLM para entender, paso 2 ejecuta
			plan.DescripcionGlobal = "Ejecutar comando del sistema"
			plan.Pasos = []PasoTarea{
				{
					ID:          1,
					Descripcion: "Interpretar comando solicitado",
					RequiereLLM: true,
				},
				{
					ID:              2,
					Descripcion:     "Ejecutar comando en terminal",
					Herramienta:     "terminal",
					DependeDe:       []int{1},
					TimeoutSegundos: 30,
				},
			}
		} else {
			return p.planificarConversacion(plan, mensaje)
		}
	} else {
		parts := strings.Fields(comando)
		var args []interface{}
		if len(parts) > 1 {
			for _, a := range parts[1:] {
				args = append(args, a)
			}
		}
		plan.DescripcionGlobal = fmt.Sprintf("Ejecutar: %s", truncarTexto(comando, 80))
		plan.Pasos = []PasoTarea{
			{
				ID:          1,
				Descripcion: fmt.Sprintf("Ejecutar: %s", truncarTexto(comando, 80)),
				Herramienta: "terminal",
				Parametros: map[string]interface{}{
					"comando": parts[0],
					"args":    args,
				},
				TimeoutSegundos: 30,
			},
			{
				ID:          2,
				Descripcion: "Interpretar resultado con LLM",
				RequiereLLM: true,
				DependeDe:   []int{1},
			},
		}
	}
	plan.EstimacionPasos = len(plan.Pasos)
	return plan, nil
}

// planificarMonitorizacion planifica la recopilación de métricas del sistema.
func (p *Planificador) planificarMonitorizacion(plan *PlanEjecucion, mensaje string) (*PlanEjecucion, error) {
	msg := strings.ToLower(mensaje)

	var metrica string
	switch {
	case strings.Contains(msg, "cpu") || strings.Contains(msg, "procesador"):
		metrica = "cpu"
	case strings.Contains(msg, "memoria") || strings.Contains(msg, "ram"):
		metrica = "memoria"
	case strings.Contains(msg, "disco") || strings.Contains(msg, "almacenamiento"):
		metrica = "disco"
	case strings.Contains(msg, "red") || strings.Contains(msg, "network"):
		metrica = "red"
	default:
		metrica = "completo"
	}

	plan.DescripcionGlobal = fmt.Sprintf("Monitoreo del sistema: %s", metrica)
	plan.Pasos = []PasoTarea{
		{
			ID:          1,
			Descripcion: fmt.Sprintf("Obtener métricas de %s del sistema", metrica),
			Herramienta: "monitor",
			Parametros: map[string]interface{}{
				"tipo": metrica,
			},
			TimeoutSegundos: 10,
		},
		{
			ID:          2,
			Descripcion: "Presentar métricas al usuario",
			RequiereLLM: true,
			DependeDe:   []int{1},
		},
	}
	plan.EstimacionPasos = 2
	return plan, nil
}

// planificarAutoCreacion planifica la creación de una herramienta nueva.
func (p *Planificador) planificarAutoCreacion(plan *PlanEjecucion, mensaje string) (*PlanEjecucion, error) {
	plan.DescripcionGlobal = "Auto-creación de herramienta"
	plan.NecesitaAutoCrear = true
	plan.Pasos = []PasoTarea{
		{
			ID:          1,
			Descripcion: "Detectar herramienta faltante",
			RequiereLLM: true,
		},
		{
			ID:          2,
			Descripcion: "Generar código Go de la herramienta",
			RequiereLLM: true,
			DependeDe:   []int{1},
		},
		{
			ID:              3,
			Descripcion:     "Compilar y registrar la herramienta",
			Herramienta:     "__auto_creacion__",
			DependeDe:       []int{2},
			TimeoutSegundos: 60,
		},
		{
			ID:          4,
			Descripcion: "Confirmar creación al usuario",
			RequiereLLM: true,
			DependeDe:   []int{3},
		},
	}
	plan.EstimacionPasos = 4
	return plan, nil
}

// planificarCodigo planifica tareas relacionadas con código fuente.
// Usa contexto del proyecto si está disponible.
func (p *Planificador) planificarCodigo(ctx context.Context, plan *PlanEjecucion, mensaje string, clasif *ResultadoClasificacion, sesion *SesionInfo, historial []turnoHistorial) (*PlanEjecucion, error) {
	// Si hay un proyecto activo, empaquetar contexto
	if p.contextoCoord != nil && sesion != nil && sesion.Proyecto != "" {
		_, err := p.contextoCoord.EmpaquetarContexto(ctx, sesion.Proyecto, mensaje, 4000)
		if err != nil {
			// No fallar si no hay contexto, continuar sin él
			_ = err
		}
	}

	// Usar LLM para planificar la tarea de código
	if p.orquestador != nil {
		return p.planificarConLLM(ctx, plan, mensaje, clasif, sesion, historial)
	}

	// Fallback sin LLM
	plan.DescripcionGlobal = "Análisis de código (sin LLM)"
	plan.Pasos = []PasoTarea{
		{
			ID:          1,
			Descripcion: "Procesar solicitud de código",
			RequiereLLM: true,
		},
	}
	plan.EstimacionPasos = 1
	return plan, nil
}

// planificarConLLM usa el LLM para descomponer la tarea en pasos con herramientas.
// Este es el planificador inteligente que conecta todo.
func (p *Planificador) planificarConLLM(ctx context.Context, plan *PlanEjecucion, mensaje string, clasif *ResultadoClasificacion, sesion *SesionInfo, historial []turnoHistorial) (*PlanEjecucion, error) {
	if p.orquestador == nil {
		return p.planificarConversacion(plan, mensaje)
	}

	// Obtener lista de herramientas disponibles
	herramientasDisp := p.obtenerHerramientasDisponibles()

	prompt := fmt.Sprintf(`Eres el planificador de Liz, un agente de IA para Linux. Tu trabajo es descomponer la siguiente tarea del usuario en pasos ejecutables.

## Herramientas disponibles:
%s

## Categoría de la tarea: %s
## Mensaje del usuario: "%s"

Responde SOLO en JSON con un array de pasos:
[
  {
    "id": 1,
    "descripcion": "qué hacer",
    "herramienta": "nombre_herramienta_o_vacio",
    "parametros": {},
    "requiere_llm": true/false,
    "prompt_llm": "prompt específico si requiere_llm es true"
  }
]

Reglas:
- Si la tarea no necesita herramientas (solo conversación), devuelve un solo paso con herramienta="" y requiere_llm=true
- Elige la herramienta más apropiada de la lista
- Si no hay herramienta adecuada, usa herramienta="" (Liz lo manejará)
- Los parámetros deben ser un objeto JSON válido
- Máximo 5 pasos`, herramientasDisp, clasif.Categoria, mensaje)

	respuesta, err := p.orquestador.Completar(ctx, prompt, clasif.PrioridadModelo())
	if err != nil {
		// Fallback: plan simple
		return p.planificarFallback(plan, mensaje, clasif)
	}

	pasos, err := parsearPasosPlan(respuesta)
	if err != nil {
		return p.planificarFallback(plan, mensaje, clasif)
	}

	plan.DescripcionGlobal = fmt.Sprintf("Tarea de %s: %s", clasif.Categoria, truncarTexto(mensaje, 60))
	plan.Pasos = pasos
	plan.EstimacionPasos = len(pasos)

	// Verificar si alguna herramienta no existe en el catálogo
	if p.catalogo != nil {
		for _, paso := range pasos {
			if paso.Herramienta != "" && paso.Herramienta != "__auto_creacion__" {
				if !p.catalogo.Existe(paso.Herramienta) {
					plan.NecesitaAutoCrear = true
					break
				}
			}
		}
	}

	return plan, nil
}

// planificarFallback genera un plan simple cuando el LLM no está disponible.
func (p *Planificador) planificarFallback(plan *PlanEjecucion, mensaje string, clasif *ResultadoClasificacion) (*PlanEjecucion, error) {
	plan.DescripcionGlobal = fmt.Sprintf("Tarea de %s (fallback)", clasif.Categoria)
	plan.Pasos = []PasoTarea{
		{
			ID:          1,
			Descripcion: "Procesar solicitud con LLM",
			RequiereLLM: true,
		},
	}
	plan.EstimacionPasos = 1
	return plan, nil
}

// obtenerHerramientasDisponibles retorna una lista formateada de herramientas para el prompt.
func (p *Planificador) obtenerHerramientasDisponibles() string {
	if p.catalogo == nil {
		return "No hay herramientas registradas"
	}
	snapshot := p.catalogo.Snapshot()
	if len(snapshot) == 0 {
		return "No hay herramientas registradas"
	}

	var sb strings.Builder
	for _, h := range snapshot {
		sb.WriteString(fmt.Sprintf("- %s: %s (params: %v)\n", h.Nombre, h.Descripcion, h.Parametros))
	}
	return sb.String()
}

// parsearPasosPlan extrae los pasos del JSON retornado por el LLM.
func parsearPasosPlan(texto string) ([]PasoTarea, error) {
	jsonStr := extraerJSON(texto)
	if jsonStr == "" {
		return nil, fmt.Errorf("no se encontró JSON en la respuesta del planificador")
	}

	var pasos []PasoTarea
	if err := json.Unmarshal([]byte(jsonStr), &pasos); err != nil {
		// Intentar como array directo
		var rawSteps []json.RawMessage
		if err2 := json.Unmarshal([]byte(jsonStr), &rawSteps); err2 != nil {
			return nil, fmt.Errorf("error parseando pasos: %w", err)
		}
		for _, raw := range rawSteps {
			var paso PasoTarea
			if err := json.Unmarshal(raw, &paso); err != nil {
				continue
			}
			pasos = append(pasos, paso)
		}
	}

	if len(pasos) == 0 {
		return nil, fmt.Errorf("no se encontraron pasos en el plan")
	}

	// Normalizar IDs si no están asignados
	for i := range pasos {
		if pasos[i].ID == 0 {
			pasos[i].ID = i + 1
		}
		if pasos[i].TimeoutSegundos == 0 {
			pasos[i].TimeoutSegundos = 30
		}
	}

	return pasos, nil
}
