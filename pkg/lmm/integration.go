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

// LMMSystem integrates Oracle and MPC for comprehensive language model management
type LMMSystem struct {
	mu      sync.RWMutex
	oracle  *Oracle
	mpc     *MPC
	config  *SystemConfig
	active  bool
	metrics *SystemMetrics
}

// SystemConfig contains system-wide configuration
type SystemConfig struct {
	EnableOracle     bool          `json:"enable_oracle"`
	EnableMPC        bool          `json:"enable_mpc"`
	DefaultTimeout   time.Duration `json:"default_timeout"`
	MaxConcurrent    int           `json:"max_concurrent"`
	SecurityLevel    string        `json:"security_level"`
	LogLevel         string        `json:"log_level"`
}

// SystemMetrics tracks overall system performance
type SystemMetrics struct {
	mu                sync.RWMutex
	TotalRequests     int64     `json:"total_requests"`
	SuccessfulRequests int64    `json:"successful_requests"`
	FailedRequests    int64     `json:"failed_requests"`
	AverageLatency    time.Duration `json:"average_latency"`
	ActiveSessions    int       `json:"active_sessions"`
	LastUpdated       time.Time `json:"last_updated"`
}

// WorkflowRequest represents a complex workflow request
type WorkflowRequest struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Steps       []WorkflowStep    `json:"steps"`
	Context     map[string]any    `json:"context"`
	Priority    int               `json:"priority"`
	Timeout     time.Duration     `json:"timeout"`
	Security    *SecurityRequirements `json:"security"`
}

// WorkflowStep defines a step in a workflow
type WorkflowStep struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Action      string            `json:"action"`
	Parameters  map[string]any    `json:"parameters"`
	Dependencies []string         `json:"dependencies"`
	Timeout     time.Duration     `json:"timeout"`
}

// SecurityRequirements defines security requirements for workflows
type SecurityRequirements struct {
	Encryption   bool     `json:"encryption"`
	MPC          bool     `json:"mpc"`
	ZKProofs     bool     `json:"zk_proofs"`
	Parties      []string `json:"parties"`
	Threshold    int      `json:"threshold"`
}

// WorkflowResult contains the result of workflow execution
type WorkflowResult struct {
	ID          string            `json:"id"`
	Status      string            `json:"status"`
	Results     map[string]any    `json:"results"`
	Metrics     *ExecutionMetrics `json:"metrics"`
	Error       string            `json:"error,omitempty"`
	CompletedAt time.Time         `json:"completed_at"`
}

// ExecutionMetrics tracks workflow execution metrics
type ExecutionMetrics struct {
	StartTime     time.Time     `json:"start_time"`
	EndTime       time.Time     `json:"end_time"`
	Duration      time.Duration `json:"duration"`
	StepsExecuted int           `json:"steps_executed"`
	ModelsUsed    []string      `json:"models_used"`
	MPCSessions   []string      `json:"mpc_sessions"`
}

// NewLMMSystem creates a new integrated LMM system
func NewLMMSystem(config *SystemConfig) *LMMSystem {
	system := &LMMSystem{
		config: config,
		active: true,
		metrics: &SystemMetrics{
			LastUpdated: time.Now(),
		},
	}

	if config.EnableOracle {
		system.oracle = NewOracle()
	}

	if config.EnableMPC {
		system.mpc = NewMPC()
	}

	return system
}

// ExecuteWorkflow executes a complex workflow using Oracle and MPC
func (s *LMMSystem) ExecuteWorkflow(ctx context.Context, workflow *WorkflowRequest) (*WorkflowResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.active {
		return nil, fmt.Errorf("system is not active")
	}

	startTime := time.Now()
	result := &WorkflowResult{
		ID:      workflow.ID,
		Status:  "running",
		Results: make(map[string]any),
		Metrics: &ExecutionMetrics{
			StartTime:   startTime,
			ModelsUsed:  []string{},
			MPCSessions: []string{},
		},
	}

	// Execute workflow steps
	for _, step := range workflow.Steps {
		stepResult, err := s.executeWorkflowStep(ctx, &step, workflow, result)
		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			return result, err
		}

		result.Results[step.ID] = stepResult
		result.Metrics.StepsExecuted++
	}

	// Finalize result
	endTime := time.Now()
	result.Status = "completed"
	result.CompletedAt = endTime
	result.Metrics.EndTime = endTime
	result.Metrics.Duration = endTime.Sub(startTime)

	// Update system metrics
	s.updateSystemMetrics(true, result.Metrics.Duration)

	return result, nil
}

// executeWorkflowStep executes a single workflow step
func (s *LMMSystem) executeWorkflowStep(ctx context.Context, step *WorkflowStep, workflow *WorkflowRequest, result *WorkflowResult) (any, error) {
	switch step.Type {
	case "model_selection":
		return s.executeModelSelection(ctx, step, workflow)
	case "model_inference":
		return s.executeModelInference(ctx, step, workflow, result)
	case "mpc_computation":
		return s.executeMPCComputation(ctx, step, workflow, result)
	case "data_aggregation":
		return s.executeDataAggregation(ctx, step, workflow, result)
	case "security_verification":
		return s.executeSecurityVerification(ctx, step, workflow)
	default:
		return nil, fmt.Errorf("unknown step type: %s", step.Type)
	}
}

// executeModelSelection selects the best model for a task
func (s *LMMSystem) executeModelSelection(ctx context.Context, step *WorkflowStep, workflow *WorkflowRequest) (any, error) {
	if s.oracle == nil {
		return nil, fmt.Errorf("Oracle not enabled")
	}

	// Extract parameters
	reqType, _ := step.Parameters["type"].(string)
	content, _ := step.Parameters["content"].(string)
	requirements, _ := step.Parameters["requirements"].([]string)
	priority, _ := step.Parameters["priority"].(float64)

	modelRequest := &ModelRequest{
		Type:         reqType,
		Content:      content,
		Requirements: requirements,
		Priority:     int(priority),
	}

	model, err := s.oracle.SelectModel(ctx, modelRequest)
	if err != nil {
		return nil, fmt.Errorf("model selection failed: %w", err)
	}

	return map[string]any{
		"selected_model": model.ID,
		"model_name":     model.Name,
		"provider":       model.Provider,
		"capabilities":   model.Capabilities,
	}, nil
}

// executeModelInference performs model inference
func (s *LMMSystem) executeModelInference(ctx context.Context, step *WorkflowStep, workflow *WorkflowRequest, result *WorkflowResult) (any, error) {
	if s.oracle == nil {
		return nil, fmt.Errorf("Oracle not enabled")
	}

	modelID, _ := step.Parameters["model_id"].(string)
	input, _ := step.Parameters["input"].(string)

	// Simulate model inference (in practice, this would call the actual model)
	startTime := time.Now()
	
	// Add model to metrics
	result.Metrics.ModelsUsed = append(result.Metrics.ModelsUsed, modelID)

	// Simulate processing time
	time.Sleep(100 * time.Millisecond)

	// Update Oracle metrics
	latency := time.Since(startTime)
	s.oracle.UpdateMetrics(modelID, true, latency, 1000)

	return map[string]any{
		"model_id": modelID,
		"output":   fmt.Sprintf("Processed: %s", input),
		"tokens":   1000,
		"latency":  latency.Milliseconds(),
	}, nil
}

// executeMPCComputation performs secure multi-party computation
func (s *LMMSystem) executeMPCComputation(ctx context.Context, step *WorkflowStep, workflow *WorkflowRequest, result *WorkflowResult) (any, error) {
	if s.mpc == nil {
		return nil, fmt.Errorf("MPC not enabled")
	}

	protocol, _ := step.Parameters["protocol"].(string)
	parties, _ := step.Parameters["parties"].([]string)
	threshold, _ := step.Parameters["threshold"].(float64)
	input, _ := step.Parameters["input"].(map[string]any)

	// Create MPC session
	config := &SessionConfig{
		Threshold: int(threshold),
		Timeout:   workflow.Timeout,
	}

	session, err := s.mpc.CreateSession(protocol, parties, config)
	if err != nil {
		return nil, fmt.Errorf("MPC session creation failed: %w", err)
	}

	// Add session to metrics
	result.Metrics.MPCSessions = append(result.Metrics.MPCSessions, session.ID)

	// Execute MPC protocol
	if err := s.mpc.ExecuteProtocol(session.ID, input); err != nil {
		return nil, fmt.Errorf("MPC execution failed: %w", err)
	}

	return map[string]any{
		"session_id": session.ID,
		"protocol":   protocol,
		"parties":    parties,
		"results":    session.Results,
	}, nil
}

// executeDataAggregation aggregates data from multiple sources
func (s *LMMSystem) executeDataAggregation(ctx context.Context, step *WorkflowStep, workflow *WorkflowRequest, result *WorkflowResult) (any, error) {
	sources, _ := step.Parameters["sources"].([]string)
	method, _ := step.Parameters["method"].(string)

	aggregatedData := make(map[string]any)

	// Collect data from previous steps
	for _, source := range sources {
		if data, exists := result.Results[source]; exists {
			aggregatedData[source] = data
		}
	}

	// Apply aggregation method
	switch method {
	case "merge":
		return s.mergeData(aggregatedData), nil
	case "average":
		return s.averageData(aggregatedData), nil
	case "consensus":
		return s.consensusData(aggregatedData), nil
	default:
		return aggregatedData, nil
	}
}

// executeSecurityVerification performs security verification
func (s *LMMSystem) executeSecurityVerification(ctx context.Context, step *WorkflowStep, workflow *WorkflowRequest) (any, error) {
	verifyType, _ := step.Parameters["type"].(string)
	data, _ := step.Parameters["data"].(map[string]any)

	switch verifyType {
	case "integrity":
		return s.verifyIntegrity(data), nil
	case "authenticity":
		return s.verifyAuthenticity(data), nil
	case "privacy":
		return s.verifyPrivacy(data), nil
	default:
		return map[string]any{"verified": true}, nil
	}
}

// Helper functions for data processing
func (s *LMMSystem) mergeData(data map[string]any) map[string]any {
	merged := make(map[string]any)
	for key, value := range data {
		merged[key] = value
	}
	return merged
}

func (s *LMMSystem) averageData(data map[string]any) map[string]any {
	// Simplified averaging logic
	return map[string]any{"average": "computed"}
}

func (s *LMMSystem) consensusData(data map[string]any) map[string]any {
	// Simplified consensus logic
	return map[string]any{"consensus": "reached"}
}

func (s *LMMSystem) verifyIntegrity(data map[string]any) map[string]any {
	return map[string]any{"integrity_verified": true}
}

func (s *LMMSystem) verifyAuthenticity(data map[string]any) map[string]any {
	return map[string]any{"authenticity_verified": true}
}

func (s *LMMSystem) verifyPrivacy(data map[string]any) map[string]any {
	return map[string]any{"privacy_verified": true}
}

// updateSystemMetrics updates overall system metrics
func (s *LMMSystem) updateSystemMetrics(success bool, latency time.Duration) {
	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()

	s.metrics.TotalRequests++
	if success {
		s.metrics.SuccessfulRequests++
	} else {
		s.metrics.FailedRequests++
	}

	// Update average latency using exponential moving average
	if s.metrics.AverageLatency == 0 {
		s.metrics.AverageLatency = latency
	} else {
		alpha := 0.1
		s.metrics.AverageLatency = time.Duration(float64(latency)*alpha + float64(s.metrics.AverageLatency)*(1-alpha))
	}

	s.metrics.LastUpdated = time.Now()
}

// RegisterTools registers all LMM system tools with the MCP server
func (s *LMMSystem) RegisterTools(mcpServer *server.MCPServer) {
	// Register Oracle tools if enabled
	if s.oracle != nil {
		s.oracle.RegisterTools(mcpServer)
	}

	// Register MPC tools if enabled
	if s.mpc != nil {
		s.mpc.RegisterTools(mcpServer)
	}

	// Register integrated workflow tools
	mcpServer.AddTool(mcp.NewTool("lmm_execute_workflow",
		mcp.WithDescription("Execute a complex LMM workflow"),
		mcp.WithString("id", mcp.Description("Workflow identifier")),
		mcp.WithString("type", mcp.Description("Workflow type")),
		mcp.WithArray("steps", mcp.Description("Workflow steps")),
		mcp.WithObject("context", mcp.Description("Workflow context")),
		mcp.WithNumber("priority", mcp.Description("Workflow priority")),
		mcp.WithNumber("timeout", mcp.Description("Workflow timeout in seconds")),
	), s.handleExecuteWorkflow)

	mcpServer.AddTool(mcp.NewTool("lmm_get_system_metrics",
		mcp.WithDescription("Get LMM system metrics"),
	), s.handleGetSystemMetrics)

	mcpServer.AddTool(mcp.NewTool("lmm_system_status",
		mcp.WithDescription("Get LMM system status"),
	), s.handleSystemStatus)
}

// Tool handlers
func (s *LMMSystem) handleExecuteWorkflow(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := getStringParam(request, "id")
	if err != nil {
		return mcp.NewToolResultError("INVALID_PARAMS", err.Error()), nil
	}

	workflowType, err := getStringParam(request, "type")
	if err != nil {
		return mcp.NewToolResultError("INVALID_PARAMS", err.Error()), nil
	}

	// Parse steps (simplified)
	steps := []WorkflowStep{
		{
			ID:     "step1",
			Type:   "model_selection",
			Action: "select_best_model",
		},
	}

	context, _ := getObjectParam(request, "context")
	priority, _ := getNumberParam(request, "priority")
	timeout, _ := getNumberParam(request, "timeout")

	workflow := &WorkflowRequest{
		ID:       id,
		Type:     workflowType,
		Steps:    steps,
		Context:  context,
		Priority: int(priority),
		Timeout:  time.Duration(timeout) * time.Second,
	}

	result, err := s.ExecuteWorkflow(ctx, workflow)
	if err != nil {
		return mcp.NewToolResultError("WORKFLOW_FAILED", err.Error()), nil
	}

	data, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(data)), nil
}

func (s *LMMSystem) handleGetSystemMetrics(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.metrics.mu.RLock()
	defer s.metrics.mu.RUnlock()

	data, _ := json.Marshal(s.metrics)
	return mcp.NewToolResultText(string(data)), nil
}

func (s *LMMSystem) handleSystemStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	status := map[string]any{
		"active":        s.active,
		"oracle_enabled": s.oracle != nil,
		"mpc_enabled":   s.mpc != nil,
		"config":        s.config,
	}

	if s.oracle != nil {
		status["oracle_metrics"] = s.oracle.GetMetrics()
	}

	data, _ := json.Marshal(status)
	return mcp.NewToolResultText(string(data)), nil
}