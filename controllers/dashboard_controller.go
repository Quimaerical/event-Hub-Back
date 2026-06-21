package controllers

import (
	"net/http"
	"strconv"
	"strings"

	"event-hub/models"
	"github.com/gin-gonic/gin"
)

type DashboardController struct{}

func NewDashboardController() *DashboardController {
	return &DashboardController{}
}

// ShowDashboard handles rendering the main event board (cartelera) with filters.
func (ctrl *DashboardController) ShowDashboard(c *gin.Context) {
	searchQuery := c.Query("search")
	categoryIDStr := c.Query("category")

	var categoryID int
	if categoryIDStr != "" {
		if id, err := strconv.Atoi(categoryIDStr); err == nil {
			categoryID = id
		}
	}

	ctx := c.Request.Context()

	// Query events (indexed search and filter category)
	events, err := models.SearchEventos(ctx, searchQuery, categoryID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "dashboard/index.html", gin.H{
			"error": "Error al recuperar los eventos de la base de datos",
		})
		return
	}

	// Fetch all categories for filter component
	categories, err := models.GetAllCategorias(ctx)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "dashboard/index.html", gin.H{
			"error": "Error al recuperar las categorías de la base de datos",
		})
		return
	}

	// Read session user status if set by CurrentUser middleware
	userID, _ := c.Get("userID")
	email, _ := c.Get("email")

	// Return JSON if requested by API clients (mobile)
	importStrings := c.GetHeader("Accept")
	if strings.Contains(importStrings, "application/json") {
		c.JSON(http.StatusOK, gin.H{
			"eventos":            events,
			"categorias":         categories,
			"searchQuery":        searchQuery,
			"selectedCategoryID": categoryID,
			"userID":             userID,
			"email":              email,
		})
		return
	}

	c.HTML(http.StatusOK, "dashboard/index.html", gin.H{
		"eventos":            events,
		"categorias":         categories,
		"searchQuery":        searchQuery,
		"selectedCategoryID": categoryID,
		"userID":             userID,
		"email":              email,
	})
}
