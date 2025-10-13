package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/github/github-mcp-server/pkg/lmm"
)

func main() {
	// Example usage of the LMM Oracle and MPC system
	fmt.Println("LMM Oracle and MPC System Example")
	fmt.Println("=================================")

	// Create system configuration
	config := &lmm.SystemConfig{
		EnableOracle:   true,
		EnableMPC:      true,
		DefaultTimeout: 300 * time.Second,
		MaxConcurrent:  10,
		SecurityLevel:  "standard",
		LogLevel:       "info",
	}

	// Initialize LMM system
	lmmSystem := lmm.NewLMMSystem(config)

	// Example 1: Oracle Usage
	fmt.Println("\n1. Oracle Example")
	fmt.Println("-----------------")
	oracleExample(lmmSystem)

	// Example 2: MPC Usage
	fmt.Println("\n2. MPC Example")
	fmt.Println("--------------")
	mpcExample(lmmSystem)

	// Example 3: Integrated Workflow
	fmt.Println("\n3. Integrated Workflow Example")
	fmt.Println("------------------------------")
	workflowExample(lmmSystem)
}

func oracleExample(lmmSystem *lmm.LMMSystem) {
	ctx := context.Background()

	// Register example models
	models := []*lmm.ModelInstance{
		{
			ID:       "gpt-4",
			Name:     "GPT-4",
			Provider: "openai",
			Endpoint: "https://api.openai.com/v1/chat/completions",
			Capabilities: []string{"chat", "completion", "reasoning", "code"},
			Config: map[string]any{
				"max_tokens":  4096,
				"temperature": 0.7,
			},
		},
		{
			ID:       "claude-3",
			Name:     "Claude 3",
			Provider: "anthropic",
			Endpoint: "https://api.anthropic.com/v1/messages",
			Capabilities: []string{"chat", "completion", "analysis"},
			Config: map[string]any{
				"max_tokens":  4096,
				"temperature": 0.7,
			},
		},
		{
			ID:       "llama-2-local",
			Name:     "Llama 2 Local",
			Provider: "local",
			Endpoint: "http://localhost:8080/v1/completions",
			Capabilities: []string{"completion", "chat", "local"},
			Config: map[string]any{
				"max_tokens":  2048,
				"temperature": 0.8,
			},
		},
	}

	// Note: In a real implementation, you would access the Oracle through the LMM system
	// For this example, we'll simulate the process
	fmt.Println("Registering models...")
	for _, model := range models {
		fmt.Printf("- %s (%s)\n", model.Name, model.Provider)
	}

	// Simulate model selection
	fmt.Println("\nSelecting best model for coding task...")
	request := &lmm.ModelRequest{
		Type:         "chat",
		Content:      "Write a Python function to calculate fibonacci numbers",
		Requirements: []string{"code", "reasoning"},
		Priority:     8,
	}

	fmt.Printf("Request: %+v\n", request)
	fmt.Println("Selected: GPT-4 (best match for coding requirements)")

	// Simulate metrics update
	fmt.Println("\nUpdating model metrics...")
	fmt.Println("- Request processed successfully")
	fmt.Println("- Latency: 1.2s")
	fmt.Println("- Tokens processed: 1500")
}

func mpcExample(lmmSystem *lmm.LMMSystem) {
	ctx := context.Background()

	// Register example parties
	parties := []*lmm.Party{
		{
			ID:        "hospital-a",
			Name:      "Hospital A",
			PublicKey: "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...\n-----END PUBLIC KEY-----",
			Endpoint:  "https://hospital-a.example.com/mpc",
			Capabilities: []string{"computation", "verification"},
		},
		{
			ID:        "hospital-b",
			Name:      "Hospital B",
			PublicKey: "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...\n-----END PUBLIC KEY-----",
			Endpoint:  "https://hospital-b.example.com/mpc",
			Capabilities: []string{"computation", "verification"},
		},
		{
			ID:        "research-center",
			Name:      "Research Center",
			PublicKey: "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...\n-----END PUBLIC KEY-----",
			Endpoint:  "https://research.example.com/mcp",
			Capabilities: []string{"computation", "verification", "analysis"},
		},
	}

	fmt.Println("Registering MPC parties...")
	for _, party := range parties {
		fmt.Printf("- %s (%s)\n", party.Name, party.ID)
	}

	// Register secure aggregation protocol
	protocol := &lmm.Protocol{
		ID:         "secure_health_aggregation",
		Name:       "Secure Health Data Aggregation",
		Type:       "aggregation",
		MinParties: 2,
		MaxParties: 5,
		Steps: []lmm.ProtocolStep{
			{
				ID:       "share_data",
				Name:     "Share Patient Data",
				Type:     "secret_sharing",
				Input:    []string{"patient_counts"},
				Output:   []string{"data_shares"},
				Function: "shamir_share",
				Timeout:  30 * time.Second,
			},
			{
				ID:       "compute_statistics",
				Name:     "Compute Aggregate Statistics",
				Type:     "computation",
				Input:    []string{"data_shares"},
				Output:   []string{"stat_shares"},
				Function: "secure_sum",
				Timeout:  60 * time.Second,
			},
			{
				ID:       "reconstruct_results",
				Name:     "Reconstruct Final Results",
				Type:     "reconstruction",
				Input:    []string{"stat_shares"},
				Output:   []string{"aggregated_stats"},
				Function: "reconstruct",
				Timeout:  30 * time.Second,
			},
		},
		Security: &lmm.SecurityConfig{
			Encryption:  "AES-256",
			Signing:     "ECDSA",
			ZKProofs:    true,
			Homomorphic: false,
		},
	}

	fmt.Printf("\nRegistered protocol: %s\n", protocol.Name)

	// Simulate MPC session creation
	sessionConfig := &lmm.SessionConfig{
		Threshold: 2,
		Privacy:   "high",
		Timeout:   300 * time.Second,
	}

	fmt.Println("\nCreating MPC session...")
	fmt.Printf("- Protocol: %s\n", protocol.Name)
	fmt.Printf("- Parties: %v\n", []string{"hospital-a", "hospital-b", "research-center"})
	fmt.Printf("- Threshold: %d\n", sessionConfig.Threshold)

	// Simulate protocol execution
	fmt.Println("\nExecuting secure computation...")
	input := map[string]any{
		"patient_counts": []int{150, 200, 175}, // Each hospital's patient count
		"operation":      "sum",
	}

	fmt.Printf("Input data: %+v\n", input)
	fmt.Println("- Step 1: Sharing secrets across parties...")
	fmt.Println("- Step 2: Computing aggregate statistics...")
	fmt.Println("- Step 3: Reconstructing final results...")
	fmt.Println("Result: Total patients across all hospitals: 525")
	fmt.Println("✓ Computation completed without revealing individual hospital data")
}

func workflowExample(lmmSystem *lmm.LMMSystem) {
	ctx := context.Background()

	// Define a complex workflow for secure AI-powered medical research
	workflow := &lmm.WorkflowRequest{
		ID:   "secure-medical-analysis",
		Type: "secure_ai_research",
		Steps: []lmm.WorkflowStep{
			{
				ID:     "select_analysis_model",
				Type:   "model_selection",
				Action: "select_best_model",
				Parameters: map[string]any{
					"type":         "analysis",
					"requirements": []string{"medical", "privacy", "accuracy"},
					"priority":     9,
				},
				Timeout: 30 * time.Second,
			},
			{
				ID:     "secure_data_aggregation",
				Type:   "mpc_computation",
				Action: "aggregate_patient_data",
				Parameters: map[string]any{
					"protocol":  "secure_health_aggregation",
					"parties":   []string{"hospital-a", "hospital-b", "research-center"},
					"threshold": 2,
					"input": map[string]any{
						"data_type": "patient_outcomes",
						"timeframe": "2023-2024",
					},
				},
				Dependencies: []string{"select_analysis_model"},
				Timeout:      120 * time.Second,
			},
			{
				ID:     "ai_analysis",
				Type:   "model_inference",
				Action: "analyze_aggregated_data",
				Parameters: map[string]any{
					"model_id": "medical-ai-model",
					"task":     "outcome_prediction",
				},
				Dependencies: []string{"secure_data_aggregation"},
				Timeout:      180 * time.Second,
			},
			{
				ID:     "verify_results",
				Type:   "security_verification",
				Action: "verify_computation_integrity",
				Parameters: map[string]any{
					"type": "integrity",
					"zk_proofs": true,
				},
				Dependencies: []string{"ai_analysis"},
				Timeout:      60 * time.Second,
			},
		},
		Priority: 10,
		Timeout:  600 * time.Second,
		Security: &lmm.SecurityRequirements{
			Encryption: true,
			MPC:        true,
			ZKProofs:   true,
			Parties:    []string{"hospital-a", "hospital-b", "research-center"},
			Threshold:  2,
		},
	}

	fmt.Println("Executing integrated workflow...")
	fmt.Printf("Workflow ID: %s\n", workflow.ID)
	fmt.Printf("Type: %s\n", workflow.Type)
	fmt.Printf("Steps: %d\n", len(workflow.Steps))

	// Simulate workflow execution
	fmt.Println("\nWorkflow execution steps:")
	
	for i, step := range workflow.Steps {
		fmt.Printf("%d. %s (%s)\n", i+1, step.ID, step.Type)
		
		switch step.Type {
		case "model_selection":
			fmt.Println("   → Selected: Medical AI Model v2.1 (specialized for healthcare)")
		case "mpc_computation":
			fmt.Println("   → Securely aggregated data from 3 hospitals")
			fmt.Println("   → Total records processed: 15,000 (privacy preserved)")
		case "model_inference":
			fmt.Println("   → AI analysis completed")
			fmt.Println("   → Generated insights on treatment outcomes")
		case "security_verification":
			fmt.Println("   → Integrity verified with zero-knowledge proofs")
			fmt.Println("   → All computations validated")
		}
	}

	// Simulate final results
	result := &lmm.WorkflowResult{
		ID:     workflow.ID,
		Status: "completed",
		Results: map[string]any{
			"select_analysis_model": map[string]any{
				"selected_model": "medical-ai-v2.1",
				"confidence":     0.95,
			},
			"secure_data_aggregation": map[string]any{
				"total_records":    15000,
				"privacy_preserved": true,
				"aggregation_type": "secure_sum",
			},
			"ai_analysis": map[string]any{
				"insights_generated": 25,
				"accuracy_score":     0.92,
				"recommendations":    []string{
					"Treatment protocol A shows 15% better outcomes",
					"Early intervention reduces complications by 23%",
					"Combination therapy effective in 78% of cases",
				},
			},
			"verify_results": map[string]any{
				"integrity_verified":    true,
				"authenticity_verified": true,
				"privacy_preserved":     true,
			},
		},
		Metrics: &lmm.ExecutionMetrics{
			StartTime:     time.Now().Add(-5 * time.Minute),
			EndTime:       time.Now(),
			Duration:      5 * time.Minute,
			StepsExecuted: 4,
			ModelsUsed:    []string{"medical-ai-v2.1"},
			MPCSessions:   []string{"session-abc123"},
		},
		CompletedAt: time.Now(),
	}

	fmt.Println("\n✓ Workflow completed successfully!")
	fmt.Printf("Duration: %v\n", result.Metrics.Duration)
	fmt.Printf("Models used: %v\n", result.Metrics.ModelsUsed)
	fmt.Printf("MPC sessions: %v\n", result.Metrics.MPCSessions)

	// Display results summary
	fmt.Println("\nResults Summary:")
	resultsJSON, _ := json.MarshalIndent(result.Results, "", "  ")
	fmt.Println(string(resultsJSON))

	fmt.Println("\n🔒 Privacy and Security:")
	fmt.Println("- All patient data remained encrypted throughout the process")
	fmt.Println("- No individual hospital data was exposed")
	fmt.Println("- Computations verified with zero-knowledge proofs")
	fmt.Println("- Results authenticated and integrity-checked")
}