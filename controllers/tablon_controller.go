package controllers

import (
	"log/slog"
	"net/http"

	"event-hub/models"

	"github.com/gin-gonic/gin"
)

type TablonController struct{}

func NewTablonController() *TablonController {
	return &TablonController{}
}

func (ctrl *TablonController) ShowTablon(c *gin.Context) {
	userID, err := extractUserID(c)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	ctx := c.Request.Context()

	// 1. Obtener eventos a los que asistirá el usuario (reservas activas)
	eventosAsistira, err := models.GetEventosInscritoByUsuario(ctx, userID)
	if err != nil {
		slog.Error("Error al obtener eventos a los que asistirá el usuario", "user_id", userID, "error", err)
		eventosAsistira = []models.Evento{}
	}

	// 2. Obtener eventos creados por el usuario
	eventosCreados, err := models.GetEventosCreadosByUsuario(ctx, userID)
	if err != nil {
		slog.Error("Error al obtener eventos creados por el usuario", "user_id", userID, "error", err)
		eventosCreados = []models.Evento{}
	}

	email, _ := c.Get("email")
	var roleID int
	if rVal, exists := c.Get("roleID"); exists {
		switch r := rVal.(type) {
		case int:
			roleID = r
		case int64:
			roleID = int(r)
		case float64:
			roleID = int(r)
		}
	}

	responderDual(c, http.StatusOK, "tablon/index.html",
		gin.H{
			"eventos_asistira": eventosAsistira,
			"eventos_creados":  eventosCreados,
		},
		gin.H{
			"eventos_asistira": eventosAsistira,
			"eventos_creados":  eventosCreados,
			"userID":           userID,
			"email":            email,
			"roleID":           roleID,
			"activeTab":        c.DefaultQuery("tab", "asistire"),
		},
	)
}
