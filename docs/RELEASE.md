# Proceso de Release — Liz AI Agent

> **Fase 10 — Release v0.1.0**
>
> Este documento describe cómo se construyen, publican y verifican los releases
> de Liz. Toda la pipeline está automatizada vía GitHub Actions.

## Filosofía

> **"Si no está en GitHub, no existe."**

Todo cambio se commitea y pushea lo antes posible. Todo release se taggea y
se publica con assets verificables (binarios + checksums). Todo se documenta
en `docs/` y `CHANGELOG.md`.

## Versionado semántico

Liz usa [SemVer](https://semver.org/lang/es/): `MAJOR.MINOR.PATCH`

- **MAJOR** (0.x.x → 1.0.0): Cambios incompatibles en la API. Hasta 1.0.0, cualquier versión puede romper compatibilidad.
- **MINOR** (0.1.x → 0.2.0): Nuevas features retrocompatibles. Cada Fase del roadmap corresponde a un bump MINOR.
- **PATCH** (0.1.0 → 0.1.1): Bug fixes retrocompatibles.

**Convención actual:**
- Fase 9 → `v0.10.0` (desarrollo)
- Fase 10 → `v0.10.0` (release estable, primer release público)
- Próximas fases → `v0.11.0`, `v0.12.0`, ... hasta `v1.0.0` (estable)

> Nota: El issue #18 menciona "v0.1.0" como tag del primer release, pero tras
> la discusión del equipo se adopta `v0.10.0` para mantener continuidad con
> la numeración de desarrollo (0.9.0 Fase 8 → 0.10.0 Fase 9/10). El CHANGELOG
> registra la entrada como `[0.1.0]` para preservar la referencia histórica.

## Pipeline de release (CI/CD)

### Workflow `ci.yml` (push/PR triggered)

Se ejecuta en cada push a `main` y en cada Pull Request. Verifica:

1. **go vet** (headless + nucleo) — análisis estático
2. **go build** (headless, CGO=0) — compila binario estático
3. **go test** (paquetes estables) — logger, config, permisos, memoria, orquestador, herramientas
4. **gofmt check** — todos los archivos .go formateados
5. **build desktop** — compila binario con GUI Fyne (instala deps OpenGL)
6. **smoke test** — arranca servidor headless y verifica que responde

### Workflow `release.yml` (tag triggered)

Se ejecuta al pushear un tag `v*.*.*`. Pipeline de 4 jobs:

#### Job 1: `build-binaries` (matrix strategy, 5 targets paralelos)

| Target | GOOS | GOARCH | CGO | Tags | Output |
|--------|------|--------|-----|------|--------|
| desktop-linux-amd64 | linux | amd64 | 1 | (none) | `liz-linux-amd64` (~30MB) |
| headless-linux-amd64 | linux | amd64 | 0 | headless | `liz-server-linux-amd64` (~7MB) |
| headless-linux-arm64 | linux | arm64 | 0 | headless | `liz-server-linux-arm64` (~7MB) |
| headless-darwin-amd64 | darwin | amd64 | 0 | headless | `liz-server-darwin-amd64` (~8MB) |
| headless-darwin-arm64 | darwin | arm64 | 0 | headless | `liz-server-darwin-arm64` (~8MB) |

Cada target:
- Usa `actions/setup-go@v5` con Go 1.22 + cache de módulos
- Compila con `-trimpath -ldflags="-s -w -X main.version=..."`
- Verifica que el binario arranca con `--version` (solo linux/amd64)
- Sube como artifact de GitHub Actions

#### Job 2: `package` (depende de `build-binaries`)

Descarga todos los artifacts y crea:

- **Tarballs**: `<binario>-v<ver>.tar.gz` con binario + config de ejemplo + README + LICENSE + install.sh + uninstall.sh
- **Checksums**: `checksums-v<ver>.txt` con SHA-256 de cada tarball
- **DEB**: `liz_<ver>_amd64.deb` usando `dpkg-deb` con control file + postinst + prerm + .desktop entry
- **RPM**: `liz-<ver>-1.x86_64.rpm` usando `rpmbuild` con spec file completo

#### Job 3: `docker` (depende de `build-binaries`)

Construye y publica imagen Docker multi-arch:

- Usa `docker/setup-qemu-action` + `docker/setup-buildx-action`
- Plataformas: `linux/amd64, linux/arm64`
- Multi-stage build: `golang:1.22-alpine` → `distroless/static-debian12:nonroot`
- Publica en `ghcr.io/caos1codex-hash/liz-ai-agent` con 3 tags:
  - `v<ver>` (ej: `v0.10.0`)
  - `latest`
  - `sha-<commit>` (para trazabilidad)
- Cache de layers vía `cache-from/cache-to: type=gha`

#### Job 4: `release` (depende de los 3 anteriores)

Crea el GitHub Release:

- Descarga todos los artifacts (binarios + tarballs + DEB + RPM + checksums)
- Genera notas de release con tabla de plataformas, instrucciones de instalación y Docker
- Usa `softprops/action-gh-release@v2` para crear el release
- Sube todos los assets
- `generate_release_notes: true` añade el changelog de commits automáticamente

## Cómo publicar un nuevo release

### Paso a paso

```bash
# 1. Asegúrate de estar en main y actualizado
git checkout main
git pull origin main

# 2. Bump de versión en cmd/liz/main.go
#    (cambiar `const version = "0.10.0"` → "0.11.0" por ejemplo)
vim cmd/liz/main.go

# 3. Actualizar CHANGELOG.md con la nueva entrada
vim CHANGELOG.md

# 4. Actualizar docs/ARQUITECTURA.md roadmap si aplica
vim docs/ARQUITECTURA.md

# 5. Commit y push
git add -A
git commit -m "release: vX.Y.Z — descripción corta"
git push origin main

# 6. Crear tag anotado
git tag -a vX.Y.Z -m "Liz vX.Y.Z — Fase N: descripción"

# 7. Push del tag (¡esto dispara el workflow de release!)
git push origin vX.Y.Z

# 8. Monitorear el workflow
#    https://github.com/caos1codex-hash/liz-ai-agent/actions
```

### Atajos del Makefile

```bash
# Crear tag y pushear (automático)
make release-tag

# Preparar dist/ local + crear tag y pushear
make release
```

## Verificación post-release

Después de que el workflow `release.yml` complete exitosamente:

### 1. Verificar GitHub Release

- URL: `https://github.com/caos1codex-hash/liz-ai-agent/releases/tag/vX.Y.Z`
- Assets esperados (para v0.10.0):
  - `liz-linux-amd64` (~30MB)
  - `liz-server-linux-amd64` (~7MB)
  - `liz-server-linux-arm64` (~7MB)
  - `liz-server-darwin-amd64` (~8MB)
  - `liz-server-darwin-arm64` (~8MB)
  - `liz-linux-amd64-v0.10.0.tar.gz`
  - `liz-server-linux-amd64-v0.10.0.tar.gz`
  - `liz-server-linux-arm64-v0.10.0.tar.gz`
  - `liz-server-darwin-amd64-v0.10.0.tar.gz`
  - `liz-server-darwin-arm64-v0.10.0.tar.gz`
  - `liz_0.10.0_amd64.deb`
  - `liz-0.10.0-1.x86_64.rpm`
  - `checksums-v0.10.0.txt`

### 2. Verificar imagen Docker

```bash
docker pull ghcr.io/caos1codex-hash/liz-ai-agent:vX.Y.Z
docker run --rm ghcr.io/caos1codex-hash/liz-ai-agent:vX.Y.Z --version
docker inspect ghcr.io/caos1codex-hash/liz-ai-agent:vX.Y.Z | grep -A5 Architecture
```

### 3. Smoke test de instalación limpia

En una VM o contenedor limpio:

```bash
# Ubuntu 22.04
docker run --rm -it ubuntu:22.04 bash
curl -fsSL https://github.com/caos1codex-hash/liz-ai-agent/releases/latest/download/install.sh | bash -s -- --headless --no-deps
liz-server --version

# Fedora 38
docker run --rm -it fedora:38 bash
curl -fsSL https://github.com/caos1codex-hash/liz-ai-agent/releases/latest/download/install.sh | bash -s -- --headless --no-deps
liz-server --version
```

### 4. Verificar checksums

```bash
curl -fsSL https://github.com/caos1codex-hash/liz-ai-agent/releases/download/v0.10.0/checksums-v0.10.0.txt -o checksums.txt
# Descargar uno de los tarballs
curl -fsSL -O https://github.com/caos1codex-hash/liz-ai-agent/releases/download/v0.10.0/liz-server-linux-amd64-v0.10.0.tar.gz
sha256sum -c checksums.txt --ignore-missing
```

## Rollback

Si un release tiene un bug crítico:

### Opción A: Marcar como `pre-release` y re-publicar

```bash
# 1. En GitHub UI: editar release → marcar "This is a pre-release"
# 2. Fix del bug en main
# 3. Bump de versión PATCH: 0.10.0 → 0.10.1
# 4. Tag y push nuevo release
```

### Opción B: Eliminar release y tag (solo si es crítico y reciente)

```bash
# 1. Eliminar GitHub Release (vía UI o API)
gh release delete v0.10.0 --yes

# 2. Eliminar tag local y remoto
git tag -d v0.10.0
git push origin :refs/tags/v0.10.0

# 3. Fix del bug
# 4. Re-crear tag y push (re-ejecuta el workflow)
```

> **Advertencia**: Eliminar un tag publicado puede romper a usuarios que ya
> lo referencian. Solo usar en casos críticos y comunicar el cambio.

## Branches y tags

- `main` — branch de desarrollo activo. Siempre debe compilar y pasar tests.
- `fase-N` — branches de trabajo para fases grandes (ej: `fase-10`). Se merguean a main al terminar.
- `v*.*.*` — tags de release. Públicos e inmutables (no se reescriben).
- `v*.*.*-rc.N` — release candidates (pre-releases).
- `v*.*.*-beta.N` — betas públicas para feedback temprano.

## Convenciones de commit

Usamos [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

Tipos:
- `feat`: nueva feature (bump MINOR en release)
- `fix`: bug fix (bump PATCH)
- `docs`: cambios en documentación
- `test`: añadir/modificar tests
- `refactor`: refactor sin cambio de comportamiento
- `perf`: mejora de performance
- `chore`: tareas de mantenimiento
- `ci`: cambios en CI/CD
- `build`: cambios en build system / dependencias

Scope: fase o paquete afectado, ej: `feat(P10.A.1): build tag headless`

Footer: `Fase 10 — Issue #18: Release v0.1.0`

## Checklist de release

Antes de taggear un nuevo release:

- [ ] `cmd/liz/main.go` tiene la nueva versión
- [ ] `CHANGELOG.md` tiene la entrada del nuevo release
- [ ] `docs/ARQUITECTURA.md` roadmap actualizado
- [ ] `README.md` badges y secciones actualizadas
- [ ] `make test-stable` pasa sin errores
- [ ] `make build-headless` produce binario que arranca
- [ ] `make cross-compile` produce todos los binarios esperados
- [ ] `make lint` (vet + gofmt) pasa
- [ ] No hay commits WIP o TODO críticos sin resolver
- [ ] Branch `main` está sincronizada con `origin/main`
- [ ] Tag anotado creado con mensaje descriptivo
- [ ] Push del tag monitoreado en GitHub Actions
- [ ] GitHub Release verificado con todos los assets
- [ ] Imagen Docker multi-arch verificada en ghcr.io
- [ ] Smoke test de instalación limpia pasado
- [ ] Issue de la fase cerrado en GitHub

---

> **Filosofía**: "Si no está en GitHub, no existe."
> Si tienes dudas sobre el proceso de release, abre un issue con label `question`.
