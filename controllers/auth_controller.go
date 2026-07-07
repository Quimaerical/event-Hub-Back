package controllers

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"

	"event-hub/middlewares"
	"event-hub/models"
	"event-hub/services"
	"github.com/gin-gonic/gin"
)

type AuthController struct {
	oauthService *services.OAuthService
}

func NewAuthController(oauth *services.OAuthService) *AuthController {
	return &AuthController{
		oauthService: oauth,
	}
}

// ShowLogin displays the login page.
func (ctrl *AuthController) ShowLogin(c *gin.Context) {
	c.HTML(http.StatusOK, "auth/login.html", nil)
}

// ShowRegister displays the registration page.
func (ctrl *AuthController) ShowRegister(c *gin.Context) {
	c.HTML(http.StatusOK, "auth/register.html", nil)
}

// HandleLogin processes credential submissions.
func (ctrl *AuthController) HandleLogin(c *gin.Context) {
	email := c.PostForm("email")
	password := c.PostForm("password")

	ctx := c.Request.Context()
	user, err := models.Authenticate(ctx, email, password)
	if err != nil {
		c.HTML(http.StatusUnauthorized, "auth/login.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	token, err := middlewares.GenerateToken(user.ID, user.Email, user.RoleID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "auth/login.html", gin.H{
			"error": "Error interno al generar sesión",
		})
		return
	}

	middlewares.SetSessionCookie(c, token)
	c.Redirect(http.StatusSeeOther, "/")
}

// HandleRegister processes user signup requests.
func (ctrl *AuthController) HandleRegister(c *gin.Context) {
	var form struct {
		Nombre   string `form:"nombre" binding:"required"`
		Email    string `form:"email" binding:"required,email"`
		Password string `form:"password" binding:"required"`
	}

	if err := c.ShouldBind(&form); err != nil {
		c.HTML(http.StatusBadRequest, "auth/register.html", gin.H{
			"error": "Datos inválidos en el formulario",
		})
		return
	}

	ctx := c.Request.Context()
	user := &models.Usuario{
		Nombre:   form.Nombre,
		Email:    form.Email,
		Password: form.Password,
	}

	err := models.CreateUsuario(ctx, user)
	if err != nil {
		c.HTML(http.StatusConflict, "auth/register.html", gin.H{
			"error": "El correo ya está registrado o los datos no son válidos",
		})
		return
	}

	token, err := middlewares.GenerateToken(user.ID, user.Email, user.RoleID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "auth/register.html", gin.H{
			"error": "Error al generar sesión del usuario",
		})
		return
	}

	middlewares.SetSessionCookie(c, token)
	c.Redirect(http.StatusSeeOther, "/")
}

// GoogleLogin redirects user to Google OAuth screen.
func (ctrl *AuthController) GoogleLogin(c *gin.Context) {
	state := generateState()
	c.SetCookie("oauth_state", state, 300, "/", "", false, true)
	url := ctrl.oauthService.GetGoogleAuthURL(state)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

// GitHubLogin redirects user to GitHub OAuth screen.
func (ctrl *AuthController) GitHubLogin(c *gin.Context) {
	state := generateState()
	c.SetCookie("oauth_state", state, 300, "/", "", false, true)
	url := ctrl.oauthService.GetGitHubAuthURL(state)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

// GoogleCallback handles the redirection back from Google.
func (ctrl *AuthController) GoogleCallback(c *gin.Context) {
	stateCookie, err := c.Cookie("oauth_state")
	if err != nil || stateCookie != c.Query("state") {
		c.HTML(http.StatusBadRequest, "auth/login.html", gin.H{"error": "Estado de verificación inválido (CSRF)"})
		return
	}

	code := c.Query("code")
	ctx := c.Request.Context()

	oauthUser, err := ctrl.oauthService.HandleGoogleCallback(ctx, code)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "auth/login.html", gin.H{"error": "Error al autenticar con Google"})
		return
	}

	ctrl.loginOrCreateOAuthUser(c, oauthUser)
}

// GitHubCallback handles the redirection back from GitHub.
func (ctrl *AuthController) GitHubCallback(c *gin.Context) {
	stateCookie, err := c.Cookie("oauth_state")
	if err != nil || stateCookie != c.Query("state") {
		c.HTML(http.StatusBadRequest, "auth/login.html", gin.H{"error": "Estado de verificación inválido (CSRF)"})
		return
	}

	code := c.Query("code")
	ctx := c.Request.Context()

	oauthUser, err := ctrl.oauthService.HandleGitHubCallback(ctx, code)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "auth/login.html", gin.H{"error": "Error al autenticar con GitHub"})
		return
	}

	ctrl.loginOrCreateOAuthUser(c, oauthUser)
}

// Logout clears authorization cookies.
func (ctrl *AuthController) Logout(c *gin.Context) {
	middlewares.ClearSessionCookie(c)
	c.Redirect(http.StatusSeeOther, "/")
}

func (ctrl *AuthController) loginOrCreateOAuthUser(c *gin.Context, oauthUser *services.OAuthUser) {
	ctx := c.Request.Context()

	// Find user by OAuth provider ID
	user, err := models.GetUsuarioByOAuth(ctx, oauthUser.Provider, oauthUser.ID)
	if err != nil {
		// Attempt finding by email to link accounts
		user, err = models.GetUsuarioByEmail(ctx, oauthUser.Email)
		if err != nil {
			// Account doesn't exist, create it
			user = &models.Usuario{
				Nombre:        oauthUser.Name,
				Email:         oauthUser.Email,
				OAuthProvider: oauthUser.Provider,
				OAuthID:       oauthUser.ID,
			}
			err = models.CreateUsuario(ctx, user)
			if err != nil {
				c.HTML(http.StatusInternalServerError, "auth/login.html", gin.H{"error": "Error al registrar el usuario de OAuth"})
				return
			}
		}
	}

	token, err := middlewares.GenerateToken(user.ID, user.Email, user.RoleID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "auth/login.html", gin.H{"error": "Error al generar sesión"})
		return
	}

	middlewares.SetSessionCookie(c, token)
	c.Redirect(http.StatusSeeOther, "/")
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// ==========================================
// NUEVO: INTEGRACIÓN CON FIREBASE/FLUTTER
// ==========================================

// Estructura esperada desde el frontend/móvil
type FCMTokenInput struct {
	Token string `json:"fcm_token" binding:"required"`
}

// HandleUpdateFCMToken recibe el token del dispositivo móvil y lo guarda en la base de datos
func (ctrl *AuthController) HandleUpdateFCMToken(c *gin.Context) {
	// 1. Extraer el ID del usuario autenticado desde el contexto del middleware
	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario no autorizado"})
		return
	}

	// Casteo seguro del ID a int64
	var userID int64
	switch v := userIDVal.(type) {
	case int:
		userID = int64(v)
	case int64:
		userID = v
	case float64:
		userID = int64(v)
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error interno con la sesión del usuario"})
		return
	}

	// 2. Validar el JSON recibido
	var input FCMTokenInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "El campo fcm_token es obligatorio"})
		return
	}

	// 3. Llamar al modelo para guardar el token en PostgreSQL
	err := models.UpdateFCMToken(c.Request.Context(), userID, input.Token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al actualizar el token en la base de datos"})
		return
	}

	// 4. Respuesta exitosa
	c.JSON(http.StatusOK, gin.H{
		"message": "Token de notificaciones actualizado correctamente",
	})
}
