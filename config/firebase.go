package config

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// FCMClient será nuestra variable global para acceder a los envíos push desde cualquier servicio
var FCMClient *messaging.Client

// InitFirebase conecta el backend con el proyecto de la nube
func InitFirebase() error {
	// Verificar si el archivo de credenciales existe
	if _, err := os.Stat("firebase-credentials.json"); os.IsNotExist(err) {
		slog.Warn("Archivo firebase-credentials.json no encontrado. Firebase Cloud Messaging no se inicializará en este entorno.")
		return nil
	}

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
