package models

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"event-hub/config"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// Constantes de estado para la máquina de estados
const (
	EstadoSolicitado = "solicitado"
	EstadoEnRevision = "en_revision"
	EstadoAprobado   = "aprobado"
	EstadoProgramado = "programado"
	EstadoRealizado  = "realizado"
	EstadoCancelado  = "cancelado"
	EstadoRechazado  = "rechazado"
)

// Constantes de Roles (Fix: Adiós a los Magic Strings)
const (
	RolAprobador = "aprobador"
)

// Errores de negocio (Sentinel Errors)
var ErrEspacioOcupado = errors.New("el espacio ya está ocupado en ese horario")
var ErrEstadoInvalido = errors.New("estado de evento inválido")
var ErrObservacionesRequeridas = errors.New("las observaciones son obligatorias para rechazar o cancelar un evento")
var ErrTransicionEstadoInvalida = errors.New("transición de estado inválida")

// FiltroEvento contiene los parámetros de búsqueda y paginación
type FiltroEvento struct {
	Search        string
	CategoryID    int
	Estado        string
	EspacioID     int
	OrganizadorID int64
	Page          int
	Limit         int
}

// Evento mapea la tabla 'eventos'
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

// GetEventoByID obtiene todos los campos
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

// ActualizarEstadoEvento maneja transiciones y validaciones del ciclo de vida.
func ActualizarEstadoEvento(ctx context.Context, id int64, nuevoEstado string, aprobadorID *int64, observaciones string) error {
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

	actorID := int64(0)
	if aprobadorID != nil {
		actorID = *aprobadorID
	}
	_, err = tx.Exec(ctx, "SET LOCAL myapp.current_user_id = $1", actorID)
	if err != nil {
		return err
	}

	var estadoActual string
	err = tx.QueryRow(ctx, "SELECT estado FROM eventos WHERE id = $1 FOR UPDATE", id).Scan(&estadoActual)
	if err != nil {
		return errors.New("no se encontró el evento para actualizar")
	}

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

// SearchEventos recupera eventos con filtros dinámicos, paginación, y elimina N+1 con array_agg.
func SearchEventos(ctx context.Context, filtro FiltroEvento) ([]Evento, int, error) {
	if filtro.Page < 1 {
		filtro.Page = 1
	}
	if filtro.Limit < 1 || filtro.Limit > 100 {
		filtro.Limit = 20
	}
	offset := (filtro.Page - 1) * filtro.Limit

	var query strings.Builder
	query.WriteString(`
		SELECT 
			e.id, e.titulo, e.descripcion, e.fecha_inicio, e.fecha_fin, 
			e.estado, es.nombre as espacio_nombre, u.nombre as organizador_nombre,
			COALESCE(array_agg(c.nombre) FILTER (WHERE c.id IS NOT NULL), '{}') as categorias_nombres,
			count(*) OVER() as total_count
		FROM eventos e
		JOIN usuarios u ON e.organizador_id = u.id
		JOIN espacios es ON e.espacio_id = es.id
		LEFT JOIN evento_categorias ec ON e.id = ec.evento_id
		LEFT JOIN categorias c ON ec.categoria_id = c.id
		WHERE 1=1
	`)

	args := []interface{}{}
	argIndex := 1

	if filtro.Search != "" {
		query.WriteString(fmt.Sprintf(" AND (e.titulo ILIKE $%d OR e.descripcion ILIKE $%d OR es.nombre ILIKE $%d)", argIndex, argIndex, argIndex))
		args = append(args, "%"+filtro.Search+"%")
		argIndex++
	}
	if filtro.CategoryID > 0 {
		query.WriteString(fmt.Sprintf(" AND EXISTS (SELECT 1 FROM evento_categorias ec2 WHERE ec2.evento_id = e.id AND ec2.categoria_id = $%d)", argIndex))
		args = append(args, filtro.CategoryID)
		argIndex++
	}
	if filtro.Estado != "" {
		query.WriteString(fmt.Sprintf(" AND e.estado = $%d", argIndex))
		args = append(args, filtro.Estado)
		argIndex++
	}
	if filtro.EspacioID > 0 {
		query.WriteString(fmt.Sprintf(" AND e.espacio_id = $%d", argIndex))
		args = append(args, filtro.EspacioID)
		argIndex++
	}
	if filtro.OrganizadorID > 0 {
		query.WriteString(fmt.Sprintf(" AND e.organizador_id = $%d", argIndex))
		args = append(args, filtro.OrganizadorID)
		argIndex++
	}

	query.WriteString(` GROUP BY e.id, es.nombre, u.nombre `)
	query.WriteString(fmt.Sprintf(" ORDER BY e.fecha_inicio ASC LIMIT $%d OFFSET $%d", argIndex, argIndex+1))
	args = append(args, filtro.Limit, offset)

	rows, err := config.DB.Query(ctx, query.String(), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	// Fix 2: Serialización segura inicializando el slice en lugar de dejarlo null
	events := make([]Evento, 0)
	totalCount := 0

	for rows.Next() {
		var e Evento
		var catNombres []string
		err = rows.Scan(
			&e.ID, &e.Titulo, &e.Descripcion, &e.FechaInicio, &e.FechaFin,
			&e.Estado, &e.EspacioNombre, &e.OrganizadorNombre,
			&catNombres, &totalCount,
		)
		if err != nil {
			return nil, 0, err
		}

		for _, cn := range catNombres {
			e.Categorias = append(e.Categorias, Categoria{Nombre: cn})
		}
		events = append(events, e)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	return events, totalCount, nil
}

// GetCategoriasForEvento (Mantenido para compatibilidad con GetEventoByID)
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

	return categories, rows.Err()
}

