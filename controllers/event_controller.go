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

	userID, _ := c.Get("userID")
	email, _ := c.Get("email")

	responderDual(c, http.StatusOK, "events/create.html",
		gin.H{"categorias": categories, "userID": userID, "email": email},
		gin.H{"categorias": categories, "userID": userID, "email": email},
	)
}

func (ctrl *EventController) HandleCreate(c *gin.Context) {
	var input struct {
		models.Evento
		Categorias []int `form:"categorias" json:"categorias" binding:"required,min=1"`
	}

	if err := c.ShouldBind(&input); err != nil {
		slog.Warn("Intento de creación con datos inválidos", "error", err)
		responderDual(c, http.StatusBadRequest, "events/create.html",
			gin.H{"error": "Datos inválidos: " + err.Error()},
			gin.H{"error": "Datos inválidos: " + err.Error()},
		)
		return
	}

	if !input.Evento.FechaFin.After(input.Evento.FechaInicio) {
		slog.Warn("Intento de creación con fechas incoherentes", "fecha_inicio", input.Evento.FechaInicio, "fecha_fin", input.Evento.FechaFin)
		responderDual(c, http.StatusBadRequest, "events/create.html",
			gin.H{"error": "La fecha de fin debe ser posterior a la fecha de inicio"},
			gin.H{"error": "La fecha de fin debe ser posterior a la fecha de inicio"},
		)
		return
	}

	userID, err := extractUserID(c)
	if err != nil {
		slog.Warn("Fallo de autenticación", "error", err)
		manejarErrorAPI(c, http.StatusUnauthorized, err.Error())
		return
	}
	input.Evento.OrganizadorID = userID

	ctx := c.Request.Context()
	err = models.CreateEvento(ctx, &input.Evento, input.Categorias)

	if err != nil {
		if errors.Is(err, models.ErrEspacioOcupado) {
			slog.Warn("Conflicto de espacio ocupado", "user_id", userID, "espacio_id", input.EspacioID)
			responderDual(c, http.StatusConflict, "events/create.html", gin.H{"error": err.Error()}, gin.H{"error": err.Error()})
			return
		}
		slog.Error("Error interno al crear evento", "error", err, "user_id", userID)
		responderDual(c, http.StatusInternalServerError, "events/create.html", gin.H{"error": "Error interno al guardar"}, gin.H{"error": "Error interno"})
		return
	}

	slog.Info("Evento creado localmente", "evento_id", input.Evento.ID, "user_id", userID)

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
		}(input.Evento, tokenOAuth)
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

	responderDual(c, http.StatusOK, "events/detail.html", gin.H{"evento": evento}, gin.H{"evento": evento})
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
