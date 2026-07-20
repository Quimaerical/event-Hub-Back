package services

import (
	"context"
	"fmt"
	"log/slog"

	"firebase.google.com/go/v4/messaging"
)

// NotificationService encapsula la lógica para enviar alertas push a través de Firebase Cloud Messaging (FCM)
type NotificationService struct {
	client *messaging.Client
}

// NewNotificationService inyecta el cliente de mensajería de Firebase
func NewNotificationService(client *messaging.Client) *NotificationService {
	return &NotificationService{
		client: client,
	}
}

// SendDirectNotification envía una notificación push a un dispositivo específico usando su token FCM.
// Ideal para avisar a un organizador que su evento fue aprobado o rechazado.
func (s *NotificationService) SendDirectNotification(ctx context.Context, fcmToken, titulo, cuerpo string, extraData map[string]string) error {
	if fcmToken == "" {
		return fmt.Errorf("token FCM vacío, abortando envío de notificación")
	}

	if s.client == nil {
		slog.Warn("Firebase Cloud Messaging no está inicializado. Omitiendo envío de notificación directa.")
		return nil
	}

	// Construimos el mensaje con el formato que Flutter espera
	message := &messaging.Message{
		Token: fcmToken,
		Notification: &messaging.Notification{
			Title: titulo,
			Body:  cuerpo,
		},
		Data: extraData, // Payload oculto para navegación o lógica interna en Flutter
	}

	responseID, err := s.client.Send(ctx, message)
	if err != nil {
		slog.Error("Fallo al enviar notificación push directa", "error", err, "fcm_token", fcmToken)
		return fmt.Errorf("error enviando push a FCM: %w", err)
	}

	slog.Info("Notificación push directa enviada exitosamente", "message_id", responseID)
	return nil
}

// BroadcastToTopic envía una notificación a todos los usuarios suscritos a un "tema".
// Ideal para enviar alertas generales a grupos (ej: todos los estudiantes inscritos en una categoría).
func (s *NotificationService) BroadcastToTopic(ctx context.Context, topic, titulo, cuerpo string, extraData map[string]string) error {
	if topic == "" {
		return fmt.Errorf("el tema (topic) no puede estar vacío")
	}

	if s.client == nil {
		slog.Warn("Firebase Cloud Messaging no está inicializado. Omitiendo broadcast.")
		return nil
	}

	message := &messaging.Message{
		Topic: topic,
		Notification: &messaging.Notification{
			Title: titulo,
			Body:  cuerpo,
		},
		Data: extraData,
	}

	responseID, err := s.client.Send(ctx, message)
	if err != nil {
		slog.Error("Fallo al enviar notificación al topic", "error", err, "topic", topic)
		return fmt.Errorf("error enviando broadcast a FCM: %w", err)
	}

	slog.Info("Notificación broadcast enviada exitosamente", "message_id", responseID, "topic", topic)
	return nil
}
