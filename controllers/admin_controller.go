package controllers

import (
	"log"
	"net/http"
	"strconv"

	"event-hub/models"
	"github.com/gin-gonic/gin"
)

type AdminController struct{}

func NewAdminController() *AdminController {
	return &AdminController{}
}

// ShowUsers muestra el panel de administración de usuarios y gestión de roles.
func (ctrl *AdminController) ShowUsers(c *gin.Context) {
	ctx := c.Request.Context()
	usuarios, err := models.GetAllUsuariosWithRoles(ctx)
	if err != nil {
		log.Printf("Error obteniendo usuarios en AdminController: %v", err)
		c.HTML(http.StatusInternalServerError, "admin/users.html", gin.H{
			"error": "Error al cargar la lista de usuarios",
		})
		return
	}

	roles, err := models.GetAllRoles(ctx)
	if err != nil {
		log.Printf("Error obteniendo roles en AdminController: %v", err)
	}

	userID, _ := c.Get("userID")
	email, _ := c.Get("email")

	c.HTML(http.StatusOK, "admin/users.html", gin.H{
		"usuarios": usuarios,
		"roles":    roles,
		"userID":   userID,
		"email":    email,
		"roleID":   1,
	})
}

// UpdateUserRole procesa la solicitud de cambio de rol para un usuario.
func (ctrl *AdminController) UpdateUserRole(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/admin/usuarios")
		return
	}

	roleIDStr := c.PostForm("role_id")
	roleID, err := strconv.Atoi(roleIDStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/admin/usuarios")
		return
	}

	ctx := c.Request.Context()
	err = models.UpdateUsuarioRole(ctx, userID, roleID)
	if err != nil {
		log.Printf("Error al actualizar rol del usuario %d: %v", userID, err)
	}

	c.Redirect(http.StatusSeeOther, "/admin/usuarios")
}

// DeleteUser permite a un administrador eliminar cualquier cuenta de usuario.
func (ctrl *AdminController) DeleteUser(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/admin/usuarios")
		return
	}

	ctx := c.Request.Context()
	err = models.DeleteUsuario(ctx, userID)
	if err != nil {
		log.Printf("Error al eliminar usuario %d por el admin: %v", userID, err)
	}

	c.Redirect(http.StatusSeeOther, "/admin/usuarios")
}
