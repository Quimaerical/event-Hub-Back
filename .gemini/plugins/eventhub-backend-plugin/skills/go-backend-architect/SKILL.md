---
name: go-backend-architect
description: >-
  Provides architectural validation, best practices, and guidelines for writing
  clean, performant, and secure Go and Gin framework backend code. Use when
  refactoring, reviewing, or writing Go endpoints, controllers, and services.
---

# Go Backend Architecture & Clean Code Guidelines

Esta habilidad proporciona pautas y patrones arquitectónicos específicos para el backend de Go con el framework Gin, garantizando la estabilidad y mantenibilidad del ecosistema Event Hub.

## Principios de Diseño

### 1. Gestión Explícita de Recursos
Toda conexión, fila o recurso abierto de base de datos debe liberarse de forma explícita tan pronto como ya no sea necesario para evitar fugas de memoria o de conexiones en el pool.
* **Cierre de Filas (`pgx.Rows`)**: Llama inmediatamente a `defer rows.Close()` tras validar el error de la consulta.
* **Evitar ORMs Pesados**: Utilizar directamente el pool de conexiones `pgxpool.Pool` para transacciones rápidas y consultas SQL nativas.

#### Ejemplo Correcto:
```go
rows, err := db.Query(ctx, "SELECT id, titulo FROM eventos")
if err != nil {
    return nil, fmt.Errorf("error al buscar eventos: %w", err)
}
defer rows.Close() // Garantiza la liberación del recurso

for rows.Next() {
    // Escaneo de datos...
}
```

### 2. Control Inmediato y Explícito de Errores
En Go, los errores son valores. Deben ser validados inmediatamente después de que ocurran.
* Evita ignorar errores con `_`.
* Retorna los errores de manera estructurada envolviéndolos con `fmt.Errorf("contexto del error: %w", err)` para mantener la trazabilidad.
* En los controladores de Gin, detecta el error y responde con el código de estado HTTP adecuado (ej. `400 Bad Request`, `500 Internal Server Error`).

### 3. Binding de Parámetros y Validaciones
* Utiliza los tags de los structs de Go (`form`, `json`, `binding`) para realizar un bindeo estricto y seguro de los parámetros de entrada.
* Maneja los fallos del bindeo de forma consistente con `c.JSON` o `c.HTML` según el tipo de cliente.

#### Ejemplo de Struct con Binding en Go:
```go
type CrearEventoRequest struct {
    Titulo    string    `json:"titulo" binding:"required,min=5"`
    Fecha     time.Time `json:"fecha" binding:"required"`
    Ubicacion string    `json:"ubicacion" binding:"required"`
}
```

### 4. Separación de Responsabilidades
* **Modelos (`/models`)**: Única capa que realiza consultas directas a PostgreSQL o pgxpool. Encapsula las transacciones SQL.
* **Servicios (`/services`)**: Lógica de negocio pura e integración con APIs externas (OAuth, Gemini, Google Calendar).
* **Controladores (`/controllers`)**: Manejan el contexto de Gin (`gin.Context`), bindean datos, llaman a los modelos o servicios y deciden si retornar JSON (API REST) o renderizar HTML.
* **Middlewares (`/middlewares`)**: Lógicas transversales de seguridad (auditoría JWT, manejo de cookies de sesión, CORS).
