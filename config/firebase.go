package config

import (
	"context"
	"fmt"
	"log/slog"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// FCMClient será nuestra variable global para acceder a los envíos push desde cualquier servicio
var FCMClient *messaging.Client

// InitFirebase conecta el backend con el proyecto de la nube
func InitFirebase() error {
	// Asegúrate de que este string sea exactamente el nombre que le pusiste a tu archivo descargado
	opt := option.WithCredentialsFile("firebase-credentials.json") 
	
	// Inicializamos la app de Firebase con esa llave
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		return fmt.Errorf("error inicializando Firebase App: %w", err)
	}

	// Extraemos específicamente el cliente de notificaciones (Cloud Messaging)
	client, err := app.Messaging(context.Background())
	if err != nil {
		return fmt.Errorf("error obteniendo el cliente de mensajería (FCM): %w", err)
	}

	FCMClient = client
	slog.Info("Firebase Cloud Messaging inicializado correctamente")
	return nil
}
