package models

import (
	"context"
	"event-hub/config"
)

type Role struct {
	ID     int    `json:"id" form:"id"`
	Nombre string `json:"nombre" form:"nombre" binding:"required"`
}

// GetRoleByID retrieves a role by its ID.
func GetRoleByID(ctx context.Context, id int) (*Role, error) {
	query := `SELECT id, nombre FROM roles WHERE id = $1`
	var r Role
	err := config.DB.QueryRow(ctx, query, id).Scan(&r.ID, &r.Nombre)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// GetRoleByName retrieves a role by its name (e.g., 'usuario', 'administrador').
func GetRoleByName(ctx context.Context, nombre string) (*Role, error) {
	query := `SELECT id, nombre FROM roles WHERE nombre = $1`
	var r Role
	err := config.DB.QueryRow(ctx, query, nombre).Scan(&r.ID, &r.Nombre)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// GetAllRoles retrieves all roles from the database.
func GetAllRoles(ctx context.Context) ([]Role, error) {
	query := `SELECT id, nombre FROM roles ORDER BY nombre ASC`
	rows, err := config.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []Role
	for rows.Next() {
		var r Role
		if err := rows.Scan(&r.ID, &r.Nombre); err != nil {
			return nil, err
		}
		roles = append(roles, r)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return roles, nil
}
