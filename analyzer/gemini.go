package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	geminiBaseURL      = "https://generativelanguage.googleapis.com/v1beta/models"
	defaultGeminiModel = "gemini-2.5-pro"
)

type GeminiClient struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewGeminiClient(apiKey, model string) *GeminiClient {
	if model == "" {
		model = defaultGeminiModel
	}
	return &GeminiClient{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

type GeminiRequest struct {
	Contents         []Content        `json:"contents"`
	Tools            []GeminiTool     `json:"tools,omitempty"`
	GenerationConfig GenerationConfig `json:"generationConfig,omitempty"`
}

type Content struct {
	Role  string `json:"role"`
	Parts []Part `json:"parts"`
}

type Part struct {
	Text             string            `json:"text,omitempty"`
	FunctionCall     *FunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *FunctionResponse `json:"functionResponse,omitempty"`
}

type FunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type FunctionResponse struct {
	Name     string `json:"name"`
	Response any    `json:"response"`
}

type GeminiTool struct {
	FunctionDeclarations []FunctionDeclaration `json:"functionDeclarations"`
}

type FunctionDeclaration struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  *FunctionParams `json:"parameters,omitempty"`
}

type FunctionParams struct {
	Type       string                    `json:"type"`
	Properties map[string]PropertySchema `json:"properties"`
	Required   []string                  `json:"required,omitempty"`
}

type PropertySchema struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type GenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

type GeminiResponse struct {
	Candidates    []Candidate    `json:"candidates"`
	UsageMetadata *UsageMetadata `json:"usageMetadata,omitempty"`
}

type Candidate struct {
	Content      Content `json:"content"`
	FinishReason string  `json:"finishReason"`
}

type UsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

func (c *GeminiClient) GenerateWithTools(ctx context.Context, prompt string, tools []FunctionDeclaration) (*GeminiResponse, error) {
	contents := []Content{
		{
			Role: "user",
			Parts: []Part{
				{Text: prompt},
			},
		},
	}
	return c.generate(ctx, contents, tools)
}

func (c *GeminiClient) ContinueWithToolResult(ctx context.Context, history []Content, tools []FunctionDeclaration, toolName string, result any) (*GeminiResponse, error) {
	history = append(history, Content{
		Role: "user",
		Parts: []Part{
			{
				FunctionResponse: &FunctionResponse{
					Name:     toolName,
					Response: result,
				},
			},
		},
	})
	return c.generate(ctx, history, tools)
}

func (c *GeminiClient) generate(ctx context.Context, contents []Content, tools []FunctionDeclaration) (*GeminiResponse, error) {
	url := fmt.Sprintf("%s/%s:generateContent?key=%s", geminiBaseURL, c.model, c.apiKey)

	req := GeminiRequest{
		Contents: contents,
		GenerationConfig: GenerationConfig{
			Temperature:     0.7,
			MaxOutputTokens: 4096,
		},
	}

	if len(tools) > 0 {
		req.Tools = []GeminiTool{
			{FunctionDeclarations: tools},
		}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &geminiResp, nil
}

func (r *GeminiResponse) HasFunctionCall() bool {
	if len(r.Candidates) == 0 {
		return false
	}
	for _, part := range r.Candidates[0].Content.Parts {
		if part.FunctionCall != nil {
			return true
		}
	}
	return false
}

func (r *GeminiResponse) GetFunctionCall() *FunctionCall {
	if len(r.Candidates) == 0 {
		return nil
	}
	for _, part := range r.Candidates[0].Content.Parts {
		if part.FunctionCall != nil {
			return part.FunctionCall
		}
	}
	return nil
}

func (r *GeminiResponse) GetTextResponse() string {
	if len(r.Candidates) == 0 {
		return ""
	}
	for _, part := range r.Candidates[0].Content.Parts {
		if part.Text != "" {
			return part.Text
		}
	}
	return ""
}

func (r *GeminiResponse) GetContent() Content {
	if len(r.Candidates) == 0 {
		return Content{}
	}
	return r.Candidates[0].Content
}
