package controllers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"event-hub/models"
	"event-hub/services"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/oauth2"
	"golang.org/x/time/rate"
)

// Wrapper para el Rate Limiter que permite limpieza (Garbage Collection)
type visitorLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type EventController struct {
	geminiService       *services.GeminiService
	calendarService     *services.CalendarService
	notificationService *services.NotificationService // NUEVO: Servicio inyectado
	rateLimiters        map[string]*visitorLimiter
	mu                  sync.RWMutex
}

// Inicialización del controlador con servicios inyectados
func NewEventController(gemini *services.GeminiService, calendar *services.CalendarService, notification *services.NotificationService) *EventController {
	ctrl := &EventController{
		geminiService:       gemini,
		calendarService:     calendar,
		notificationService: notification,
		rateLimiters:        make(map[string]*visitorLimiter),
	}
	
	// Lanzar Goroutine en background para evitar Memory Leaks
	go ctrl.cleanupVisitors()
	return ctrl
}

// cleanupVisitors elimina de RAM los limitadores inactivos por más de 5 minutos
func (ctrl *EventController) cleanupVisitors() {
	for {
		time.Sleep(time.Minute * 2)
		ctrl.mu.Lock()
		for ip, v := range ctrl.rateLimiters {
			if time.Since(v.lastSeen) > time.Minute*5 {
				delete(ctrl.rateLimiters, ip)
			}
		}
		ctrl.mu.Unlock()
	}
}

// ==========================================
// FUNCIONES AUXILIARES (Seguridad y DRY)
// ==========================================

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

func manejarErrorAPI(c *gin.Context, status int, mensaje string) {
	c.JSON(status, gin.H{"error": mensaje})
}

func manejarErrorWeb(c *gin.Context, status int, template string, mensaje string, data gin.H) {
	data["error"] = mensaje
	c.HTML(status, template, data)
}

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

func (ctrl *EventController) ShowCreate(c *gin.Context) {
	ctx := c.Request.Context()
	categories, err := models.GetAllCategorias(ctx)
	if err != nil {
		slog.Error("Error al cargar categorías", "error", err)
		manejarErrorWeb(c, http.StatusInternalServerError, "events/create.html", "Error al cargar las categorías", gin.H{})
		return
	}

	espacios, err := models.GetAllEspacios(ctx)
	if err != nil {
		slog.Error("Error al cargar espacios", "error", err)
		manejarErrorWeb(c, http.StatusInternalServerError, "events/create.html", "Error al cargar los espacios", gin.H{})
		return
	}

	userID, _ := c.Get("userID")
	email, _ := c.Get("email")

	responderDual(c, http.StatusOK, "events/create.html",
		gin.H{"categorias": categories, "espacios": espacios, "userID": userID, "email": email},
		gin.H{"categorias": categories, "espacios": espacios, "userID": userID, "email": email},
	)
}

func (ctrl *EventController) HandleCreate(c *gin.Context) {
	ctx := c.Request.Context()
	categories, _ := models.GetAllCategorias(ctx)
	espacios, _ := models.GetAllEspacios(ctx)

	var form struct {
		Titulo          string `form:"titulo" json:"titulo" binding:"required"`
		Descripcion     string `form:"descripcion" json:"descripcion" binding:"required"`
		EspacioID       int    `form:"espacio_id" json:"espacio_id" binding:"required"`
		FechaInicioStr  string `form:"fecha_inicio" json:"fecha_inicio" binding:"required"`
		FechaFinStr     string `form:"fecha_fin" json:"fecha_fin" binding:"required"`
		CapacidadMaxima int    `form:"capacidad_maxima" json:"capacidad_maxima" binding:"required,gt=0"`
		Categorias      []int  `form:"categorias" json:"categorias" binding:"required,min=1"`
	}

	var err error
	if strings.Contains(c.ContentType(), "application/json") {
		err = c.ShouldBindWith(&form, binding.JSON)
	} else {
		err = c.ShouldBindWith(&form, binding.Form)
	}

	if err != nil {
		slog.Warn("Intento de creación con datos inválidos", "error", err)
		responderDual(c, http.StatusBadRequest, "events/create.html",
			gin.H{"error": "Datos inválidos: " + err.Error()},
			gin.H{"error": "Por favor selecciona al menos una categoría y completa todos los campos", "categorias": categories, "espacios": espacios},
		)
		return
	}

	// Parse datetime-local string inputs preserving local timezone and exact hours/minutes
	parseDate := func(s string) (time.Time, error) {
		formats := []string{
			"2006-01-02T15:04",
			"2006-01-02T15:04:05",
			time.RFC3339,
		}
		for _, f := range formats {
			if t, err := time.ParseInLocation(f, s, time.Local); err == nil {
				return t, nil
			}
		}
		return time.Time{}, fmt.Errorf("formato de fecha inválido: %s", s)
	}

	fechaInicio, err := parseDate(form.FechaInicioStr)
	if err != nil {
		responderDual(c, http.StatusBadRequest, "events/create.html",
			gin.H{"error": err.Error()},
			gin.H{"error": "Formato de fecha de inicio inválido", "categorias": categories, "espacios": espacios},
		)
		return
	}

	fechaFin, err := parseDate(form.FechaFinStr)
	if err != nil {
		responderDual(c, http.StatusBadRequest, "events/create.html",
			gin.H{"error": err.Error()},
			gin.H{"error": "Formato de fecha de fin inválido", "categorias": categories, "espacios": espacios},
		)
		return
	}

	if !fechaFin.After(fechaInicio) {
		msg := fmt.Sprintf("La fecha y hora de fin (%s) debe ser posterior a la de inicio (%s). Si tu evento termina en la madrugada o al día siguiente, recuerda cambiar el día en la fecha de fin.",
			fechaFin.Format("02/01/2006 03:04 PM"),
			fechaInicio.Format("02/01/2006 03:04 PM"),
		)
		slog.Warn("Intento de creación con fechas incoherentes", "fecha_inicio", fechaInicio, "fecha_fin", fechaFin)
		responderDual(c, http.StatusBadRequest, "events/create.html",
			gin.H{"error": msg},
			gin.H{"error": msg, "categorias": categories, "espacios": espacios},
		)
		return
	}

	userID, err := extractUserID(c)
	if err != nil {
		slog.Warn("Fallo de autenticación", "error", err)
		manejarErrorAPI(c, http.StatusUnauthorized, err.Error())
		return
	}

	evento := models.Evento{
		Titulo:          form.Titulo,
		Descripcion:     pgtype.Text{String: form.Descripcion, Valid: form.Descripcion != ""},
		EspacioID:       form.EspacioID,
		OrganizadorID:   userID,
		FechaInicio:     fechaInicio,
		FechaFin:        fechaFin,
		CapacidadMaxima: form.CapacidadMaxima,
	}

	err = models.CreateEvento(ctx, &evento, form.Categorias)

	if err != nil {
		if errors.Is(err, models.ErrEspacioOcupado) {
			slog.Warn("Conflicto de espacio ocupado", "user_id", userID, "espacio_id", form.EspacioID)
			responderDual(c, http.StatusConflict, "events/create.html", gin.H{"error": err.Error()}, gin.H{"error": err.Error(), "categorias": categories, "espacios": espacios})
			return
		}
		slog.Error("Error interno al crear evento", "error", err, "user_id", userID)
		responderDual(c, http.StatusInternalServerError, "events/create.html", gin.H{"error": "Error interno al guardar"}, gin.H{"error": "Error interno al guardar el evento", "categorias": categories, "espacios": espacios})
		return
	}

	slog.Info("Evento creado localmente", "evento_id", evento.ID, "user_id", userID)

	// FIX: Reemplazar el mock por la obtención real del token desde BD
	tokenOAuth, err := models.GetGoogleToken(ctx, int(userID))
	if err != nil {
		slog.Warn("Usuario sin token de Google Calendar, omitiendo sincronización", "user_id", userID)
	} else if ctrl.calendarService != nil {
		go func(ev models.Evento, token *oauth2.Token) {
			bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			calID, errCal := ctrl.calendarService.CreateCalendarEvent(bgCtx, token, &ev)
			if errCal != nil {
				slog.Error("Fallo al sincronizar con Google Calendar", "evento_id", ev.ID, "error", errCal)
				return
			}
			
			_ = models.UpdateCalendarEventID(bgCtx, ev.ID, calID)
		}(evento, tokenOAuth)
	}

	if strings.Contains(c.GetHeader("Accept"), "application/json") {
		c.JSON(http.StatusCreated, gin.H{"message": "Evento creado", "evento": evento})
		return
	}

	c.Redirect(http.StatusSeeOther, "/")
}

// ==========================================
// ENDPOINTS DE LECTURA (GET)
// ==========================================

func (ctrl *EventController) HandleListEvents(c *gin.Context) {
	var filtro models.FiltroEvento
	
	filtro.Search = c.Query("search")
	if id, err := strconv.Atoi(c.Query("category_id")); err == nil {
		filtro.CategoryID = id
	}
	filtro.Estado = c.Query("estado")
	if id, err := strconv.Atoi(c.Query("espacio_id")); err == nil {
		filtro.EspacioID = id
	}
	if id, err := strconv.ParseInt(c.Query("organizador_id"), 10, 64); err == nil {
		filtro.OrganizadorID = id
	}
	if p, err := strconv.Atoi(c.Query("page")); err == nil {
		filtro.Page = p
	}
	if l, err := strconv.Atoi(c.Query("limit")); err == nil {
		filtro.Limit = l
	}

	ctx := c.Request.Context()
	eventos, total, err := models.SearchEventos(ctx, filtro)
	if err != nil {
		slog.Error("Error al buscar eventos", "error", err, "filtro", filtro)
		responderDual(c, http.StatusInternalServerError, "events/list.html", gin.H{"error": "Error al buscar eventos"}, gin.H{"error": "Error al buscar"})
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(filtro.Limit)))
	if totalPages < 1 {
		totalPages = 1
	}

	slog.Info("Eventos listados exitosamente", "total_resultados", total)
	respuestaJSON := gin.H{"eventos": eventos, "meta": gin.H{"total": total, "page": filtro.Page, "limit": filtro.Limit, "total_pages": totalPages}}
	respuestaHTML := gin.H{"eventos": eventos, "total": total, "page": filtro.Page, "limit": filtro.Limit, "total_pages": totalPages}
	responderDual(c, http.StatusOK, "events/list.html", respuestaJSON, respuestaHTML)
}

func (ctrl *EventController) HandleGetEvent(c *gin.Context) {
	eventoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		manejarErrorAPI(c, http.StatusBadRequest, "ID de evento inválido")
		return
	}

	ctx := c.Request.Context()
	evento, err := models.GetEventoByID(ctx, eventoID)
	if err != nil {
		slog.Warn("Intento de acceso a evento no encontrado", "evento_id", eventoID)
		responderDual(c, http.StatusNotFound, "events/detail.html", gin.H{"error": "Evento no encontrado"}, gin.H{"error": "Evento no encontrado"})
		return
	}

	// Obtener categorías del evento
	categorias, _ := models.GetCategoriasForEvento(ctx, eventoID)
	evento.Categorias = categorias

	// Conteo de inscritos y disponibilidad de cupos
	conteoInscritos, _ := models.GetConteoInscritos(ctx, eventoID)
	cuposDisponibles := evento.CapacidadMaxima - conteoInscritos
	if cuposDisponibles < 0 {
		cuposDisponibles = 0
	}

	// Comprobar estado de autenticación y rol del usuario
	var userID int64
	var roleID int
	if uVal, exists := c.Get("userID"); exists {
		if id, ok := uVal.(int64); ok {
			userID = id
		}
	}
	if rVal, exists := c.Get("role_id"); exists {
		if r, ok := rVal.(int); ok {
			roleID = r
		}
	}

	esCreador := userID > 0 && evento.OrganizadorID == userID
	esAdminOrApprover := roleID == 1 || roleID == 2

	// Si el evento está en estado 'solicitado', solo es visible para su creador y admin/aprobador
	if evento.Estado == models.EstadoSolicitado && !esCreador && !esAdminOrApprover {
		responderDual(c, http.StatusForbidden, "events/detail.html",
			gin.H{"error": "Este evento aún está en revisión y no es público"},
			gin.H{"error": "Este evento aún está pendiente de aprobación por la administración"})
		return
	}

	// Verificar si el usuario actual está inscrito
	var estaInscrito bool
	if userID > 0 {
		estaInscrito, _ = models.EstaInscrito(ctx, eventoID, userID)
	}

	email, _ := c.Get("email")

	data := gin.H{
		"evento":            evento,
		"categorias":        categorias,
		"conteoInscritos":   conteoInscritos,
		"cuposDisponibles":  cuposDisponibles,
		"estaInscrito":      estaInscrito,
		"esCreador":         esCreador,
		"esAdminOrApprover": esAdminOrApprover,
		"userID":            userID,
		"email":             email,
	}

	responderDual(c, http.StatusOK, "events/detail.html", data, data)
}

func (ctrl *EventController) HandleInscribirEvent(c *gin.Context) {
	eventoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		manejarErrorAPI(c, http.StatusBadRequest, "ID de evento inválido")
		return
	}

	userID, err := extractUserID(c)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	emailVal, _ := c.Get("email")
	emailStr, _ := emailVal.(string)

	ctx := c.Request.Context()
	_, err = models.InscribirUsuario(ctx, eventoID, userID, emailStr)
	if err != nil {
		slog.Warn("Fallo al inscribir usuario", "evento_id", eventoID, "user_id", userID, "error", err)
		if errors.Is(err, models.ErrUsuarioYaInscrito) || errors.Is(err, models.ErrCupoCompleto) || errors.Is(err, models.ErrEventoNoInscribible) {
			responderDual(c, http.StatusBadRequest, "events/detail.html", gin.H{"error": err.Error()}, gin.H{"error": err.Error()})
			return
		}
		responderDual(c, http.StatusInternalServerError, "events/detail.html", gin.H{"error": "Error interno al procesar la reserva"}, gin.H{"error": "Error interno al procesar tu inscripción"})
		return
	}

	slog.Info("Inscripción exitosa", "evento_id", eventoID, "user_id", userID)
	if strings.Contains(c.GetHeader("Accept"), "application/json") {
		c.JSON(http.StatusOK, gin.H{"message": "Inscripción confirmada"})
		return
	}
	c.Redirect(http.StatusSeeOther, fmt.Sprintf("/eventos/%d", eventoID))
}

func (ctrl *EventController) HandleCancelarInscripcion(c *gin.Context) {
	eventoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		manejarErrorAPI(c, http.StatusBadRequest, "ID de evento inválido")
		return
	}

	userID, err := extractUserID(c)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	ctx := c.Request.Context()
	err = models.CancelarInscripcionPorEventoYUsuario(ctx, eventoID, userID)
	if err != nil {
		slog.Warn("Fallo al cancelar reserva", "evento_id", eventoID, "user_id", userID, "error", err)
		responderDual(c, http.StatusBadRequest, "events/detail.html", gin.H{"error": err.Error()}, gin.H{"error": "No se pudo cancelar la inscripción"})
		return
	}

	slog.Info("Inscripción cancelada", "evento_id", eventoID, "user_id", userID)
	if strings.Contains(c.GetHeader("Accept"), "application/json") {
		c.JSON(http.StatusOK, gin.H{"message": "Inscripción cancelada"})
		return
	}
	c.Redirect(http.StatusSeeOther, fmt.Sprintf("/eventos/%d", eventoID))
}

func (ctrl *EventController) HandleAprobarEvent(c *gin.Context) {
	eventoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		manejarErrorAPI(c, http.StatusBadRequest, "ID de evento inválido")
		return
	}

	userID, err := extractUserID(c)
	if err != nil {
		manejarErrorAPI(c, http.StatusUnauthorized, "Autenticación requerida")
		return
	}

	ctx := c.Request.Context()
	err = models.ActualizarEstadoEvento(ctx, eventoID, models.EstadoAprobado, &userID, "Aprobado por administración")
	if err != nil {
		slog.Error("Error al aprobar evento", "evento_id", eventoID, "error", err)
		responderDual(c, http.StatusBadRequest, "events/detail.html", gin.H{"error": err.Error()}, gin.H{"error": "No se pudo aprobar el evento: " + err.Error()})
		return
	}

	slog.Info("Evento aprobado exitosamente", "evento_id", eventoID, "aprobador_id", userID)
	if strings.Contains(c.GetHeader("Accept"), "application/json") {
		c.JSON(http.StatusOK, gin.H{"message": "Evento aprobado exitosamente"})
		return
	}
	c.Redirect(http.StatusSeeOther, fmt.Sprintf("/eventos/%d", eventoID))
}

func (ctrl *EventController) HandleRechazarEvent(c *gin.Context) {
	eventoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		manejarErrorAPI(c, http.StatusBadRequest, "ID de evento inválido")
		return
	}

	userID, err := extractUserID(c)
	if err != nil {
		manejarErrorAPI(c, http.StatusUnauthorized, "Autenticación requerida")
		return
	}

	observaciones := c.PostForm("observaciones")
	if observaciones == "" {
		observaciones = "Rechazado por administración"
	}

	ctx := c.Request.Context()
	err = models.ActualizarEstadoEvento(ctx, eventoID, models.EstadoRechazado, &userID, observaciones)
	if err != nil {
		slog.Error("Error al rechazar evento", "evento_id", eventoID, "error", err)
		responderDual(c, http.StatusBadRequest, "events/detail.html", gin.H{"error": err.Error()}, gin.H{"error": "No se pudo rechazar el evento: " + err.Error()})
		return
	}

	slog.Info("Evento rechazado", "evento_id", eventoID, "aprobador_id", userID)
	if strings.Contains(c.GetHeader("Accept"), "application/json") {
		c.JSON(http.StatusOK, gin.H{"message": "Evento rechazado"})
		return
	}
	c.Redirect(http.StatusSeeOther, fmt.Sprintf("/eventos/%d", eventoID))
}

func (ctrl *EventController) ShowEdit(c *gin.Context) {
	eventoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}

	userID, err := extractUserID(c)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	ctx := c.Request.Context()
	evento, err := models.GetEventoByID(ctx, eventoID)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}

	rolNombre, _ := c.Get("role_nombre")
	esAdmin := rolNombre != nil && (rolNombre.(string) == "administrador" || rolNombre.(string) == "aprobador")
	if evento.OrganizadorID != userID && !esAdmin {
		c.Redirect(http.StatusSeeOther, fmt.Sprintf("/eventos/%d", eventoID))
		return
	}

	categories, _ := models.GetAllCategorias(ctx)
	espacios, _ := models.GetAllEspacios(ctx)
	eventoCat, _ := models.GetCategoriasForEvento(ctx, eventoID)

	catMap := make(map[int]bool)
	for _, c := range eventoCat {
		catMap[c.ID] = true
	}

	email, _ := c.Get("email")
	responderDual(c, http.StatusOK, "events/edit.html",
		gin.H{"evento": evento, "categorias": categories, "espacios": espacios},
		gin.H{
			"evento":           evento,
			"categorias":       categories,
			"espacios":         espacios,
			"catMap":           catMap,
			"fecha_inicio_str": evento.FechaInicio.Format("2006-01-02T15:04"),
			"fecha_fin_str":    evento.FechaFin.Format("2006-01-02T15:04"),
			"userID":           userID,
			"email":            email,
		},
	)
}

func (ctrl *EventController) HandleEdit(c *gin.Context) {
	eventoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		manejarErrorAPI(c, http.StatusBadRequest, "ID inválido")
		return
	}

	userID, err := extractUserID(c)
	if err != nil {
		manejarErrorAPI(c, http.StatusUnauthorized, "Autenticación requerida")
		return
	}

	ctx := c.Request.Context()
	eventoOriginal, err := models.GetEventoByID(ctx, eventoID)
	if err != nil {
		manejarErrorAPI(c, http.StatusNotFound, "Evento no encontrado")
		return
	}

	rolNombre, _ := c.Get("role_nombre")
	esAdmin := rolNombre != nil && (rolNombre.(string) == "administrador" || rolNombre.(string) == "aprobador")
	if eventoOriginal.OrganizadorID != userID && !esAdmin {
		manejarErrorAPI(c, http.StatusForbidden, "No tienes permiso para editar este evento")
		return
	}

	categories, _ := models.GetAllCategorias(ctx)
	espacios, _ := models.GetAllEspacios(ctx)

	var form struct {
		Titulo          string `form:"titulo" json:"titulo" binding:"required"`
		Descripcion     string `form:"descripcion" json:"descripcion" binding:"required"`
		EspacioID       int    `form:"espacio_id" json:"espacio_id" binding:"required"`
		FechaInicioStr  string `form:"fecha_inicio" json:"fecha_inicio" binding:"required"`
		FechaFinStr     string `form:"fecha_fin" json:"fecha_fin" binding:"required"`
		CapacidadMaxima int    `form:"capacidad_maxima" json:"capacidad_maxima" binding:"required,gt=0"`
		Categorias      []int  `form:"categorias" json:"categorias" binding:"required,min=1"`
	}

	if strings.Contains(c.ContentType(), "application/json") {
		err = c.ShouldBindWith(&form, binding.JSON)
	} else {
		err = c.ShouldBindWith(&form, binding.Form)
	}

	if err != nil {
		responderDual(c, http.StatusBadRequest, "events/edit.html",
			gin.H{"error": "Datos inválidos: " + err.Error()},
			gin.H{"error": "Por favor completa todos los campos correctamente", "evento": eventoOriginal, "categorias": categories, "espacios": espacios},
		)
		return
	}

	parseDate := func(s string) (time.Time, error) {
		formats := []string{"2006-01-02T15:04", "2006-01-02T15:04:05", time.RFC3339}
		for _, f := range formats {
			if t, err := time.ParseInLocation(f, s, time.Local); err == nil {
				return t, nil
			}
		}
		return time.Time{}, fmt.Errorf("formato de fecha inválido: %s", s)
	}

	fechaInicio, err := parseDate(form.FechaInicioStr)
	if err != nil {
		responderDual(c, http.StatusBadRequest, "events/edit.html", gin.H{"error": err.Error()}, gin.H{"error": "Fecha de inicio inválida", "evento": eventoOriginal, "categorias": categories, "espacios": espacios})
		return
	}

	fechaFin, err := parseDate(form.FechaFinStr)
	if err != nil {
		responderDual(c, http.StatusBadRequest, "events/edit.html", gin.H{"error": err.Error()}, gin.H{"error": "Fecha de fin inválida", "evento": eventoOriginal, "categorias": categories, "espacios": espacios})
		return
	}

	if !fechaFin.After(fechaInicio) {
		msg := "La fecha y hora de fin debe ser posterior a la de inicio"
		responderDual(c, http.StatusBadRequest, "events/edit.html", gin.H{"error": msg}, gin.H{"error": msg, "evento": eventoOriginal, "categorias": categories, "espacios": espacios})
		return
	}

	eventoOriginal.Titulo = form.Titulo
	eventoOriginal.Descripcion = pgtype.Text{String: form.Descripcion, Valid: form.Descripcion != ""}
	eventoOriginal.EspacioID = form.EspacioID
	eventoOriginal.FechaInicio = fechaInicio
	eventoOriginal.FechaFin = fechaFin
	eventoOriginal.CapacidadMaxima = form.CapacidadMaxima

	err = models.UpdateEvento(ctx, eventoOriginal, form.Categorias)
	if err != nil {
		slog.Error("Error al actualizar evento", "evento_id", eventoID, "error", err)
		responderDual(c, http.StatusInternalServerError, "events/edit.html", gin.H{"error": "Error interno al guardar cambios"}, gin.H{"error": "Error al guardar cambios: " + err.Error(), "evento": eventoOriginal, "categorias": categories, "espacios": espacios})
		return
	}

	slog.Info("Evento actualizado exitosamente", "evento_id", eventoID, "user_id", userID)
	if strings.Contains(c.GetHeader("Accept"), "application/json") {
		c.JSON(http.StatusOK, gin.H{"message": "Evento actualizado exitosamente", "evento": eventoOriginal})
		return
	}

	c.Redirect(http.StatusSeeOther, fmt.Sprintf("/eventos/%d", eventoID))
}

// ==========================================
// ENDPOINTS DE ACTUALIZACIÓN Y CANCELACIÓN
// ==========================================

func (ctrl *EventController) HandleActualizarEstado(c *gin.Context) {
	rolNombre, exists := c.Get("role_nombre")
	if !exists || rolNombre.(string) != models.RolAprobador {
		slog.Warn("Acceso denegado: Intento de actualizar estado sin rol aprobador", "rol", rolNombre)
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
	
	// Necesitamos obtener el evento antes de actualizarlo para saber quién es el organizador
	eventoOriginal, err := models.GetEventoByID(ctx, eventoID)
	if err != nil {
		manejarErrorAPI(c, http.StatusNotFound, "Evento no encontrado")
		return
	}

	err = models.ActualizarEstadoEvento(ctx, eventoID, input.Estado, &aprobadorID, input.Observaciones)

	if err != nil {
		if errors.Is(err, models.ErrEstadoInvalido) || errors.Is(err, models.ErrObservacionesRequeridas) || errors.Is(err, models.ErrTransicionEstadoInvalida) {
			slog.Warn("Violación de reglas de negocio en transición", "evento_id", eventoID, "error", err)
			responderDual(c, http.StatusBadRequest, "events/detail.html",
				gin.H{"error": err.Error()},
				gin.H{"error": err.Error()})
			return
		}
		slog.Error("Error interno al actualizar estado", "evento_id", eventoID, "error", err)
		responderDual(c, http.StatusInternalServerError, "events/detail.html",
			gin.H{"error": "Error interno al actualizar estado"},
			gin.H{"error": "Error interno al actualizar estado"})
		return
	}

	slog.Info("Estado de evento actualizado", "evento_id", eventoID, "nuevo_estado", input.Estado, "aprobador_id", aprobadorID)

	// NUEVO: Lógica de Notificaciones Push vía Firebase
	if ctrl.notificationService != nil {
		fcmToken, errToken := models.GetFCMToken(ctx, eventoOriginal.OrganizadorID)
		if errToken == nil && fcmToken != "" {
			// Lanzamos el envío de la notificación en una goroutine para no bloquear la respuesta HTTP
			go func(token, tituloEvento, nuevoEstado string, idEvento int64) {
				tituloPush := "Actualización de tu evento"
				cuerpoPush := fmt.Sprintf("Tu evento '%s' ha cambiado al estado: %s", tituloEvento, strings.ToUpper(nuevoEstado))
				
				extraData := map[string]string{
					"evento_id": fmt.Sprintf("%d", idEvento),
					"tipo":      "cambio_estado",
				}
				
				// Usamos context.Background() porque la petición original ya respondió
				_ = ctrl.notificationService.SendDirectNotification(context.Background(), token, tituloPush, cuerpoPush, extraData)
			}(fcmToken, eventoOriginal.Titulo, input.Estado, eventoID)
		} else {
			slog.Info("No se envió notificación push: Organizador sin fcm_token registrado", "organizador_id", eventoOriginal.OrganizadorID)
		}
	}

	responderDual(c, http.StatusOK, "events/detail.html",
		gin.H{"mensaje": "Estado actualizado a: " + input.Estado},
		gin.H{"mensaje": "Estado actualizado a: " + input.Estado})
}

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

	if evento.Estado == models.EstadoCancelado {
		manejarErrorAPI(c, http.StatusBadRequest, "El evento ya está cancelado")
		return
	}

	rolNombre, _ := c.Get("role_nombre")
	esAprobador := rolNombre != nil && rolNombre.(string) == models.RolAprobador
	esOrganizador := evento.OrganizadorID == userID

	if !esAprobador && !esOrganizador {
		slog.Warn("Acceso denegado para cancelación de evento", "evento_id", eventoID, "user_id", userID)
		manejarErrorAPI(c, http.StatusForbidden, "Solo el organizador o la Coordinación de Cultura pueden cancelar este evento")
		return
	}

	var aprobadorID *int64
	if esAprobador {
		aprobadorID = &userID
	}

	err = models.ActualizarEstadoEvento(ctx, eventoID, models.EstadoCancelado, aprobadorID, observaciones)
	if err != nil {
		if errors.Is(err, models.ErrTransicionEstadoInvalida) || errors.Is(err, models.ErrObservacionesRequeridas) {
			slog.Warn("Transición inválida al cancelar", "evento_id", eventoID, "error", err)
			manejarErrorAPI(c, http.StatusBadRequest, err.Error())
			return
		}
		slog.Error("Error interno al cancelar", "evento_id", eventoID, "error", err)
		manejarErrorAPI(c, http.StatusInternalServerError, "Error al cancelar el evento: "+err.Error())
		return
	}

	slog.Info("Evento cancelado", "evento_id", eventoID, "por_user_id", userID)
	if strings.Contains(c.GetHeader("Accept"), "application/json") {
		c.JSON(http.StatusOK, gin.H{"mensaje": "Evento cancelado exitosamente"})
		return
	}
	c.Redirect(http.StatusSeeOther, "/eventos/")
}

func (ctrl *EventController) HandleDeleteEvent(c *gin.Context) {
	eventoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		manejarErrorAPI(c, http.StatusBadRequest, "ID de evento inválido")
		return
	}
	ctx := c.Request.Context()
	evento, err := models.GetEventoByID(ctx, eventoID)
	if err != nil {
		manejarErrorAPI(c, http.StatusNotFound, "Evento no encontrado")
		return
	}

	userID, _ := extractUserID(c)
	roleIDVal, _ := c.Get("roleID")
	var roleID int
	if r, ok := roleIDVal.(int); ok {
		roleID = r
	} else if r64, ok := roleIDVal.(int64); ok {
		roleID = int(r64)
	} else if rf, ok := roleIDVal.(float64); ok {
		roleID = int(rf)
	}

	esCreador := userID > 0 && evento.OrganizadorID == userID
	esAdminOrApprover := roleID == 1 || roleID == 2

	if !esCreador && !esAdminOrApprover {
		slog.Warn("Acceso denegado para eliminación de evento", "evento_id", eventoID, "user_id", userID)
		responderDual(c, http.StatusForbidden, "events/detail.html",
			gin.H{"error": "No tienes permisos para eliminar este evento"},
			gin.H{"error": "No tienes permisos para eliminar este evento"})
		return
	}

	err = models.DeleteEvento(ctx, eventoID)
	if err != nil {
		slog.Error("Error al eliminar evento", "evento_id", eventoID, "error", err)
		responderDual(c, http.StatusInternalServerError, "events/detail.html",
			gin.H{"error": "Error al eliminar evento: " + err.Error()},
			gin.H{"error": "Error al eliminar evento: " + err.Error()})
		return
	}

	slog.Info("Evento eliminado permanentemente", "evento_id", eventoID, "por_user_id", userID)
	if strings.Contains(c.GetHeader("Accept"), "application/json") {
		c.JSON(http.StatusOK, gin.H{"mensaje": "Evento eliminado exitosamente"})
		return
	}
	c.Redirect(http.StatusSeeOther, "/")
}

// ==========================================
// SERVICIOS EXTERNOS (Gemini + Rate Limiting)
// ==========================================

func (ctrl *EventController) SuggestDescription(c *gin.Context) {
	if ctrl.geminiService == nil {
		manejarErrorAPI(c, http.StatusInternalServerError, "El servicio de IA no está configurado")
		return
	}

	ip := c.ClientIP()
	
	ctrl.mu.RLock()
	v, exists := ctrl.rateLimiters[ip]
	ctrl.mu.RUnlock()

	if !exists {
		ctrl.mu.Lock()
		v, exists = ctrl.rateLimiters[ip]
		if !exists {
			v = &visitorLimiter{
				limiter:  rate.NewLimiter(rate.Every(time.Minute/10), 10),
				lastSeen: time.Now(),
			}
			ctrl.rateLimiters[ip] = v
		}
		ctrl.mu.Unlock()
	}

	ctrl.mu.Lock()
	v.lastSeen = time.Now()
	limiter := v.limiter
	ctrl.mu.Unlock()

	if !limiter.Allow() {
		slog.Warn("Rate limit excedido para Gemini", "ip", ip)
		manejarErrorAPI(c, http.StatusTooManyRequests, "Demasiadas peticiones a la IA. Intente de nuevo en un minuto.")
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
		slog.Error("Fallo en generación de IA", "error", err, "ip", ip)
		manejarErrorAPI(c, http.StatusInternalServerError, "Error de generación con Gemini: "+err.Error())
		return
	}

	slog.Info("Descripción generada por IA exitosamente", "ip", ip)
	c.JSON(http.StatusOK, gin.H{"descripcion": suggestion})
}
