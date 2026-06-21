# Walkthrough de Inicialización del Proyecto: Event Hub

Este documento detalla la estructura inicial, los componentes de software y las decisiones de diseño arquitectónico tomadas para el proyecto **event-hub** bajo el patrón MVC Extendido en Golang.

---

## ✉️ Prompt Inicial del Usuario

> **Actúa como un Ingeniero de Software Principal especialista en Golang y Arquitectura Limpia. Necesito inicializar un proyecto web utilizando el patrón MVC estructurado a través de Antigravity CLI, optimizado para ejecutarse eficientemente con recursos limitados en entornos de hosting como Render y PostgreSQL en la nube (Neon).**
> 
> Genera el andamiaje del proyecto "event-hub" bajo los siguientes parámetros técnicos estrictos:
> 
> **1. CONFIGURACIÓN DEL MÓDULO Y DEPENDENCIAS NÚCLEO:**
> - Module Name: event-hub
> - Go Version: 1.21 o superior
> - Web Framework: GitHub.com/gin-gonic/gin
> - Database Driver: GitHub.com/jackc/pgx/v5/pgxpool (Pool de conexiones concurrentes para PostgreSQL nativo, sin ORM pesado para maximizar eficiencia).
> - Env Management: GitHub.com/joho/godotenv
> 
> **2. ESTRUCTURA DE CAPAS (MVC EXTENDIDO):**
> Crea de forma estricta los siguientes directorios y archivos base estructurados:
> - `/config`: Incluir `database.go` para inicializar el pool de conexiones con soporte para 'DATABASE_URL' y `sslmode=require`.
> - `/models`: Estructuras puras de Go para mapear las tablas (usuarios, roles, categorias, eventos, evento_categorias). Debe incluir métodos limpios que reciban el contexto (`context.Context`) y ejecuten consultas indexadas.
> - `/controllers`: Handlers de Gin que procesen el contexto HTTP, validen datos en el servidor y consuman la capa de servicios. Incluir `dashboard_controller.go`.
> - `/services`: Capa aislada para lógica de APIs externas. Incluir `gemini_service.go` (cliente para el SDK oficial de Gemini) y `oauth_service.go` (flujo de callback para Google y GitHub OAuth 2.0).
> - `/middlewares`: Middleware de sesión y validación de cookies seguras de autenticación.
> - `/views`: Configuración del motor nativo `html/template` de Gin.
>   * `/views/layouts`: Archivo `base.html` (Layout base que cargue Tailwind CSS vía CDN).
>   * `/views/partials`: Componentes reutilizables (barra de navegación, barra de búsqueda indexada y filtros de categorías).
>   * `/views/dashboard`: Archivo `index.html` para la cartelera de eventos.
> 
> **3. RESTRICCIONES DE CÓDIGO LIMPIO (CLEAN CODE):**
> - Estricto manejo de errores inmediatamente después de cada asignación (`if err != nil`).
> - Asegurar que todas las filas y conexiones de base de datos se liberen explícitamente mediante la cláusula defer (`defer rows.Close()`).
> - Estructuras con tags JSON y de formulario de Gin (`form:"campo" json:"campo"`) explícitos para el bindeo de datos seguro.
> 
> Por favor, genera los comandos de inicialización de Antigravity CLI y las estructuras de archivos vacías con sus respectivos packages de Go declarados de forma limpia.

---

## 🏗️ Arquitectura y Estructura de Directorios

El andamiaje final del repositorio sigue una arquitectura MVC extendida y desacoplada:

```text
eventHubBack/
├── config/
│   └── database.go           # Pool de conexiones pgxpool
├── controllers/
│   ├── auth_controller.go     # Login, Register y callbacks OAuth
│   ├── dashboard_controller.go# Renderización de la cartelera principal
│   └── event_controller.go    # Creación de eventos y endpoint de Gemini
├── middlewares/
│   └── session_middleware.go  # Autenticación segura mediante cookies y JWT
├── models/
│   ├── categoria.go           # Modelo y consultas de categorías
│   ├── evento_categoria.go    # Relación muchos a muchos (join table)
│   ├── evento.go              # Transacciones y búsqueda de eventos indexados
│   ├── role.go                # Modelo y consultas de roles
│   └── usuario.go             # Almacenamiento, hashing y autenticación
├── services/
│   ├── gemini_service.go      # Cliente oficial de Google Gemini SDK (gemini-2.5-flash)
│   └── oauth_service.go       # Flujo OAuth 2.0 (Google y GitHub)
├── views/
│   ├── auth/
│   │   ├── login.html         # Formulario de inicio de sesión
│   │   └── register.html      # Formulario de registro
│   ├── dashboard/
│   │   └── index.html         # Cartelera de eventos
│   ├── events/
│   │   └── create.html        # Formulario de creación con botón Gemini
│   ├── layouts/
│   │   └── base.html          # Contenedor Tailwind CSS CDN y Google Fonts
│   └── partials/
│       ├── category_filter.html # Filtro dinámico de categorías
│       ├── navbar.html        # Navegación adaptable con estado de sesión
│       └── search_bar.html    # Búsqueda preservando el estado de los filtros
├── .env                       # Variables de configuración del entorno
├── go.mod                     # Gestión de dependencias
├── go.sum                     # Checksums de dependencias
├── main.go                    # Inicializador de la aplicación
└── schema.sql                 # Definición de tablas de la base de datos e índices
```

---

## 🛠️ Implementaciones de Capa Clave

### 1. Base de Datos Relacional (`schema.sql` y `config/database.go`)
- **Neon / PostgreSQL en la Nube**: El pool de conexiones de `pgxpool` se limitó a un máximo de 8 conexiones concurrentes en `config/database.go` para evitar exceder las cuotas en planes gratuitos de Neon.
- **Consultas Indexadas**: `schema.sql` define índices en `eventos.fecha`, `eventos.titulo`, `usuarios.email` y en las columnas de la tabla puente `evento_categorias` para garantizar respuestas por debajo de los 10ms.

### 2. Generación Inteligente de Contenido (`services/gemini_service.go`)
Se implementó un cliente integrado con el SDK oficial de **Google Gemini (`github.com/google/generative-ai-go/genai`)**, configurado con el modelo ultra-rápido `gemini-2.5-flash`. Este servicio se utiliza en el controlador de eventos para rellenar de forma inteligente la descripción de un evento a partir de su título y ubicación usando JavaScript asíncrono (`fetch`).

### 3. Autenticación Híbrida (`middlewares/session_middleware.go` y `services/oauth_service.go`)
- **OAuth 2.0 Integrado**: Soporte para autenticación asíncrona a través de las APIs de Google y GitHub.
- **Cookies HTTPOnly y JWT**: Validación mediante JWTs con caducidad de 24 horas y cookies encriptadas. En entornos locales se utiliza `Secure = false`, mientras que en producción cambia dinámicamente a `Secure = true` para forzar HTTPS.

### 4. Vistas Premium y Búsqueda Preservada (`views/`)
- **Estética Glassmorphism**: La cartelera de eventos cuenta con un fondo oscuro translúcido con desenfoque de fondo.
- **Búsqueda Indexada**: La barra de búsqueda y los pills de categoría preservan mutuamente el estado de los parámetros HTTP Query (`search` y `category`) en las URLs de navegación para no perder los filtros.

---

## 💎 Prácticas Estrictas de Código Limpio (Clean Code)

1. **Manejo Explicito de Recursos**: Todos los punteros de filas (`pgx.Rows`) se liberan inmediatamente mediante cláusulas defer (ej. `defer rows.Close()`).
2. **Control de Errores Inmediato**: Cada asignación y llamada del pool de base de datos se acompaña de validaciones `if err != nil`.
3. **Tags de Bindings de Gin**: Las estructuras utilizadas para bindeos de datos seguros en el backend cuentan con tags `json` y `form` explícitos.
