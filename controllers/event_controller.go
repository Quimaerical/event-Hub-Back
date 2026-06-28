package controllers

import (
	"errors"
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

// ShowCreate renderiza el formulario de creación de eventos.
func (ctrl *EventController) ShowCreate(c *gin.Context) {
	ctx := c.Request.Context()
	categories, err := models.GetAllCategorias(ctx)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "events/create.html", gin.H{
			"error": "Error al cargar las categorías",
		})
		return
	}

	userID, _ := c.Get("userID")
	email, _ := c.Get("email")

	acceptHeader := c.GetHeader("Accept")
	if strings.Contains(acceptHeader, "application/json") {
		c.JSON(http.StatusOK, gin.H{
			"categorias": categories,
			"userID":     userID,
			"email":      email,
		})
		return
	}

	c.HTML(http.StatusOK, "events/create.html", gin.H{
		"categorias": categories,
		"userID":     userID,
		"email":      email,
	})
}

// HandleCreate procesa la creación de eventos adaptándose a JSON o Formularios HTML
func (ctrl *EventController) HandleCreate(c *gin.Context) {
	var input struct {
		models.Evento
		Categorias []int `form:"categorias" json:"categorias" binding:"required,min=1"`
	}

	// 1. Binding Automático
	if err := c.ShouldBind(&input); err != nil {
		manejarErrorRespuesta(c, http.StatusBadRequest, "Datos de entrada inválidos: "+err.Error())
		return
	}

	// 2. Extraer el ID del usuario del contexto
	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autenticado"})
		return
	}
	input.Evento.OrganizadorID = int64(userIDVal.(int))

	// 3. Ejecutar la lógica de base de datos
	ctx := c.Request.Context()
	err := models.CreateEvento(ctx, &input.Evento, input.Categorias)

	if err != nil {
		// Detección robusta del error de conflicto usando Sentinel Errors
		if errors.Is(err, models.ErrEspacioOcupado) {
			manejarErrorRespuesta(c, http.StatusConflict, "El espacio ya está reservado para ese horario.")
			return
		}
		manejarErrorRespuesta(c, http.StatusInternalServerError, "Error interno al guardar el evento")
		return
	}

	// 4. Respuesta Exitosa Dual
	acceptHeader := c.GetHeader("Accept")
	if strings.Contains(acceptHeader, "application/json") {
		c.JSON(http.StatusCreated, gin.H{
			"message": "Evento creado exitosamente. Estado actual: " + models.EstadoSolicitado,
			"evento":  input.Evento,
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/")
}

// HandleActualizarEstado maneja la aprobación o rechazo de un evento (Solo Coordinación de Cultura)
func (ctrl *EventController) HandleActualizarEstado(c *gin.Context) {
	// 1. Validación estricta de Roles (Seguridad)
	rolNombre, exists := c.Get("role_nombre")
	if !exists || rolNombre.(string) != "aprobador" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Acceso denegado: Solo la Coordinación de Cultura puede aprobar o rechazar eventos."})
		return
	}

	// 2. Extraer el ID del evento de la URL (Ej: PATCH /eventos/:id/estado)
	eventoIDStr := c.Param("id")
	eventoID, err := strconv.ParseInt(eventoIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID de evento inválido"})
		return
	}

	// 3. Parsear el cuerpo de la petición
	var input struct {
		Estado        string `json:"estado" binding:"required"`
		Observaciones string `json:"observaciones"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datos inválidos: " + err.Error()})
		return
	}

	// 4. Obtener el ID del aprobador actual
	userIDVal, _ := c.Get("userID")
	aprobadorID := int64(userIDVal.(int))

	// 5. Llamar al modelo de base de datos
	ctx := c.Request.Context()
	err = models.ActualizarEstadoEvento(ctx, eventoID, input.Estado, &aprobadorID, input.Observaciones)
	
	if err != nil {
		if err.Error() == "estado de evento inválido" || err.Error() == "las observaciones son obligatorias para rechazar o cancelar un evento" {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error interno al actualizar el estado"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"mensaje": "Estado del evento actualizado correctamente a: " + input.Estado,
	})
}

// SuggestDescription se conecta con Gemini para generar descripciones enriquecidas
func (ctrl *EventController) SuggestDescription(c *gin.Context) {
	var req struct {
		Titulo    string `json:"titulo" binding:"required"`
		Ubicacion string `json:"ubicacion" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "El título y la ubicación son requeridos"})
		return
	}

	ctx := c.Request.Context()
	suggestion, err := ctrl.geminiService.SuggestEventDescription(ctx, req.Titulo, req.Ubicacion)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error de generación con Gemini: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"descripcion": suggestion})
}

// manejarErrorRespuesta es una función auxiliar privada para mantener el código limpio (DRY)
func manejarErrorRespuesta(c *gin.Context, status int, mensaje string) {
	acceptHeader := c.GetHeader("Accept")
	if strings.Contains(acceptHeader, "application/json") {
		c.JSON(status, gin.H{"error": mensaje})
		return
	}
	
	c.HTML(status, "events/create.html", gin.H{"error": mensaje})
}
