package models

import (
	"context"
	"event-hub/config"
)

type EventoCategoria struct {
	EventoID    int `json:"evento_id" form:"evento_id" binding:"required"`
	CategoriaID int `json:"categoria_id" form:"categoria_id" binding:"required"`
}

// AddCategoriaToEvento links a category to an event.
func AddCategoriaToEvento(ctx context.Context, eventoID, categoriaID int) error {
	query := `INSERT INTO evento_categorias (evento_id, categoria_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := config.DB.Exec(ctx, query, eventoID, categoriaID)
	return err
}

// RemoveCategoriaFromEvento unlinks a category from an event.
func RemoveCategoriaFromEvento(ctx context.Context, eventoID, categoriaID int) error {
	query := `DELETE FROM evento_categorias WHERE evento_id = $1 AND categoria_id = $2`
	_, err := config.DB.Exec(ctx, query, eventoID, categoriaID)
	return err
}
