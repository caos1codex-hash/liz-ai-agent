# Liz Web — Frontend React

Interfaz ChatGPT-clásica para Liz AI Agent. Stack: **React 18 + TypeScript + Vite + Tailwind CSS**.

## Quick start

```bash
# 1. Instalar dependencias
cd web
npm install

# 2. Levantar backend (en otra terminal, desde la raíz del repo)
make dev   # ó: go run ./cmd/liz

# 3. Levantar frontend (Vite dev server con proxy a :3000)
npm run dev
# → http://localhost:5173
```

El dev server de Vite hace proxy de `/api/*` a `http://localhost:3000`, así que
no hay que configurar CORS ni backend URLs.

## Scripts

| Comando | Descripción |
|---------|-------------|
| `npm run dev` | Dev server con HMR en `:5173` |
| `npm run build` | Build producción a `web/dist/` |
| `npm run preview` | Sirve el build de producción localmente |
| `npm run typecheck` | Solo `tsc --noEmit` (validación de tipos) |
| `npm run lint` | ESLint (cuando se añada configuración) |

## Estructura

```
web/
├── public/
│   └── liz.svg                # Logo / favicon
├── src/
│   ├── components/            # Componentes reutilizables (StatusDot, AppShell, ...)
│   ├── hooks/                 # Custom hooks (useBackendHealth, useTheme, ...)
│   ├── lib/
│   │   ├── api.ts             # Cliente fetch base (timeout, errores tipados)
│   │   ├── sse.ts             # Cliente SSE para streaming de chat
│   │   ├── endpoints.ts       # Funciones por dominio (chatApi, orquestadorApi, ...)
│   │   └── utils.ts           # Helpers (cn, formatDuration, formatRelative, ...)
│   ├── pages/                 # Páginas (StatusPage, ChatPage en P8.2, ...)
│   ├── styles/
│   │   └── globals.css        # Tailwind + estilos base
│   ├── types/
│   │   └── api.ts             # Tipos espejo de los structs Go del backend
│   ├── App.tsx                # Root component
│   └── main.tsx               # Entry point
├── index.html
├── package.json
├── tailwind.config.js
├── tsconfig.json
├── tsconfig.node.json
├── vite.config.ts
└── postcss.config.js
```

## Tema

- **Dark por defecto** (persistido en `localStorage`).
- Toggle de tema en P8.4 (header).
- Paleta: morado `liz` (50–950) + tokens semánticos `surface` / `text` / `border`.

## Proxy API

```ts
// vite.config.ts
server: {
  proxy: {
    '/api': { target: 'http://localhost:3000', changeOrigin: true }
  }
}
```

Todas las peticiones a `/api/v1/*` se redirigen al backend Go. En producción,
servir el build desde el mismo origen que el backend (o configurar CORS).

## Roadmap Fase 8

| Parte | Estado | Qué incluye |
|-------|--------|-------------|
| P8.1 | ✅ | Scaffolding, Tailwind, theme, API client, health check |
| P8.2 | ⏳ | Chat core: ChatWindow, MessageList, MessageInput, SSE streaming, Markdown, syntax highlight |
| P8.3 | ⏳ | Sidebar de conversaciones, CRUD sesiones, LocalStorage |
| P8.4 | ⏳ | Header con status/modelo/tools/metrics, selector proyecto, responsive |
| P8.5 | ⏳ | Polish, build setup, Makefile, docs, bump versión |
