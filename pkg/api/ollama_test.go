package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewOllamaClient(t *testing.T) {
	c := NewOllamaClient("localhost", 0)
	if c.baseURL != "http://localhost:11434" {
		t.Errorf("expected default port 11434, got %s", c.baseURL)
	}

	c = NewOllamaClient("192.168.1.1", 8080)
	if c.baseURL != "http://192.168.1.1:8080" {
		t.Errorf("expected custom port, got %s", c.baseURL)
	}
}

func TestPing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(modelsResponse{Models: []Model{}})
	}))
	defer server.Close()

	c := &OllamaClient{
		baseURL:    server.URL,
		httpClient: server.Client(),
		timeout:    5 * time.Second,
	}

	ctx := context.Background()
	if err := c.Ping(ctx); err != nil {
		t.Errorf("Ping failed: %v", err)
	}
}

func TestPingUnreachable(t *testing.T) {
	c := NewOllamaClient("localhost", 59999)
	c.httpClient.Timeout = 100 * time.Millisecond

	ctx := context.Background()
	err := c.Ping(ctx)
	if err == nil {
		t.Error("expected error for unreachable server")
	}
}

func TestListModels(t *testing.T) {
	models := []Model{
		{Name: "qwen2.5-coder:7b", Size: 4000000000},
		{Name: "codellama:7b", Size: 3500000000},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(modelsResponse{Models: models})
	}))
	defer server.Close()

	c := &OllamaClient{
		baseURL:    server.URL,
		httpClient: server.Client(),
		timeout:    5 * time.Second,
	}

	ctx := context.Background()
	result, err := c.ListModels(ctx)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 models, got %d", len(result))
	}
	if result[0].Name != "qwen2.5-coder:7b" {
		t.Errorf("expected first model to be qwen2.5-coder:7b, got %s", result[0].Name)
	}
}

func TestIsModelLoaded(t *testing.T) {
	models := []Model{
		{Name: "qwen2.5-coder:7b"},
		{Name: "codellama:7b"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(modelsResponse{Models: models})
	}))
	defer server.Close()

	c := &OllamaClient{
		baseURL:    server.URL,
		httpClient: server.Client(),
		timeout:    5 * time.Second,
	}

	ctx := context.Background()

	loaded, err := c.IsModelLoaded(ctx, "qwen2.5-coder:7b")
	if err != nil {
		t.Fatalf("IsModelLoaded failed: %v", err)
	}
	if !loaded {
		t.Error("expected model to be loaded")
	}

	loaded, err = c.IsModelLoaded(ctx, "nonexistent:latest")
	if err != nil {
		t.Fatalf("IsModelLoaded failed: %v", err)
	}
	if loaded {
		t.Error("expected model to not be loaded")
	}
}

func TestGenerate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/generate" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var req generateRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Model != "qwen2.5-coder:7b" {
			t.Errorf("expected model qwen2.5-coder:7b, got %s", req.Model)
		}
		if req.Stream {
			t.Error("expected stream to be false")
		}

		resp := GenerateResponse{
			Model:    req.Model,
			Response: "Hello! I'm ready to help.",
			Done:     true,
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := &OllamaClient{
		baseURL:    server.URL,
		httpClient: server.Client(),
		timeout:    5 * time.Second,
	}

	ctx := context.Background()
	resp, err := c.Generate(ctx, "qwen2.5-coder:7b", "Say hello")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if resp.Response != "Hello! I'm ready to help." {
		t.Errorf("unexpected response: %s", resp.Response)
	}
	if !resp.Done {
		t.Error("expected done to be true")
	}
}

func TestHealthCheck(t *testing.T) {
	models := []Model{{Name: "qwen2.5-coder:7b"}}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(modelsResponse{Models: models})
	}))
	defer server.Close()

	c := &OllamaClient{
		baseURL:    server.URL,
		httpClient: server.Client(),
		timeout:    5 * time.Second,
	}

	ctx := context.Background()

	if err := c.HealthCheck(ctx, "qwen2.5-coder:7b"); err != nil {
		t.Errorf("HealthCheck failed for loaded model: %v", err)
	}

	if err := c.HealthCheck(ctx, ""); err != nil {
		t.Errorf("HealthCheck failed without model check: %v", err)
	}

	err := c.HealthCheck(ctx, "nonexistent:latest")
	if err == nil {
		t.Error("expected error for unloaded model")
	}
}

func TestContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := &OllamaClient{
		baseURL:    server.URL,
		httpClient: server.Client(),
		timeout:    5 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := c.Ping(ctx)
	if err == nil {
		t.Error("expected context deadline exceeded error")
	}
}
