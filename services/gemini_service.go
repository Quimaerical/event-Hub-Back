package services

import (
	"context"
	"fmt"
	"os"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GeminiService handles interaction with the Google Gemini API.
type GeminiService struct {
	apiKey string
}

// NewGeminiService instantiates a new GeminiService using the GEMINI_API_KEY environment variable.
func NewGeminiService() *GeminiService {
	return &GeminiService{
		apiKey: os.Getenv("GEMINI_API_KEY"),
	}
}

// GenerateText sends a prompt to Gemini and returns the generated text.
func (s *GeminiService) GenerateText(ctx context.Context, prompt string) (string, error) {
	if s.apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY environment variable is not configured")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(s.apiKey))
	if err != nil {
		return "", fmt.Errorf("failed to create Gemini client: %w", err)
	}
	defer client.Close()

	// Use gemini-2.5-flash for fast and cost-effective generation.
	model := client.GenerativeModel("gemini-2.5-flash")
	
	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("failed to generate content from Gemini API: %w", err)
	}

	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response candidate returned from Gemini")
	}

	var result string
	for _, part := range resp.Candidates[0].Content.Parts {
		if textPart, ok := part.(genai.Text); ok {
			result += string(textPart)
		}
	}

	return result, nil
}

// SuggestEventDescription leverages Gemini to generate an attractive description for a given event and location.
func (s *GeminiService) SuggestEventDescription(ctx context.Context, title, location string) (string, error) {
	prompt := fmt.Sprintf(
		"Escribe una descripción atractiva, profesional y concisa (máximo 120 palabras) en español para un evento titulado '%s' que se llevará a cabo en '%s'. Devuelve únicamente el texto de la descripción generada.",
		title, location,
	)
	return s.GenerateText(ctx, prompt)
}
