package models

import (
	"context"
	"fmt"
	"time"

	"event-hub/config"
)

type Evento struct {
	ID            int         `json:"id" form:"id"`
	Titulo        string      `json:"titulo" form:"titulo" binding:"required"`
	Descripcion   string      `json:"descripcion" form:"descripcion" binding:"required"`
	Fecha         time.Time   `json:"fecha" form:"fecha" binding:"required" time_format:"2006-01-02T15:04"`
	Ubicacion     string      `json:"ubicacion" form:"ubicacion" binding:"required"`
	CreadorID     int         `json:"creador_id"`
	CreadorNombre string      `json:"creador_nombre"`
	Categorias    []Categoria `json:"categorias"`
	CreatedAt     time.Time   `json:"created_at"`
}

// CreateEvento inserts a new event and joins it with categories within a transaction.
func CreateEvento(ctx context.Context, e *Evento, categoryIDs []int) error {
	tx, err := config.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO eventos (titulo, descripcion, fecha, ubicacion, creador_id, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		RETURNING id, created_at
	`
	err = tx.QueryRow(ctx, query, e.Titulo, e.Descripcion, e.Fecha, e.Ubicacion, e.CreadorID).Scan(&e.ID, &e.CreatedAt)
	if err != nil {
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

// GetEventoByID retrieves an event details, creator info, and categories.
func GetEventoByID(ctx context.Context, id int) (*Evento, error) {
	query := `
		SELECT e.id, e.titulo, e.descripcion, e.fecha, e.ubicacion, e.creador_id, u.nombre, e.created_at
		FROM eventos e
		JOIN usuarios u ON e.creador_id = u.id
		WHERE e.id = $1
	`
	var e Evento
	err := config.DB.QueryRow(ctx, query, id).Scan(
		&e.ID, &e.Titulo, &e.Descripcion, &e.Fecha, &e.Ubicacion, &e.CreadorID, &e.CreadorNombre, &e.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Fetch categories
	e.Categorias, err = GetCategoriasForEvento(ctx, e.ID)
	if err != nil {
		return nil, err
	}

	return &e, nil
}

// SearchEventos retrieves events with dynamic query filters: search text & category ID.
func SearchEventos(ctx context.Context, search string, categoryID int) ([]Evento, error) {
	var events []Evento

	// Base query
	query := `
		SELECT DISTINCT e.id, e.titulo, e.descripcion, e.fecha, e.ubicacion, e.creador_id, u.nombre, e.created_at
		FROM eventos e
		JOIN usuarios u ON e.creador_id = u.id
		LEFT JOIN evento_categorias ec ON e.id = ec.evento_id
		WHERE 1=1
	`
	args := []interface{}{}
	argIndex := 1

	if search != "" {
		query += fmt.Sprintf(" AND (e.titulo ILIKE $%d OR e.descripcion ILIKE $%d OR e.ubicacion ILIKE $%d)", argIndex, argIndex, argIndex)
		args = append(args, "%"+search+"%")
		argIndex++
	}

	if categoryID > 0 {
		query += fmt.Sprintf(" AND ec.categoria_id = $%d", argIndex)
		args = append(args, categoryID)
		argIndex++
	}

	query += " ORDER BY e.fecha ASC"

	rows, err := config.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var e Evento
		err = rows.Scan(&e.ID, &e.Titulo, &e.Descripcion, &e.Fecha, &e.Ubicacion, &e.CreadorID, &e.CreadorNombre, &e.CreatedAt)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	// Retrieve categories for each matching event
	for i := range events {
		events[i].Categorias, err = GetCategoriasForEvento(ctx, events[i].ID)
		if err != nil {
			return nil, err
		}
	}

	return events, nil
}

// GetCategoriasForEvento fetches all categories associated with a given event.
func GetCategoriasForEvento(ctx context.Context, eventoID int) ([]Categoria, error) {
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
		var desc *string
		if err := rows.Scan(&c.ID, &c.Nombre, &desc); err != nil {
			return nil, err
		}
		if desc != nil {
			c.Descripcion = *desc
		}
		categories = append(categories, c)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return categories, nil
}
