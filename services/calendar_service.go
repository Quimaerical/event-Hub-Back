package services

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"event-hub/models"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type CalendarService struct {
	// NUEVO: Configuración OAuth inyectada para el cliente
	oauthConfig *oauth2.Config

	// NUEVO: Control de concurrencia por evento para AddAttendee
	eventMutexes map[string]*sync.Mutex
	mu           sync.RWMutex
}

func NewCalendarService() *CalendarService {
	redirectURL := os.Getenv("GOOGLE_REDIRECT_URL")
	if redirectURL == "" {
		redirectURL = "http://localhost:8080/auth/google/callback"
	}

	config := &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  redirectURL,
		Scopes:       []string{calendar.CalendarScope},
		Endpoint:     google.Endpoint,
	}

	return &CalendarService{
		oauthConfig:  config,
		eventMutexes: make(map[string]*sync.Mutex),
	}
}

// NUEVO: getEventMutex recupera o crea un mutex específico para un evento de calendario (Serialización)
func (s *CalendarService) getEventMutex(calendarEventID string) *sync.Mutex {
	s.mu.RLock()
	mu, exists := s.eventMutexes[calendarEventID]
	s.mu.RUnlock()

	if !exists {
		s.mu.Lock()
		// Doble comprobación
		mu, exists = s.eventMutexes[calendarEventID]
		if !exists {
			mu = &sync.Mutex{}
			s.eventMutexes[calendarEventID] = mu
		}
		s.mu.Unlock()
	}
	return mu
}

// getClient inicializa el cliente de Google Calendar usando el token OAuth del usuario
func (s *CalendarService) getClient(ctx context.Context, token *oauth2.Token) (*calendar.Service, error) {
	// FIX: Utilizar la configuración instanciada para generar el cliente HTTP, permitiendo el refresh automático
	client := s.oauthConfig.Client(ctx, token)
	return calendar.NewService(ctx, option.WithHTTPClient(client))
}

// CreateCalendarEvent crea un evento en el calendario principal del organizador
func (s *CalendarService) CreateCalendarEvent(ctx context.Context, token *oauth2.Token, evento *models.Evento) (string, error) {
	srv, err := s.getClient(ctx, token)
	if err != nil {
		return "", fmt.Errorf("error al inicializar cliente de calendario: %w", err)
	}

	event := &calendar.Event{
		Summary:     evento.Titulo,
		Location:    evento.EspacioNombre,
		Description: evento.Descripcion.String,
		Start: &calendar.EventDateTime{
			DateTime: evento.FechaInicio.Format(time.RFC3339),
			TimeZone: "America/Caracas",
		},
		End: &calendar.EventDateTime{
			DateTime: evento.FechaFin.Format(time.RFC3339),
			TimeZone: "America/Caracas",
		},
		Reminders: &calendar.EventReminders{
			UseDefault:      false,
			Overrides: []*calendar.EventReminder{
				{Method: "email", Minutes: 24 * 60}, // 24h antes
				{Method: "popup", Minutes: 60},      // 1h antes
			},
		},
	}

	calendarId := "primary"
	
	// FIX: Añadido SendUpdates("all") para forzar las notificaciones de Google Calendar
	createdEvent, err := srv.Events.Insert(calendarId, event).SendUpdates("all").Do()
	if err != nil {
		return "", fmt.Errorf("error llamando a Google Calendar API: %w", err)
	}

	slog.Info("Evento creado en Google Calendar", "calendar_event_id", createdEvent.Id)
	return createdEvent.Id, nil
}

// AddAttendee agrega un asistente al evento existente mediante actualización completa atómica
func (s *CalendarService) AddAttendee(ctx context.Context, token *oauth2.Token, calendarEventID string, email string) error {
	srv, err := s.getClient(ctx, token)
	if err != nil {
		return fmt.Errorf("error al inicializar cliente de calendario: %w", err)
	}

	// FIX: Bloquear el recurso por ID para evitar "Lost Updates" (Race Condition) en concurrencia masiva
	mu := s.getEventMutex(calendarEventID)
	mu.Lock()
	defer mu.Unlock()

	// Obtenemos el evento completo para actualizar los asistentes
	event, err := srv.Events.Get("primary", calendarEventID).Do()
	if err != nil {
		return fmt.Errorf("error obteniendo evento de calendario: %w", err)
	}

	// Añadir el nuevo asistente
	event.Attendees = append(event.Attendees, &calendar.EventAttendee{
		Email: email,
	})

	// FIX: Añadido SendUpdates("all") para notificar al nuevo inscrito
	_, err = srv.Events.Update("primary", calendarEventID, event).SendUpdates("all").Do()
	if err != nil {
		return fmt.Errorf("error actualizando asistentes en Google Calendar API: %w", err)
	}

	slog.Info("Asistente añadido a Google Calendar", "calendar_event_id", calendarEventID, "email", email)
	return nil
}
