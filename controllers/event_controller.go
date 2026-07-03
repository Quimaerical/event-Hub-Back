package controllers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"event-hub/models"
	"event-hub/services"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

type EventController struct {
	geminiService *services.GeminiService
}

func NewEventController(gemini *services.GeminiService) *EventController {
	return &EventController{
		geminiService: gemini,
	}
}

// ShowCreate displays the event creation form.
func (ctrl *EventController) ShowCreate(c *gin.Context) {
	ctx := c.Request.Context()
	categories, err := models.GetAllCategorias(ctx)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "events/create.html", gin.H{
			"error": "Error al cargar las categorías",
		})
		return
	}

	spaces, err := models.GetAllEspacios(ctx)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "events/create.html", gin.H{
			"error": "Error al cargar los espacios",
		})
		return
	}

	userID, _ := c.Get("userID")
	email, _ := c.Get("email")

	// Return JSON if requested by mobile client
	acceptHeader := c.GetHeader("Accept")
	if strings.Contains(acceptHeader, "application/json") {
		c.JSON(http.StatusOK, gin.H{
			"categorias": categories,
			"espacios":   spaces,
			"userID":     userID,
			"email":      email,
		})
		return
	}

	c.HTML(http.StatusOK, "events/create.html", gin.H{
		"categorias": categories,
		"espacios":   spaces,
		"userID":     userID,
		"email":      email,
	})
}

// HandleCreate processes the form submission to publish a new event.
func (ctrl *EventController) HandleCreate(c *gin.Context) {
	ctx := c.Request.Context()
	userIDVal, exists := c.Get("userID")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}
	userID := userIDVal.(int)

	titulo := c.PostForm("titulo")
	descripcion := c.PostForm("descripcion")
	fechaInicioStr := c.PostForm("fecha_inicio")
	fechaFinStr := c.PostForm("fecha_fin")
	espacioIDStr := c.PostForm("espacio_id")
	capacidadMaximaStr := c.PostForm("capacidad_maxima")
	categoriaIDsStr := c.PostFormArray("categorias")

	// Parse datetimes
	fechaInicio, err := time.Parse("2006-01-02T15:04", fechaInicioStr)
	if err != nil {
		categories, _ := models.GetAllCategorias(ctx)
		spaces, _ := models.GetAllEspacios(ctx)
		c.HTML(http.StatusBadRequest, "events/create.html", gin.H{
			"error":      "Formato de fecha de inicio inválido",
			"categorias": categories,
			"espacios":   spaces,
		})
		return
	}

	fechaFin, err := time.Parse("2006-01-02T15:04", fechaFinStr)
	if err != nil {
		categories, _ := models.GetAllCategorias(ctx)
		spaces, _ := models.GetAllEspacios(ctx)
		c.HTML(http.StatusBadRequest, "events/create.html", gin.H{
			"error":      "Formato de fecha de fin inválido",
			"categorias": categories,
			"espacios":   spaces,
		})
		return
	}

	espacioID, err := strconv.Atoi(espacioIDStr)
	if err != nil {
		categories, _ := models.GetAllCategorias(ctx)
		spaces, _ := models.GetAllEspacios(ctx)
		c.HTML(http.StatusBadRequest, "events/create.html", gin.H{
			"error":      "Espacio seleccionado inválido",
			"categorias": categories,
			"espacios":   spaces,
		})
		return
	}

	capacidadMaxima, err := strconv.Atoi(capacidadMaximaStr)
	if err != nil {
		categories, _ := models.GetAllCategorias(ctx)
		spaces, _ := models.GetAllEspacios(ctx)
		c.HTML(http.StatusBadRequest, "events/create.html", gin.H{
			"error":      "Capacidad máxima inválida",
			"categorias": categories,
			"espacios":   spaces,
		})
		return
	}

	// Parse categories list
	var categoryIDs []int
	for _, idStr := range categoriaIDsStr {
		if id, err := strconv.Atoi(idStr); err == nil {
			categoryIDs = append(categoryIDs, id)
		}
	}

	event := &models.Evento{
		Titulo:          titulo,
		Descripcion:     pgtype.Text{String: descripcion, Valid: descripcion != ""},
		EspacioID:       espacioID,
		OrganizadorID:   int64(userID),
		FechaInicio:     fechaInicio,
		FechaFin:        fechaFin,
		CapacidadMaxima: capacidadMaxima,
	}

	err = models.CreateEvento(ctx, event, categoryIDs)
	if err != nil {
		// Handle JSON error format
		acceptHeader := c.GetHeader("Accept")
		if strings.Contains(acceptHeader, "application/json") {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al guardar el evento: " + err.Error()})
			return
		}
		categories, _ := models.GetAllCategorias(ctx)
		spaces, _ := models.GetAllEspacios(ctx)
		c.HTML(http.StatusInternalServerError, "events/create.html", gin.H{
			"error":      "Error al guardar el evento: " + err.Error(),
			"categorias": categories,
			"espacios":   spaces,
		})
		return
	}

	// Return JSON if requested
	acceptHeader := c.GetHeader("Accept")
	if strings.Contains(acceptHeader, "application/json") {
		c.JSON(http.StatusCreated, gin.H{
			"message": "Evento creado exitosamente",
			"evento":  event,
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/")
}

// SuggestDescription handles AJAX requests to generate rich event descriptions using Gemini.
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
