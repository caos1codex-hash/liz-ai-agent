package resumen_llm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/buscador"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/orquestador"
)

// GeneradorResumenLLM genera resúmenes de fragmentos usando LLM.
type GeneradorResumenLLM struct {
	dirCache string
	orch     *orquestador.Orquestador
	mu       sync.Mutex
	logFunc  func(string, ...interface{})
}

// NuevoGeneradorResumenLLM crea un generador. dirCache es la ruta al directorio de cache.
// Si orch es nil, no se generan resúmenes LLM.
func NuevoGeneradorResumenLLM(dirCache string, orch *orquestador.Orquestador) *GeneradorResumenLLM {
	_ = os.MkdirAll(dirCache, 0755)
	return &GeneradorResumenLLM{
		dirCache: dirCache,
		orch:     orch,
		logFunc:  func(string, ...interface{}) {},
	}
}

// ConLog asigna función de log.
func (g *GeneradorResumenLLM) ConLog(fn func(string, ...interface{})) *GeneradorResumenLLM {
	if fn != nil {
		g.logFunc = fn
	}
	return g
}

// GenerarResumen genera un resumen LLM para un fragmento.
// Retorna el resumen cacheado si existe, o genera uno nuevo via LLM.
// Si el orquestador es nil, retorna "".
func (g *GeneradorResumenLLM) GenerarResumen(f buscador.FragmentoBuscable) string {
	if g.orch == nil {
		return ""
	}

	rutaCache := filepath.Join(g.dirCache, f.ID+".txt")
	data, err := os.ReadFile(rutaCache)
	if err == nil && len(data) > 0 {
		return strings.TrimSpace(string(data))
	}

	prompt := fmt.Sprintf("Genera un resumen de 1-2 oraciones para este fragmento de codigo. Describe que hace y su proposito.\n\n```%s\n%s\n```", f.Lenguaje, f.Contenido)

	resp, err := g.orch.Completar(orquestador.SolicitudChat{
		Tarea: orquestador.TareaResumen,
		Mensajes: []orquestador.MensajeChat{
			{Rol: "system", Contenido: "Eres un asistente que genera resúmenes concisos de código fuente. Responde SOLO con el resumen, sin explicaciones adicionales."},
			{Rol: "user", Contenido: prompt},
		},
		MaxTokens:   100,
		Temperatura: 0.3,
	})
	if err != nil || resp == nil || resp.Contenido == "" {
		g.logFunc("error generando resumen LLM para %s: %v", f.ID, err)
		return ""
	}

	resumen := strings.TrimSpace(resp.Contenido)

	g.mu.Lock()
	_ = os.WriteFile(rutaCache, []byte(resumen), 0644)
	g.mu.Unlock()

	return resumen
}

// TieneOrquestador retorna true si hay orquestador disponible.
func (g *GeneradorResumenLLM) TieneOrquestador() bool {
	return g.orch != nil
}
