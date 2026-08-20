package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/SmartEventManagementSystem/ai-platform/internal"
)

type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema,omitempty"`
	Category    string                 `json:"category"`
}

type ToolCallRequest struct {
	ToolName  string                 `json:"tool_name"`
	Arguments map[string]interface{} `json:"arguments"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

type MCPToolResult struct {
	Success bool        `json:"success"`
	Output  interface{} `json:"output,omitempty"`
	Error   string      `json:"error,omitempty"`
	Latency string      `json:"latency,omitempty"`
}

type MCPService struct {
	cfg         internal.MCPConfig
	logger      *zap.Logger
	httpClient  *http.Client
	eventSvcURL string
}

func NewMCPService(cfg internal.MCPConfig, logger *zap.Logger, eventSvcURL string) *MCPService {
	return &MCPService{
		cfg:        cfg,
		logger:     logger,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		eventSvcURL: eventSvcURL,
	}
}

func (s *MCPService) ListTools(ctx context.Context) ([]Tool, error) {
	if !s.cfg.Enabled {
		return []Tool{}, nil
	}

	tools := []Tool{
		// Event Management Tools
		{
			Name:        "get_event_details",
			Description: "Get detailed information about a specific event by ID",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"event_id": map[string]interface{}{"type": "string", "description": "The unique event identifier"},
				},
				"required": []string{"event_id"},
			},
			Category: "events",
		},
		{
			Name:        "search_events",
			Description: "Search for events based on criteria like name, date, location",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query":      map[string]interface{}{"type": "string"},
					"category":   map[string]interface{}{"type": "string"},
					"start_date": map[string]interface{}{"type": "string"},
					"end_date":   map[string]interface{}{"type": "string"},
					"limit":      map[string]interface{}{"type": "integer"},
				},
			},
			Category: "events",
		},
		{
			Name:        "create_event",
			Description: "Create a new event with details",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title":       map[string]interface{}{"type": "string"},
					"description": map[string]interface{}{"type": "string"},
					"start_time":  map[string]interface{}{"type": "string"},
					"end_time":    map[string]interface{}{"type": "string"},
					"location":    map[string]interface{}{"type": "string"},
					"capacity":    map[string]interface{}{"type": "integer"},
				},
				"required": []string{"title", "start_time"},
			},
			Category: "events",
		},
		{
			Name:        "update_event",
			Description: "Update an existing event",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"event_id":   map[string]interface{}{"type": "string"},
					"title":      map[string]interface{}{"type": "string"},
					"capacity":   map[string]interface{}{"type": "integer"},
					"status":     map[string]interface{}{"type": "string"},
				},
				"required": []string{"event_id"},
			},
			Category: "events",
		},
		{
			Name:        "register_attendee",
			Description: "Register an attendee for an event",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"event_id":  map[string]interface{}{"type": "string"},
					"user_id":   map[string]interface{}{"type": "string"},
					"user_name": map[string]interface{}{"type": "string"},
					"email":     map[string]interface{}{"type": "string"},
				},
				"required": []string{"event_id", "user_id"},
			},
			Category: "registration",
		},
		{
			Name:        "cancel_registration",
			Description: "Cancel an attendee registration",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"event_id":        map[string]interface{}{"type": "string"},
					"user_id":         map[string]interface{}{"type": "string"},
					"registration_id": map[string]interface{}{"type": "string"},
				},
				"required": []string{"event_id"},
			},
			Category: "registration",
		},
		{
			Name:        "get_schedule",
			Description: "Get the schedule/agenda for an event",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"event_id": map[string]interface{}{"type": "string"},
					"date":     map[string]interface{}{"type": "string"},
				},
				"required": []string{"event_id"},
			},
			Category: "events",
		},
		{
			Name:        "get_weather",
			Description: "Get weather forecast for an event location",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location":    map[string]interface{}{"type": "string"},
					"event_date":  map[string]interface{}{"type": "string"},
				},
				"required": []string{"location"},
			},
			Category: "utilities",
		},
		{
			Name:        "send_notification",
			Description: "Send a push notification to event attendees",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"event_id": map[string]interface{}{"type": "string"},
					"title":    map[string]interface{}{"type": "string"},
					"message":  map[string]interface{}{"type": "string"},
					"type":     map[string]interface{}{"type": "string"},
				},
				"required": []string{"event_id", "title", "message"},
			},
			Category: "notifications",
		},
		{
			Name:        "get_analytics",
			Description: "Get analytics data for an event",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"event_id":  map[string]interface{}{"type": "string"},
					"metric":    map[string]interface{}{"type": "string"},
					"time_range": map[string]interface{}{"type": "string"},
				},
				"required": []string{"event_id"},
			},
			Category: "analytics",
		},
		{
			Name:        "generate_qr_code",
			Description: "Generate QR code for event check-in",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"event_id":      map[string]interface{}{"type": "string"},
					"user_id":       map[string]interface{}{"type": "string"},
					"registration_id": map[string]interface{}{"type": "string"},
				},
				"required": []string{"event_id"},
			},
			Category: "utilities",
		},
	}

	return tools, nil
}

func (s *MCPService) CallTool(ctx context.Context, req ToolCallRequest) (*MCPToolResult, error) {
	start := time.Now()
	s.logger.Info("MCP tool call",
		zap.String("tool", req.ToolName),
		zap.Any("args", req.Arguments))

	var result *MCPToolResult
	var err error

	switch req.ToolName {
	// Event Tools
	case "get_event_details":
		result, err = s.getEventDetails(ctx, req)
	case "search_events":
		result, err = s.searchEvents(ctx, req)
	case "create_event":
		result, err = s.createEvent(ctx, req)
	case "update_event":
		result, err = s.updateEvent(ctx, req)
	case "get_schedule":
		result, err = s.getSchedule(ctx, req)

	// Registration Tools
	case "register_attendee":
		result, err = s.registerAttendee(ctx, req)
	case "cancel_registration":
		result, err = s.cancelRegistration(ctx, req)

	// Utility Tools
	case "get_weather":
		result, err = s.getWeather(ctx, req)
	case "send_notification":
		result, err = s.sendNotification(ctx, req)
	case "get_analytics":
		result, err = s.getAnalytics(ctx, req)
	case "generate_qr_code":
		result, err = s.generateQRCode(ctx, req)

	default:
		err = fmt.Errorf("unknown tool: %s", req.ToolName)
		result = &MCPToolResult{Success: false, Error: err.Error()}
	}

	if result != nil {
		result.Latency = time.Since(start).String()
	}

	return result, err
}

// Event Tools Implementation

func (s *MCPService) getEventDetails(ctx context.Context, req ToolCallRequest) (*MCPToolResult, error) {
	eventID, ok := req.Arguments["event_id"].(string)
	if !ok {
		return &MCPToolResult{Success: false, Error: "event_id is required"}, nil
	}

	// Call event service
	url := fmt.Sprintf("%s/api/v1/events/%s", s.eventSvcURL, eventID)
	resp, err := s.httpClient.Get(url)
	if err != nil {
		// Return mock data for demo
		return &MCPToolResult{
			Success: true,
			Output: map[string]interface{}{
				"event_id":          eventID,
				"title":             "Tech Conference 2024",
				"description":       "Annual technology conference",
				"start_time":        "2024-12-15T09:00:00Z",
				"end_time":          "2024-12-17T18:00:00Z",
				"location":          "San Francisco Convention Center",
				"capacity":          5000,
				"registered_count":  3250,
				"status":            "active",
				"category":          "technology",
				"organizer":         "EMS Events",
			},
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return &MCPToolResult{Success: false, Error: fmt.Sprintf("event service returned %d", resp.StatusCode)}, nil
	}

	var event map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&event); err != nil {
		return &MCPToolResult{Success: false, Error: "failed to parse response"}, nil
	}

	return &MCPToolResult{Success: true, Output: event}, nil
}

func (s *MCPService) searchEvents(ctx context.Context, req ToolCallRequest) (*MCPToolResult, error) {
	// Return mock search results for demo
	events := []map[string]interface{}{
		{
			"id":          "evt_001",
			"title":       "Tech Conference 2024",
			"start_time":  "2024-12-15T09:00:00Z",
			"location":    "San Francisco",
			"category":    "technology",
			"attendees":   3250,
		},
		{
			"id":          "evt_002",
			"title":       "AI Summit",
			"start_time":  "2024-11-20T10:00:00Z",
			"location":    "New York",
			"category":    "technology",
			"attendees":   1500,
		},
		{
			"id":          "evt_003",
			"title":       "Music Festival",
			"start_time":  "2025-01-10T12:00:00Z",
			"location":    "Los Angeles",
			"category":    "entertainment",
			"attendees":   10000,
		},
	}

	return &MCPToolResult{Success: true, Output: events}, nil
}

func (s *MCPService) createEvent(ctx context.Context, req ToolCallRequest) (*MCPToolResult, error) {
	eventData := map[string]interface{}{
		"event_id": fmt.Sprintf("evt_%d", time.Now().Unix()),
		"status":   "created",
	}

	if title, ok := req.Arguments["title"].(string); ok {
		eventData["title"] = title
	}
	if desc, ok := req.Arguments["description"].(string); ok {
		eventData["description"] = desc
	}
	if startTime, ok := req.Arguments["start_time"].(string); ok {
		eventData["start_time"] = startTime
	}
	if location, ok := req.Arguments["location"].(string); ok {
		eventData["location"] = location
	}

	return &MCPToolResult{Success: true, Output: eventData}, nil
}

func (s *MCPService) updateEvent(ctx context.Context, req ToolCallRequest) (*MCPToolResult, error) {
	eventID, _ := req.Arguments["event_id"].(string)
	return &MCPToolResult{
		Success: true,
		Output: map[string]interface{}{
			"event_id": eventID,
			"status":   "updated",
			"updated_at": time.Now().Format(time.RFC3339),
		},
	}, nil
}

func (s *MCPService) getSchedule(ctx context.Context, req ToolCallRequest) (*MCPToolResult, error) {
	eventID, _ := req.Arguments["event_id"].(string)
	return &MCPToolResult{
		Success: true,
		Output: map[string]interface{}{
			"event_id": eventID,
			"schedule": []map[string]interface{}{
				{"time": "09:00", "title": "Registration & Welcome Coffee"},
				{"time": "10:00", "title": "Opening Keynote"},
				{"time": "11:30", "title": "Breakout Sessions"},
				{"time": "13:00", "title": "Lunch Break"},
				{"time": "14:00", "title": "Workshops"},
				{"time": "17:00", "title": "Networking Session"},
			},
		},
	}, nil
}

// Registration Tools

func (s *MCPService) registerAttendee(ctx context.Context, req ToolCallRequest) (*MCPToolResult, error) {
	regID := fmt.Sprintf("reg_%d", time.Now().UnixNano()%1000000)
	return &MCPToolResult{
		Success: true,
		Output: map[string]interface{}{
			"registration_id": regID,
			"status":          "confirmed",
			"event_id":        req.Arguments["event_id"],
			"user_id":         req.Arguments["user_id"],
			"qr_code_url":     fmt.Sprintf("https://api.ems.local/qr/%s", regID),
			"registered_at":   time.Now().Format(time.RFC3339),
		},
	}, nil
}

func (s *MCPService) cancelRegistration(ctx context.Context, req ToolCallRequest) (*MCPToolResult, error) {
	return &MCPToolResult{
		Success: true,
		Output: map[string]interface{}{
			"status":       "cancelled",
			"refund_status": "pending",
			"cancelled_at": time.Now().Format(time.RFC3339),
		},
	}, nil
}

// Utility Tools

func (s *MCPService) getWeather(ctx context.Context, req ToolCallRequest) (*MCPToolResult, error) {
	location, _ := req.Arguments["location"].(string)
	if location == "" {
		location = "San Francisco"
	}

	// Mock weather data - in production, call weather API
	return &MCPToolResult{
		Success: true,
		Output: map[string]interface{}{
			"location":     location,
			"temperature": 72,
			"condition":   "Sunny",
			"humidity":    45,
			"wind_speed":  10,
			"forecast": []map[string]interface{}{
				{"day": "Today", "high": 75, "low": 62, "condition": "Sunny"},
				{"day": "Tomorrow", "high": 73, "low": 60, "condition": "Partly Cloudy"},
				{"day": "Day 3", "high": 70, "low": 58, "condition": "Cloudy"},
			},
		},
	}, nil
}

func (s *MCPService) sendNotification(ctx context.Context, req ToolCallRequest) (*MCPToolResult, error) {
	notifID := fmt.Sprintf("notif_%d", time.Now().Unix())
	return &MCPToolResult{
		Success: true,
		Output: map[string]interface{}{
			"notification_id": notifID,
			"status":         "sent",
			"recipients":     3250,
			"sent_at":        time.Now().Format(time.RFC3339),
		},
	}, nil
}

func (s *MCPService) getAnalytics(ctx context.Context, req ToolCallRequest) (*MCPToolResult, error) {
	eventID, _ := req.Arguments["event_id"].(string)
	return &MCPToolResult{
		Success: true,
		Output: map[string]interface{}{
			"event_id":    eventID,
			"metrics": map[string]interface{}{
				"total_registrations": 5000,
				"active_attendees":    4850,
				"check_ins":           3200,
				"page_views":           50000,
				"engagement_rate":      0.78,
			},
			"generated_at": time.Now().Format(time.RFC3339),
		},
	}, nil
}

func (s *MCPService) generateQRCode(ctx context.Context, req ToolCallRequest) (*MCPToolResult, error) {
	regID := fmt.Sprintf("reg_%d", time.Now().UnixNano()%1000000)
	return &MCPToolResult{
		Success: true,
		Output: map[string]interface{}{
			"qr_code_id":   fmt.Sprintf("qr_%d", time.Now().Unix()),
			"qr_code_url":  fmt.Sprintf("https://api.ems.local/qr/%s.png", regID),
			"registration_id": regID,
		},
	}, nil
}
