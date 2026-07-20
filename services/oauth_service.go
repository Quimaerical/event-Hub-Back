package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

type OAuthService struct {
	GoogleConfig *oauth2.Config
	GitHubConfig *oauth2.Config
}

type OAuthUser struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
}

// GetRedirectURLFromRequest determina dinámicamente el protocolo (http/https) y host real de la petición HTTP
func GetRedirectURLFromRequest(c *gin.Context, envVar string, callbackPath string) string {
	if val := os.Getenv(envVar); val != "" {
		return val
	}

	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" || os.Getenv("VERCEL") != "" {
		scheme = "https"
	}

	host := c.Request.Host
	if host != "" {
		return scheme + "://" + host + callbackPath
	}

	return "http://localhost:8080" + callbackPath
}

// NewOAuthService initializes the base OAuth configs.
func NewOAuthService() *OAuthService {
	googleConfig := &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.profile",
			"https://www.googleapis.com/auth/userinfo.email",
		},
		Endpoint: google.Endpoint,
	}

	githubConfig := &oauth2.Config{
		ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		Scopes:       []string{"user:email", "read:user"},
		Endpoint:     github.Endpoint,
	}

	return &OAuthService{
		GoogleConfig: googleConfig,
		GitHubConfig: githubConfig,
	}
}

// GetGoogleAuthURL returns the redirect URL for Google login using dynamic request host.
func (s *OAuthService) GetGoogleAuthURL(c *gin.Context, state string) string {
	config := *s.GoogleConfig
	config.RedirectURL = GetRedirectURLFromRequest(c, "GOOGLE_REDIRECT_URL", "/auth/google/callback")
	return config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// GetGitHubAuthURL returns the redirect URL for GitHub login using dynamic request host.
func (s *OAuthService) GetGitHubAuthURL(c *gin.Context, state string) string {
	config := *s.GitHubConfig
	config.RedirectURL = GetRedirectURLFromRequest(c, "GITHUB_REDIRECT_URL", "/auth/github/callback")
	return config.AuthCodeURL(state)
}

// HandleGoogleCallback exchanges auth code for token and retrieves the profile details.
func (s *OAuthService) HandleGoogleCallback(c *gin.Context, code string) (*OAuthUser, error) {
	config := *s.GoogleConfig
	config.RedirectURL = GetRedirectURLFromRequest(c, "GOOGLE_REDIRECT_URL", "/auth/google/callback")

	ctx := c.Request.Context()
	token, err := config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed google token exchange: %w", err)
	}

	client := config.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch userinfo from google: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google api returned status code: %s", resp.Status)
	}

	var googleProfile struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&googleProfile); err != nil {
		return nil, fmt.Errorf("failed to decode google user profile: %w", err)
	}

	return &OAuthUser{
		ID:       googleProfile.ID,
		Email:    googleProfile.Email,
		Name:     googleProfile.Name,
		Provider: "google",
	}, nil
}

// HandleGitHubCallback exchanges auth code for token and retrieves the profile details.
func (s *OAuthService) HandleGitHubCallback(c *gin.Context, code string) (*OAuthUser, error) {
	config := *s.GitHubConfig
	config.RedirectURL = GetRedirectURLFromRequest(c, "GITHUB_REDIRECT_URL", "/auth/github/callback")

	ctx := c.Request.Context()
	token, err := config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed github token exchange: %w", err)
	}

	client := config.Client(ctx, token)
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch userinfo from github: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned status code: %s", resp.Status)
	}

	var githubProfile struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&githubProfile); err != nil {
		return nil, fmt.Errorf("failed to decode github user profile: %w", err)
	}

	email := githubProfile.Email
	if email == "" {
		emailResp, err := client.Get("https://api.github.com/user/emails")
		if err == nil {
			defer emailResp.Body.Close()
			if emailResp.StatusCode == http.StatusOK {
				var emails []struct {
					Email    string `json:"email"`
					Primary  bool   `json:"primary"`
					Verified bool   `json:"verified"`
				}
				if err := json.NewDecoder(emailResp.Body).Decode(&emails); err == nil {
					for _, e := range emails {
						if e.Primary {
							email = e.Email
							break
						}
					}
					if email == "" && len(emails) > 0 {
						email = emails[0].Email
					}
				}
			}
		}
	}

	name := githubProfile.Name
	if name == "" {
		name = githubProfile.Login
	}

	return &OAuthUser{
		ID:       fmt.Sprintf("%d", githubProfile.ID),
		Email:    email,
		Name:     name,
		Provider: "github",
	}, nil
}
