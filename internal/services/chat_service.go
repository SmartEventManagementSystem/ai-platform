package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/SmartEventManagementSystem/ai-platform/internal"
)

type Message struct {
	Role      string                 `json:"role"`
	Content   string                 `json:"content"`
	ToolCalls []ToolCall             `json:"tool_calls,omitempty"`
	ToolUseID string                 `json:"tool_call_id,omitempty"`
	Name      string                 `json:"name,omitempty"`
}

type ToolCall struct {
	ID       string                 `json:"id,omitempty"`
	Type     string                 `json:"type,omitempty"`
	Function FunctionCall            `json:"function,omitempty"`
}

type FunctionCall struct {
	Name      string                 `json:"name,omitempty"`
	Arguments string                 `json:"arguments,omitempty"`
}

type ChatRequest struct {
	Messages      []Message           `json:"messages"`
	EventID      string              `json:"event_id,omitempty"`
	SessionID    string              `json:"session_id,omitempty"`
	UserID       string              `json:"user_id,omitempty"`
	Model        string              `json:"model,omitempty"`
	Stream       bool                `json:"stream,omitempty"`
	Temperature  float64             `json:"temperature,omitempty"`
	MaxTokens    int                 `json:"max_tokens,omitempty"`
	Tools        []Tool              `json:"tools,omitempty"`
	ToolChoice   string              `json:"tool_choice,omitempty"`
}

type ChatResponse struct {
	Content   string                 `json:"content"`
	Model     string                `json:"model"`
	Usage     Usage                  `json:"usage,omitempty"`
	ToolCalls []ToolCallResult      `json:"tool_calls,omitempty"`
	FinishReason string              `json:"finish_reason,omitempty"`
}

type ToolCallResult struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Content  string `json:"content"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens     int `json:"total_tokens"`
}

type ChatCompletionChoice struct {
	Message      Message `json:"message"`
	FinishReason string `json:"finish_reason"`
}

type ChatCompletionResponse struct {
	ID      string               `json:"id"`
	Object  string               `json:"object"`
	Created int64                `json:"created"`
	Model   string               `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   Usage                `json:"usage"`
}

type session struct {
	Messages []Message
	mu      sync.RWMutex
}

type ChatService struct {
	cfg         internal.ChatConfig
	logger      *zap.Logger
	client      *http.Client
	sessions    map[string]*session
	sessionsMu  sync.RWMutex
	mcpService  *MCPService
}

func NewChatService(cfg internal.ChatConfig, logger *zap.Logger, mcp *MCPService) *ChatService {
	return &ChatService{
		cfg:        cfg,
		logger:     logger,
		client:     &http.Client{Timeout: 120 * time.Second},
		sessions:   make(map[string]*session),
		mcpService: mcp,
	}
}

func (s *ChatService) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = s.cfg.LLMModel
	}

	sessionID := req.SessionID
	if sessionID != "" {
		s.addToSession(sessionID, req.Messages)
	}

	prompt := s.buildPrompt(req.Messages)
	temperature := req.Temperature
	if temperature == 0 {
		temperature = s.cfg.Temperature
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = s.cfg.MaxTokens
	}

	s.logger.Info("Calling HuggingFace API",
		zap.String("model", model),
		zap.Int("history_size", len(req.Messages)))

	// Call HuggingFace Inference API
	result, err := s.callHuggingFaceAPI(ctx, model, prompt, temperature, maxTokens)
	if err != nil {
		s.logger.Error("HuggingFace API error", zap.Error(err))
		return nil, fmt.Errorf("failed to call AI service: %w", err)
	}

	// Handle tool calls if enabled
	if len(req.Tools) > 0 && s.containsToolCall(result) {
		toolResults, err := s.executeToolCalls(ctx, result, req)
		if err != nil {
			s.logger.Warn("Tool execution failed", zap.Error(err))
		}

		// Add tool results to context and generate final response
		if len(toolResults) > 0 {
			req.Messages = append(req.Messages, Message{Role: "assistant", Content: result})
			for _, tr := range toolResults {
				req.Messages = append(req.Messages, Message{
					Role:    "tool",
					Content: tr.Content,
					Name:    tr.Name,
					ToolUseID: tr.ID,
				})
			}
			result, err = s.callHuggingFaceAPI(ctx, model, s.buildPrompt(req.Messages), temperature, maxTokens)
			if err != nil {
				return &ChatResponse{Content: result, Model: model}, nil
			}
		}

		return &ChatResponse{
			Content:    result,
			Model:      model,
			ToolCalls:  toolResults,
			FinishReason: "tool_calls",
		}, nil
	}

	if sessionID != "" {
		s.addToSession(sessionID, []Message{{Role: "assistant", Content: result}})
	}

	return &ChatResponse{
		Content:      result,
		Model:        model,
		Usage:        Usage{PromptTokens: len(prompt) / 4, CompletionTokens: len(result) / 4, TotalTokens: (len(prompt) + len(result)) / 4},
		FinishReason: "stop",
	}, nil
}

func (s *ChatService) callHuggingFaceAPI(ctx context.Context, model, prompt string, temperature float64, maxTokens int) (string, error) {
	// Use HuggingFace Inference API (free tier)
	apiURL := fmt.Sprintf("https://api-inference.huggingface.co/models/%s", model)

	// Check if it's a chat model that expects messages format
	chatModels := map[string]bool{
		"mistralai/Mistral-7B-Instruct-v0.2": true,
		"mistralai/Mistral-7B-Instruct-v0.3": true,
		"meta-llama/Llama-3.1-8B-Instruct": true,
		"meta-llama/Llama-3.2-3B-Instruct": true,
		"Qwen/Qwen2.5-7B-Instruct": true,
		"Qwen/Qwen2.5-14B-Instruct": true,
		"microsoft/Phi-3-mini-128k-instruct": true,
		"google/gemma-2-9b-it": true,
	}

	var payload map[string]interface{}

	if chatModels[model] {
		// Use messages format for chat models
		messages := s.promptToMessages(prompt)
		payload = map[string]interface{}{
			"inputs":            messages,
			"parameters": map[string]interface{}{
				"max_new_tokens": maxTokens,
				"temperature":   temperature,
				"top_p":         s.cfg.TopP,
				"return_full_text": false,
			},
			"options": map[string]interface{}{
				"use_cache":  false,
				"wait_for_model": true,
			},
		}
	} else {
		// Use prompt format for older models
		payload = map[string]interface{}{
			"inputs":            prompt,
			"parameters": map[string]interface{}{
				"max_new_tokens": maxTokens,
				"temperature":   temperature,
				"top_p":         s.cfg.TopP,
				"return_full_text": false,
			},
			"options": map[string]interface{}{
				"use_cache":  false,
				"wait_for_model": true,
			},
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if s.cfg.HuggingFaceAPIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.cfg.HuggingFaceAPIKey))
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Handle rate limiting
	if resp.StatusCode == 429 {
		return "", fmt.Errorf("rate limited by HuggingFace. Please try again later")
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return s.parseHuggingFaceResponse(respBody, model)
}

func (s *ChatService) parseHuggingFaceResponse(body []byte, model string) (string, error) {
	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Handle array response (older format)
	if arr, ok := result.([]interface{}); ok {
		if len(arr) > 0 {
			switch v := arr[0].(type) {
			case string:
				return strings.TrimSpace(v), nil
			case map[string]interface{}:
				if gen, ok := v["generated_text"].(string); ok {
					return strings.TrimSpace(gen), nil
				}
			}
		}
		return "", fmt.Errorf("unexpected response format")
	}

	// Handle map response (newer format)
	if m, ok := result.(map[string]interface{}); ok {
		// Chat format
		if gen, ok := m["generated_text"].(string); ok {
			return strings.TrimSpace(gen), nil
		}
		if msg, ok := m["message"].(map[string]interface{}); ok {
			if content, ok := msg["content"].(string); ok {
				return strings.TrimSpace(content), nil
			}
		}
		if msg, ok := m["choices"].([]interface{}); ok {
			if len(msg) > 0 {
				if choice, ok := msg[0].(map[string]interface{}); ok {
					if msgContent, ok := choice["message"].(map[string]interface{}); ok {
						if content, ok := msgContent["content"].(string); ok {
							return strings.TrimSpace(content), nil
						}
					}
				}
			}
		}
	}

	return "", fmt.Errorf("could not parse response")
}

func (s *ChatService) promptToMessages(prompt string) []map[string]string {
	// Parse prompt to extract messages
	messages := []map[string]string{
		{"role": "system", "content": "You are an AI assistant for an event management platform called EMS. You help users with event organization, registration, scheduling, and general inquiries about events."},
	}

	// Try to extract user messages from prompt
	parts := strings.Split(prompt, "[INST]")
	for i, part := range parts {
		if i == 0 {
			continue
		}
		parts2 := strings.Split(part, "[/INST]")
		if len(parts2) > 0 {
			content := strings.TrimSpace(parts2[0])
			content = strings.Trim(content, " ")
			if content != "" {
				role := "user"
				if i%2 == 0 {
					role = "assistant"
				}
				messages = append(messages, map[string]string{"role": role, "content": content})
			}
		}
	}

	return messages
}

func (s *ChatService) buildPrompt(messages []Message) string {
	var sb strings.Builder
	sb.WriteString("[INST] You are an AI assistant for an event management platform. Help users with events, registration, scheduling, and general questions. Be helpful, accurate, and concise. [/INST]\n")

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			sb.WriteString(fmt.Sprintf("[INST] System: %s [/INST]\n", msg.Content))
		case "user":
			sb.WriteString(fmt.Sprintf("[INST] %s [/INST]\n", msg.Content))
		case "assistant":
			sb.WriteString(fmt.Sprintf("%s\n", msg.Content))
		case "tool":
			sb.WriteString(fmt.Sprintf("[TOOL_RESULT] %s: %s [/TOOL_RESULT]\n", msg.Name, msg.Content))
		}
	}
	return sb.String()
}

func (s *ChatService) StreamChat(ctx context.Context, req ChatRequest, callback func(string)) error {
	resp, err := s.Chat(ctx, req)
	if err != nil {
		return err
	}

	words := strings.Fields(resp.Content)
	for i, word := range words {
		callback(word + " ")
		if i < len(words)-1 {
			time.Sleep(15 * time.Millisecond)
		}
	}
	return nil
}

func (s *ChatService) containsToolCall(content string) bool {
	toolKeywords := []string{"search", "find", "get", "retrieve", "lookup", "check", "show", "list"}
	contentLower := strings.ToLower(content)
	for _, kw := range toolKeywords {
		if strings.Contains(contentLower, kw) {
			return true
		}
	}
	return false
}

func (s *ChatService) executeToolCalls(ctx context.Context, content string, req ChatRequest) ([]ToolCallResult, error) {
	var results []ToolCallResult

	// Simple tool extraction from content
	// In production, use proper function calling parsing
	contentLower := strings.ToLower(content)

	if strings.Contains(contentLower, "event") {
		if s.mcpService != nil {
			result, _ := s.mcpService.CallTool(ctx, ToolCallRequest{
				ToolName:  "get_event_details",
				Arguments: map[string]interface{}{"event_id": req.EventID},
			})
			if result != nil && result.Success {
				output, _ := json.Marshal(result.Output)
				results = append(results, ToolCallResult{
					ID:      "call_1",
					Name:    "get_event_details",
					Content: string(output),
				})
			}
		}
	}

	if strings.Contains(contentLower, "weather") || strings.Contains(contentLower, "forecast") {
		if s.mcpService != nil {
			result, _ := s.mcpService.CallTool(ctx, ToolCallRequest{
				ToolName:  "get_weather",
				Arguments: map[string]interface{}{"location": "event location"},
			})
			if result != nil && result.Success {
				output, _ := json.Marshal(result.Output)
				results = append(results, ToolCallResult{
					ID:      "call_2",
					Name:    "get_weather",
					Content: string(output),
				})
			}
		}
	}

	return results, nil
}

func (s *ChatService) addToSession(sessionID string, messages []Message) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	if _, exists := s.sessions[sessionID]; !exists {
		s.sessions[sessionID] = &session{Messages: make([]Message, 0)}
	}

	s.sessions[sessionID].mu.Lock()
	defer s.sessions[sessionID].mu.Unlock()

	s.sessions[sessionID].Messages = append(s.sessions[sessionID].Messages, messages...)

	// Trim history if too long
	if len(s.sessions[sessionID].Messages) > s.cfg.MaxHistory*2 {
		s.sessions[sessionID].Messages = s.sessions[sessionID].Messages[len(s.sessions[sessionID].Messages)-s.cfg.MaxHistory*2:]
	}
}

func (s *ChatService) GetSessionHistory(sessionID string) []Message {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	if session, exists := s.sessions[sessionID]; exists {
		session.mu.RLock()
		defer session.mu.RUnlock()
		return session.Messages
	}
	return []Message{}
}

func (s *ChatService) ClearSession(sessionID string) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	delete(s.sessions, sessionID)
}
