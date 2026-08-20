package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/SmartEventManagementSystem/ai-platform/internal/services"
)

type MCPHandler struct {
	svc *services.MCPService
}

func NewMCPHandler(svc *services.MCPService) *MCPHandler {
	return &MCPHandler{svc: svc}
}

func (h *MCPHandler) ListTools(c *gin.Context) {
	tools, err := h.svc.ListTools(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tools": tools,
		"count": len(tools),
	})
}

func (h *MCPHandler) CallTool(c *gin.Context) {
	var req services.ToolCallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.ToolName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tool_name is required"})
		return
	}

	result, err := h.svc.CallTool(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, result)
}
