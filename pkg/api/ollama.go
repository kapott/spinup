package api

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
	DefaultOllamaPort    = 11434
	DefaultOllamaTimeout = 30 * time.Second
)

var (
	ErrOllamaNotReachable = fmt.Errorf("ollama service not reachable")
	ErrModelNotLoaded     = fmt.Errorf("model not loaded")
	ErrGenerateFailed     = fmt.Errorf("generate request failed")
)

type OllamaClient struct {
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
}

type OllamaOption func(*OllamaClient)

func WithHTTPClient(client *http.Client) OllamaOption {
	return func(c *OllamaClient) {
		c.httpClient = client
	}
}

func WithTimeout(timeout time.Duration) OllamaOption {
	return func(c *OllamaClient) {
		c.timeout = timeout
		c.httpClient.Timeout = timeout
	}
}

func NewOllamaClient(host string, port int, opts ...OllamaOption) *OllamaClient {
	if port == 0 {
		port = DefaultOllamaPort
	}

	c := &OllamaClient{
		baseURL: fmt.Sprintf("http://%s:%d", host, port),
		httpClient: &http.Client{
			Timeout: DefaultOllamaTimeout,
		},
		timeout: DefaultOllamaTimeout,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

type Model struct {
	Name       string    `json:"name"`
	ModifiedAt time.Time `json:"modified_at"`
	Size       int64     `json:"size"`
	Digest     string    `json:"digest"`
}

type modelsResponse struct {
	Models []Model `json:"models"`
}

type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type GenerateResponse struct {
	Model              string `json:"model"`
	Response           string `json:"response"`
	Done               bool   `json:"done"`
	TotalDuration      int64  `json:"total_duration"`
	LoadDuration       int64  `json:"load_duration"`
	PromptEvalCount    int    `json:"prompt_eval_count"`
	PromptEvalDuration int64  `json:"prompt_eval_duration"`
	EvalCount          int    `json:"eval_count"`
	EvalDuration       int64  `json:"eval_duration"`
}

func (c *OllamaClient) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrOllamaNotReachable, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrOllamaNotReachable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: status %d", ErrOllamaNotReachable, resp.StatusCode)
	}

	return nil
}

func (c *OllamaClient) ListModels(ctx context.Context) ([]Model, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOllamaNotReachable, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOllamaNotReachable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%w: status %d: %s", ErrOllamaNotReachable, resp.StatusCode, string(body))
	}

	var result modelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Models, nil
}

func (c *OllamaClient) IsModelLoaded(ctx context.Context, modelName string) (bool, error) {
	models, err := c.ListModels(ctx)
	if err != nil {
		return false, err
	}

	for _, m := range models {
		if m.Name == modelName {
			return true, nil
		}
	}

	return false, nil
}

func (c *OllamaClient) WaitForModel(ctx context.Context, modelName string, pollInterval time.Duration) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			loaded, err := c.IsModelLoaded(ctx, modelName)
			if err != nil {
				continue
			}
			if loaded {
				return nil
			}
		}
	}
}

func (c *OllamaClient) Generate(ctx context.Context, modelName, prompt string) (*GenerateResponse, error) {
	reqBody := generateRequest{
		Model:  modelName,
		Prompt: prompt,
		Stream: false,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/generate", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGenerateFailed, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGenerateFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%w: status %d: %s", ErrGenerateFailed, resp.StatusCode, string(body))
	}

	var result GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func (c *OllamaClient) HealthCheck(ctx context.Context, modelName string) error {
	if err := c.Ping(ctx); err != nil {
		return err
	}

	if modelName != "" {
		loaded, err := c.IsModelLoaded(ctx, modelName)
		if err != nil {
			return err
		}
		if !loaded {
			return fmt.Errorf("%w: %s", ErrModelNotLoaded, modelName)
		}
	}

	return nil
}

func (c *OllamaClient) BaseURL() string {
	return c.baseURL
}
