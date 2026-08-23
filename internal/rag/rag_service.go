package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/SmartEventManagementSystem/ai-platform/internal"
)

type Document struct {
	Content  string   `json:"content"`
	Metadata Metadata `json:"metadata"`
}

type Metadata struct {
	Source  string `json:"source"`
	EventID string `json:"event_id,omitempty"`
}

type Chunk struct {
	ID       string   `json:"id"`
	Content  string   `json:"content"`
	Metadata Metadata `json:"metadata"`
}

type SearchResult struct {
	ID       string   `json:"id"`
	Content  string   `json:"content"`
	Score    float64  `json:"score"`
	Metadata Metadata `json:"metadata"`
}

type RAGService struct {
	cfg    internal.RAGConfig
	logger *zap.Logger
	client *http.Client
}

func NewRAGService(cfg internal.RAGConfig, logger *zap.Logger) *RAGService {
	return &RAGService{cfg: cfg, logger: logger, client: &http.Client{Timeout: 60 * time.Second}}
}

func (s *RAGService) GetEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	model := s.cfg.EmbeddingModel
	if model == "" {
		model = "sentence-transformers/all-MiniLM-L6-v2"
	}
	payload := map[string]interface{}{"inputs": texts}
	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://api-inference.huggingface.co/pipeline/feature-extraction/%s", model)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	if s.cfg.HuggingFaceAPIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.cfg.HuggingFaceAPIKey))
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var embeddings [][]float32
	if err := json.NewDecoder(resp.Body).Decode(&embeddings); err != nil {
		return nil, err
	}
	return embeddings, nil
}

func (s *RAGService) ChunkDocument(doc Document) []Chunk {
	chunkSize, overlap := 512, 128
	if s.cfg.ChunkSize > 0 {
		chunkSize = s.cfg.ChunkSize
	}
	if s.cfg.ChunkOverlap > 0 {
		overlap = s.cfg.ChunkOverlap
	}
	words := strings.Fields(doc.Content)
	if len(words) == 0 {
		return nil
	}
	chunks := make([]Chunk, 0)
	for start := 0; start < len(words); {
		end := start + chunkSize
		if end > len(words) {
			end = len(words)
		}
		chunks = append(chunks, Chunk{
			ID:       uuid.New().String(),
			Content:  strings.Join(words[start:end], " "),
			Metadata: Metadata{Source: doc.Metadata.Source, EventID: doc.Metadata.EventID},
		})
		start = end - overlap
		if start >= len(words)-overlap {
			break
		}
	}
	return chunks
}

func (s *RAGService) Query(ctx context.Context, query string, eventID string, topK int) (*RAGQueryResult, error) {
	if topK == 0 {
		topK = 5
	}
	embeddings, err := s.GetEmbeddings(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	results := s.SearchVectorStore(embeddings[0], eventID, topK)
	var ctxBuilder strings.Builder
	for i, r := range results {
		ctxBuilder.WriteString(fmt.Sprintf("[%d] %s\n", i+1, r.Content))
	}
	answer, _ := s.generateAnswer(ctx, query, ctxBuilder.String())
	if answer == "" {
		answer = "I'm sorry, I couldn't generate a response."
	}
	return &RAGQueryResult{Query: query, Answer: answer, Sources: results}, nil
}

type RAGQueryResult struct {
	Query   string         `json:"query"`
	Answer  string         `json:"answer"`
	Sources []SearchResult `json:"sources"`
}

func (s *RAGService) SearchVectorStore(embedding []float32, eventID string, topK int) []SearchResult {
	return []SearchResult{}
}

func (s *RAGService) generateAnswer(ctx context.Context, query, context string) (string, error) {
	model := s.cfg.LLMModel
	if model == "" {
		model = "mistralai/Mistral-7B-Instruct-v0.2"
	}
	prompt := fmt.Sprintf(`[INST] Based on context, answer question. Context: %s Question: %s Answer: [/INST]`, context, query)
	payload := map[string]interface{}{
		"inputs":     prompt,
		"parameters": map[string]interface{}{"max_new_tokens": 512, "temperature": 0.7},
	}
	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://api-inference.huggingface.co/models/%s", model)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	if s.cfg.HuggingFaceAPIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.cfg.HuggingFaceAPIKey))
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	switch v := result.(type) {
	case string:
		return strings.TrimSpace(v), nil
	case []interface{}:
		if len(v) > 0 {
			if s, ok := v[0].(string); ok {
				return strings.TrimSpace(s), nil
			}
		}
	}
	return "", nil
}

func (s *RAGService) StoreChunk(ctx context.Context, chunk Chunk, embedding []float32) error {
	return nil
}

func (s *RAGService) DeleteChunk(ctx context.Context, id string) error {
	s.logger.Info("DeleteChunk called", zap.String("id", id))
	return nil
}

func (s *RAGService) DeleteByEventID(ctx context.Context, eventID string) error {
	s.logger.Info("DeleteByEventID called", zap.String("event_id", eventID))
	return nil
}
