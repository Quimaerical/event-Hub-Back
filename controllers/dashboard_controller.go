package controllers

import (
	"log"
	"math"
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

	pageStr := c.Query("page")
	limitStr := c.Query("limit")
	page := 1
	limit := 20

	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	ctx := c.Request.Context()

	var currentUserID int64
	if uVal, exists := c.Get("userID"); exists {
		if id, ok := uVal.(int64); ok {
			currentUserID = id
		} else if idInt, ok := uVal.(int); ok {
			currentUserID = int64(idInt)
		}
	}

	var isAdminOrApprover bool
	var roleID int
	if rVal, exists := c.Get("roleID"); exists {
		switch r := rVal.(type) {
		case int:
			roleID = r
		case int64:
			roleID = int(r)
		case float64:
			roleID = int(r)
		}
		if roleID == 1 || roleID == 2 {
			isAdminOrApprover = true
		}
	}

	filtro := models.FiltroEvento{
		Search:            searchQuery,
		CategoryID:        categoryID,
		UserID:            currentUserID,
		IsAdminOrApprover: isAdminOrApprover,
		Page:              page,
		Limit:             limit,
	}

	var dbErrorMessage string

	// Query events (indexed search and filter category)
	events, totalCount, err := models.SearchEventos(ctx, filtro)
	if err != nil {
		log.Printf("Aviso en ShowDashboard (SearchEventos): %v", err)
		dbErrorMessage = "La base de datos se está reactivando o no tiene tablas registradas aún."
		events = []models.Evento{}
		totalCount = 0
	}

	// Fetch all categories for filter component
	categories, errCat := models.GetAllCategorias(ctx)
	if errCat != nil {
		log.Printf("Aviso en ShowDashboard (GetAllCategorias): %v", errCat)
		if dbErrorMessage == "" {
			dbErrorMessage = "No se pudieron cargar las categorías de la base de datos."
		}
		categories = []models.Categoria{}
	}

	// Read session user status if set by CurrentUser middleware
	userID, _ := c.Get("userID")
	email, _ := c.Get("email")

	// Calculate total pages for UI
	totalPages := int(math.Ceil(float64(totalCount) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}

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
			"roleID":             roleID,
			"page":               page,
			"limit":              limit,
			"totalCount":         totalCount,
			"totalPages":         totalPages,
			"error":              dbErrorMessage,
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
		"roleID":             roleID,
		"page":               page,
		"limit":              limit,
		"totalCount":         totalCount,
		"totalPages":         totalPages,
		"error":              dbErrorMessage,
	})
}
