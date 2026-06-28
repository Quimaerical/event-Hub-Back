package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"event-hub/config"
	"event-hub/controllers"
	"event-hub/middlewares"
	"event-hub/services"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// 1. Load environment variables.
	// We ignore error because in cloud environments like Render/Neon, env variables are injected directly.
	if err := godotenv.Load(); err != nil {
		log.Println("Aviso: No se encontró archivo .env, leyendo del sistema")
	}

	// 2. Connect to Database connection pool
	config.ConnectDB()
	defer config.CloseDB()

	// 3. Initialize Services
	geminiSvc := services.NewGeminiService()
	oauthSvc := services.NewOAuthService()

	// 4. Initialize Controllers
	dashboardCtrl := controllers.NewDashboardController()
	authCtrl := controllers.NewAuthController(oauthSvc)
	eventCtrl := controllers.NewEventController(geminiSvc)

	// 5. Configure Gin Router
	router := gin.Default()

	// 6. Load HTML Templates recursively
	loadTemplates(router)

	// 7. Register Public Routes
	router.GET("/", middlewares.CurrentUser(), dashboardCtrl.ShowDashboard)
	router.GET("/login", authCtrl.ShowLogin)
	router.POST("/login", authCtrl.HandleLogin)
	router.GET("/register", authCtrl.ShowRegister)
	router.POST("/register", authCtrl.HandleRegister)
	router.GET("/auth/logout", authCtrl.Logout)

	// OAuth routes
	router.GET("/auth/google", authCtrl.GoogleLogin)
	router.GET("/auth/google/callback", authCtrl.GoogleCallback)
	router.GET("/auth/github", authCtrl.GitHubLogin)
	router.GET("/auth/github/callback", authCtrl.GitHubCallback)

	// 8. Register Protected Routes
	protected := router.Group("/eventos")
	protected.Use(middlewares.AuthRequired())
	{
		// Rutas estáticas (Deben ir antes de las rutas dinámicas como /:id para evitar conflictos en Gin)
		protected.GET("/crear", eventCtrl.ShowCreate)
		protected.POST("/crear", eventCtrl.HandleCreate)
		protected.POST("/sugerir-descripcion", eventCtrl.SuggestDescription)

		// Rutas de colección y detalle
		protected.GET("/", eventCtrl.HandleListEvents)
		protected.GET("/:id", eventCtrl.HandleGetEvent)

		// Rutas de actualización y borrado
		protected.PATCH("/:id/estado", eventCtrl.HandleActualizarEstado)
		protected.DELETE("/:id", eventCtrl.HandleCancelEvent)
	}

	// 9. Start Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Servidor de Event Hub iniciado en el puerto %s...", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Error al iniciar el servidor HTTP: %v", err)
	}
}

// loadTemplates finds all template files nested inside the views folder and registers them.
func loadTemplates(r *gin.Engine) {
	var files []string
	err := filepath.Walk("views", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".html") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Error al leer directorio de vistas: %v", err)
	}

	r.LoadHTMLFiles(files...)
}
