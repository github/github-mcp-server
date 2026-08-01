# Sequential Thinking MCP Server

This example shows a Model Context Protocol (MCP) server that enables dynamic and reflective problem-solving through structured thinking processes. It helps break down complex problems into manageable, sequential thought steps with support for revision and branching.

## Features

The server provides three main tools for managing thinking sessions:

### 1. Start Thinking (`start_thinking`)

Begins a new sequential thinking session for a complex problem.

**Parameters:**

- `problem` (string): The problem or question to think about
- `sessionId` (string, optional): Custom session identifier
- `estimatedSteps` (int, optional): Initial estimate of thinking steps needed

### 2. Continue Thinking (`continue_thinking`)

Adds the next thought step, revises previous steps, or creates alternative branches.

**Parameters:**

- `sessionId` (string): The thinking session to continue
- `thought` (string): The current thought or analysis
- `nextNeeded` (bool, optional): Whether another thinking step is needed
- `reviseStep` (int, optional): Step number to revise (1-based)
- `createBranch` (bool, optional): Create an alternative reasoning path
- `estimatedTotal` (int, optional): Update total estimated steps

### 3. Review Thinking (`review_thinking`)

Provides a complete review of the thinking process for a session.

**Parameters:**

- `sessionId` (string): The session to review

## Resources

### Thinking History (`thinking://sessions` or `thinking://{sessionId}`)

Access thinking session data and history in JSON format.

- `thinking://sessions` - List all thinking sessions
- `thinking://{sessionId}` - Get specific session details

## Core Concepts

### Sequential Processing

Problems are broken down into numbered thought steps that build upon each other, maintaining context and allowing for systematic analysis.

### Dynamic Revision

Any previous thought step can be revised and updated, with the system tracking which thoughts have been modified.

### Alternative Branching

Create alternative reasoning paths to explore different approaches to the same problem, allowing for comparative analysis.

### Adaptive Planning

The estimated number of thinking steps can be adjusted dynamically as understanding of the problem evolves.

## Running the Server

### Standard I/O Mode

```bash
go run .
```

### HTTP Mode  

```bash
go run . -http :8080
```

## Example Usage

### Starting a Thinking Session

```json
{
  "method": "tools/call",
  "params": {
    "name": "start_thinking",
    "arguments": {
      "problem": "How should I design a scalable microservices architecture?",
      "sessionId": "architecture_design",
      "estimatedSteps": 8
    }
  }
}
```

### Adding Sequential Thoughts

```json
{
  "method": "tools/call", 
  "params": {
    "name": "continue_thinking",
    "arguments": {
      "sessionId": "architecture_design",
      "thought": "First, I need to identify the core business domains and their boundaries to determine service decomposition."
    }
  }
}
```

### Revising a Previous Step

```json
{
  "method": "tools/call",
  "params": {
    "name": "continue_thinking", 
    "arguments": {
      "sessionId": "architecture_design",
      "thought": "Actually, before identifying domains, I should analyze the current system's pain points and requirements.",
      "reviseStep": 1
    }
  }
}
```

### Creating an Alternative Branch

```json
{
  "method": "tools/call",
  "params": {
    "name": "continue_thinking",
    "arguments": {
      "sessionId": "architecture_design", 
      "thought": "Alternative approach: Start with a monolith-first strategy and extract services gradually.",
      "createBranch": true
    }
  }
}
```

### Completing the Thinking Process

```json
{
  "method": "tools/call",
  "params": {
    "name": "continue_thinking",
    "arguments": {
      "sessionId": "architecture_design",
      "thought": "Based on this analysis, I recommend starting with 3 core services: User Management, Order Processing, and Inventory Management.",
      "nextNeeded": false
    }
  }
}
```

### Reviewing the Complete Process

```json
{
  "method": "tools/call",
  "params": {
    "name": "review_thinking", 
    "arguments": {
      "sessionId": "architecture_design"
    }
  }
}
```

## Session State Management

Each thinking session maintains:

- **Session metadata**: ID, problem statement, creation time, current status
- **Thought sequence**: Ordered list of thoughts with timestamps and revision history  
- **Progress tracking**: Current step and estimated total steps
- **Branch relationships**: Links to alternative reasoning paths
- **Status management**: Active, completed, or paused sessions

## Use Cases

**Ideal for:**

- Complex problem analysis requiring step-by-step breakdown
- Design decisions needing systematic evaluation
- Scenarios where initial scope is unclear and may evolve
- Problems requiring alternative approach exploration
- Situations needing detailed reasoning documentation

**Examples:**

- Software architecture design
- Research methodology planning  
- Strategic business decisions
- Technical troubleshooting
- Creative problem solving
- Academic research planning
