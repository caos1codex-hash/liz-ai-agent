// Package orquestador implementa el orquestador multi-modelo de Liz (Fase 4).
//
// Se conecta a la API de NVIDIA (integrate.api.nvidia.com/v1), que es
// compatible con la API de OpenAI (mismo formato de chat completions).
//
// Características principales:
//
//   1. MULTI-MODELO: 8+ modelos disponibles (Llama 3.1, Mixtral, Nemotron,
//      Gemma, Phi-3, CodeLlama, etc.)
//
//   2. SELECCIÓN INTELIGENTE: dado un tipo de tarea ("codigo", "razonamiento",
//      "general", "contexto_largo"), elige el mejor modelo configurado.
//      Estrategia: prioridad configurada → métricas históricas → aleatorio.
//
//   3. FALLBACK AUTOMÁTICO: si un modelo falla (timeout, 5xx, rate limit),
//      intenta el siguiente modelo en la cadena de fallback.
//
//   4. STREAMING SSE: soporta streaming de respuestas vía Server-Sent Events
//      (estilo OpenAI), para integrarse con el pipeline de chat Fase 7.
//
//   5. MÉTRICAS: registra latencia, éxito/fallo y tokens consumidos por modelo.
//      Las métricas se usan para mejorar selecciones futuras.
//
//   6. SIN DEPENDENCIAS EXTERNAS: usa solo net/http de la stdlib.
//
// Decisiones de diseño (refs docs/DECISIONES.md D-004 y D-009):
//   - API NVIDIA por compatibilidad con OpenAI (mismo formato)
//   - Selector por tipo de tarea + prioridad + métricas
//   - Fallback determinista (orden de prioridad), no aleatorio
//   - Thread-safe (sync.RWMutex) para uso concurrente
package orquestador

import (
        "fmt"
        "sync"
        "time"

        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/config"
)

// ═══════════════════════════════════════════════════════
// TIPOS
// ═══════════════════════════════════════════════════════

// TipoTarea clasifica el tipo de solicitud al LLM.
type TipoTarea string

const (
        TareaCodigo          TipoTarea = "codigo"
        TareaRazonamiento    TipoTarea = "razonamiento"
        TareaGeneral         TipoTarea = "general"
        TareaContextoLargo   TipoTarea = "contexto_largo"
        TareaResumen         TipoTarea = "resumen"
        TareaAnalisis        TipoTarea = "analisis"
        TareaCreatividad     TipoTarea = "creatividad"
)

// MensajeChat es un mensaje en formato OpenAI/NVIDIA.
type MensajeChat struct {
        Rol       string                 `json:"role"`    // "system", "user", "assistant", "tool"
        Contenido string                 `json:"content"`
        Nombre    string                 `json:"name,omitempty"`
        ToolCalls []map[string]interface{} `json:"tool_calls,omitempty"`
        ToolCallID string                `json:"tool_call_id,omitempty"`
}

// SolicitudChat es la solicitud completa al modelo.
type SolicitudChat struct {
        Modelo          string        // ID del modelo a usar (vacío = auto-selección)
        Tarea           TipoTarea     // tipo de tarea para selección inteligente
        Mensajes        []MensajeChat // contexto de la conversación
        Temperatura     float64       // 0.0 = determinista, 1.0 = creativo (0 = default del modelo)
        MaxTokens       int           // 0 = default del modelo
        Stream          bool          // true = SSE streaming
        TopP            float64       // 0 = default
        FrecuenciaPenal float64       // 0 = default
        PresenciaPenal  float64       // 0 = default
        Stop            []string      // secuencias de parada
}

// RespuestaChat es la respuesta del modelo.
type RespuestaChat struct {
        Contenido     string        `json:"contenido"`
        ModeloUsado   string        `json:"modelo_usado"`
        TokensPrompt  int           `json:"tokens_prompt"`
        TokensComplet int           `json:"tokens_complet"`
        TokensTotal   int           `json:"tokens_total"`
        Latencia      time.Duration `json:"latencia"`
        AcabadoRazon  string        `json:"acabado_razon"` // "stop", "length", "tool_calls", "content_filter"
        Intentos      int           `json:"intentos"`      // cuántos modelos se probaron
        Error         string        `json:"error,omitempty"`
}

// ChunkStream es un chunk SSE recibido durante streaming.
type ChunkStream struct {
        Contenido    string // delta de contenido
        Acabado      bool   // true si es el último chunk
        ModeloUsado  string
        Error        error
}

// MetricasModelo registra estadísticas de uso de un modelo.
type MetricasModelo struct {
        Modelo           string        `json:"modelo"`
        TotalSolicitudes int           `json:"total_solicitudes"`
        Exitos           int           `json:"exitos"`
        Fallos           int           `json:"fallos"`
        TasaExito        float64       `json:"tasa_exito"`
        LatenciaPromedio time.Duration `json:"latencia_promedio"`
        TokensConsumidos int           `json:"tokens_consumidos"`
        UltimoUso        string        `json:"ultimo_uso"` // RFC3339
}

// Orquestador es el punto de entrada del sistema multi-modelo.
type Orquestador struct {
        mu          sync.RWMutex
        cliente     *ClienteNVIDIA
        modelos     []config.ConfiguracionModelo
        metricas    map[string]*MetricasModelo // modeloID → métricas
        logFunc     func(string, ...interface{})
}

// ═══════════════════════════════════════════════════════
// CONSTRUCTOR
// ═══════════════════════════════════════════════════════

// NuevoOrquestador crea un orquestador con la configuración dada.
// Requiere al menos un modelo configurado con APIKey.
func NuevoOrquestador(cfg *config.Gestor) (*Orquestador, error) {
        modelos := cfg.ObtenerModelos()
        if len(modelos) == 0 {
                return nil, fmt.Errorf("no hay modelos configurados")
        }

        // Buscar API key válida
        apiKey := ""
        endpoint := ""
        for _, m := range modelos {
                if m.APIKey != "" && m.APIKey != "***" {
                        apiKey = m.APIKey
                        if m.URL != "" {
                                endpoint = m.URL
                        }
                        break
                }
        }
        if apiKey == "" {
                return nil, fmt.Errorf("no se encontró API key válida en la configuración")
        }
        if endpoint == "" {
                endpoint = "https://integrate.api.nvidia.com/v1"
        }

        cliente := NuevoClienteNVIDIA(endpoint, apiKey)

        o := &Orquestador{
                cliente:  cliente,
                modelos:  modelos,
                metricas: make(map[string]*MetricasModelo),
                logFunc:  func(string, ...interface{}) {},
        }

        // Inicializar métricas por modelo
        for _, m := range modelos {
                o.metricas[m.Nombre] = &MetricasModelo{Modelo: m.Nombre}
        }

        return o, nil
}

// ConLog asigna función de log.
func (o *Orquestador) ConLog(fn func(string, ...interface{})) *Orquestador {
        if fn != nil {
                o.logFunc = fn
        }
        return o
}

// ModelosDisponibles retorna la lista de modelos configurados.
func (o *Orquestador) ModelosDisponibles() []config.ConfiguracionModelo {
        o.mu.RLock()
        defer o.mu.RUnlock()
        copia := make([]config.ConfiguracionModelo, len(o.modelos))
        copy(copia, o.modelos)
        return copia
}

// Cliente retorna el cliente NVIDIA subyacente (para usar el provider de
// embeddings u otras operaciones directas).
func (o *Orquestador) Cliente() *ClienteNVIDIA {
        return o.cliente
}

// Metricas retorna una copia de las métricas de todos los modelos.
func (o *Orquestador) Metricas() []MetricasModelo {
        o.mu.RLock()
        defer o.mu.RUnlock()
        resultado := make([]MetricasModelo, 0, len(o.metricas))
        for _, m := range o.metricas {
                copia := *m
                if copia.TotalSolicitudes > 0 {
                        copia.TasaExito = float64(copia.Exitos) / float64(copia.TotalSolicitudes)
                }
                resultado = append(resultado, copia)
        }
        return resultado
}
