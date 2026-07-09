package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"event-hub/config"

	"github.com/jackc/pgx/v5"
)

// CronService gestiona las tareas programadas en segundo plano
type CronService struct {
	notificationSvc *NotificationService
	ticker          *time.Ticker
	quit            chan struct{}
}

// NewCronService inyecta el servicio de notificaciones requerido
func NewCronService(notificationSvc *NotificationService) *CronService {
	return &CronService{
		notificationSvc: notificationSvc,
		ticker:          time.NewTicker(5 * time.Minute),
		quit:            make(chan struct{}),
	}
}

// Start lanza el loop del cron de forma no bloqueante
func (s *CronService) Start() {
	slog.Info("Motor Automático de Recordatorios iniciado (Ciclo: 5 minutos)")
	
	go func() {
		// Ejecutar inmediatamente la primera vez al arrancar el servidor
		s.procesarRecordatorios()
		
		for {
			select {
			case <-s.ticker.C:
				s.procesarRecordatorios()
			case <-s.quit:
				s.ticker.Stop()
				slog.Info("Motor Automático de Recordatorios detenido")
				return
			}
		}
	}()
}

// Stop permite apagar el motor elegantemente (Graceful Shutdown)
func (s *CronService) Stop() {
	close(s.quit)
}

// procesarRecordatorios busca los candidatos y delega el trabajo concurrente
func (s *CronService) procesarRecordatorios() {
	ctx := context.Background()

	// 1. Buscamos solo los IDs candidatos sin bloquear la tabla
	queryCandidatos := `
		SELECT id FROM recordatorios
		WHERE estado = 'pendiente' AND fecha_envio_programada <= NOW()
	`
	rows, err := config.DB.Query(ctx, queryCandidatos)
	if err != nil {
		slog.Error("Cron: Error al buscar recordatorios pendientes", "error", err)
		return
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}

	// 2. Procesamiento Concurrente: Una goroutine por cada envío
	for _, id := range ids {
		go s.procesarUnRecordatorio(id)
	}
}

// procesarUnRecordatorio garantiza la integridad transaccional individual
func (s *CronService) procesarUnRecordatorio(recordatorioID int64) {
	// Timeout de seguridad por si Firebase o la BD tardan demasiado
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Iniciamos transacción
	tx, err := config.DB.Begin(ctx)
	if err != nil {
		slog.Error("Cron: No se pudo abrir transacción", "id", recordatorioID, "error", err)
		return
	}
	defer tx.Rollback(ctx)

	// Bloqueo pesimista concurrente: FOR UPDATE SKIP LOCKED
	// Si tenemos 3 instancias del backend corriendo, y la Instancia A toma este registro,
	// las Instancias B y C lo ignorarán automáticamente sin quedarse colgadas esperando.
	queryLock := `
		SELECT r.evento_id, r.contenido, u.fcm_token
		FROM recordatorios r
		JOIN usuarios u ON r.destinatario_id = u.id
		WHERE r.id = $1 AND r.estado = 'pendiente'
		FOR UPDATE SKIP LOCKED
	`

	var eventoID int64
	var contenido string
	var fcmToken *string

	err = tx.QueryRow(ctx, queryLock, recordatorioID).Scan(&eventoID, &contenido, &fcmToken)
	if err != nil {
		if err == pgx.ErrNoRows {
			// Es normal: Otro worker/servidor ya lo tomó o ya fue enviado
			return
		}
		slog.Error("Cron: Error al bloquear el registro", "id", recordatorioID, "error", err)
		return
	}

	// 3. Intento de envío a Firebase
	estadoFinal := "fallido"
	if fcmToken != nil && *fcmToken != "" {
		extraData := map[string]string{
			"evento_id": fmt.Sprintf("%d", eventoID),
			"tipo":      "recordatorio_evento",
		}

		errPush := s.notificationSvc.SendDirectNotification(ctx, *fcmToken, "¡Evento próximo!", contenido, extraData)
		if errPush == nil {
			estadoFinal = "enviado"
		} else {
			slog.Error("Cron: Error con Firebase", "id", recordatorioID, "error", errPush)
		}
	} else {
		slog.Warn("Cron: Destinatario sin token registrado, marcando como fallido", "recordatorio_id", recordatorioID)
	}

	// 4. Actualización del estado
	queryUpdate := `
		UPDATE recordatorios
		SET estado = $1, fecha_envio_real = NOW()
		WHERE id = $2
	`
	_, err = tx.Exec(ctx, queryUpdate, estadoFinal, recordatorioID)
	if err != nil {
		slog.Error("Cron: Error al actualizar estado final", "id", recordatorioID, "error", err)
		return
	}

	// 5. Commit de la transacción
	if err := tx.Commit(ctx); err != nil {
		slog.Error("Cron: Error en el commit", "id", recordatorioID, "error", err)
	}
}
