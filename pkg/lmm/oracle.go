package lmm

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Oracle manages language model interactions and provides intelligent routing
type Oracle struct {
	mu       sync.RWMutex
	models   map[string]*ModelInstance
	policies map[string]*Policy
	metrics  *Metrics
}

// ModelInstance represents a language model endpoint
type ModelInstance struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Provider    string            `json:"provider"`
	Endpoint    string            `json:"endpoint"`
	Capabilities []string         `json:"capabilities"`
	Config      map[string]any   `json:"config"`
	Status      string           `json:"status"`
	LastUsed    time.Time        `json:"last_used"`
	Metrics     *ModelMetrics    `json:"metrics"`
}

// Policy defines routing and usage policies for models
type Policy struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Rules       []Rule           `json:"rules"`
	Priority    int              `json:"priority"`
	Active      bool             `json:"active"`
}

// Rule defines conditions for model selection
type Rule struct {
	Condition   string           `json:"condition"`
	Action      string           `json:"action"`
	Parameters  map[string]any   `json:"parameters"`
}

// ModelMetrics tracks performance and usage
type ModelMetrics struct {
	RequestCount    int64         `json:"request_count"`
	SuccessRate     float64       `json:"success_rate"`
	AvgLatency      time.Duration `json:"avg_latency"`
	TokensProcessed int64         `json:"tokens_processed"`
	ErrorCount      int64         `json:"error_count"`
}

// Metrics tracks overall system performance
type Metrics struct {
	mu              sync.RWMutex
	TotalRequests   int64                    `json:"total_requests"`
	ActiveModels    int                      `json:"active_models"`
	ModelMetrics    map[string]*ModelMetrics `json:"model_metrics"`
	LastUpdated     time.Time                `json:"last_updated"`
}

// NewOracle creates a new LMM Oracle instance
func NewOracle() *Oracle {
	return &Oracle{
		models:   make(map[string]*ModelInstance),
		policies: make(map[string]*Policy),
		metrics:  &Metrics{
			ModelMetrics: make(map[string]*ModelMetrics),
			LastUpdated:  time.Now(),
		},
	}
}

// RegisterModel adds a new model instance to the oracle
func (o *Oracle) RegisterModel(model *ModelInstance) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if model.ID == "" {
		return fmt.Errorf("model ID cannot be empty")
	}

	model.Status = "active"
	model.LastUsed = time.Now()
	if model.Metrics == nil {
		model.Metrics = &ModelMetrics{}
	}

	o.models[model.ID] = model
	o.metrics.mu.Lock()
	o.metrics.ModelMetrics[model.ID] = model.Metrics
	o.metrics.ActiveModels = len(o.models)
	o.metrics.mu.Unlock()

	return nil
}

// SelectModel chooses the best model for a given request
func (o *Oracle) SelectModel(ctx context.Context, request *ModelRequest) (*ModelInstance, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	// Apply policies to select the best model
	for _, policy := range o.policies {
		if !policy.Active {
			continue
		}

		for _, rule := range policy.Rules {
			if o.evaluateRule(rule, request) {
				if modelID, ok := rule.Parameters["model_id"].(string); ok {
					if model, exists := o.models[modelID]; exists {
						return model, nil
					}
				}
			}
		}
	}

	// Default selection: find the best available model
	var bestModel *ModelInstance
	var bestScore float64

	for _, model := range o.models {
		if model.Status != "active" {
			continue
		}

		score := o.calculateModelScore(model, request)
		if score > bestScore {
			bestScore = score
			bestModel = model
		}
	}

	if bestModel == nil {
		return nil, fmt.Errorf("no suitable model found")
	}

	return bestModel, nil
}

// ModelRequest represents a request for model inference
type ModelRequest struct {
	Type        string         `json:"type"`
	Content     string         `json:"content"`
	Context     map[string]any `json:"context"`
	Requirements []string      `json:"requirements"`
	Priority    int           `json:"priority"`
}

// evaluateRule checks if a rule applies to the given request
func (o *Oracle) evaluateRule(rule Rule, request *ModelRequest) bool {
	switch rule.Condition {
	case "type_equals":
		if reqType, ok := rule.Parameters["type"].(string); ok {
			return request.Type == reqType
		}
	case "has_capability":
		if capability, ok := rule.Parameters["capability"].(string); ok {
			for _, req := range request.Requirements {
				if req == capability {
					return true
				}
			}
		}
	case "priority_gte":
		if minPriority, ok := rule.Parameters["min_priority"].(float64); ok {
			return float64(request.Priority) >= minPriority
		}
	}
	return false
}

// calculateModelScore computes a score for model selection
func (o *Oracle) calculateModelScore(model *ModelInstance, request *ModelRequest) float64 {
	score := 0.0

	// Base score from success rate
	score += model.Metrics.SuccessRate * 50

	// Capability matching
	capabilityScore := 0.0
	for _, req := range request.Requirements {
		for _, cap := range model.Capabilities {
			if req == cap {
				capabilityScore += 10
			}
		}
	}
	score += capabilityScore

	// Latency penalty (lower is better)
	if model.Metrics.AvgLatency > 0 {
		latencyPenalty := float64(model.Metrics.AvgLatency.Milliseconds()) / 1000.0
		score -= latencyPenalty
	}

	// Recent usage bonus
	timeSinceLastUse := time.Since(model.LastUsed)
	if timeSinceLastUse < time.Hour {
		score += 5
	}

	return score
}

// UpdateMetrics updates model performance metrics
func (o *Oracle) UpdateMetrics(modelID string, success bool, latency time.Duration, tokens int64) {
	o.mu.Lock()
	defer o.mu.Unlock()

	model, exists := o.models[modelID]
	if !exists {
		return
	}

	model.Metrics.RequestCount++
	model.Metrics.TokensProcessed += tokens
	model.LastUsed = time.Now()

	if success {
		// Update success rate using exponential moving average
		alpha := 0.1
		model.Metrics.SuccessRate = alpha + (1-alpha)*model.Metrics.SuccessRate
	} else {
		model.Metrics.ErrorCount++
		alpha := 0.1
		model.Metrics.SuccessRate = (1-alpha) * model.Metrics.SuccessRate
	}

	// Update average latency using exponential moving average
	if model.Metrics.AvgLatency == 0 {
		model.Metrics.AvgLatency = latency
	} else {
		alpha := 0.1
		model.Metrics.AvgLatency = time.Duration(float64(latency)*alpha + float64(model.Metrics.AvgLatency)*(1-alpha))
	}

	o.metrics.mu.Lock()
	o.metrics.TotalRequests++
	o.metrics.LastUpdated = time.Now()
	o.metrics.mu.Unlock()
}

// GetMetrics returns current system metrics
func (o *Oracle) GetMetrics() *Metrics {
	o.metrics.mu.RLock()
	defer o.metrics.mu.RUnlock()

	// Create a deep copy to avoid race conditions
	metrics := &Metrics{
		TotalRequests: o.metrics.TotalRequests,
		ActiveModels:  o.metrics.ActiveModels,
		ModelMetrics:  make(map[string]*ModelMetrics),
		LastUpdated:   o.metrics.LastUpdated,
	}

	for id, m := range o.metrics.ModelMetrics {
		metrics.ModelMetrics[id] = &ModelMetrics{
			RequestCount:    m.RequestCount,
			SuccessRate:     m.SuccessRate,
			AvgLatency:      m.AvgLatency,
			TokensProcessed: m.TokensProcessed,
			ErrorCount:      m.ErrorCount,
		}
	}

	return metrics
}

// RegisterTools registers Oracle tools with the MCP server
func (o *Oracle) RegisterTools(s *server.MCPServer) {
	// Register model management tools
	s.AddTool(mcp.NewTool("lmm_register_model",
		mcp.WithDescription("Register a new language model with the Oracle"),
		mcp.WithString("id", mcp.Description("Unique identifier for the model")),
		mcp.WithString("name", mcp.Description("Human-readable name for the model")),
		mcp.WithString("provider", mcp.Description("Model provider (e.g., openai, anthropic)")),
		mcp.WithString("endpoint", mcp.Description("API endpoint URL")),
		mcp.WithArray("capabilities", mcp.Description("List of model capabilities")),
		mcp.WithObject("config", mcp.Description("Model configuration parameters")),
	), o.handleRegisterModel)

	s.AddTool(mcp.NewTool("lmm_select_model",
		mcp.WithDescription("Select the best model for a given request"),
		mcp.WithString("type", mcp.Description("Request type (e.g., chat, completion, embedding)")),
		mcp.WithString("content", mcp.Description("Request content")),
		mcp.WithArray("requirements", mcp.Description("Required capabilities")),
		mcp.WithNumber("priority", mcp.Description("Request priority (1-10)")),
	), o.handleSelectModel)

	s.AddTool(mcp.NewTool("lmm_get_metrics",
		mcp.WithDescription("Get Oracle performance metrics"),
	), o.handleGetMetrics)

	s.AddTool(mcp.NewTool("lmm_list_models",
		mcp.WithDescription("List all registered models"),
	), o.handleListModels)

	s.AddTool(mcp.NewTool("lmm_update_model_status",
		mcp.WithDescription("Update model status"),
		mcp.WithString("model_id", mcp.Description("Model identifier")),
		mcp.WithString("status", mcp.Description("New status (active, inactive, maintenance)")),
	), o.handleUpdateModelStatus)
}

// Tool handlers
func (o *Oracle) handleRegisterModel(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := getStringParam(request, "id")
	if err != nil {
		return mcp.NewToolResultError("INVALID_PARAMS", err.Error()), nil
	}

	name, err := getStringParam(request, "name")
	if err != nil {
		return mcp.NewToolResultError("INVALID_PARAMS", err.Error()), nil
	}

	provider, err := getStringParam(request, "provider")
	if err != nil {
		return mcp.NewToolResultError("INVALID_PARAMS", err.Error()), nil
	}

	endpoint, err := getStringParam(request, "endpoint")
	if err != nil {
		return mcp.NewToolResultError("INVALID_PARAMS", err.Error()), nil
	}

	capabilities, _ := getArrayParam(request, "capabilities")
	config, _ := getObjectParam(request, "config")

	model := &ModelInstance{
		ID:           id,
		Name:         name,
		Provider:     provider,
		Endpoint:     endpoint,
		Capabilities: capabilities,
		Config:       config,
	}

	if err := o.RegisterModel(model); err != nil {
		return mcp.NewToolResultError("REGISTRATION_FAILED", err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Model %s registered successfully", id)), nil
}

func (o *Oracle) handleSelectModel(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	reqType, _ := getStringParam(request, "type")
	content, _ := getStringParam(request, "content")
	requirements, _ := getArrayParam(request, "requirements")
	priority, _ := getNumberParam(request, "priority")

	modelRequest := &ModelRequest{
		Type:         reqType,
		Content:      content,
		Requirements: requirements,
		Priority:     int(priority),
	}

	model, err := o.SelectModel(ctx, modelRequest)
	if err != nil {
		return mcp.NewToolResultError("SELECTION_FAILED", err.Error()), nil
	}

	data, _ := json.Marshal(model)
	return mcp.NewToolResultText(string(data)), nil
}

func (o *Oracle) handleGetMetrics(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	metrics := o.GetMetrics()
	data, _ := json.Marshal(metrics)
	return mcp.NewToolResultText(string(data)), nil
}

func (o *Oracle) handleListModels(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	models := make([]*ModelInstance, 0, len(o.models))
	for _, model := range o.models {
		models = append(models, model)
	}

	data, _ := json.Marshal(models)
	return mcp.NewToolResultText(string(data)), nil
}

func (o *Oracle) handleUpdateModelStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelID, err := getStringParam(request, "model_id")
	if err != nil {
		return mcp.NewToolResultError("INVALID_PARAMS", err.Error()), nil
	}

	status, err := getStringParam(request, "status")
	if err != nil {
		return mcp.NewToolResultError("INVALID_PARAMS", err.Error()), nil
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	model, exists := o.models[modelID]
	if !exists {
		return mcp.NewToolResultError("MODEL_NOT_FOUND", "Model not found"), nil
	}

	model.Status = status
	return mcp.NewToolResultText(fmt.Sprintf("Model %s status updated to %s", modelID, status)), nil
}

// Helper functions for parameter extraction
func getStringParam(request mcp.CallToolRequest, name string) (string, error) {
	if val, ok := request.GetArguments()[name]; ok {
		if str, ok := val.(string); ok {
			return str, nil
		}
		return "", fmt.Errorf("parameter %s is not a string", name)
	}
	return "", fmt.Errorf("parameter %s is required", name)
}

func getNumberParam(request mcp.CallToolRequest, name string) (float64, error) {
	if val, ok := request.GetArguments()[name]; ok {
		if num, ok := val.(float64); ok {
			return num, nil
		}
		return 0, fmt.Errorf("parameter %s is not a number", name)
	}
	return 0, nil
}

func getArrayParam(request mcp.CallToolRequest, name string) ([]string, error) {
	if val, ok := request.GetArguments()[name]; ok {
		if arr, ok := val.([]interface{}); ok {
			result := make([]string, len(arr))
			for i, v := range arr {
				if str, ok := v.(string); ok {
					result[i] = str
				} else {
					return nil, fmt.Errorf("array element %d is not a string", i)
				}
			}
			return result, nil
		}
		return nil, fmt.Errorf("parameter %s is not an array", name)
	}
	return []string{}, nil
}

func getObjectParam(request mcp.CallToolRequest, name string) (map[string]any, error) {
	if val, ok := request.GetArguments()[name]; ok {
		if obj, ok := val.(map[string]interface{}); ok {
			return obj, nil
		}
		return nil, fmt.Errorf("parameter %s is not an object", name)
	}
	return map[string]any{}, nil
}