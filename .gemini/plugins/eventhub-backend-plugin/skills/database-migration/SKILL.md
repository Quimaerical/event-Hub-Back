---
name: database-migration
description: >-
  Provides guidelines, transactional patterns, and validation rules for PostgreSQL database schemas, indexes, and queries using pgx/v5 in Go. Use when designing tables, writing raw SQL queries, or handling concurrent database writes.
---

# PostgreSQL Database & Transactional Integrity Guidelines

Esta habilidad proporciona las pautas necesarias para diseñar esquemas relacionales eficientes y transacciones concurrentes seguras utilizando PostgreSQL y el controlador `pgx/v5` en Go.

## Principios y Patrones

### 1. Control de Concurrencia y Bloqueos (`SELECT FOR UPDATE`)
Cuando múltiples usuarios intentan realizar una acción simultánea (como inscribirse a un evento con cupo limitado), existe el riesgo de condiciones de carrera (race conditions) que causen sobreventa de entradas.
* Toda validación y posterior decremento/incremento de contadores debe ejecutarse dentro de una transacción (`BEGIN ... COMMIT`).
* Usa `SELECT ... FOR UPDATE` para bloquear la fila de interés hasta que finalice la transacción.

#### Ejemplo de Transacción Segura en Go con pgx/v5:
```go
tx, err := pool.Begin(ctx)
if err != nil {
    return fmt.Errorf("no se pudo iniciar transacción: %w", err)
}
defer tx.Rollback(ctx) // Se ejecuta si ocurre un error antes de Commit

// 1. Obtener y bloquear el cupo actual del evento
var cupoMaximo, inscritos int
err = tx.QueryRow(ctx, 
    "SELECT cupo_maximo, inscritos FROM eventos WHERE id = $1 FOR UPDATE", 
    eventoID).Scan(&cupoMaximo, &inscritos)
if err != nil {
    return fmt.Errorf("error al obtener cupo con bloqueo: %w", err)
}

// 2. Validar reglas de negocio
if inscritos >= cupoMaximo {
    return fmt.Errorf("el evento ya no cuenta con cupos disponibles")
}

// 3. Registrar al participante e incrementar el contador
_, err = tx.Exec(ctx, 
    "INSERT INTO inscripciones (evento_id, usuario_id) VALUES ($1, $2)", 
    eventoID, usuarioID)
if err != nil {
    return fmt.Errorf("error al registrar inscripción: %w", err)
}

_, err = tx.Exec(ctx, 
    "UPDATE eventos SET inscritos = inscritos + 1 WHERE id = $1", 
    eventoID)
if err != nil {
    return fmt.Errorf("error al incrementar inscritos: %w", err)
}

// 4. Consolidar cambios de forma atómica
if err := tx.Commit(ctx); err != nil {
    return fmt.Errorf("error al consolidar transacción: %w", err)
}
```

### 2. Uso de Índices y Optimización de Búsqueda
Las consultas de filtrado dinámico en la cartelera de eventos deben estar optimizadas con índices de PostgreSQL para evitar escaneos secuenciales lentos (`seq scan`).
* Índices de Llave Foránea: Toda tabla intermedia o foránea (ej. `evento_categorias`) debe tener índices en sus columnas `evento_id` y `categoria_id`.
* Índices de Texto y Fecha: Crea índices en los campos más buscados: `eventos.titulo`, `eventos.fecha` y `usuarios.email`.

#### DDL Recomendado (`schema.sql`):
```sql
CREATE UNIQUE INDEX IF NOT EXISTS idx_usuarios_email ON usuarios(email);
CREATE INDEX IF NOT EXISTS idx_eventos_fecha ON eventos(fecha);
CREATE INDEX IF NOT EXISTS idx_eventos_titulo_trgm ON eventos USING gin (titulo gin_trgm_ops); -- Para búsquedas ILIKE rápidas
```

### 3. Seguridad Contra Inyección SQL
* **NUNCA** concatenes variables directamente en cadenas de texto SQL.
* Utiliza siempre los marcadores de posición nativos de pgx (ej. `$1, $2, $3`) para permitir que PostgreSQL prepare y limpie los argumentos de forma segura.
* Asegura el bindeo de tipos correctos (ej. no enviar valores string a columnas enteras).
