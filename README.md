# 📅 Event Hub (Versión Ecosistema Híbrido)

[![Go Version](https://img.shields.io/badge/Go-1.26.4-00ADD8?style=flat-square&logo=go)](https://golang.org/)
[![Gin Framework](https://img.shields.io/badge/Framework-Gin_v1.12.0-008080?style=flat-square)](https://gin-gonic.github.io/)
[![Database](https://img.shields.io/badge/Database-PostgreSQL-336791?style=flat-square&logo=postgresql)](https://www.postgresql.org/)
[![Mobile Framework](https://img.shields.io/badge/Mobile-Flutter_/_Dart-02569B?style=flat-square&logo=flutter)](https://flutter.dev/)
[![AI Integration](https://img.shields.io/badge/AI-Google_Gemini-8E75C2?style=flat-square&logo=googlegemini)](https://deepmind.google/technologies/gemini/)

Event Hub es un ecosistema de software diseñado bajo un patrón arquitectónico híbrido que unifica una plataforma web nativa y una aplicación móvil multiplataforma. Permite gestionar, buscar y sincronizar eventos de manera inteligente a través de la integración de inteligencia artificial y servicios en la nube.

---

## 📖 Índice
- [1. Información General del Sistema](#1-información-general-del-sistema)
- [2. Pilares del Alcance Técnico](#2-pilares-del-alcance-técnico)
- [3. Arquitectura Modular del Sistema](#3-arquitectura-modular-del-sistema)
- [4. Estructura del Proyecto](#4-estructura-del-proyecto)
- [5. Diseño de la Base de Datos](#5-diseño-de-la-base-de-datos)
- [6. Matriz de Componentes Tecnológicos](#6-matriz-de-componentes-tecnológicos)
- [7. Plan de Desarrollo por Fases](#7-plan-de-desarrollo-por-fases)
- [8. Guía de Instalación y Configuración](#8-guía-de-instalación-y-configuración)
- [9. Ejecución de Pruebas](#9-ejecución-de-pruebas)
- [10. Gestión de Issues (GitHub)](#10-gestión-de-issues-github)
- [11. Buenas Prácticas y Código Limpio](#11-buenas-prácticas-y-código-limpio)
- [12. Créditos](#12-créditos)

---

## 1. Información General del Sistema
- **Nombre del Software:** Event Hub (Versión ECOSISTEMA)
- **Enfoque Arquitectónico:** MVC (Model-View-Controller) con Capa de Servicios Centralizada y API REST para Clientes Híbridos.
- **Stack Tecnológico Principal:** Go (Gin Framework) & PostgreSQL.
- **Ecosistema de Clientes:**
  - **Cliente Web:** HTML5 / Plantillas Nativas de Go (`html/template`) con estilo Glassmorphism y Tailwind CSS.
  - **Cliente Móvil:** Dart (Flutter Framework) para dispositivos Android e iOS.
- **APIs de Terceros:** Google Cloud Console (OAuth 2.0, Google Calendar API), Google Gemini API SDK.
- **Diseñadores Principales:** Sebastian Piñango, Edgar Gutiérrez

---

## 2. Pilares del Alcance Técnico

### A. Autenticación Delegada (OAuth 2.0)
Eliminación de los formularios clásicos de registro manual. El sistema delega la gestión de identidad a **Google** y **GitHub** de manera segura para obtener el nombre, correo electrónico y avatar, permitiendo el aprovisionamiento y creación de perfiles con un solo clic.

### B. Generación de Contenido por IA (Gemini API)
Asistente inteligente integrado en el formulario de creación de eventos. El organizador ingresa el título y ubicación de un evento, y al presionar "Sugerir con IA", el SDK de Gemini procesa un prompt estructurado mediante el modelo **gemini-2.5-flash**, devolviendo una descripción profesional, detallada y formateada de forma instantánea.

### C. Dashboard de Búsqueda Indexada y Filtrado Dinámico
Un motor de búsqueda en la cartelera principal que combina filtros por categorías y búsqueda por palabras clave en tiempo real. Todo el filtrado se procesa en el servidor PostgreSQL mediante queries optimizadas con operadores `ILIKE` y `JOINs` para evitar sobrecargar al cliente.

### D. Sincronización con Google Calendar
Al crear un evento, el sistema se conecta a la API de Google Calendar usando las credenciales del organizador, insertando el evento con recordatorios y notificaciones automáticas. Al inscribirse un participante en el sistema local, el backend añade de forma asíncrona su correo electrónico a la lista de asistentes (`attendees`) del evento en Google Calendar usando métodos `PATCH`/`UPDATE`.

### E. Expansión a Ecosistema Móvil (Flutter)
Aplicación móvil nativa construida en Flutter que consume la misma lógica de negocio del backend a través de endpoints JSON protegidos con JWT.

---

## 3. Arquitectura Modular del Sistema

```mermaid
graph TD
    subgraph Clientes [Clientes / Vista]
        Web[Vista Web: HTML + Tailwind]
        Mobile[Cliente Móvil: Flutter]
    end

    subgraph Backend [Backend Go - Gin]
        Router[Router de Gin]
        Middlewares[Middlewares JWT / Sesión]
        Controller[Controladores / API Endpoints]
        Model[Modelos / Persistencia SQL]
    end

    subgraph Servicios [Capa de Servicios]
        OAuthSvc[OAuth Service]
        GeminiSvc[Gemini Service]
        CalendarSvc[Google Calendar Service]
    end

    subgraph BD [Base de Datos]
        Postgres[(PostgreSQL)]
    end

    subgraph Externos [APIs de Terceros]
        GoogleOAuth[Google Cloud OAuth]
        GitHubOAuth[GitHub OAuth]
        GeminiAPI[Gemini Developer API]
        CalendarAPI[Google Calendar API]
    end

    %% Flujos de conexión
    Web -->|HTTP Requests| Router
    Mobile -->|JSON REST Requests| Router
    Router --> Middlewares
    Middlewares --> Controller
    Controller --> Model
    Controller --> Servicios
    Model --> Postgres
    
    Servicios --> GoogleOAuth
    Servicios --> GitHubOAuth
    Servicios --> GeminiAPI
    Servicios --> CalendarAPI
```

- **Modelos (M):** Contiene los Structs de Go y las sentencias SQL nativas que interactúan con PostgreSQL, utilizando optimizaciones del pool de conexiones sin ORMs pesados.
- **Vistas (V) y Clientes:** La interfaz web utiliza vistas server-side inyectadas con Tailwind CSS. La app móvil (Flutter) consume la lógica de negocio a través de la API REST de manera desacoplada.
- **Controladores (C):** Controladores de Gin con doble responsabilidad: renderizan plantillas HTML para navegadores web y devuelven respuestas estructuradas JSON cuando se detecta el header `Accept: application/json`.
- **Servicios:** Centralizan el consumo de las APIs de terceros (OAuth, Gemini, Calendar) independientemente del canal (web o móvil) que realice la solicitud.

---

## 4. Estructura del Proyecto

```text
eventHubBack/
├── config/
│   └── database.go           # Pool de conexiones pgxpool
├── controllers/
│   ├── auth_controller.go     # Login, Register y callbacks OAuth
│   ├── dashboard_controller.go# Renderización de la cartelera principal
│   └── event_controller.go    # Creación de eventos y endpoint de Gemini
├── middlewares/
│   ├── session_middleware.go  # Autenticación segura mediante cookies y JWT
│   └── session_middleware_test.go # Pruebas unitarias de JWT y middlewares
├── models/
│   ├── categoria.go           # Modelo y consultas de categorías
│   ├── evento_categoria.go    # Relación muchos a muchos (join table)
│   ├── evento.go              # Transacciones y búsqueda de eventos indexados
│   ├── role.go                # Modelo y consultas de roles
│   └── usuario.go             # Almacenamiento, hashing y autenticación
├── services/
│   ├── gemini_service.go      # Cliente oficial de Google Gemini SDK (gemini-2.5-flash)
│   ├── oauth_service.go       # Flujo OAuth 2.0 (Google y GitHub)
│   └── oauth_service_test.go  # Pruebas unitarias del flujo de OAuth
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

## 5. Diseño de la Base de Datos

El esquema relacional en PostgreSQL está diseñado para garantizar atomicidad y rapidez de acceso, soportando restricciones del negocio (como cupo máximo) bajo transacciones aisladas.

### Elementos Clave
- **Índices de Búsqueda:** Índices en `eventos.titulo`, `eventos.fecha`, `usuarios.email` y en las llaves foráneas de `evento_categorias` para evitar escaneos secuenciales masivos.
- **Sincronización:** Almacenamiento directo del identificador externo `calendar_event_id` devuelto por Google Calendar en la tabla `eventos`.
- **Restricciones Check:** Control de cupos máximos en transacciones concurrentes mediante `SELECT FOR UPDATE` para evitar sobreventa.

---

## 6. Matriz de Componentes Tecnológicos

| Componente Técnico | Capa del Sistema | Función y Comportamiento |
| :--- | :--- | :--- |
| **Gin HTTP Context** | Controlador / API | Captura parámetros de búsqueda, mapea JSONs de Flutter, gestiona cookies seguras y maneja callbacks OAuth. |
| **PostgreSQL Engine** | Base de Datos | Operaciones atómicas bajo transacciones (`BEGIN`, `COMMIT`), restricciones de cupo y búsquedas indexadas. |
| **Gemini Go SDK** | Capa de Servicio | Establece conexión con el modelo `gemini-2.5-flash` para generar descripciones inteligentes basadas en títulos y locaciones. |
| **Google Calendar API Client** | Capa de Servicio | Inserta eventos en los calendarios de los organizadores y añade asistentes dinámicamente mediante `PATCH`. |
| **Tailwind CSS Engine** | Vista Web | Estilo responsivo con efectos translúcidos (Glassmorphism), filtros intuitivos y formularios dinámicos. |
| **Flutter SDK (Dart)** | Cliente Móvil | Gestiona el estado local, consume los endpoints JSON de Gin y realiza el flujo OAuth nativo. |

---

## 7. Plan de Desarrollo por Fases

### Fase 1: Infraestructura Migrada y Esquema Relacional
- Inicialización del entorno del servidor Go y descarga de dependencias núcleo (`gin-gonic/gin`, `pgxpool`, `godotenv`, etc.).
- Ejecución del script DDL `schema.sql` en PostgreSQL, preparando columnas nulables para perfiles de redes sociales y campos de Google Cloud.

### Fase 2: Flujo de Acceso sin Fricción (OAuth 2.0)
- Configuración de credenciales de cliente OAuth (Google e ID de GitHub) en variables de entorno.
- Implementación de flujos de consentimiento y callbacks. Aprovisionamiento automático en PostgreSQL ante el primer ingreso exitoso.

### Fase 3: Asistente de Descripciones Automatizadas (Gemini API)
- Aislamiento de las credenciales de Gemini.
- Creación del servicio en Go `services/gemini_service.go` conectado a `gemini-2.5-flash`.
- Integración AJAX en el formulario web para que el botón "Sugerir con IA" autocomplete la descripción del evento.

### Fase 4: Creación de Eventos y Sincronización con Google Calendar
- Implementación del formulario con campos de fecha/hora formateados.
- Desarrollo del servicio `calendar_service.go` para insertar el evento en el calendario de Google en tiempo real y guardar el `calendar_event_id` retornado.

### Fase 5: Dashboard Interactivo e Inscripción de Asistentes
- Construcción de las vistas web con Tailwind CSS y filtros combinados indexados.
- Gestión transaccional de cupos con `SELECT FOR UPDATE` para evitar sobreventa.
- Inyección asíncrona de inscritos en el array `attendees` del evento de Google Calendar mediante llamadas `PATCH` a la API.

### Fase 6: Expansión a Aplicación Móvil (Flutter)
- Creación del proyecto móvil Flutter organizando las capas de Data, Domain y Presentation.
- Refactorización de controladores en el backend de Go para admitir peticiones JSON.
- Integración de login OAuth en la app móvil y sincronización segura de sesión mediante JWT.

---

## 8. Guía de Instalación y Configuración

### Prerrequisitos
- **Go** (Versión 1.26.4 o compatible)
- **PostgreSQL** (Local o en la nube mediante Neon)
- **Variables de Entorno** configuradas en un archivo `.env` en la raíz del proyecto.

### Ejemplo de Archivo `.env`
```env
PORT=8080
DATABASE_URL=postgres://usuario:contraseña@host:puerto/base_datos?sslmode=require
JWT_SECRET=tu-clave-secreta-para-firmar-tokens
GEMINI_API_KEY=tu-api-key-de-google-gemini

# Google Cloud OAuth 2.0 Credentials
GOOGLE_CLIENT_ID=tu-google-client-id
GOOGLE_CLIENT_SECRET=tu-google-client-secret
GOOGLE_REDIRECT_URL=http://localhost:8080/auth/google/callback

# GitHub OAuth 2.0 Credentials
GITHUB_CLIENT_ID=tu-github-client-id
GITHUB_CLIENT_SECRET=tu-github-client-secret
GITHUB_REDIRECT_URL=http://localhost:8080/auth/github/callback
```

### Ejecutar Servidor Localmente
1. Clona el repositorio.
2. Crea e inicializa tu base de datos utilizando el archivo `schema.sql`.
3. Configura el archivo `.env`.
4. Descarga dependencias y ejecuta la aplicación:
   ```bash
   go mod download
   go run main.go
   ```

---

## 9. Ejecución de Pruebas

Para garantizar la estabilidad del proyecto, el código incluye suites de pruebas automatizadas que verifican la lógica de negocio sin realizar llamadas de red externas o requerir bases de datos locales.

Ejecuta el conjunto de pruebas con el siguiente comando:
```bash
go test -v ./...
```

---

## 10. Gestión de Issues (GitHub)

Para el correcto mantenimiento del repositorio, se definen plantillas estrictas para la creación de Issues:

### 🟢 ISSUE TIPO 1: FEATURE
- **Título sugerido:** `feat: [Módulo] Breve descripción en infinitivo`
- **Descripción:** Explicar el requerimiento `[X]` en el módulo `[Y]` para cumplir con el alcance `[Z]`.
- **Tareas Técnicas:** Checklist con la configuración de envs, lógica en `/services`, controladores en `/controllers` y vistas.
- **Criterios de Aceptación:** Diseño Mobile-First en web / seguro en móviles, no variables hardcodeadas, y control estructurado de fallos de APIs externas.

### 🔴 ISSUE TIPO 2: BUG
- **Título sugerido:** `fix: [Módulo] Descripción corta del error`
- **Descripción:** Detallar el comportamiento anómalo detectado.
- **Pasos para reproducir:** Instrucciones paso a paso.
- **Esperado vs Actual:** Comparación del flujo correcto contra la excepción actual.
- **Solución Propuesta:** Control preventivo con `if err != nil`.

### 🔵 ISSUE TIPO 3: REFACTOR
- **Título sugerido:** `refactor: [Módulo] Optimización o limpieza de código`
- **Descripción:** Mejoras que no alteran la funcionalidad del sistema.
- **Objetivos:** Reusabilidad de layouts/widgets, optimización de consultas SQL indexadas, liberación segura de recursos con `defer rows.Close()`.

---

## 11. Buenas Prácticas y Código Limpio
1. **Manejo Explicito de Recursos:** Cada consulta o fila abierta de base de datos se cierra explícitamente usando `defer rows.Close()`.
2. **Control Inmediato de Errores:** Se validan todos los errores inmediatamente tras su ocurrencia (`if err != nil`).
3. **Bindings Seguros:** Bindeo estricto utilizando tags explícitos `form` y `json` en los Structs de Go.
4. **Resiliencia ante Fallas:** El fallo de un servicio externo (como Google Calendar o Gemini) no debe congelar o provocar pánico en el servidor. Debe registrarse en logs y alertar amigablemente al cliente final.

---

## 12. Créditos
- **Diseñadores y Desarrolladores Principales:** Sebastian Piñango, Edgar Gutiérrez.
- **Tecnologías:** Golang, Gin Framework, pgxpool, Flutter, Google Cloud Platform, Gemini API.
