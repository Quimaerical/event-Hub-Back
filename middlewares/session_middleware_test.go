package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGenerateToken(t *testing.T) {
	userID := 123
	email := "test@example.com"
	roleID := 2

	token, err := GenerateToken(userID, email, roleID)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if token == "" {
		t.Fatal("Expected token not to be empty")
	}
}

func TestAuthRequired_NoToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(AuthRequired())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "success")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized, got %d", w.Code)
	}
}

func TestAuthRequired_ValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(AuthRequired())
	router.GET("/test", func(c *gin.Context) {
		userID, _ := c.Get("userID")
		email, _ := c.Get("email")
		roleID, _ := c.Get("roleID")
		c.JSON(http.StatusOK, gin.H{
			"userID": userID,
			"email":  email,
			"roleID": roleID,
		})
	})

	// Generate a valid token
	token, err := GenerateToken(42, "user@test.com", 1)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %d", w.Code)
	}
}

func TestCurrentUser_NonBlocking(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CurrentUser())
	router.GET("/test", func(c *gin.Context) {
		userID, exists := c.Get("userID")
		c.JSON(http.StatusOK, gin.H{
			"exists": exists,
			"userID": userID,
		})
	})

	// Case 1: No token, should not block and exists should be false
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %d", w1.Code)
	}

	// Case 2: Valid token via cookie, should populate userID
	token, _ := GenerateToken(100, "current@test.com", 1)
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req2.AddCookie(&http.Cookie{
		Name:  "session_token",
		Value: token,
	})
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %d", w2.Code)
	}
}
