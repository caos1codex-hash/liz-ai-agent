package pipeline

import (
	"encoding/json"
	"strings"
)

// extraerJSON intenta extraer un objeto JSON de un texto que puede contener
// texto adicional antes o después del JSON.
func extraerJSON(texto string) string {
	// Buscar el primer { y el último }
	inicio := strings.Index(texto, "{")
	fin := strings.LastIndex(texto, "}")
	if inicio == -1 || fin == -1 || fin <= inicio {
		// Intentar con array
		inicio = strings.Index(texto, "[")
		fin = strings.LastIndex(texto, "]")
		if inicio == -1 || fin == -1 || fin <= inicio {
			return ""
		}
	}
	return texto[inicio : fin+1]
}

// parsearJSON es un helper para deserializar JSON string a un struct.
func parsearJSON(data string, v interface{}) error {
	return json.Unmarshal([]byte(data), v)
}

// serializarJSON convierte un valor a JSON string.
func serializarJSON(v interface{}) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
