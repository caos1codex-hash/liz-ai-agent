# Guía de Configuración de Liz

## Archivo de Configuración

Liz usa un archivo YAML ubicado en `~/.liz/liz.yaml` para toda su configuración.
Si no existe, Liz crea uno con valores por defecto al primer inicio.

## Estructura Completa

```yaml
liz:
  nombre: "Liz"
  version: "0.9.0"
  puerto: 3000
  host: "localhost"
  log_nivel: "INFO"

  permisos:
    habilitado: true
    nivel: "total"  # "total" o "restringido"

  orquestador:
    api_key: "nvapi-..."
    modelo_por_defecto: "nvidia/llama-3.1-nemotron-70b-instruct"
    timeout_segundos: 30
    max_tokens: 4096
    temperatura: 0.7

  contexto:
    directorio_base: "~"
    max_fragmentos_por_archivo: 50
    max_tokens_contexto: 8000
    cache_habilitado: true
```

## Sección `liz`

| Campo | Tipo | Default | Descripción |
|-------|------|---------|-------------|
| `nombre` | string | `"Liz"` | Nombre del agente |
| `version` | string | `"0.9.0"` | Versión actual |
| `puerto` | int | `3000` | Puerto del servidor HTTP |
| `host` | string | `"localhost"` | Host de escucha (`0.0.0.0` para red) |
| `log_nivel` | string | `"INFO"` | Nivel mínimo de log: `DEBUG`, `INFO`, `WARN`, `ERROR` |

## Sección `permisos`

| Campo | Tipo | Default | Descripción |
|-------|------|---------|-------------|
| `habilitado` | bool | `true` | Activa el sistema de permisos |
| `nivel` | string | `"total"` | `"total"` (todo permitido) o `"restringido"` (pregunta) |

### Nivel `total`

Cuando `nivel: "total"`, Liz tiene permisos completos desde el inicio.
Nunca vuelve a preguntar. Este es el modo recomendado para uso personal.

### Nivel `restringido`

Cuando `nivel: "restringido"`, Liz pedirá confirmación antes de ejecutar
operaciones potencialmente peligrosas (eliminar archivos, matar procesos, instalar software).

## Sección `orquestador`

| Campo | Tipo | Default | Descripción |
|-------|------|---------|-------------|
| `api_key` | string | `""` | API key de NVIDIA (requerida para IA) |
| `modelo_por_defecto` | string | auto | Modelo a usar por defecto |
| `timeout_segundos` | int | `30` | Timeout para llamadas al LLM |
| `max_tokens` | int | `4096` | Máximo de tokens en la respuesta |
| `temperatura` | float | `0.7` | Creatividad del modelo (0.0 - 1.0) |

### Obtener API Key de NVIDIA

1. Ir a [build.nvidia.com](https://build.nvidia.com/)
2. Crear cuenta o iniciar sesión
3. Generar API key
4. Copiar y pegar en `liz.yaml`

```bash
# Configurar rápidamente
mkdir -p ~/.liz
cat > ~/.liz/liz.yaml << 'EOF'
liz:
  orquestador:
    api_key: "nvapi-TU_API_KEY_AQUI"
EOF
```

## Sección `contexto`

| Campo | Tipo | Default | Descripción |
|-------|------|---------|-------------|
| `directorio_base` | string | `~` | Directorio base para indexar proyectos |
| `max_fragmentos_por_archivo` | int | `50` | Máximo de fragmentos por archivo |
| `max_tokens_contexto` | int | `8000` | Token budget para contexto empaquetado |
| `cache_habilitado` | bool | `true` | Activa caché de embeddings y resúmenes |

## Variables de Entorno

Liz también soporta variables de entorno como override:

| Variable | Equivalente YAML |
|----------|------------------|
| `LIZ_PUERTO` | `liz.puerto` |
| `LIZ_HOST` | `liz.host` |
| `LIZ_NIVEL_LOG` | `liz.log_nivel` |
| `NVIDIA_API_KEY` | `liz.orquestador.api_key` |

Las variables de entorno tienen prioridad sobre el archivo YAML.

## Ejemplos de Configuración

### Desarrollo Local

```yaml
liz:
  puerto: 3000
  host: "localhost"
  log_nivel: "DEBUG"
  orquestador:
    api_key: "nvapi-..."
    temperatura: 0.9
```

### Servidor de Producción

```yaml
liz:
  puerto: 8080
  host: "0.0.0.0"
  log_nivel: "WARN"
  permisos:
    nivel: "total"
  orquestador:
    api_key: "nvapi-..."
    timeout_segundos: 60
    max_tokens: 8192
```

### Sin IA (Solo Herramientas)

```yaml
liz:
  puerto: 3000
  log_nivel: "INFO"
  # Sin sección orquestador → funciona sin LLM
```

Liz degrada gracefully sin API key. Las 7 herramientas integradas funcionan
sin necesidad de IA. Solo las respuestas inteligentes requieren el orquestador.

## Validación

Liz valida la configuración al inicio. Errores comunes:

- `puerto` fuera de rango (1-65535)
- `temperatura` fuera de rango (0.0-2.0)
- `log_nivel` inválido
- `nivel` de permisos inválido

En caso de error, Liz muestra el problema y termina con código 1.