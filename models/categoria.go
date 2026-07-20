package models

import (
	"context"
	"errors"
	"event-hub/config"
)

type Categoria struct {
	ID          int    `json:"id" form:"id"`
	Nombre      string `json:"nombre" form:"nombre" binding:"required"`
	Descripcion string `json:"descripcion" form:"descripcion"`
}

// GetCategoriaByID retrieves a category by its ID.
func GetCategoriaByID(ctx context.Context, id int) (*Categoria, error) {
	if config.DB == nil {
		return nil, errors.New("la base de datos no está disponible")
	}
	query := `SELECT id, nombre, descripcion FROM categorias WHERE id = $1`
	var c Categoria
	var desc *string
	err := config.DB.QueryRow(ctx, query, id).Scan(&c.ID, &c.Nombre, &desc)
	if err != nil {
		return nil, err
	}
	if desc != nil {
		c.Descripcion = *desc
	}
	return &c, nil
}

// GetAllCategorias retrieves all categories.
func GetAllCategorias(ctx context.Context) ([]Categoria, error) {
	if config.DB == nil {
		return nil, errors.New("la base de datos no está disponible")
	}
	query := `SELECT id, nombre, descripcion FROM categorias ORDER BY nombre ASC`
	rows, err := config.DB.Query(ctx, query)
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
