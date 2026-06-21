# ⚙️ Event Hub Backend Plugin

Este plugin contiene un conjunto de **habilidades (skills)** y **subagentes (agents)** diseñados para optimizar el desarrollo, la auditoría de seguridad y el testeo unitario/de integración del backend del ecosistema **Event Hub** (escrito en Go y Gin con PostgreSQL).

---

## 📂 Estructura del Plugin

```text
eventhub-backend-plugin/
├── plugin.json                # Manifiesto del plugin
├── README.md                  # Esta documentación interna
├── agents/                    # Subagentes especializados (Personas)
│   ├── db-migration-agent.md   # Experto en DB relacional y pgx
│   ├── go-security-auditor.md  # Auditor de seguridad, OAuth y JWT
│   └── integration-api-agent.md# Especialista en APIs externas y mocks
└── skills/                    # Habilidades y guías procedimentales
    ├── go-backend-architect/
    │   └── SKILL.md           # Patrones de Go/Gin, cierres de recursos y errores
    ├── api-client-testing/
    │   └── SKILL.md           # Guías de testeo y simulación de red
    └── database-migration/
        └── SKILL.md           # Transacciones concurrentes e integridad
```

---

## 🛠️ Cómo Usar las Skills y Agentes

### 1. Uso de Habilidades (Skills)
Las habilidades se cargan automáticamente en el contexto del agente de IA principal. Cuando estés editando código de backend:
* **`go-backend-architect`**: Se activará o referenciará de forma automática cuando solicites refactorizar controladores de Gin, estructurar servicios o auditar el flujo de errores (`if err != nil`) y liberación de recursos (`defer rows.Close()`).
* **`api-client-testing`**: Pídela explícitamente o el agente la usará al escribir tests para llamadas HTTP a APIs como Google Calendar, GitHub/Google OAuth, o Gemini Developer API.
* **`database-migration`**: Utilizada para diseñar esquemas SQL o crear queries atómicas concurrentes.

### 2. Invocación de Subagentes (Agents)
Puedes delegar tareas específicas a los subagentes especializados utilizando comandos en tu chat:
* **`db-migration-agent`**: Para tareas complejas de base de datos.
  * *Ejemplo:* "Delega a `db-migration-agent` la optimización de las queries y la creación de índices en la tabla de participantes de `schema.sql`."
* **`integration-api-agent`**: Para conectar nuevos endpoints a servicios de terceros.
  * *Ejemplo:* "Pide a `integration-api-agent` que implemente un nuevo servicio de prueba para sincronizar eventos con Outlook Calendar."
* **`go-security-auditor`**: Para auditar la robustez del código.
  * *Ejemplo:* "Usa `go-security-auditor` para analizar la seguridad de las cookies de sesión en `session_middleware.go`."

---

## ✍️ Cómo Modificar, Mejorar o Agregar Elementos

Este plugin es totalmente extensible y modular. Sigue estas pautas para mantenerlo al día:

### A. Modificar o Mejorar una Skill/Agente Existente
1. Abre el archivo correspondiente (por ejemplo, [database-migration/SKILL.md](skills/database-migration/SKILL.md) o [db-migration-agent.md](agents/db-migration-agent.md)).
2. Modifica el prompt, las directrices, los ejemplos de código o los prerrequisitos según las nuevas necesidades del proyecto.
3. El agente de IA recargará las especificaciones en el siguiente turno.

### B. Agregar una Nueva Skill (Habilidad)
Las habilidades contienen guías paso a paso de resolución (CUJs) para el agente.
1. Crea un directorio en `skills/` en minúsculas y separado por guiones (kebab-case):
   ```bash
   mkdir -p skills/mi-nueva-habilidad
   ```
2. Crea el archivo obligatorio `SKILL.md` dentro de la carpeta:
   ```bash
   touch skills/mi-nueva-habilidad/SKILL.md
   ```
3. Define el archivo con **YAML Frontmatter** al inicio y cuerpo en Markdown:
   ```markdown
   ---
   name: mi-nueva-habilidad
   description: Explicación corta y concisa de cuándo debe activarse esta habilidad (ej. "Guías para estructurar micro-animaciones").
   ---

   # Guía de la Habilidad
   Aquí van los principios, mejores prácticas, comandos a usar y checklists específicos.
   ```

### C. Agregar un Nuevo Subagente (Agent)
Los subagentes definen identidades o "personas" con un enfoque y rol definidos.
1. Crea un archivo `.md` en la carpeta `agents/`:
   ```bash
   touch agents/mi-nuevo-agente.md
   ```
2. Estructura el archivo con **YAML Frontmatter** y el prompt del sistema:
   ```markdown
   ---
   name: mi-nuevo-agente
   description: Propósito y enfoque del agente.
   model: gemini-3.5-flash  # Opcional
   ---

   # Instrucciones del Sistema para Mi Nuevo Agente
   Eres un agente experto en...
   Tus responsabilidades son:
   1. ...
   ```

---

## 📜 Reglas del Ciclo de Vida del Plugin
* **Actualizar `plugin.json`**: Si agregas nuevas dependencias o realizas cambios significativos, incrementa la propiedad `"version"` en [plugin.json](plugin.json).
* **Usar Enlaces de Archivos (Rutas Relativas)**: Al escribir guías, incluye enlaces clickables a los archivos utilizando rutas relativas al proyecto o al plugin (ej. `[nombre](file://ruta/relativa)` o `[nombre](ruta/relativa)`). Nunca incluyas rutas absolutas de tu máquina local en los documentos ni plugins.
