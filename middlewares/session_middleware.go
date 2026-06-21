package middlewares

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte(getSecret())

func getSecret() string {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "event-hub-super-secret-key-change-it-in-production"
	}
	return secret
}

type Claims struct {
	UserID int    `json:"user_id"`
	Email  string `json:"email"`
	RoleID int    `json:"role_id"`
	jwt.RegisteredClaims
}

// GenerateToken creates a JWT signed token for a user.
func GenerateToken(userID int, email string, roleID int) (string, error) {
	claims := Claims{
		UserID: userID,
		Email:  email,
		RoleID: roleID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// SetSessionCookie sets a secure, HTTP-only cookie.
func SetSessionCookie(c *gin.Context, token string) {
	// Secure should be true in production (over HTTPS).
	host := c.Request.Host
	secure := !strings.HasPrefix(host, "localhost:") && !strings.HasPrefix(host, "127.0.0.1:")
	
	// Max age is 24 hours
	c.SetCookie("session_token", token, 86400, "/", "", secure, true)
}

// ClearSessionCookie deletes the session cookie.
func ClearSessionCookie(c *gin.Context) {
	host := c.Request.Host
	secure := !strings.HasPrefix(host, "localhost:") && !strings.HasPrefix(host, "127.0.0.1:")
	c.SetCookie("session_token", "", -1, "/", "", secure, true)
}

// AuthRequired validates session token and blocks unauthorized requests.
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, err := c.Cookie("session_token")
		if err != nil {
			// Check Authorization header as fallback
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
			} else {
				handleUnauthorized(c)
				c.Abort()
				return
			}
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			handleUnauthorized(c)
			c.Abort()
			return
		}

		c.Set("userID", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("roleID", claims.RoleID)
		c.Next()
	}
}

// CurrentUser is a non-blocking middleware that populates user info in the context if logged in.
func CurrentUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, err := c.Cookie("session_token")
		if err == nil {
			claims := &Claims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
				return jwtSecret, nil
			})
			if err == nil && token.Valid {
				c.Set("userID", claims.UserID)
				c.Set("email", claims.Email)
				c.Set("roleID", claims.RoleID)
			}
		}
		c.Next()
	}
}

func handleUnauthorized(c *gin.Context) {
	acceptHeader := c.GetHeader("Accept")
	if strings.Contains(acceptHeader, "text/html") {
		// Redirect HTML requests to login page
		c.Redirect(http.StatusSeeOther, "/login")
	} else {
		// JSON response for APIs
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Debe iniciar sesión para acceder a este recurso"})
	}
}
