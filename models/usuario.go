package models

import (
	"context"
	"errors"
	"time"

	"event-hub/config"
	"golang.org/x/crypto/bcrypt"
)

type Usuario struct {
	ID            int       `json:"id" form:"id"`
	Nombre        string    `json:"nombre" form:"nombre" binding:"required"`
	Email         string    `json:"email" form:"email" binding:"required,email"`
	PasswordHash  string    `json:"-"`
	Password      string    `json:"password,omitempty" form:"password"`
	RoleID        int       `json:"role_id" form:"role_id"`
	OAuthProvider string    `json:"oauth_provider"`
	OAuthID       string    `json:"oauth_id"`
	CreatedAt     time.Time `json:"created_at"`
}

// CreateUsuario inserts a new user, hashing the password if it is local.
func CreateUsuario(ctx context.Context, u *Usuario) error {
	if u.OAuthProvider == "" {
		u.OAuthProvider = "local"
	}

	if u.OAuthProvider == "local" && u.Password != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		u.PasswordHash = string(hashed)
	}

	// Default to role 'usuario' if not specified
	if u.RoleID == 0 {
		role, err := GetRoleByName(ctx, "usuario")
		if err == nil {
			u.RoleID = role.ID
		} else {
			u.RoleID = 2 // Fallback default ID
		}
	}

	query := `
		INSERT INTO usuarios (nombre, email, password_hash, role_id, oauth_provider, oauth_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		RETURNING id, created_at
	`
	
	var passHash *string
	if u.PasswordHash != "" {
		passHash = &u.PasswordHash
	}
	
	var oID *string
	if u.OAuthID != "" {
		oID = &u.OAuthID
	}

	return config.DB.QueryRow(ctx, query, u.Nombre, u.Email, passHash, u.RoleID, u.OAuthProvider, oID).Scan(&u.ID, &u.CreatedAt)
}

// GetUsuarioByID retrieves a user by ID.
func GetUsuarioByID(ctx context.Context, id int) (*Usuario, error) {
	query := `
		SELECT id, nombre, email, password_hash, role_id, oauth_provider, oauth_id, created_at
		FROM usuarios WHERE id = $1
	`
	var u Usuario
	var passHash *string
	var oauthID *string
	err := config.DB.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.Nombre, &u.Email, &passHash, &u.RoleID, &u.OAuthProvider, &oauthID, &u.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if passHash != nil {
		u.PasswordHash = *passHash
	}
	if oauthID != nil {
		u.OAuthID = *oauthID
	}
	return &u, nil
}

// GetUsuarioByEmail retrieves a user by Email.
func GetUsuarioByEmail(ctx context.Context, email string) (*Usuario, error) {
	query := `
		SELECT id, nombre, email, password_hash, role_id, oauth_provider, oauth_id, created_at
		FROM usuarios WHERE email = $1
	`
	var u Usuario
	var passHash *string
	var oauthID *string
	err := config.DB.QueryRow(ctx, query, email).Scan(
		&u.ID, &u.Nombre, &u.Email, &passHash, &u.RoleID, &u.OAuthProvider, &oauthID, &u.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if passHash != nil {
		u.PasswordHash = *passHash
	}
	if oauthID != nil {
		u.OAuthID = *oauthID
	}
	return &u, nil
}

// GetUsuarioByOAuth retrieves a user by OAuth provider and ID.
func GetUsuarioByOAuth(ctx context.Context, provider, oauthID string) (*Usuario, error) {
	query := `
		SELECT id, nombre, email, password_hash, role_id, oauth_provider, oauth_id, created_at
		FROM usuarios WHERE oauth_provider = $1 AND oauth_id = $2
	`
	var u Usuario
	var passHash *string
	var oID *string
	err := config.DB.QueryRow(ctx, query, provider, oauthID).Scan(
		&u.ID, &u.Nombre, &u.Email, &passHash, &u.RoleID, &u.OAuthProvider, &oID, &u.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if passHash != nil {
		u.PasswordHash = *passHash
	}
	if oID != nil {
		u.OAuthID = *oID
	}
	return &u, nil
}

// Authenticate verifies password for a given email.
func Authenticate(ctx context.Context, email, password string) (*Usuario, error) {
	u, err := GetUsuarioByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if u.PasswordHash == "" {
		return nil, errors.New("esta cuenta no tiene contraseña local configurada (intenta acceder por OAuth)")
	}
	err = bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	if err != nil {
		return nil, errors.New("correo o contraseña incorrectos")
	}
	return u, nil
}
