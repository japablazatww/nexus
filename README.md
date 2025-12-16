# CentralNexus: Nexus Service & CLI

Este repositorio aloja el **Servidor Central (Nexus)** y la **Smart CLI** de la arquitectura CentralNexus. Actúa como el cerebro que unifica las librerías bajo un servicio centralizado y expone un SDK dinámico.

## Características Clave

1.  **Smart CLI (`nexus-cli`)**: Herramienta que descubre, instala (`go get`) e indexa automáticamente librerías soportadas.
2.  **Proxy Dinámico**: El servidor mapea automáticamente peticiones a las librerías subyacentes, manejando conversiones de nombres (Camel/Pascal/Snake case).
3.  **Generación de Adaptadores**: Crea código Go optimizado (`nexus/generated`) basado en el análisis AST de las librerías.

## Estructura del Repositorio

- **nexus**: Código fuente del servidor y herramientas.
  - `cmd/nexus-cli`: Herramienta principal (Builder + Search).
  - `generated`: Ubicación por defecto del código autogenerado.

## Guía Rápida

### 1. Nexus CLI

La CLI gestiona el ciclo de vida de las librerías y la generación de código.

```bash
# Instalación desde el módulo
$env:GOPROXY="direct"
go install github.com/japablazatww/nexus/nexus/cmd/nexus-cli@develop

# Uso (dentro del repo)
# Uso: Búsqueda
nexus-cli search --search-param user_id
nexus-cli search --search-param LoanRequest    # Búsqueda por Struct
nexus-cli search --search-param Approved       # Búsqueda por Campo de Struct
```

### 2. Ejecución con Docker

Para levantar el servidor Nexus (y opcionalmente el consumidor si está orquestado junto):

```bash
# Desde la raíz donde se encuentre tu docker-compose.yml global
docker-compose up --build
```

Esto levantará el **Nexus Server** en el puerto `8080`.

## Flujo de Trabajo

### Generación de Código
El código dentro de `nexus/generated` es **crítico** y **versionado**.
Se debe regenerar y comitear cuando:
1.  Se actualiza una librería dependencia en `go.mod`.
2.  Se modifica la lógica del generador.

```bash
# Regenerar todo (Catalog + Code)
nexus-cli build

# Solo actualizar Catálogo (Búsqueda) sin regenerar código
nexus-cli build --catalog-only
```

Si estás colaborando, siempre sube los cambios de `nexus/generated` para que otros devs (o el CI/CD) tengan el servidor listo para correr. 
