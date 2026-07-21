package handler

import (
	"context"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"event-hub/config"
	"event-hub/controllers"
	"event-hub/middlewares"
	"event-hub/services"
	"event-hub/views"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/joho/godotenv"
)

var (
	app  *gin.Engine
	once sync.Once
)

type CustomHTMLRenderer struct {
	templates map[string]*template.Template
}

func (r *CustomHTMLRenderer) Instance(name string, data any) render.Render {
	tmpl, exists := r.templates[name]
	if !exists || tmpl == nil {
		log.Printf("ERROR CRÍTICO: Plantilla HTML '%s' no encontrada en el renderizador", name)
		fallback := template.Must(template.New("fallback").Parse(`<!DOCTYPE html><html><head><title>Error de Sistema</title></head><body style="font-family:sans-serif;padding:2rem;background:#020617;color:#f8fafc;"><h1>Event Hub - Error de Vista</h1><p>La plantilla solicitada (<strong>{{.}}</strong>) no se pudo cargar.</p></body></html>`))
		return render.HTML{
			Template: fallback,
			Name:     "fallback",
			Data:     name,
		}
	}
	return render.HTML{
		Template: tmpl,
		Name:     name,
		Data:     data,
	}
}

func loadTemplates(r *gin.Engine) {
	renderer := &CustomHTMLRenderer{
		templates: make(map[string]*template.Template),
	}

	var layoutsAndPartials []string

	// 1. Recopilar layouts y partials desde el sistema de archivos embebido (views.FS)
	_ = fs.WalkDir(views.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		cleanPath := filepath.ToSlash(path)
		if !d.IsDir() && strings.HasSuffix(cleanPath, ".html") {
			if strings.Contains(cleanPath, "layouts/") || strings.Contains(cleanPath, "partials/") {
				layoutsAndPartials = append(layoutsAndPartials, cleanPath)
			}
		}
		return nil
	})

	// 2. Cargar páginas principales asociando layouts y partials
	_ = fs.WalkDir(views.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		cleanPath := filepath.ToSlash(path)
		if !d.IsDir() && strings.HasSuffix(cleanPath, ".html") {
			isLayoutOrPartial := strings.Contains(cleanPath, "layouts/") || strings.Contains(cleanPath, "partials/")

			if !isLayoutOrPartial {
				tmpl := template.New(cleanPath)
				content, errRead := views.FS.ReadFile(cleanPath)
				if errRead != nil {
					log.Printf("Error leyendo plantilla embebida %s: %v", cleanPath, errRead)
					return nil
				}

				var errParse error
				tmpl, errParse = tmpl.Parse(string(content))
				if errParse != nil {
					log.Printf("Error parseando plantilla embebida %s: %v", cleanPath, errParse)
					return nil
				}

				for _, lp := range layoutsAndPartials {
					lpContent, errLP := views.FS.ReadFile(lp)
					if errLP == nil {
						tmpl, errParse = tmpl.Parse(string(lpContent))
						if errParse != nil {
							log.Printf("Error asociando partial/layout embebido %s a %s: %v", lp, cleanPath, errParse)
						}
					}
				}

				renderer.templates[cleanPath] = tmpl
			}
		}
		return nil
	})

	log.Printf("Plantillas HTML embebidas cargadas exitosamente. Total vistas: %d", len(renderer.templates))
	r.HTMLRender = renderer
}

func initApp() {
	if os.Getenv("VERCEL") == "" {
		_ = godotenv.Load()
	}
	gin.SetMode(gin.ReleaseMode)

	// Conectar a BD
	config.ConnectDB()

	_ = config.InitFirebase()

	geminiSvc := services.NewGeminiService()
	oauthSvc := services.NewOAuthService()
	calendarSvc := services.NewCalendarService()
	notificationSvc := services.NewNotificationService(config.FCMClient)

	dashboardCtrl := controllers.NewDashboardController()
	authCtrl := controllers.NewAuthController(oauthSvc)
	eventCtrl := controllers.NewEventController(geminiSvc, calendarSvc, notificationSvc)
	profileCtrl := controllers.NewProfileController()
	tablonCtrl := controllers.NewTablonController()

	r := gin.New()
	r.Use(gin.Recovery())

	loadTemplates(r)

	// Endpoint de Diagnóstico y Salud para Serverless
	r.GET("/health", func(c *gin.Context) {
		dbStatus := "disconnected"
		dbErrStr := ""
		if config.DB != nil {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
			defer cancel()
			if err := config.DB.Ping(ctx); err == nil {
				dbStatus = "connected"
			} else {
				dbStatus = "ping_failed"
				dbErrStr = err.Error()
			}
		}

		templatesCount := 0
		if renderer, ok := r.HTMLRender.(*CustomHTMLRenderer); ok && renderer != nil {
			templatesCount = len(renderer.templates)
		}

		c.JSON(http.StatusOK, gin.H{
			"status":           "ok",
			"database_status":  dbStatus,
			"database_error":   dbErrStr,
			"templates_loaded": templatesCount,
			"embedded_views":   true,
			"has_database_url": os.Getenv("DATABASE_URL") != "",
			"has_postgres_url": os.Getenv("POSTGRES_URL") != "",
			"vercel_env":       os.Getenv("VERCEL_ENV"),
		})
	})

	// Endpoint seguro de Migración Web en Vercel
	r.GET("/api/migrate", func(c *gin.Context) {
		secretParam := c.Query("secret")
		expectedSecret := os.Getenv("MIGRATE_SECRET")
		if expectedSecret == "" {
			expectedSecret = "saga_migrate_2026"
		}

		if secretParam != expectedSecret {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Clave secreta de migración inválida o ausente",
			})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
		defer cancel()

		if err := config.ForceMigrate(ctx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "error",
				"error":  "Fallo al ejecutar la migración: " + err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "Base de datos Neon PostgreSQL migrada y poblada correctamente con todas sus tablas y datos de prueba.",
		})
	})

	_ = os.MkdirAll("uploads", 0755)
	r.Static("/uploads", "./uploads")

	r.GET("/", middlewares.CurrentUser(), dashboardCtrl.ShowDashboard)
	r.GET("/eventos/:id", middlewares.CurrentUser(), eventCtrl.HandleGetEvent)
	r.GET("/login", authCtrl.ShowLogin)
	r.POST("/login", authCtrl.HandleLogin)
	r.GET("/register", authCtrl.ShowRegister)
	r.POST("/register", authCtrl.HandleRegister)
	r.GET("/auth/logout", authCtrl.Logout)

	r.GET("/auth/google", authCtrl.GoogleLogin)
	r.GET("/auth/google/callback", authCtrl.GoogleCallback)
	r.GET("/auth/github", authCtrl.GitHubLogin)
	r.GET("/auth/github/callback", authCtrl.GitHubCallback)

	r.GET("/tablon", middlewares.AuthRequired(), tablonCtrl.ShowTablon)

	perfilProtegido := r.Group("/perfil")
	perfilProtegido.Use(middlewares.AuthRequired())
	{
		perfilProtegido.GET("", profileCtrl.ShowProfile)
		perfilProtegido.POST("", profileCtrl.UpdateProfile)
		perfilProtegido.POST("/password", profileCtrl.UpdatePassword)
		perfilProtegido.POST("/eliminar", profileCtrl.DeleteAccount)
		perfilProtegido.POST("/fcm-token", authCtrl.HandleUpdateFCMToken)
	}

	protected := r.Group("/eventos")
	protected.Use(middlewares.AuthRequired())
	{
		protected.GET("/crear", eventCtrl.ShowCreate)
		protected.POST("/crear", eventCtrl.HandleCreate)
		protected.POST("/sugerir-descripcion", eventCtrl.SuggestDescription)
		protected.GET("/", eventCtrl.HandleListEvents)
		protected.POST("/:id/inscribir", eventCtrl.HandleInscribirEvent)
		protected.POST("/:id/cancelar-inscripcion", eventCtrl.HandleCancelarInscripcion)
		protected.GET("/:id/editar", eventCtrl.ShowEdit)
		protected.POST("/:id/editar", eventCtrl.HandleEdit)
		protected.POST("/:id/aprobar", eventCtrl.HandleAprobarEvent)
		protected.POST("/:id/rechazar", eventCtrl.HandleRechazarEvent)
		protected.PATCH("/:id/estado", eventCtrl.HandleActualizarEstado)
		protected.POST("/:id/eliminar", eventCtrl.HandleDeleteEvent)
		protected.DELETE("/:id", eventCtrl.HandleDeleteEvent)
	}

	adminCtrl := controllers.NewAdminController()
	adminGroup := r.Group("/admin")
	adminGroup.Use(middlewares.AuthRequired(), middlewares.AdminRequired())
	{
		adminGroup.GET("/usuarios", adminCtrl.ShowUsers)
		adminGroup.POST("/usuarios/:id/role", adminCtrl.UpdateUserRole)
		adminGroup.POST("/usuarios/:id/eliminar", adminCtrl.DeleteUser)
	}

	app = r
}

// Handler es el entrypoint exportado para Vercel Serverless Functions
func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(initApp)
	app.ServeHTTP(w, r)
}
