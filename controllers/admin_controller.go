package controllers

import (
	"log"
	"net/http"
	"strconv"

	"event-hub/models"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

type AdminController struct{}

func NewAdminController() *AdminController {
	return &AdminController{}
}

// ShowUsers muestra el panel de administración de usuarios, gestión de roles y ubicaciones/espacios.
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

	espacios, err := models.GetAllEspacios(ctx)
	if err != nil {
		log.Printf("Error obteniendo espacios en AdminController: %v", err)
	}

	userID, _ := c.Get("userID")
	email, _ := c.Get("email")

	c.HTML(http.StatusOK, "admin/users.html", gin.H{
		"usuarios": usuarios,
		"roles":    roles,
		"espacios": espacios,
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

// CreateEspacio permite a un administrador agregar un nuevo espacio/ubicación para eventos.
func (ctrl *AdminController) CreateEspacio(c *gin.Context) {
	nombre := c.PostForm("nombre")
	tipo := c.PostForm("tipo")
	capacidadStr := c.PostForm("capacidad")
	ubicacionStr := c.PostForm("ubicacion")

	capacidad, err := strconv.Atoi(capacidadStr)
	if err != nil || capacidad <= 0 {
		capacidad = 50
	}

	espacio := models.Espacio{
		Nombre:    nombre,
		Tipo:      tipo,
		Capacidad: capacidad,
		Ubicacion: pgtype.Text{String: ubicacionStr, Valid: ubicacionStr != ""},
	}

	ctx := c.Request.Context()
	if err := models.CreateEspacio(ctx, &espacio); err != nil {
		log.Printf("Error al crear espacio por el admin: %v", err)
	}

	c.Redirect(http.StatusSeeOther, "/admin/usuarios")
}
