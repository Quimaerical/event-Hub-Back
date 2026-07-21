package controllers

import (
	"net/http"
	"strings"

	"event-hub/middlewares"
	"event-hub/models"

	"github.com/gin-gonic/gin"
)

type ProfileController struct{}

func NewProfileController() *ProfileController {
	return &ProfileController{}
}

// ShowProfile renders the user profile settings page.
func (ctrl *ProfileController) ShowProfile(c *gin.Context) {
	userIDVal, exists := c.Get("userID")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}
	userID := userIDVal.(int)

	ctx := c.Request.Context()
	user, err := models.GetUsuarioByID(ctx, userID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "auth/login.html", gin.H{
			"error": "Error al cargar la información del perfil",
		})
		return
	}

	roleName := "usuario"
	role, err := models.GetRoleByID(ctx, user.RoleID)
	if err == nil {
		roleName = role.Nombre
	}

	c.HTML(http.StatusOK, "perfil/index.html", gin.H{
		"userID":       userID,
		"email":        user.Email,
		"user":         user,
		"roleName":     roleName,
		"departamento": user.Departamento.String,
		"telefono":     user.Telefono.String,
	})
}

// UpdateProfile processes personal data updates.
func (ctrl *ProfileController) UpdateProfile(c *gin.Context) {
	userIDVal, exists := c.Get("userID")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}
	userID := userIDVal.(int)

	var form struct {
		Nombre       string `form:"nombre" binding:"required"`
		Departamento string `form:"departamento"`
		Telefono     string `form:"telefono"`
	}

	ctx := c.Request.Context()
	user, _ := models.GetUsuarioByID(ctx, userID)

	if err := c.ShouldBind(&form); err != nil {
		roleName := "usuario"
		if user != nil {
			if r, err := models.GetRoleByID(ctx, user.RoleID); err == nil {
				roleName = r.Nombre
			}
		}
		c.HTML(http.StatusBadRequest, "perfil/index.html", gin.H{
			"userID":       userID,
			"email":        user.Email,
			"user":         user,
			"roleName":     roleName,
			"departamento": user.Departamento.String,
			"telefono":     user.Telefono.String,
			"error":        "Por favor completa todos los campos requeridos correctamente",
		})
		return
	}

	if err := models.UpdateUsuarioProfile(ctx, userID, form.Nombre, form.Departamento, form.Telefono); err != nil {
		roleName := "usuario"
		if user != nil {
			if r, err := models.GetRoleByID(ctx, user.RoleID); err == nil {
				roleName = r.Nombre
			}
		}
		c.HTML(http.StatusInternalServerError, "perfil/index.html", gin.H{
			"userID":       userID,
			"email":        user.Email,
			"user":         user,
			"roleName":     roleName,
			"departamento": form.Departamento,
			"telefono":     form.Telefono,
			"error":        "Error al actualizar la información del perfil",
		})
		return
	}

	// Reload updated profile
	updatedUser, _ := models.GetUsuarioByID(ctx, userID)
	roleName := "usuario"
	if r, err := models.GetRoleByID(ctx, updatedUser.RoleID); err == nil {
		roleName = r.Nombre
	}

	c.HTML(http.StatusOK, "perfil/index.html", gin.H{
		"userID":       userID,
		"email":        updatedUser.Email,
		"user":         updatedUser,
		"roleName":     roleName,
		"departamento": updatedUser.Departamento.String,
		"telefono":     updatedUser.Telefono.String,
		"success":      "¡Perfil actualizado exitosamente!",
	})
}

// UpdatePassword handles password changes for local users.
func (ctrl *ProfileController) UpdatePassword(c *gin.Context) {
	userIDVal, exists := c.Get("userID")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}
	userID := userIDVal.(int)

	var form struct {
		CurrentPassword string `form:"current_password" binding:"required"`
		NewPassword     string `form:"new_password" binding:"required,min=6"`
		ConfirmPassword string `form:"confirm_password" binding:"required"`
	}

	ctx := c.Request.Context()
	user, _ := models.GetUsuarioByID(ctx, userID)
	roleName := "usuario"
	if user != nil {
		if r, err := models.GetRoleByID(ctx, user.RoleID); err == nil {
			roleName = r.Nombre
		}
	}

	if err := c.ShouldBind(&form); err != nil {
		c.HTML(http.StatusBadRequest, "perfil/index.html", gin.H{
			"userID":        userID,
			"email":         user.Email,
			"user":          user,
			"roleName":      roleName,
			"departamento":  user.Departamento.String,
			"telefono":      user.Telefono.String,
			"passwordError": "La nueva contraseña debe tener al menos 6 caracteres",
		})
		return
	}

	if form.NewPassword != form.ConfirmPassword {
		c.HTML(http.StatusBadRequest, "perfil/index.html", gin.H{
			"userID":        userID,
			"email":         user.Email,
			"user":          user,
			"roleName":      roleName,
			"departamento":  user.Departamento.String,
			"telefono":      user.Telefono.String,
			"passwordError": "La nueva contraseña y su confirmación no coinciden",
		})
		return
	}

	if err := models.UpdateUsuarioPassword(ctx, userID, form.CurrentPassword, form.NewPassword); err != nil {
		c.HTML(http.StatusBadRequest, "perfil/index.html", gin.H{
			"userID":        userID,
			"email":         user.Email,
			"user":          user,
			"roleName":      roleName,
			"departamento":  user.Departamento.String,
			"telefono":      user.Telefono.String,
			"passwordError": err.Error(),
		})
		return
	}

	c.HTML(http.StatusOK, "perfil/index.html", gin.H{
		"userID":          userID,
		"email":           user.Email,
		"user":            user,
		"roleName":        roleName,
		"departamento":    user.Departamento.String,
		"telefono":        user.Telefono.String,
		"passwordSuccess": "¡Contraseña actualizada con éxito!",
	})
}

// DeleteAccount handles self-service user account deletion with double verification.
func (ctrl *ProfileController) DeleteAccount(c *gin.Context) {
	userIDVal, exists := c.Get("userID")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}
	userID := userIDVal.(int)

	confirmacion := strings.TrimSpace(c.PostForm("confirmacion"))
	ctx := c.Request.Context()
	user, _ := models.GetUsuarioByID(ctx, userID)
	roleName := "usuario"
	if user != nil {
		if r, err := models.GetRoleByID(ctx, user.RoleID); err == nil {
			roleName = r.Nombre
		}
	}

	if confirmacion != "ELIMINAR" {
		c.HTML(http.StatusBadRequest, "perfil/index.html", gin.H{
			"userID":       userID,
			"email":        user.Email,
			"user":         user,
			"roleName":     roleName,
			"departamento": user.Departamento.String,
			"telefono":     user.Telefono.String,
			"deleteError":  "Debes escribir exactamente la palabra ELIMINAR en mayúsculas para confirmar.",
		})
		return
	}

	if err := models.DeleteUsuario(ctx, userID); err != nil {
		c.HTML(http.StatusInternalServerError, "perfil/index.html", gin.H{
			"userID":       userID,
			"email":        user.Email,
			"user":         user,
			"roleName":     roleName,
			"departamento": user.Departamento.String,
			"telefono":     user.Telefono.String,
			"deleteError":  "Error al eliminar la cuenta: " + err.Error(),
		})
		return
	}

	middlewares.ClearSessionCookie(c)
	c.Redirect(http.StatusSeeOther, "/")
}
