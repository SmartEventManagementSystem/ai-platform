# AI Platform - Event Management System

AI-powered services including RAG chatbot, MCP integration, and HuggingFace model inference.

## Features

- 🤖 **Chat Service**: LLM-powered chatbot using HuggingFace Inference API (FREE models)
- 🔍 **RAG (Retrieval-Augmented Generation)**: Context-aware responses from event data
- 🔌 **MCP (Model Context Protocol)**: Integration with third-party tools
- 💬 **Streaming Responses**: Real-time AI responses
- 🛠️ **Tool System**: Built-in tools for event management tasks
- 🔄 **Kafka Integration**: Async event processing and analytics

## Supported Models (All FREE - No API Key Required)

| Model | Context Length | Type | Quality |
|-------|----------------|------|---------|
| Mistral 7B Instruct v0.2 | 8K | Chat | ⭐⭐⭐ |
| Mistral 7B Instruct v0.3 | 128K | Chat | ⭐⭐⭐⭐ |
| Llama 3.1 8B Instruct | 128K | Chat | ⭐⭐⭐⭐⭐ |
| Llama 3.2 3B Instruct | 128K | Chat | ⭐⭐⭐ |
| Qwen 2.5 7B Instruct | 32K | Chat | ⭐⭐⭐⭐ |
| Qwen 2.5 14B Instruct | 32K | Chat | ⭐⭐⭐⭐⭐ |
| Phi-3 Mini | 128K | Chat | ⭐⭐⭐⭐ |
| Gemma 2 9B | 8K | Chat | ⭐⭐⭐⭐ |
| MiniLM Embeddings | - | Embedding | ⭐⭐⭐⭐ |

**Note:** All models work WITHOUT an API key on the free tier!

## Quick Start

```bash
# 1. Clone and setup
cd ai-platform
cp .env.example .env

# 2. Run the service (no dependencies needed for basic usage)
go run ./cmd/api/main.go

# Service will start on http://localhost:8081
```

## Docker

```bash
# Build and run
docker build -t ai-service .
docker run -p 8081:8081 -p 9091:9091 ai-service
```

## Kubernetes

```bash
kubectl apply -k k8s/
```

## Configuration

```env
# Server
PORT=8081
ENVIRONMENT=production
METRICS_PORT=9091

# HuggingFace (Optional - free tier works without API key)
HUGGINGFACE_API_KEY=
LLM_MODEL=mistralai/Mistral-7B-Instruct-v0.2
EMBEDDING_MODEL=sentence-transformers/all-MiniLM-L6-v2

# Services
KAFKA_BROKERS=localhost:9092
VECTOR_DB_URL=http://localhost:6333
REDIS_URL=localhost:6379
EVENT_SERVICE_URL=http://localhost:8080

# MCP
MCP_ENABLED=true
```

## API Endpoints

### Chat
```
POST /api/v1/ai/chat              - Send chat message
POST /api/v1/ai/chat/stream       - Stream chat response
GET  /api/v1/ai/chat/history      - Get chat history
GET  /api/v1/ai/models            - List available models
```

### RAG
```
POST /api/v1/ai/rag/query         - Query with RAG
POST /api/v1/ai/rag/ingest        - Ingest documents
POST /api/v1/ai/rag/search        - Semantic search
DELETE /api/v1/ai/rag/documents   - Delete documents
```

### MCP Tools
```
GET  /api/v1/ai/mcp/tools         - List available tools
POST /api/v1/ai/mcp/call         - Call a tool
```

### Health
```
GET  /health                      - Health check
GET  /ready                       - Readiness check
GET  /metrics                     - Prometheus metrics
```

## MCP Tools

| Tool | Description |
|------|-------------|
| `get_event_details` | Get event information by ID |
| `search_events` | Search for events |
| `create_event` | Create a new event |
| `update_event` | Update an event |
| `register_attendee` | Register attendee |
| `cancel_registration` | Cancel registration |
| `get_schedule` | Get event schedule |
| `get_weather` | Get weather forecast |
| `send_notification` | Send push notification |
| `get_analytics` | Get event analytics |
| `generate_qr_code` | Generate check-in QR code |

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Web Client                          │
└──────────────────────────┬──────────────────────────────────┘
                           │ HTTPS
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                      AI Platform (Go/Gin)                  │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐   │
│  │ Chat Handler │  │ RAG Handler │  │ MCP Handler    │   │
│  └──────┬──────┘  └──────┬──────┘  └───────┬────────┘   │
│         │                 │                  │            │
│  ┌──────▼──────┐  ┌──────▼──────┐  ┌───────▼────────┐   │
│  │Chat Service │  │RAG Service  │  │ MCP Service    │   │
│  └──────┬──────┘  └──────┬──────┘  └───────┬────────┘   │
│         │                 │                  │            │
│         └─────────────────┼──────────────────┘            │
│                           │                               │
│  ┌────────────────────────▼────────────────────────┐    │
│  │           HuggingFace Inference API              │    │
│  │  (FREE models - No API key required)           │    │
│  └─────────────────────────────────────────────────┘    │
│                           │                               │
│         ┌─────────────────┼─────────────────┐          │
│         ▼                 ▼                 ▼             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐   │
│  │   Qdrant    │  │    Kafka    │  │     Redis       │   │
│  │ (Vector DB) │  │ (Event Bus) │  │   (Cache)       │   │
│  └─────────────┘  └─────────────┘  └─────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

## Kafka Topics

| Topic | Description |
|-------|-------------|
| `chat.requests` | Incoming chat requests |
| `chat.responses` | Chat responses |
| `chat.events` | Chat events |
| `rag.events` | RAG processing events |
| `mcp.events` | MCP tool call events |

## Metrics

Prometheus-compatible metrics at `GET /metrics`:

- `ai_platform_requests_total`
- `ai_platform_requests_success`
- `ai_platform_requests_failed`
- `ai_platform_latency_seconds`

## License

MIT

Last updated: Sun Aug 23 13:29:37 +07 2026
# Trigger
