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
