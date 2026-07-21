package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"event-hub/config"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Inscripcion struct {
	ID               int64     `json:"id"`
	EventoID         int64     `json:"evento_id"`
	UsuarioID        int64     `json:"usuario_id"`
	Email            string    `json:"email"`
	FechaInscripcion time.Time `json:"fecha_inscripcion"`
	Estado           string    `json:"estado"` // "confirmada", "cancelada"
}

// EstaInscrito consulta si un usuario tiene una reserva activa para un evento.
func EstaInscrito(ctx context.Context, eventoID, usuarioID int64) (bool, error) {
	var id int64
	err := config.DB.QueryRow(ctx, `SELECT id FROM reservas WHERE evento_id = $1 AND usuario_id = $2 AND estado = 'confirmada'`, eventoID, usuarioID).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetConteoInscritos obtiene el número total de reservas confirmadas para un evento.
func GetConteoInscritos(ctx context.Context, eventoID int64) (int, error) {
	var count int
	err := config.DB.QueryRow(ctx, `SELECT COUNT(*) FROM reservas WHERE evento_id = $1 AND estado = 'confirmada'`, eventoID).Scan(&count)
	return count, err
}

// InscribirUsuario gestiona el alta de una inscripción/reserva con blindaje de concurrencia y programa el recordatorio.
func InscribirUsuario(ctx context.Context, eventoID, usuarioID int64, email string) (*Evento, error) {
	tx, err := config.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// 1. Bloqueo pesimista del evento para evitar sobreventa
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
	err = tx.QueryRow(ctx, `SELECT 1 FROM reservas WHERE evento_id = $1 AND usuario_id = $2 AND estado = 'confirmada'`, eventoID, usuarioID).Scan(&existe)
	if err == nil {
		return nil, ErrUsuarioYaInscrito
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// 4. Validar Cupo
	var inscritos int
	err = tx.QueryRow(ctx, `SELECT COUNT(*) FROM reservas WHERE evento_id = $1 AND estado = 'confirmada'`, eventoID).Scan(&inscritos)
	if err != nil {
		return nil, err
	}

	if inscritos >= evento.CapacidadMaxima {
		return nil, ErrCupoCompleto
	}

	// 5. Insertar Reserva
	_, err = tx.Exec(ctx, `
		INSERT INTO reservas (evento_id, usuario_id, estado) 
		VALUES ($1, $2, 'confirmada')
	`, eventoID, usuarioID)
	if err != nil {
		return nil, err
	}

	// 6. Programar Notificación de Recordatorio (2 horas antes del evento)
	fechaEnvio := evento.FechaInicio.Add(-2 * time.Hour)
	if fechaEnvio.Before(time.Now()) {
		fechaEnvio = time.Now().Add(1 * time.Minute)
	}

	contenidoMsg := fmt.Sprintf("Recordatorio: El evento '%s' comenzará el %s en %s.",
		evento.Titulo,
		evento.FechaInicio.Format("02/01/2006 a las 15:04"),
		evento.EspacioNombre,
	)

	_, _ = tx.Exec(ctx, `
		INSERT INTO recordatorios (evento_id, destinatario_id, tipo, contenido, fecha_envio_programada, estado)
		VALUES ($1, $2, 'email', $3, $4, 'pendiente')
	`, eventoID, usuarioID, contenidoMsg, fechaEnvio)

	err = tx.Commit(ctx)
	if err != nil {
		return nil, err
	}

	return evento, nil
}

// CancelarInscripcionPorEventoYUsuario cancela la reserva activa de un usuario en un evento.
func CancelarInscripcionPorEventoYUsuario(ctx context.Context, eventoID, usuarioID int64) error {
	tag, err := config.DB.Exec(ctx, `UPDATE reservas SET estado = 'cancelada' WHERE evento_id = $1 AND usuario_id = $2 AND estado = 'confirmada'`, eventoID, usuarioID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrInscripcionNoEncontrada
	}
	return nil
}

// GetEventosInscritoByUsuario obtiene los eventos a los que se ha inscrito el usuario.
func GetEventosInscritoByUsuario(ctx context.Context, usuarioID int64) ([]Evento, error) {
	query := `
		SELECT 
			e.id, e.titulo, e.descripcion, e.fecha_inicio, e.fecha_fin, 
			e.estado, e.capacidad_maxima, es.nombre as espacio_nombre, u.nombre as organizador_nombre
		FROM reservas r
		JOIN eventos e ON r.evento_id = e.id
		JOIN espacios es ON e.espacio_id = es.id
		JOIN usuarios u ON e.organizador_id = u.id
		WHERE r.usuario_id = $1 AND r.estado = 'confirmada'
		ORDER BY e.fecha_inicio ASC
	`
	rows, err := config.DB.Query(ctx, query, usuarioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var eventos []Evento
	for rows.Next() {
		var e Evento
		var desc pgtype.Text
		err := rows.Scan(
			&e.ID, &e.Titulo, &desc, &e.FechaInicio, &e.FechaFin,
			&e.Estado, &e.CapacidadMaxima, &e.EspacioNombre, &e.OrganizadorNombre,
		)
		if err != nil {
			return nil, err
		}
		e.Descripcion = desc
		eventos = append(eventos, e)
	}
	return eventos, nil
}

// GetEventosCreadosByUsuario obtiene los eventos organizados por el usuario.
func GetEventosCreadosByUsuario(ctx context.Context, organizadorID int64) ([]Evento, error) {
	query := `
		SELECT 
			e.id, e.titulo, e.descripcion, e.fecha_inicio, e.fecha_fin, 
			e.estado, e.capacidad_maxima, es.nombre as espacio_nombre
		FROM eventos e
		JOIN espacios es ON e.espacio_id = es.id
		WHERE e.organizador_id = $1
		ORDER BY e.fecha_creacion DESC
	`
	rows, err := config.DB.Query(ctx, query, organizadorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var eventos []Evento
	for rows.Next() {
		var e Evento
		var desc pgtype.Text
		err := rows.Scan(
			&e.ID, &e.Titulo, &desc, &e.FechaInicio, &e.FechaFin,
			&e.Estado, &e.CapacidadMaxima, &e.EspacioNombre,
		)
		if err != nil {
			return nil, err
		}
		e.Descripcion = desc
		eventos = append(eventos, e)
	}
	return eventos, nil
}

// CancelarInscripcion cancela una reserva por ID de reserva.
func CancelarInscripcion(ctx context.Context, inscripcionID, usuarioID int64, esAprobador bool) error {
	var query string
	var args []interface{}

	if esAprobador {
		query = `UPDATE reservas SET estado = 'cancelada' WHERE id = $1 AND estado = 'confirmada'`
		args = []interface{}{inscripcionID}
	} else {
		query = `UPDATE reservas SET estado = 'cancelada' WHERE id = $1 AND usuario_id = $2 AND estado = 'confirmada'`
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

type AsistenteInfo struct {
	UsuarioID    int64     `json:"usuario_id"`
	Nombre       string    `json:"nombre"`
	Email        string    `json:"email"`
	Departamento string    `json:"departamento"`
	Telefono     string    `json:"telefono"`
	FechaReserva time.Time `json:"fecha_reserva"`
}

// GetAsistentesPorEvento obtiene la lista de todos los usuarios con reserva confirmada para un evento.
func GetAsistentesPorEvento(ctx context.Context, eventoID int64) ([]AsistenteInfo, error) {
	query := `
		SELECT 
			u.id, u.nombre, u.email, 
			COALESCE(u.departamento, ''), COALESCE(u.telefono, ''), 
			r.fecha_reserva
		FROM reservas r
		JOIN usuarios u ON r.usuario_id = u.id
		WHERE r.evento_id = $1 AND r.estado = 'confirmada'
		ORDER BY r.fecha_reserva ASC
	`
	rows, err := config.DB.Query(ctx, query, eventoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var asistentes []AsistenteInfo
	for rows.Next() {
		var a AsistenteInfo
		err := rows.Scan(&a.UsuarioID, &a.Nombre, &a.Email, &a.Departamento, &a.Telefono, &a.FechaReserva)
		if err != nil {
			return nil, err
		}
		asistentes = append(asistentes, a)
	}
	return asistentes, nil
}
