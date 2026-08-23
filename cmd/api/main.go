package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"go.uber.org/zap"

	ai "github.com/SmartEventManagementSystem/ai-platform/internal"
	"github.com/SmartEventManagementSystem/ai-platform/internal/handlers"
	"github.com/SmartEventManagementSystem/ai-platform/internal/middleware"
	"github.com/SmartEventManagementSystem/ai-platform/internal/rag"
	"github.com/SmartEventManagementSystem/ai-platform/internal/services"
)

func main() {
	_ = godotenv.Load()

	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("🚀 Starting EMS AI Platform")

	cfg := ai.LoadConfig()

	// Initialize services
	eventSvcURL := os.Getenv("EVENT_SERVICE_URL")
	if eventSvcURL == "" {
		eventSvcURL = "http://event-service:8080"
	}

	mcpService := services.NewMCPService(cfg.MCPConfig, logger, eventSvcURL)
	chatService := services.NewChatService(cfg.ChatConfig, logger, mcpService)
	ragService := rag.NewRAGService(cfg.RAGConfig, logger)

	// Initialize handlers
	aiHandler := handlers.NewAIHandler(chatService)
	ragHandler := handlers.NewRAGHandler(ragService)
	mcpHandler := handlers.NewMCPHandler(mcpService)

	// Initialize Kafka producer for async events
	var kafkaProducer *services.KafkaProducer
	if cfg.KafkaBrokers != "" {
		kafkaProducer, err = services.NewKafkaProducer(cfg.KafkaBrokers, logger)
		if err != nil {
			logger.Warn("Failed to initialize Kafka producer", zap.Error(err))
		} else {
			defer kafkaProducer.Close()
			logger.Info("Kafka producer initialized", zap.String("brokers", cfg.KafkaBrokers))
		}
	}

	// Initialize metrics server
	if cfg.MetricsConfig.Enabled {
		go startMetricsServer(cfg.MetricsConfig.Port, logger)
	}

	// Set Gin mode
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create Gin router
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger(logger))
	r.Use(middleware.CORS())
	r.Use(middleware.RateLimiter())

	// Health check endpoints
	r.GET("/health", healthHandler(cfg))
	r.GET("/ready", readyHandler(chatService))
	r.GET("/metrics", metricsHandler())

	// API v1 routes
	v1 := r.Group("/api/v1/ai")
	{
		// Chat endpoints
		chat := v1.Group("/chat")
		{
			chat.POST("", aiHandler.Chat)
			chat.POST("/stream", aiHandler.ChatStream)
			chat.GET("/history", aiHandler.GetHistory)
			chat.DELETE("/history", aiHandler.ClearHistory)
			chat.GET("/models", aiHandler.ListModels)
		}

		// RAG endpoints
		rag := v1.Group("/rag")
		{
			rag.POST("/query", ragHandler.Query)
			rag.POST("/ingest", ragHandler.Ingest)
			rag.POST("/search", ragHandler.Search)
			rag.DELETE("/documents", ragHandler.DeleteDocuments)
		}

		// MCP endpoints
		mcp := v1.Group("/mcp")
		{
			mcp.GET("/tools", mcpHandler.ListTools)
			mcp.POST("/call", mcpHandler.CallTool)
		}

		// Streaming chat with Kafka events
		v1.POST("/chat/kafka", chatWithKafka(chatService, kafkaProducer))
	}

	// API v2 (newer endpoints)
	v2 := r.Group("/api/v2")
	{
		v2.POST("/ai/completions", completionHandler(chatService))
		v2.GET("/ai/models", availableModelsHandler(cfg))
	}

	// Start server
	port := cfg.ServerPort
	if port == "" {
		port = "8081"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		logger.Info(fmt.Sprintf("AI Platform listening on :%s", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited properly")
}

func healthHandler(cfg ai.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":      "ok",
			"service":     "ai-platform",
			"version":     "1.0.0",
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
			"environment": cfg.Environment,
		})
	}
}

func readyHandler(chatService *services.ChatService) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"ready":     true,
			"llm":       "healthy",
			"rag":       "healthy",
			"mcp":       "healthy",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}
}

func metricsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		metrics := map[string]interface{}{
			"requests_total":   0,
			"requests_success": 0,
			"requests_failed":  0,
			"avg_latency_ms":   0,
			"timestamp":        time.Now().UTC().Format(time.RFC3339),
		}
		c.JSON(http.StatusOK, metrics)
	}
}

func chatWithKafka(chatService *services.ChatService, producer *services.KafkaProducer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req services.ChatRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Send to Kafka for async processing
		if producer != nil {
			event := map[string]interface{}{
				"type":       "chat_request",
				"session_id": req.SessionID,
				"user_id":    req.UserID,
				"event_id":   req.EventID,
				"timestamp":  time.Now().UTC().Format(time.RFC3339),
			}
			producer.Publish("chat.requests", event)
		}

		// Process synchronously
		resp, err := chatService.Chat(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Send response to Kafka
		if producer != nil {
			event := map[string]interface{}{
				"type":       "chat_response",
				"session_id": req.SessionID,
				"model":      resp.Model,
				"tokens":     resp.Usage.TotalTokens,
				"timestamp":  time.Now().UTC().Format(time.RFC3339),
			}
			producer.Publish("chat.responses", event)
		}

		c.JSON(http.StatusOK, resp)
	}
}

func completionHandler(chatService *services.ChatService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Model       string        `json:"model"`
			Messages    []interface{} `json:"messages"`
			Temperature float64       `json:"temperature"`
			MaxTokens   int           `json:"max_tokens"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Convert messages
		messages := make([]services.Message, len(req.Messages))
		for i, m := range req.Messages {
			if msg, ok := m.(map[string]interface{}); ok {
				messages[i] = services.Message{
					Role:    getString(msg, "role", "user"),
					Content: getString(msg, "content", ""),
				}
			}
		}

		chatReq := services.ChatRequest{
			Messages:    messages,
			Model:       req.Model,
			Temperature: req.Temperature,
			MaxTokens:   req.MaxTokens,
		}

		resp, err := chatService.Chat(c.Request.Context(), chatReq)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, resp)
	}
}

func availableModelsHandler(cfg ai.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		models := []map[string]interface{}{
			{"id": "mistralai/Mistral-7B-Instruct-v0.2", "name": "Mistral 7B", "provider": "HuggingFace", "type": "chat", "context_length": 8192},
			{"id": "mistralai/Mistral-7B-Instruct-v0.3", "name": "Mistral 7B v3", "provider": "HuggingFace", "type": "chat", "context_length": 128000},
			{"id": "meta-llama/Llama-3.1-8B-Instruct", "name": "Llama 3.1 8B", "provider": "HuggingFace", "type": "chat", "context_length": 128000},
			{"id": "meta-llama/Llama-3.2-3B-Instruct", "name": "Llama 3.2 3B", "provider": "HuggingFace", "type": "chat", "context_length": 128000},
			{"id": "Qwen/Qwen2.5-7B-Instruct", "name": "Qwen 2.5 7B", "provider": "HuggingFace", "type": "chat", "context_length": 32768},
			{"id": "Qwen/Qwen2.5-14B-Instruct", "name": "Qwen 2.5 14B", "provider": "HuggingFace", "type": "chat", "context_length": 32768},
			{"id": "microsoft/Phi-3-mini-128k-instruct", "name": "Phi-3 Mini", "provider": "HuggingFace", "type": "chat", "context_length": 128000},
			{"id": "google/gemma-2-9b-it", "name": "Gemma 2 9B", "provider": "HuggingFace", "type": "chat", "context_length": 8192},
			{"id": "sentence-transformers/all-MiniLM-L6-v2", "name": "MiniLM Embeddings", "provider": "HuggingFace", "type": "embedding", "dimension": 384},
		}

		c.JSON(http.StatusOK, gin.H{
			"models":  models,
			"default": cfg.ChatConfig.LLMModel,
		})
	}
}

func startMetricsServer(port string, logger *zap.Logger) {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "# AI Platform Metrics\n")
		fmt.Fprintf(w, "ai_platform_up 1\n")
		fmt.Fprintf(w, "ai_platform_requests_total %d\n", 0)
	})

	addr := fmt.Sprintf(":%s", port)
	logger.Info(fmt.Sprintf("Metrics server listening on %s", addr))
	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Error("Metrics server error", zap.Error(err))
	}
}

func getString(m map[string]interface{}, key, defaultVal string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return defaultVal
}
// CI/CD Test - Sun Aug 23 14:29:28 +07 2026
