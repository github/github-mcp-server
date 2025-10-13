# LMM (Language Model Manager) Oracle and MPC System

A comprehensive system for managing language models and performing secure multi-party computations within the GitHub MCP Server ecosystem.

## Overview

The LMM system provides two main components:

1. **Oracle**: Intelligent language model selection, routing, and management
2. **MPC (Multi-Party Computation)**: Secure collaborative processing for sensitive data

## Features

### Oracle Features
- **Model Registration**: Register and manage multiple language model endpoints
- **Intelligent Routing**: Automatically select the best model for each request
- **Performance Monitoring**: Track model performance, latency, and success rates
- **Policy-Based Selection**: Define custom rules for model selection
- **Load Balancing**: Distribute requests across available models

### MPC Features
- **Secure Computation**: Perform computations on encrypted data
- **Secret Sharing**: Split sensitive data across multiple parties
- **Protocol Management**: Support for various MPC protocols
- **Privacy Preservation**: Maintain data privacy during collaborative processing
- **Verification**: Cryptographic verification of computation results

## Architecture

```
┌─────────────────┐    ┌─────────────────┐
│   LMM System    │    │   MCP Server    │
│                 │    │                 │
│  ┌───────────┐  │    │  ┌───────────┐  │
│  │  Oracle   │  │◄───┤  │   Tools   │  │
│  └───────────┘  │    │  └───────────┘  │
│                 │    │                 │
│  ┌───────────┐  │    │  ┌───────────┐  │
│  │    MPC    │  │◄───┤  │Resources  │  │
│  └───────────┘  │    │  └───────────┘  │
└─────────────────┘    └─────────────────┘
```

## Usage

### Starting the LMM Server

```bash
# Build the server
go build -o lmm-server ./cmd/lmm-server

# Start with both Oracle and MPC enabled
./lmm-server stdio --enable-oracle --enable-mpc

# Start with only Oracle
./lmm-server stdio --enable-oracle --enable-mpc=false

# Start with custom configuration
./lmm-server stdio \
  --enable-oracle \
  --enable-mpc \
  --security-level=high \
  --max-concurrent=20 \
  --default-timeout=600
```

### Oracle Operations

#### Register a Model
```json
{
  "method": "tools/call",
  "params": {
    "name": "lmm_register_model",
    "arguments": {
      "id": "gpt-4-turbo",
      "name": "GPT-4 Turbo",
      "provider": "openai",
      "endpoint": "https://api.openai.com/v1/chat/completions",
      "capabilities": ["chat", "completion", "reasoning", "code"],
      "config": {
        "max_tokens": 4096,
        "temperature": 0.7
      }
    }
  }
}
```

#### Select Best Model
```json
{
  "method": "tools/call",
  "params": {
    "name": "lmm_select_model",
    "arguments": {
      "type": "chat",
      "content": "Explain quantum computing",
      "requirements": ["reasoning", "technical"],
      "priority": 8
    }
  }
}
```

#### Get System Metrics
```json
{
  "method": "tools/call",
  "params": {
    "name": "lmm_get_metrics",
    "arguments": {}
  }
}
```

### MPC Operations

#### Register a Party
```json
{
  "method": "tools/call",
  "params": {
    "name": "mpc_register_party",
    "arguments": {
      "id": "party-1",
      "name": "Research Institution A",
      "public_key": "-----BEGIN PUBLIC KEY-----...",
      "endpoint": "https://party1.example.com/mpc",
      "capabilities": ["computation", "verification"]
    }
  }
}
```

#### Create MPC Session
```json
{
  "method": "tools/call",
  "params": {
    "name": "mpc_create_session",
    "arguments": {
      "protocol": "secure_aggregation",
      "parties": ["party-1", "party-2", "party-3"],
      "threshold": 2,
      "timeout": 300
    }
  }
}
```

#### Execute MPC Protocol
```json
{
  "method": "tools/call",
  "params": {
    "name": "mpc_execute_protocol",
    "arguments": {
      "session_id": "session-abc123",
      "input": {
        "data": [1, 2, 3, 4, 5],
        "operation": "sum"
      }
    }
  }
}
```

### Integrated Workflows

#### Execute Complex Workflow
```json
{
  "method": "tools/call",
  "params": {
    "name": "lmm_execute_workflow",
    "arguments": {
      "id": "secure-analysis-workflow",
      "type": "secure_data_analysis",
      "steps": [
        {
          "id": "model_selection",
          "type": "model_selection",
          "parameters": {
            "type": "analysis",
            "requirements": ["privacy", "accuracy"]
          }
        },
        {
          "id": "secure_computation",
          "type": "mpc_computation",
          "parameters": {
            "protocol": "private_inference",
            "parties": ["party-1", "party-2"]
          }
        }
      ],
      "priority": 9,
      "timeout": 600
    }
  }
}
```

## Configuration

### System Configuration
```go
config := &lmm.SystemConfig{
    EnableOracle:   true,
    EnableMPC:      true,
    DefaultTimeout: 300 * time.Second,
    MaxConcurrent:  10,
    SecurityLevel:  "standard",
    LogLevel:       "info",
}
```

### Model Configuration
```go
model := &lmm.ModelInstance{
    ID:       "custom-model",
    Name:     "Custom Model",
    Provider: "custom",
    Endpoint: "https://api.custom.com/v1/completions",
    Capabilities: []string{"completion", "embedding"},
    Config: map[string]any{
        "api_key":     "your-api-key",
        "max_tokens":  2048,
        "temperature": 0.8,
    },
}
```

### MPC Protocol Configuration
```go
protocol := &lmm.Protocol{
    ID:         "custom_protocol",
    Name:       "Custom Secure Protocol",
    Type:       "computation",
    MinParties: 3,
    MaxParties: 10,
    Steps: []lmm.ProtocolStep{
        {
            ID:       "step1",
            Type:     "secret_sharing",
            Function: "shamir_share",
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
```

## Security Considerations

### Oracle Security
- Model endpoint authentication
- Request/response encryption
- Rate limiting and abuse prevention
- Audit logging of all operations

### MPC Security
- Cryptographic protocols for secure computation
- Zero-knowledge proofs for verification
- Secure key management and distribution
- Protection against malicious parties

## Performance Optimization

### Oracle Optimization
- Model performance caching
- Intelligent load balancing
- Predictive model selection
- Resource usage monitoring

### MPC Optimization
- Protocol-specific optimizations
- Parallel computation where possible
- Efficient secret sharing schemes
- Network communication optimization

## Integration with GitHub MCP Server

The LMM system integrates seamlessly with the existing GitHub MCP Server:

1. **Tool Registration**: All LMM tools are registered as MCP tools
2. **Resource Management**: Leverages MCP resource capabilities
3. **Error Handling**: Uses MCP error handling patterns
4. **Logging**: Integrates with MCP logging system

## Development

### Building
```bash
# Build the LMM server
go build -o lmm-server ./cmd/lmm-server

# Build with the main GitHub MCP server
go build -o github-mcp-server ./cmd/github-mcp-server
```

### Testing
```bash
# Run tests
go test ./pkg/lmm/...

# Run with coverage
go test -cover ./pkg/lmm/...
```

### Adding New Models
1. Implement model-specific client
2. Register with Oracle using `RegisterModel`
3. Configure capabilities and endpoints
4. Test model selection and routing

### Adding New MPC Protocols
1. Define protocol steps and security requirements
2. Implement protocol-specific computation logic
3. Register with MPC system using `RegisterProtocol`
4. Test with multiple parties

## Examples

See the `examples/` directory for complete usage examples:
- `oracle_basic.go`: Basic Oracle usage
- `mpc_secure_sum.go`: Secure sum computation
- `integrated_workflow.go`: Complex workflow example

## License

This project is licensed under the same license as the GitHub MCP Server.