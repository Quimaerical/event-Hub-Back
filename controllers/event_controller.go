package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"event-hub/models"
	"event-hub/services"

	"github.com/gin-gonic/gin"
)

type EventController struct {
	geminiService *services.GeminiService
}

func NewEventController(gemini *services.GeminiService) *EventController {
	return &EventController{
		geminiService: gemini,
	}
}

// ==========================================
// FUNCIONES AUXILIARES (Seguridad y DRY)
// ==========================================

// extractUserID extrae el ID del usuario del contexto de forma segura (Type Assertion)
func extractUserID(c *gin.Context) (int64, error) {
	val, exists := c.Get("userID")
	if !exists {
		return 0, errors.New("usuario no autenticado en el contexto")
	}

	switch v := val.(type) {
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case uint:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	default:
		return 0, fmt.Errorf("tipo de userID inesperado: %T", v)
	}
}

// manejarErrorAPI devuelve siempre un JSON (para clientes móviles o endpoints puros)
func manejarErrorAPI(c *gin.Context, status int, mensaje string) {
	c.JSON(status, gin.H{"error": mensaje})
}

// manejarErrorWeb renderiza una vista HTML inyectando el error.
func manejarErrorWeb(c *gin.Context, status int, template string, mensaje string, data gin.H) {
	data["error"] = mensaje
	c.HTML(status, template, data)
}

// responderDual decide si devolver JSON o HTML basándose en el header Accept
func responderDual(c *gin.Context, status int, template string, jsonResponse gin.H, htmlData gin.H) {
	if strings.Contains(c.GetHeader("Accept"), "application/json") {
		c.JSON(status, jsonResponse)
		return
	}
	c.HTML(status, template, htmlData)
}

// ==========================================
// ENDPOINTS DE CREACIÓN
// ==========================================

// ShowCreate renderiza el formulario de creación de eventos.
func (ctrl *EventController) ShowCreate(c *gin.Context) {
	ctx := c.Request.Context()
	categories, err := models.GetAllCategorias(ctx)
	if err != nil {
		manejarErrorWeb(c, http.StatusInternalServerError, "events/create.html", "Error al cargar las categorías", gin.H{})
		return
	}

	userID, _ := c.Get("userID")
	email, _ := c.Get("email")

	responderDual(c, http.StatusOK, "events/create.html",
		gin.H{"categorias": categories, "userID": userID, "email": email},
		gin.H{"categorias": categories, "userID": userID, "email": email},
	)
}

// HandleCreate procesa la creación de eventos adaptándose a JSON o Formularios HTML
func (ctrl *EventController) HandleCreate(c *gin.Context) {
	var input struct {
		models.Evento
		Categorias []int `form:"categorias" json:"categorias" binding:"required,min=1"`
	}

	if err := c.ShouldBind(&input); err != nil {
		responderDual(c, http.StatusBadRequest, "events/create.html",
			gin.H{"error": "Datos inválidos: " + err.Error()},
			gin.H{"error": "Datos inválidos: " + err.Error()},
		)
		return
	}

	userID, err := extractUserID(c)
	if err != nil {
		manejarErrorAPI(c, http.StatusUnauthorized, err.Error())
		return
	}
	input.Evento.OrganizadorID = userID

	ctx := c.Request.Context()
	err = models.CreateEvento(ctx, &input.Evento, input.Categorias)

	if err != nil {
		if errors.Is(err, models.ErrEspacioOcupado) {
			responderDual(c, http.StatusConflict, "events/create.html", gin.H{"error": err.Error()}, gin.H{"error": err.Error()})
			return
		}
		responderDual(c, http.StatusInternalServerError, "events/create.html", gin.H{"error": "Error interno al guardar"}, gin.H{"error": "Error interno"})
		return
	}

	if strings.Contains(c.GetHeader("Accept"), "application/json") {
		c.JSON(http.StatusCreated, gin.H{"message": "Evento creado", "evento": input.Evento})
		return
	}
	c.Redirect(http.StatusSeeOther, "/")
}

// ==========================================
// ENDPOINTS DE LECTURA (GET)
// ==========================================

// HandleListEvents maneja GET /eventos
func (ctrl *EventController) HandleListEvents(c *gin.Context) {
	search := c.Query("search")
	categoryIDStr := c.Query("category_id")

	var categoryID int
	if categoryIDStr != "" {
		parsed, err := strconv.Atoi(categoryIDStr)
		if err == nil {
			categoryID = parsed
		}
	}

	ctx := c.Request.Context()
	eventos, err := models.SearchEventos(ctx, search, categoryID)
	if err != nil {
		responderDual(c, http.StatusInternalServerError, "events/list.html", gin.H{"error": "Error al buscar eventos"}, gin.H{"error": "Error al buscar"})
		return
	}

	responderDual(c, http.StatusOK, "events/list.html", gin.H{"eventos": eventos}, gin.H{"eventos": eventos})
}

// HandleGetEvent maneja GET /eventos/:id
func (ctrl *EventController) HandleGetEvent(c *gin.Context) {
	eventoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		manejarErrorAPI(c, http.StatusBadRequest, "ID de evento inválido")
		return
	}

	ctx := c.Request.Context()
	evento, err := models.GetEventoByID(ctx, eventoID)
	if err != nil {
		responderDual(c, http.StatusNotFound, "events/detail.html", gin.H{"error": "Evento no encontrado"}, gin.H{"error": "Evento no encontrado"})
		return
	}

	responderDual(c, http.StatusOK, "events/detail.html", gin.H{"evento": evento}, gin.H{"evento": evento})
}

// ==========================================
// ENDPOINTS DE ACTUALIZACIÓN Y CANCELACIÓN
// ==========================================

// HandleActualizarEstado maneja PATCH /eventos/:id/estado (Solo Cultura)
func (ctrl *EventController) HandleActualizarEstado(c *gin.Context) {
	rolNombre, exists := c.Get("role_nombre")
	if !exists || rolNombre.(string) != "aprobador" {
		manejarErrorAPI(c, http.StatusForbidden, "Acceso denegado: Solo Coordinación de Cultura puede aprobar eventos")
		return
	}

	eventoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		manejarErrorAPI(c, http.StatusBadRequest, "ID inválido")
		return
	}

	var input struct {
		Estado        string `json:"estado" form:"estado" binding:"required"`
		Observaciones string `json:"observaciones" form:"observaciones"`
	}
	if err := c.ShouldBind(&input); err != nil {
		responderDual(c, http.StatusBadRequest, "events/detail.html",
			gin.H{"error": "Datos inválidos: " + err.Error()},
			gin.H{"error": "Datos inválidos: " + err.Error()})
		return
	}

	aprobadorID, err := extractUserID(c)
	if err != nil {
		manejarErrorAPI(c, http.StatusUnauthorized, err.Error())
		return
	}

	ctx := c.Request.Context()
	err = models.ActualizarEstadoEvento(ctx, eventoID, input.Estado, &aprobadorID, input.Observaciones)

	if err != nil {
		// Detección de Sentinel Errors
		if errors.Is(err, models.ErrEstadoInvalido) || errors.Is(err, models.ErrObservacionesRequeridas) || errors.Is(err, models.ErrTransicionEstadoInvalida) {
			responderDual(c, http.StatusBadRequest, "events/detail.html",
				gin.H{"error": err.Error()},
				gin.H{"error": err.Error()})
			return
		}
		responderDual(c, http.StatusInternalServerError, "events/detail.html",
			gin.H{"error": "Error interno al actualizar estado"},
			gin.H{"error": "Error interno al actualizar estado"})
		return
	}

	responderDual(c, http.StatusOK, "events/detail.html",
		gin.H{"mensaje": "Estado actualizado a: " + input.Estado},
		gin.H{"mensaje": "Estado actualizado a: " + input.Estado})
}

// HandleCancelEvent maneja DELETE /eventos/:id (Organizador o Cultura)
func (ctrl *EventController) HandleCancelEvent(c *gin.Context) {
	eventoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		manejarErrorAPI(c, http.StatusBadRequest, "ID inválido")
		return
	}

	observaciones := c.PostForm("observaciones")
	if observaciones == "" {
		var input struct {
			Observaciones string `json:"observaciones"`
		}
		if err := c.ShouldBindJSON(&input); err == nil {
			observaciones = input.Observaciones
		}
	}

	if observaciones == "" {
		manejarErrorAPI(c, http.StatusBadRequest, "Debe proporcionar el motivo de la cancelación en observaciones")
		return
	}

	userID, err := extractUserID(c)
	if err != nil {
		manejarErrorAPI(c, http.StatusUnauthorized, err.Error())
		return
	}

	ctx := c.Request.Context()
	evento, err := models.GetEventoByID(ctx, eventoID)
	if err != nil {
		manejarErrorAPI(c, http.StatusNotFound, "Evento no encontrado")
		return
	}

	rolNombre, _ := c.Get("role_nombre")
	esAprobador := rolNombre != nil && rolNombre.(string) == "aprobador"
	esOrganizador := evento.OrganizadorID == userID

	if !esAprobador && !esOrganizador {
		manejarErrorAPI(c, http.StatusForbidden, "Solo el organizador o la Coordinación de Cultura pueden cancelar este evento")
		return
	}

	var aprobadorID *int64
	if esAprobador {
		aprobadorID = &userID
	}

	err = models.ActualizarEstadoEvento(ctx, eventoID, models.EstadoCancelado, aprobadorID, observaciones)
	if err != nil {
		// FIX: Manejo adecuado de errores de negocio enviando Bad Request (400)
		if errors.Is(err, models.ErrTransicionEstadoInvalida) || errors.Is(err, models.ErrObservacionesRequeridas) {
			manejarErrorAPI(c, http.StatusBadRequest, err.Error())
			return
		}
		manejarErrorAPI(c, http.StatusInternalServerError, "Error al cancelar el evento: "+err.Error())
		return
	}

	// FIX: Uso de responderDual en lugar de solo responder JSON
	responderDual(c, http.StatusOK, "events/list.html",
		gin.H{"mensaje": "Evento cancelado exitosamente"},
		gin.H{"mensaje": "Evento cancelado exitosamente"})
}

// ==========================================
// SERVICIOS EXTERNOS
// ==========================================

// SuggestDescription se conecta con Gemini (Requiere payload JSON)
func (ctrl *EventController) SuggestDescription(c *gin.Context) {
	if ctrl.geminiService == nil {
		manejarErrorAPI(c, http.StatusInternalServerError, "El servicio de IA no está configurado")
		return
	}

	var req struct {
		Titulo    string `json:"titulo" binding:"required"`
		Ubicacion string `json:"ubicacion" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		manejarErrorAPI(c, http.StatusBadRequest, "El título y la ubicación son requeridos")
		return
	}

	ctx := c.Request.Context()
	suggestion, err := ctrl.geminiService.SuggestEventDescription(ctx, req.Titulo, req.Ubicacion)
	if err != nil {
		manejarErrorAPI(c, http.StatusInternalServerError, "Error de generación con Gemini: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"descripcion": suggestion})
}
