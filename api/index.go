package handler

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

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
	return render.HTML{
		Template: r.templates[name],
		Name:     name,
		Data:     data,
	}
}

func loadTemplates(r *gin.Engine) {
	renderer := &CustomHTMLRenderer{
		templates: make(map[string]*template.Template),
	}

	var layoutsAndPartials []string

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
		log.Printf("Aviso: Error al leer layouts/partials: %v", err)
	}

	err = filepath.Walk("views", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".html") {
			if !strings.Contains(path, "/layouts/") && !strings.Contains(path, "/partials/") {
				relPath, errRel := filepath.Rel("views", path)
				if errRel != nil {
					return errRel
				}
				relPath = filepath.ToSlash(relPath)

				tmpl := template.New(relPath)
				content, errRead := os.ReadFile(path)
				if errRead != nil {
					return errRead
				}
				_, errParse := tmpl.Parse(string(content))
				if errParse != nil {
					return errParse
				}

				for _, lp := range layoutsAndPartials {
					lpContent, errLP := os.ReadFile(lp)
					if errLP == nil {
						tmpl.Parse(string(lpContent))
					}
				}

				renderer.templates[relPath] = tmpl
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("Aviso: Error al cargar vistas: %v", err)
	}

	r.HTMLRender = renderer
}

func initApp() {
	_ = godotenv.Load()
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
