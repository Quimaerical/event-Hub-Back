package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"event-hub/config"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// Constantes de estado para la máquina de estados de la base de datos
const (
	EstadoSolicitado = "solicitado"
	EstadoEnRevision = "en_revision"
	EstadoAprobado   = "aprobado"
	EstadoProgramado = "programado"
	EstadoRealizado  = "realizado"
	EstadoCancelado  = "cancelado"
	EstadoRechazado  = "rechazado"
)

// Errores de negocio (Sentinel Errors)
var ErrEspacioOcupado = errors.New("el espacio ya está ocupado en ese horario")
var ErrEstadoInvalido = errors.New("estado de evento inválido")
var ErrObservacionesRequeridas = errors.New("las observaciones son obligatorias para rechazar o cancelar un evento")
var ErrTransicionEstadoInvalida = errors.New("transición de estado inválida") // Nuevo error

// Estructura Categoria
type Categoria struct {
	ID          int    `json:"id"`
	Nombre      string `json:"nombre"`
	Descripcion string `json:"descripcion,omitempty"`
}

// Evento mapea exactamente la tabla 'eventos' usando pgtype para campos NULLABLES
type Evento struct {
	ID                 int64              `json:"id" form:"id"`
	Titulo             string             `json:"titulo" form:"titulo" binding:"required"`
	Descripcion        pgtype.Text        `json:"descripcion" form:"descripcion"`
	EspacioID          int                `json:"espacio_id" form:"espacio_id" binding:"required"`
	OrganizadorID      int64              `json:"organizador_id"`
	AprobadorID        pgtype.Int8        `json:"aprobador_id"`
	FechaInicio        time.Time          `json:"fecha_inicio" form:"fecha_inicio" binding:"required" time_format:"2006-01-02T15:04"`
	FechaFin           time.Time          `json:"fecha_fin" form:"fecha_fin" binding:"required" time_format:"2006-01-02T15:04"`
	Estado             string             `json:"estado"`
	CapacidadMaxima    int                `json:"capacidad_maxima" form:"capacidad_maxima" binding:"required"`
	ImagenURL          pgtype.Text        `json:"imagen_url"`
	Observaciones      pgtype.Text        `json:"observaciones"`
	FechaSolicitud     time.Time          `json:"fecha_solicitud"`
	FechaAprobacion    pgtype.Timestamptz `json:"fecha_aprobacion"`
	FechaCreacion      time.Time          `json:"fecha_creacion"`
	FechaActualizacion time.Time          `json:"fecha_actualizacion"`
	CalendarEventID    pgtype.Text        `json:"calendar_event_id"`

	OrganizadorNombre string      `json:"organizador_nombre,omitempty"`
	EspacioNombre     string      `json:"espacio_nombre,omitempty"`
	Categorias        []Categoria `json:"categorias,omitempty"`
}

// CreateEvento inserta un evento interceptando errores de solapamiento de PostgreSQL.
func CreateEvento(ctx context.Context, e *Evento, categoryIDs []int) error {
	tx, err := config.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, "SET LOCAL myapp.current_user_id = $1", e.OrganizadorID)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO eventos (
			titulo, descripcion, espacio_id, organizador_id, 
			fecha_inicio, fecha_fin, capacidad_maxima
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, estado, fecha_solicitud, fecha_creacion, fecha_actualizacion
	`
	err = tx.QueryRow(ctx, query,
		e.Titulo, e.Descripcion, e.EspacioID, e.OrganizadorID,
		e.FechaInicio, e.FechaFin, e.CapacidadMaxima,
	).Scan(
		&e.ID, &e.Estado, &e.FechaSolicitud, &e.FechaCreacion, &e.FechaActualizacion,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.ConstraintName == "chk_sin_solapamiento" {
			return ErrEspacioOcupado 
		}
		return err
	}

	for _, catID := range categoryIDs {
		linkQuery := `INSERT INTO evento_categorias (evento_id, categoria_id) VALUES ($1, $2)`
		_, err = tx.Exec(ctx, linkQuery, e.ID, catID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// GetEventoByID obtiene todos los campos, incluyendo los nuevos de pgtype.
func GetEventoByID(ctx context.Context, id int64) (*Evento, error) {
	query := `
		SELECT 
			e.id, e.titulo, e.descripcion, e.espacio_id, e.organizador_id, e.aprobador_id,
			e.fecha_inicio, e.fecha_fin, e.estado, e.capacidad_maxima, e.imagen_url, e.observaciones,
			e.fecha_solicitud, e.fecha_aprobacion, e.fecha_creacion, e.fecha_actualizacion, e.calendar_event_id,
			es.nombre as espacio_nombre, u.nombre as organizador_nombre
		FROM eventos e
		JOIN usuarios u ON e.organizador_id = u.id
		JOIN espacios es ON e.espacio_id = es.id
		WHERE e.id = $1
	`
	var e Evento
	err := config.DB.QueryRow(ctx, query, id).Scan(
		&e.ID, &e.Titulo, &e.Descripcion, &e.EspacioID, &e.OrganizadorID, &e.AprobadorID,
		&e.FechaInicio, &e.FechaFin, &e.Estado, &e.CapacidadMaxima, &e.ImagenURL, &e.Observaciones,
		&e.FechaSolicitud, &e.FechaAprobacion, &e.FechaCreacion, &e.FechaActualizacion, &e.CalendarEventID,
		&e.EspacioNombre, &e.OrganizadorNombre,
	)
	if err != nil {
		return nil, err
	}

	e.Categorias, err = GetCategoriasForEvento(ctx, e.ID)
	if err != nil {
		return nil, err
	}

	return &e, nil
}

// ActualizarEstadoEvento maneja transiciones y validaciones del ciclo de vida del evento.
// FIX: Transiciones bloqueantes con SELECT FOR UPDATE añadidas.
func ActualizarEstadoEvento(ctx context.Context, id int64, nuevoEstado string, aprobadorID *int64, observaciones string) error {
	// Validación de existencia de estado
	switch nuevoEstado {
	case EstadoSolicitado, EstadoEnRevision, EstadoAprobado, EstadoProgramado, EstadoRealizado, EstadoCancelado, EstadoRechazado:
	default:
		return ErrEstadoInvalido
	}

	if (nuevoEstado == EstadoRechazado || nuevoEstado == EstadoCancelado) && observaciones == "" {
		return ErrObservacionesRequeridas
	}

	tx, err := config.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Inyectar auditoría
	actorID := int64(0)
	if aprobadorID != nil {
		actorID = *aprobadorID
	}
	_, err = tx.Exec(ctx, "SET LOCAL myapp.current_user_id = $1", actorID)
	if err != nil {
		return err
	}

	// 1. Obtener estado actual bloqueando la fila (Prevención de race conditions)
	var estadoActual string
	err = tx.QueryRow(ctx, "SELECT estado FROM eventos WHERE id = $1 FOR UPDATE", id).Scan(&estadoActual)
	if err != nil {
		return errors.New("no se encontró el evento para actualizar")
	}

	// 2. Validar transición de estado
	transicionesValidas := map[string][]string{
		EstadoSolicitado: {EstadoEnRevision, EstadoCancelado},
		EstadoEnRevision: {EstadoAprobado, EstadoRechazado, EstadoCancelado},
		EstadoAprobado:   {EstadoProgramado, EstadoCancelado},
		EstadoProgramado: {EstadoRealizado, EstadoCancelado},
		EstadoRealizado:  {},
		EstadoCancelado:  {},
		EstadoRechazado:  {},
	}

	esValida := false
	for _, estadoPermitido := range transicionesValidas[estadoActual] {
		if nuevoEstado == estadoPermitido {
			esValida = true
			break
		}
	}

	if !esValida {
		return ErrTransicionEstadoInvalida
	}

	// 3. Ejecutar actualización
	var query string
	var args []interface{}

	if nuevoEstado == EstadoAprobado {
		query = `
			UPDATE eventos 
			SET estado = $1, aprobador_id = $2, fecha_aprobacion = NOW() 
			WHERE id = $3
		`
		args = []interface{}{nuevoEstado, aprobadorID, id}
	} else {
		query = `
			UPDATE eventos 
			SET estado = $1, observaciones = $2 
			WHERE id = $3
		`
		args = []interface{}{nuevoEstado, pgtype.Text{String: observaciones, Valid: observaciones != ""}, id}
	}

	res, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return err
	}

	if res.RowsAffected() == 0 {
		return errors.New("no se pudo actualizar el evento")
	}

	return tx.Commit(ctx)
}

// SearchEventos recupera eventos con filtros dinámicos.
func SearchEventos(ctx context.Context, search string, categoryID int) ([]Evento, error) {
	var events []Evento

	query := `
		SELECT DISTINCT 
			e.id, e.titulo, e.descripcion, e.fecha_inicio, e.fecha_fin, 
			e.estado, es.nombre as espacio_nombre, u.nombre as organizador_nombre
		FROM eventos e
		JOIN usuarios u ON e.organizador_id = u.id
		JOIN espacios es ON e.espacio_id = es.id
		LEFT JOIN evento_categorias ec ON e.id = ec.evento_id
		WHERE 1=1
	`
	args := []interface{}{}
	argIndex := 1

	if search != "" {
		query += fmt.Sprintf(" AND (e.titulo ILIKE $%d OR e.descripcion ILIKE $%d OR es.nombre ILIKE $%d)", argIndex, argIndex, argIndex)
		args = append(args, "%"+search+"%")
		argIndex++
	}

	if categoryID > 0 {
		query += fmt.Sprintf(" AND ec.categoria_id = $%d", argIndex)
		args = append(args, categoryID)
		argIndex++
	}

	query += " ORDER BY e.fecha_inicio ASC"

	rows, err := config.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var e Evento
		err = rows.Scan(
			&e.ID, &e.Titulo, &e.Descripcion, &e.FechaInicio, &e.FechaFin,
			&e.Estado, &e.EspacioNombre, &e.OrganizadorNombre,
		)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	// TODO: Optimizar N+1 Query. Actualmente hace 1 query por evento para traer categorías.
	// Solución: Usar array_agg en la query principal o cargar categorías en batch.
	for i := range events {
		events[i].Categorias, err = GetCategoriasForEvento(ctx, events[i].ID)
		if err != nil {
			return nil, err
		}
	}

	return events, nil
}

// GetCategoriasForEvento obtiene todas las categorías asociadas a un evento.
func GetCategoriasForEvento(ctx context.Context, eventoID int64) ([]Categoria, error) {
	query := `
		SELECT c.id, c.nombre, c.descripcion
		FROM categorias c
		JOIN evento_categorias ec ON c.id = ec.categoria_id
		WHERE ec.evento_id = $1
	`
	rows, err := config.DB.Query(ctx, query, eventoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []Categoria
	for rows.Next() {
		var c Categoria
		var desc pgtype.Text
		if err := rows.Scan(&c.ID, &c.Nombre, &desc); err != nil {
			return nil, err
		}
		if desc.Valid {
			c.Descripcion = desc.String
		}
		categories = append(categories, c)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return categories, nil
}

// GetAllCategorias obtiene el catálogo completo de categorías para el frontend.
func GetAllCategorias(ctx context.Context) ([]Categoria, error) {
	query := `SELECT id, nombre, descripcion FROM categorias ORDER BY nombre ASC`
	rows, err := config.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categorias []Categoria
	for rows.Next() {
		var c Categoria
		var desc pgtype.Text
		if err := rows.Scan(&c.ID, &c.Nombre, &desc); err != nil {
			return nil, err
		}
		if desc.Valid {
			c.Descripcion = desc.String
		}
		categorias = append(categorias, c)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return categorias, nil
}
