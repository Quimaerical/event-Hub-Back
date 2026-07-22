package models

import (
	"context"
	"event-hub/config"
	"github.com/jackc/pgx/v5/pgtype"
)

// Espacio maps the 'espacios' table in the database.
type Espacio struct {
	ID         int         `json:"id"`
	Nombre     string      `json:"nombre"`
	Tipo       string      `json:"tipo"`
	Capacidad  int         `json:"capacidad"`
	Ubicacion  pgtype.Text `json:"ubicacion"`
	Disponible bool        `json:"disponible"`
	Activo     bool        `json:"activo"`
}

// GetAllEspacios retrieves all active spaces.
func GetAllEspacios(ctx context.Context) ([]Espacio, error) {
	query := `SELECT id, nombre, tipo, capacidad, ubicacion, disponible, activo FROM espacios WHERE activo = true ORDER BY nombre ASC`
	rows, err := config.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var spaces []Espacio
	for rows.Next() {
		var s Espacio
		err := rows.Scan(&s.ID, &s.Nombre, &s.Tipo, &s.Capacidad, &s.Ubicacion, &s.Disponible, &s.Activo)
		if err != nil {
			return nil, err
		}
		spaces = append(spaces, s)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return spaces, nil
}

// CreateEspacio registra una nueva ubicación/espacio para eventos en la base de datos.
func CreateEspacio(ctx context.Context, e *Espacio) error {
	tipoValido := e.Tipo
	switch tipoValido {
	case "biblioteca", "auditorio", "salon", "laboratorio":
	default:
		tipoValido = "auditorio"
	}

	query := `
		INSERT INTO espacios (nombre, tipo, capacidad, ubicacion, disponible, activo)
		VALUES ($1, $2::tipo_espacio, $3, $4, true, true)
		RETURNING id
	`
	return config.DB.QueryRow(ctx, query, e.Nombre, tipoValido, e.Capacidad, e.Ubicacion).Scan(&e.ID)
}
