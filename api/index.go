package handler

import (
	"context"
	"html/template"
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

// getViewsDir localiza dinámicamente la carpeta views en entornos locales y Serverless
func getViewsDir() string {
	candidates := []string{
		"views",
		"../views",
		"./views",
		"../../views",
	}

	for _, cand := range candidates {
		if fi, err := os.Stat(cand); err == nil && fi.IsDir() {
			if absPath, errAbs := filepath.Abs(cand); errAbs == nil {
				return absPath
			}
			return cand
		}
	}

	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, sub := range []string{"views", "../views"} {
			target := filepath.Join(dir, sub)
			if fi, err := os.Stat(target); err == nil && fi.IsDir() {
				return target
			}
		}
	}

	return "views"
}

func loadTemplates(r *gin.Engine) {
	viewsDir := getViewsDir()
	log.Printf("Cargando plantillas HTML desde la ruta absoluta: %s", viewsDir)

	renderer := &CustomHTMLRenderer{
		templates: make(map[string]*template.Template),
	}

	var layoutsAndPartials []string

	_ = filepath.Walk(viewsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, ".html") {
			cleanPath := filepath.ToSlash(path)
			if strings.Contains(cleanPath, "/layouts/") || strings.Contains(cleanPath, "/partials/") {
				layoutsAndPartials = append(layoutsAndPartials, path)
			}
		}
		return nil
	})

	_ = filepath.Walk(viewsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, ".html") {
			cleanPath := filepath.ToSlash(path)
			isLayoutOrPartial := strings.Contains(cleanPath, "/layouts/") || strings.Contains(cleanPath, "/partials/")

			if !isLayoutOrPartial {
				relPath, errRel := filepath.Rel(viewsDir, path)
				if errRel != nil {
					return nil
				}
				relPath = filepath.ToSlash(relPath)

				tmpl := template.New(relPath)
				content, errRead := os.ReadFile(path)
				if errRead != nil {
					log.Printf("Error leyendo plantilla %s: %v", path, errRead)
					return nil
				}

				var errParse error
				tmpl, errParse = tmpl.Parse(string(content))
				if errParse != nil {
					log.Printf("Error parseando plantilla %s: %v", path, errParse)
					return nil
				}

				for _, lp := range layoutsAndPartials {
					lpContent, errLP := os.ReadFile(lp)
					if errLP == nil {
						tmpl, errParse = tmpl.Parse(string(lpContent))
						if errParse != nil {
							log.Printf("Error asociando partial/layout %s a %s: %v", lp, path, errParse)
						}
					}
				}

				renderer.templates[relPath] = tmpl
			}
		}
		return nil
	})

	log.Printf("Plantillas HTML cargadas exitosamente. Total vistas: %d", len(renderer.templates))
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
			"views_dir":        getViewsDir(),
			"has_database_url": os.Getenv("DATABASE_URL") != "",
			"has_postgres_url": os.Getenv("POSTGRES_URL") != "",
			"vercel_env":       os.Getenv("VERCEL_ENV"),
		})
	})

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
		protected.DELETE("/:id", eventCtrl.HandleCancelEvent)
	}

	app = r
}

// Handler es el entrypoint exportado para Vercel Serverless Functions
func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(initApp)
	app.ServeHTTP(w, r)
}
