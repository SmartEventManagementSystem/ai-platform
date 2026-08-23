package internal

import "os"

type Config struct {
	ServerPort        string
	Environment       string
	HuggingFaceAPIKey string
	LLMModel          string
	EmbeddingModel    string
	VectorDBURL       string
	KafkaBrokers      string
	RedisURL          string
	RAGConfig         RAGConfig
	ChatConfig        ChatConfig
	MCPConfig         MCPConfig
	MetricsConfig     MetricsConfig
}

type RAGConfig struct {
	HuggingFaceAPIKey string
	HuggingFaceURL    string
	EmbeddingModel    string
	LLMModel          string
	VectorDBURL       string
	VectorCollection  string
	ChunkSize         int
	ChunkOverlap      int
	TopK              int
	ScoreThreshold    float32
}

type ChatConfig struct {
	HuggingFaceAPIKey string
	LLMModel          string
	MaxHistory        int
	MaxTokens         int
	Temperature       float64
	TopP              float64
	FreeModels        []string
}

type MCPConfig struct {
	Enabled    bool
	GatewayURL string
	APIKey     string
	Tools      []MCPTool
}

type MCPTool struct {
	Name        string
	Description string
	Handler     string
}

type MetricsConfig struct {
	Enabled bool
	Port    string
}

func LoadConfig() Config {
	llmModel := getEnv("LLM_MODEL", "mistralai/Mistral-7B-Instruct-v0.2")
	if llmModel == "" {
		llmModel = "mistralai/Mistral-7B-Instruct-v0.2"
	}

	embeddingModel := getEnv("EMBEDDING_MODEL", "sentence-transformers/all-MiniLM-L6-v2")
	if embeddingModel == "" {
		embeddingModel = "sentence-transformers/all-MiniLM-L6-v2"
	}

	return Config{
		ServerPort:        getEnv("PORT", "8081"),
		Environment:       getEnv("ENVIRONMENT", "development"),
		HuggingFaceAPIKey: getEnv("HUGGINGFACE_API_KEY", ""),
		LLMModel:          llmModel,
		EmbeddingModel:    embeddingModel,
		VectorDBURL:       getEnv("VECTOR_DB_URL", "http://localhost:6333"),
		KafkaBrokers:      getEnv("KAFKA_BROKERS", "localhost:9092"),
		RedisURL:          getEnv("REDIS_URL", "localhost:6379"),
		RAGConfig: RAGConfig{
			HuggingFaceAPIKey: getEnv("HUGGINGFACE_API_KEY", ""),
			EmbeddingModel:    embeddingModel,
			LLMModel:          llmModel,
			VectorDBURL:       getEnv("VECTOR_DB_URL", "http://localhost:6333"),
			VectorCollection:  getEnv("VECTOR_COLLECTION", "ems_events"),
			ChunkSize:         512,
			ChunkOverlap:      128,
			TopK:              5,
			ScoreThreshold:    0.7,
		},
		ChatConfig: ChatConfig{
			HuggingFaceAPIKey: getEnv("HUGGINGFACE_API_KEY", ""),
			LLMModel:          llmModel,
			MaxHistory:        20,
			MaxTokens:         2048,
			Temperature:       0.7,
			TopP:              0.9,
			FreeModels: []string{
				"mistralai/Mistral-7B-Instruct-v0.2",
				"mistralai/Mistral-7B-Instruct-v0.3",
				"meta-llama/Llama-3.1-8B-Instruct",
				"meta-llama/Llama-3.2-3B-Instruct",
				"Qwen/Qwen2.5-7B-Instruct",
				"Qwen/Qwen2.5-14B-Instruct",
				"microsoft/Phi-3-mini-128k-instruct",
				"google/gemma-2-9b-it",
				"openchat/openchat-7b",
				" teknium/OpenHermes-2.5-Mistral-7B",
				" WizardLM/WizardLM-2-8x22B",
				" NousResearch/Hermes-3-Llama-3.1-8B",
			},
		},
		MCPConfig: MCPConfig{
			Enabled:    getEnv("MCP_ENABLED", "true") == "true",
			GatewayURL: getEnv("MCP_GATEWAY_URL", "http://localhost:8082"),
			APIKey:     getEnv("MCP_API_KEY", ""),
			Tools: []MCPTool{
				{Name: "get_event_details", Description: "Get event details by ID", Handler: "get_event"},
				{Name: "search_events", Description: "Search for events", Handler: "search_events"},
				{Name: "register_attendee", Description: "Register an attendee for an event", Handler: "register_attendee"},
				{Name: "get_weather", Description: "Get weather forecast for a location", Handler: "get_weather"},
				{Name: "send_notification", Description: "Send push notification", Handler: "send_notification"},
				{Name: "get_schedule", Description: "Get event schedule", Handler: "get_schedule"},
				{Name: "create_event", Description: "Create a new event", Handler: "create_event"},
			},
		},
		MetricsConfig: MetricsConfig{
			Enabled: getEnv("METRICS_ENABLED", "true") == "true",
			Port:    getEnv("METRICS_PORT", "9091"),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
