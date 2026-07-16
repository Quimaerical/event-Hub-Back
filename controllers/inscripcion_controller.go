package controllers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"event-hub/models"
	"event-hub/services"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

type InscripcionController struct {
	calendarService *services.CalendarService
}

func NewInscripcionController(calService *services.CalendarService) *InscripcionController {
	return &InscripcionController{
		calendarService: calService,
	}
}

// HandleInscribir maneja POST /eventos/:id/inscribir
func (ctrl *InscripcionController) HandleInscribir(c *gin.Context) {
	eventoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		manejarErrorAPI(c, http.StatusBadRequest, "ID de evento inválido")
		return
	}

	userID, err := extractUserID(c)
	if err != nil {
		manejarErrorAPI(c, http.StatusUnauthorized, "No autorizado")
		return
	}

	// Asumiendo que el middleware inyecta el email (requerido para Calendar)
	emailVal, exists := c.Get("email")
	if !exists {
		manejarErrorAPI(c, http.StatusBadRequest, "No se encontró el email del usuario en la sesión")
		return
	}
	email := emailVal.(string)

	ctx := c.Request.Context()
	evento, err := models.InscribirUsuario(ctx, eventoID, userID, email)

	if err != nil {
		switch {
		case errors.Is(err, models.ErrCupoCompleto):
			responderDual(c, http.StatusConflict, "events/detail.html", gin.H{"error": err.Error()}, gin.H{"error": err.Error()})
		case errors.Is(err, models.ErrUsuarioYaInscrito), errors.Is(err, models.ErrEventoNoInscribible):
			responderDual(c, http.StatusBadRequest, "events/detail.html", gin.H{"error": err.Error()}, gin.H{"error": err.Error()})
		default:
			slog.Error("Error interno al inscribir usuario", "error", err, "evento_id", eventoID)
			responderDual(c, http.StatusInternalServerError, "events/detail.html", gin.H{"error": "Error interno"}, gin.H{"error": "Error interno"})
		}
		return
	}

	slog.Info("Usuario inscrito con éxito", "evento_id", eventoID, "usuario_id", userID)

	// Llamada asíncrona a Google Calendar (Fire & Forget)
	if evento.CalendarEventID.Valid && evento.CalendarEventID.String != "" {
		// Mock: Obtener token del ORGANIZADOR del evento para darle permisos a su calendario
		var orgToken *oauth2.Token // token := services.ObtenerTokenUsuario(evento.OrganizadorID)
		
		if orgToken != nil {
			go func(calID, userEmail string, token *oauth2.Token) {
				bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				
				errCal := ctrl.calendarService.AddAttendee(bgCtx, token, calID, userEmail)
				if errCal != nil {
					slog.Error("Fallo al enviar invitación de Google Calendar", "calendar_id", calID, "email", userEmail, "error", errCal)
				}
			}(evento.CalendarEventID.String, email, orgToken)
		}
	}

	responderDual(c, http.StatusOK, "events/detail.html", 
		gin.H{"mensaje": "Inscripción exitosa"}, 
		gin.H{"mensaje": "Inscripción exitosa"},
	)
}

// HandleCancelarInscripcion maneja DELETE /inscripciones/:id
func (ctrl *InscripcionController) HandleCancelarInscripcion(c *gin.Context) {
	inscID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		manejarErrorAPI(c, http.StatusBadRequest, "ID inválido")
		return
	}

	userID, err := extractUserID(c)
	if err != nil {
		manejarErrorAPI(c, http.StatusUnauthorized, "No autorizado")
		return
	}

	rolNombre, _ := c.Get("role_nombre")
	esAprobador := rolNombre != nil && rolNombre.(string) == models.RolAprobador

	ctx := c.Request.Context()
	err = models.CancelarInscripcion(ctx, inscID, userID, esAprobador)
	
	if err != nil {
		if errors.Is(err, models.ErrInscripcionNoEncontrada) {
			manejarErrorAPI(c, http.StatusNotFound, "Inscripción no encontrada o no tienes permisos")
			return
		}
		slog.Error("Error cancelando inscripción", "inscripcion_id", inscID, "error", err)
		manejarErrorAPI(c, http.StatusInternalServerError, "Error interno")
		return
	}

	slog.Info("Inscripción cancelada", "inscripcion_id", inscID, "por_usuario", userID)
	manejarErrorAPI(c, http.StatusOK, "Inscripción cancelada exitosamente")
}
