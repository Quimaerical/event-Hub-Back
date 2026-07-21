package models

import (
	"context"
	"errors"
	"time"

	"event-hub/config"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
)

// ErrTokenNoEncontrado se dispara cuando el usuario no tiene credenciales de Google vigentes
var ErrTokenNoEncontrado = errors.New("token de Google no encontrado para el usuario")

type Usuario struct {
	ID                 int                `json:"id" form:"id"`
	Nombre             string             `json:"nombre" form:"nombre" binding:"required"`
	Email              string             `json:"email" form:"email" binding:"required,email"`
	PasswordHash       string             `json:"-"`
	Password           string             `json:"password,omitempty" form:"password"`
	RoleID             int                `json:"role_id" form:"role_id"`
	Departamento       pgtype.Text        `json:"departamento,omitempty" form:"departamento"`
	Telefono           pgtype.Text        `json:"telefono,omitempty" form:"telefono"`
	OAuthProvider      string             `json:"oauth_provider"`
	OAuthID            string             `json:"oauth_id"`
	CreatedAt          time.Time          `json:"created_at"`
	
	// Campos para sincronización con Google Calendar
	GoogleAccessToken  pgtype.Text        `json:"-"`
	GoogleRefreshToken pgtype.Text        `json:"-"`
	GoogleTokenExpiry  pgtype.Timestamptz `json:"-"`
	
	// NUEVO: Campo para Firebase Cloud Messaging (Push Notifications Flutter)
	FcmToken           pgtype.Text        `json:"fcm_token,omitempty"`
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
			u.RoleID = 4 // Fallback default ID ('usuario')
		}
	}

	query := `
		INSERT INTO usuarios (nombre, email, password_hash, role_id, oauth_provider, oauth_id, fecha_registro)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		RETURNING id, fecha_registro
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
		SELECT id, nombre, email, password_hash, role_id, departamento, telefono, oauth_provider, oauth_id, fecha_registro, fcm_token
		FROM usuarios WHERE id = $1
	`
	var u Usuario
	var passHash *string
	var oauthID *string
	err := config.DB.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.Nombre, &u.Email, &passHash, &u.RoleID, &u.Departamento, &u.Telefono, &u.OAuthProvider, &oauthID, &u.CreatedAt, &u.FcmToken,
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
		SELECT id, nombre, email, password_hash, role_id, departamento, telefono, oauth_provider, oauth_id, fecha_registro, fcm_token
		FROM usuarios WHERE email = $1
	`
	var u Usuario
	var passHash *string
	var oauthID *string
	err := config.DB.QueryRow(ctx, query, email).Scan(
		&u.ID, &u.Nombre, &u.Email, &passHash, &u.RoleID, &u.Departamento, &u.Telefono, &u.OAuthProvider, &oauthID, &u.CreatedAt, &u.FcmToken,
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
		SELECT id, nombre, email, password_hash, role_id, departamento, telefono, oauth_provider, oauth_id, fecha_registro, fcm_token
		FROM usuarios WHERE oauth_provider = $1 AND oauth_id = $2
	`
	var u Usuario
	var passHash *string
	var oID *string
	err := config.DB.QueryRow(ctx, query, provider, oauthID).Scan(
		&u.ID, &u.Nombre, &u.Email, &passHash, &u.RoleID, &u.Departamento, &u.Telefono, &u.OAuthProvider, &oID, &u.CreatedAt, &u.FcmToken,
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

// GetGoogleToken recupera el token OAuth 2.0 desde PostgreSQL
func GetGoogleToken(ctx context.Context, userID int) (*oauth2.Token, error) {
	query := `
		SELECT google_access_token, google_refresh_token, google_token_expiry
		FROM usuarios
		WHERE id = $1
	`
	
	var accessToken, refreshToken pgtype.Text
	var expiry pgtype.Timestamptz
	
	err := config.DB.QueryRow(ctx, query, userID).Scan(&accessToken, &refreshToken, &expiry)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTokenNoEncontrado
		}
		return nil, err
	}

	if !accessToken.Valid || accessToken.String == "" {
		return nil, ErrTokenNoEncontrado
	}

	token := &oauth2.Token{
		AccessToken:  accessToken.String,
		RefreshToken: refreshToken.String,
		TokenType:    "Bearer",
	}

	if expiry.Valid {
		token.Expiry = expiry.Time
	}

	return token, nil
}

// SaveGoogleToken guarda o actualiza el token persistente de un usuario tras el Login
func SaveGoogleToken(ctx context.Context, userID int, token *oauth2.Token) error {
	if token == nil {
		return errors.New("el token proporcionado es nulo")
	}

	query := `
		UPDATE usuarios
		SET google_access_token = $1,
		    google_refresh_token = COALESCE($2, google_refresh_token),
		    google_token_expiry = $3
		WHERE id = $4
	`
	
	var expiry pgtype.Timestamptz
	if !token.Expiry.IsZero() {
		expiry = pgtype.Timestamptz{Time: token.Expiry, Valid: true}
	}

	var refreshToken pgtype.Text
	if token.RefreshToken != "" {
		refreshToken = pgtype.Text{String: token.RefreshToken, Valid: true}
	}

	_, err := config.DB.Exec(ctx, query, token.AccessToken, refreshToken, expiry, userID)
	return err
}

// NUEVO: UpdateFCMToken actualiza el token del dispositivo del usuario (para notificaciones de Flutter)
func UpdateFCMToken(ctx context.Context, userID int64, fcmToken string) error {
	query := `UPDATE usuarios SET fcm_token = $1 WHERE id = $2`
	_, err := config.DB.Exec(ctx, query, fcmToken, userID)
	return err
}

// NUEVO: GetFCMToken recupera el token del dispositivo de un usuario específico
func GetFCMToken(ctx context.Context, userID int64) (string, error) {
	query := `SELECT fcm_token FROM usuarios WHERE id = $1 AND fcm_token IS NOT NULL`
	var token string
	err := config.DB.QueryRow(ctx, query, userID).Scan(&token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errors.New("usuario no tiene un token FCM registrado")
		}
		return "", err
	}
	return token, nil
}

// UpdateUsuarioProfile actualiza el nombre, departamento y teléfono del usuario.
func UpdateUsuarioProfile(ctx context.Context, userID int, nombre, departamento, telefono string) error {
	query := `
		UPDATE usuarios
		SET nombre = $1,
		    departamento = NULLIF($2, ''),
		    telefono = NULLIF($3, '')
		WHERE id = $4
	`
	_, err := config.DB.Exec(ctx, query, nombre, departamento, telefono, userID)
	return err
}

// UpdateUsuarioPassword cambia la contraseña local del usuario validando la contraseña actual.
func UpdateUsuarioPassword(ctx context.Context, userID int, currentPassword, newPassword string) error {
	u, err := GetUsuarioByID(ctx, userID)
	if err != nil {
		return err
	}

	if u.OAuthProvider != "local" && u.PasswordHash == "" {
		return errors.New("tu cuenta fue registrada mediante un proveedor externo (Google/GitHub) y no requiere contraseña local")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(currentPassword)); err != nil {
		return errors.New("la contraseña actual es incorrecta")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	query := `UPDATE usuarios SET password_hash = $1 WHERE id = $2`
	_, err = config.DB.Exec(ctx, query, string(hashed), userID)
	return err
}

