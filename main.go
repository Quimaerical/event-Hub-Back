package main

import (
	"html/template"
	"log"
	"log/slog" // FIX: Importación necesaria para el manejo estructurado de errores de Firebase
	"os"
	"path/filepath"
	"strings"

	"event-hub/config"
	"event-hub/controllers"
	"event-hub/middlewares"
	"event-hub/services"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
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

	// NUEVO: Conectar a Firebase
	if err := config.InitFirebase(); err != nil {
		slog.Error("Fallo crítico al conectar con Firebase", "error", err)
		// Puedes usar log.Fatal(err) si quieres que el servidor no arranque sin Firebase
	}

	// 3. Initialize Services
	geminiSvc := services.NewGeminiService()
	oauthSvc := services.NewOAuthService()
	calendarSvc := services.NewCalendarService()
	notificationSvc := services.NewNotificationService(config.FCMClient)

	// NUEVO: Instanciar y arrancar el motor de recordatorios
	cronSvc := services.NewCronService(notificationSvc)
	cronSvc.Start()

	// 4. Initialize Controllers
	dashboardCtrl := controllers.NewDashboardController()
	authCtrl := controllers.NewAuthController(oauthSvc)
	// FIX: Inyectamos notificationSvc al controlador de eventos
	eventCtrl := controllers.NewEventController(geminiSvc, calendarSvc, notificationSvc)

	// Opcional: Instanciamos el controlador de inscripciones (creado en la rama anterior)
	// inscripcionCtrl := controllers.NewInscripcionController(calendarSvc)

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
	
	// Crear un grupo de rutas de perfil/auth que requieren estar logueado
	perfilProtegido := router.Group("/perfil")
	perfilProtegido.Use(middlewares.AuthRequired())
	{
		perfilProtegido.POST("/fcm-token", authCtrl.HandleUpdateFCMToken)
	}

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

		// Opcional: Aquí habilitarías la ruta de inscripción
		// protected.POST("/:id/inscribir", inscripcionCtrl.HandleInscribir)
	}

	// Opcional: Ruta para cancelar inscripciones
	// router.DELETE("/inscripciones/:id", middlewares.AuthRequired(), inscripcionCtrl.HandleCancelarInscripcion)

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

// CustomHTMLRenderer implements Gin's render.HTMLRender interface
// to allow standard layout inheritance without template collisions.
type CustomHTMLRenderer struct {
	templates map[string]*template.Template
}

func (r *CustomHTMLRenderer) Instance(name string, data any) render.Render {
	return render.HTML{
		Template: r.templates[name],
		Name:     name,
		Data:     data,
	}
}

// loadTemplates finds all template files nested inside the views folder
// and registers them as independent template groups to prevent namespace collision.
func loadTemplates(r *gin.Engine) {
	renderer := &CustomHTMLRenderer{
		templates: make(map[string]*template.Template),
	}

	var layoutsAndPartials []string
	
	// 1. Gather layouts and partials
	err := filepath.Walk("views", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".html") {
			if strings.Contains(path, "/layouts/") || strings.Contains(path, "/partials/") {
				layoutsAndPartials = append(layoutsAndPartials, path)
			}
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Error al leer layouts/partials: %v", err)
	}

	// 2. Gather page templates and construct individual groups
	err = filepath.Walk("views", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".html") {
			if !strings.Contains(path, "/layouts/") && !strings.Contains(path, "/partials/") {
				relPath, errRel := filepath.Rel("views", path)
				if errRel != nil {
					log.Fatalf("Error obteniendo ruta relativa: %v", errRel)
				}
				relPath = filepath.ToSlash(relPath)

				// Create template named exactly relPath
				tmpl := template.New(relPath)

				content, errRead := os.ReadFile(path)
				if errRead != nil {
					log.Fatalf("Error leyendo template %s: %v", path, errRead)
				}

				tmpl, errParse := tmpl.Parse(string(content))
				if errParse != nil {
					log.Fatalf("Error parseando template %s: %v", path, errParse)
				}

				if len(layoutsAndPartials) > 0 {
					tmpl, errParse = tmpl.ParseFiles(layoutsAndPartials...)
					if errParse != nil {
						log.Fatalf("Error asociando layouts/partials a %s: %v", path, errParse)
					}
				}

				renderer.templates[relPath] = tmpl
			}
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Error al leer vistas: %v", err)
	}

	r.HTMLRender = renderer
}
