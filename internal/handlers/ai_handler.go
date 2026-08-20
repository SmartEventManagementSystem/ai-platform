package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/SmartEventManagementSystem/ai-platform/internal/services"
)

type AIHandler struct {
	svc *services.ChatService
}

func NewAIHandler(svc *services.ChatService) *AIHandler {
	return &AIHandler{svc: svc}
}

func (h *AIHandler) Chat(c *gin.Context) {
	var req services.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.svc.Chat(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   err.Error(),
			"message": "Failed to process chat request",
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *AIHandler) ChatStream(c *gin.Context) {
	var req services.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	_ = h.svc.StreamChat(c.Request.Context(), req, func(chunk string) {
		c.Writer.Write([]byte("data: " + chunk + "\n\n"))
		c.Writer.Flush()
	})

	c.Writer.Write([]byte("data: [DONE]\n\n"))
}

func (h *AIHandler) GetHistory(c *gin.Context) {
	sessionID := c.Query("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
		return
	}

	messages := h.svc.GetSessionHistory(sessionID)
	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"messages":   messages,
	})
}

func (h *AIHandler) ClearHistory(c *gin.Context) {
	sessionID := c.Query("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
		return
	}

	h.svc.ClearSession(sessionID)
	c.JSON(http.StatusOK, gin.H{
		"message":    "Session cleared",
		"session_id": sessionID,
	})
}

func (h *AIHandler) ListModels(c *gin.Context) {
	models := []map[string]interface{}{
		{"id": "mistralai/Mistral-7B-Instruct-v0.2", "name": "Mistral 7B", "provider": "HuggingFace", "context_length": 8192},
		{"id": "mistralai/Mistral-7B-Instruct-v0.3", "name": "Mistral 7B v3", "provider": "HuggingFace", "context_length": 128000},
		{"id": "meta-llama/Llama-3.1-8B-Instruct", "name": "Llama 3.1 8B", "provider": "HuggingFace", "context_length": 128000},
		{"id": "meta-llama/Llama-3.2-3B-Instruct", "name": "Llama 3.2 3B", "provider": "HuggingFace", "context_length": 128000},
		{"id": "Qwen/Qwen2.5-7B-Instruct", "name": "Qwen 2.5 7B", "provider": "HuggingFace", "context_length": 32768},
		{"id": "Qwen/Qwen2.5-14B-Instruct", "name": "Qwen 2.5 14B", "provider": "HuggingFace", "context_length": 32768},
		{"id": "microsoft/Phi-3-mini-128k-instruct", "name": "Phi-3 Mini", "provider": "HuggingFace", "context_length": 128000},
		{"id": "google/gemma-2-9b-it", "name": "Gemma 2 9B", "provider": "HuggingFace", "context_length": 8192},
	}

	c.JSON(http.StatusOK, gin.H{
		"models": models,
		"default": "mistralai/Mistral-7B-Instruct-v0.2",
	})
}
