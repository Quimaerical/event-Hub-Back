package models

import (
	"context"
	"errors"
	"time"

	"event-hub/config"

	"github.com/jackc/pgx/v5"
)

type Inscripcion struct {
	ID               int64     `json:"id"`
	EventoID         int64     `json:"evento_id"`
	UsuarioID        int64     `json:"usuario_id"`
	Email            string    `json:"email"`
	FechaInscripcion time.Time `json:"fecha_inscripcion"`
	Estado           string    `json:"estado"` // "confirmada", "cancelada"
}

// InscribirUsuario gestiona el alta de una inscripción con blindaje de concurrencia
func InscribirUsuario(ctx context.Context, eventoID, usuarioID int64, email string) (*Evento, error) {
	tx, err := config.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// 1. Bloqueo pesimista del evento para evitar sobreventa (Race Conditions)
	evento, err := GetEventoByIDForUpdate(ctx, tx, eventoID)
	if err != nil {
		return nil, err
	}

	// 2. Validar Estado del Evento
	if evento.Estado != EstadoAprobado && evento.Estado != EstadoProgramado {
		return nil, ErrEventoNoInscribible
	}

	// 3. Validar si ya está inscrito
	var existe int
	err = tx.QueryRow(ctx, `SELECT 1 FROM inscripciones WHERE evento_id = $1 AND usuario_id = $2 AND estado = 'confirmada'`, eventoID, usuarioID).Scan(&existe)
	
	if err == nil {
		return nil, ErrUsuarioYaInscrito
	} else if !errors.Is(err, pgx.ErrNoRows) {
		// FIX: Si el error NO es pgx.ErrNoRows (falla real de BD), interrumpir y propagar el error
		return nil, err
	}

	// 4. Validar Cupo
	var inscritos int
	err = tx.QueryRow(ctx, `SELECT COUNT(*) FROM inscripciones WHERE evento_id = $1 AND estado = 'confirmada'`, eventoID).Scan(&inscritos)
	if err != nil {
		return nil, err
	}

	if inscritos >= evento.CapacidadMaxima {
		return nil, ErrCupoCompleto
	}

	// 5. Insertar Inscripción
	_, err = tx.Exec(ctx, `
		INSERT INTO inscripciones (evento_id, usuario_id, email, estado) 
		VALUES ($1, $2, $3, 'confirmada')
	`, eventoID, usuarioID, email)
	
	if err != nil {
		return nil, err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, err
	}

	return evento, nil // Retornamos el evento para poder extraer el CalendarID luego
}

func CancelarInscripcion(ctx context.Context, inscripcionID, usuarioID int64, esAprobador bool) error {
	var query string
	var args []interface{}

	if esAprobador {
		query = `UPDATE inscripciones SET estado = 'cancelada' WHERE id = $1 AND estado = 'confirmada'`
		args = []interface{}{inscripcionID}
	} else {
		// Validar que la inscripción pertenezca al usuario que cancela
		query = `UPDATE inscripciones SET estado = 'cancelada' WHERE id = $1 AND usuario_id = $2 AND estado = 'confirmada'`
		args = []interface{}{inscripcionID, usuarioID}
	}

	tag, err := config.DB.Exec(ctx, query, args...)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return ErrInscripcionNoEncontrada
	}

	return nil
}

func GetInscripcionesByEvento(ctx context.Context, eventoID int64) ([]Inscripcion, error) {
	query := `SELECT id, evento_id, usuario_id, email, fecha_inscripcion, estado 
			  FROM inscripciones WHERE evento_id = $1 ORDER BY fecha_inscripcion DESC`
	
	rows, err := config.DB.Query(ctx, query, eventoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lista []Inscripcion
	for rows.Next() {
		var i Inscripcion
		if err := rows.Scan(&i.ID, &i.EventoID, &i.UsuarioID, &i.Email, &i.FechaInscripcion, &i.Estado); err != nil {
			return nil, err
		}
		lista = append(lista, i)
	}
	return lista, nil
}
