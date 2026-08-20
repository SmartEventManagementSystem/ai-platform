package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/SmartEventManagementSystem/ai-platform/internal/rag"
)

type RAGHandler struct {
	svc *rag.RAGService
}

func NewRAGHandler(svc *rag.RAGService) *RAGHandler {
	return &RAGHandler{svc: svc}
}

type QueryRequest struct {
	Query  string `json:"query" binding:"required"`
	EventID string `json:"event_id"`
	TopK   int    `json:"top_k"`
}

type IngestRequest struct {
	Content string `json:"content" binding:"required"`
	Source  string `json:"source"`
	EventID string `json:"event_id"`
}

func (h *RAGHandler) Query(c *gin.Context) {
	var req QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.svc.Query(c.Request.Context(), req.Query, req.EventID, req.TopK)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *RAGHandler) Ingest(c *gin.Context) {
	var req IngestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	doc := rag.Document{
		Content: req.Content,
		Metadata: rag.Metadata{
			Source:  req.Source,
			EventID: req.EventID,
		},
	}

	chunks := h.svc.ChunkDocument(doc)
	for _, chunk := range chunks {
		embeddings, err := h.svc.GetEmbeddings(c.Request.Context(), []string{chunk.Content})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get embeddings"})
			return
		}
		if err := h.svc.StoreChunk(c.Request.Context(), chunk, embeddings[0]); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store chunk"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"chunks_created": len(chunks), "status": "success"})
}

func (h *RAGHandler) Search(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query is required"})
		return
	}

	embeddings, err := h.svc.GetEmbeddings(c.Request.Context(), []string{query})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	results := h.svc.SearchVectorStore(embeddings[0], c.Query("event_id"), 10)
	c.JSON(http.StatusOK, gin.H{"results": results})
}

type DeleteDocumentsRequest struct {
	IDs     []string `json:"ids"`
	EventID string   `json:"event_id"`
}

func (h *RAGHandler) DeleteDocuments(c *gin.Context) {
	var req DeleteDocumentsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.IDs) == 0 && req.EventID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "either ids or event_id is required"})
		return
	}

	deleted := 0
	if req.EventID != "" {
		if err := h.svc.DeleteByEventID(c.Request.Context(), req.EventID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		deleted++
	}

	for _, id := range req.IDs {
		if err := h.svc.DeleteChunk(c.Request.Context(), id); err == nil {
			deleted++
		}
	}

	c.JSON(http.StatusOK, gin.H{"deleted": deleted, "status": "success"})
}
