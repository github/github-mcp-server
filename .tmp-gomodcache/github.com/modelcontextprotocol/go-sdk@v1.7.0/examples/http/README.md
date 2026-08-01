# MCP HTTP Example

This example demonstrates how to use the Model Context Protocol (MCP) over HTTP using the streamable transport. It includes both a server and client implementation.

## Overview

The example implements:
- A server that provides a `cityTime` tool
- A client that connects to the server, lists available tools, and calls the `cityTime` tool

## Usage

### Start the Server

```bash
go run . server
```
This starts an MCP server on `http://localhost:8000` (default) that provides a `cityTime` tool.

To run a client in another terminal:

```bash
go run . client
```

The client will:
1. Connect to the server
2. List available tools
3. Call the `cityTime` tool for NYC, San Francisco, and Boston
4. Display the results

At any given time you can pass a custom URL to the program to run it on a custom host/port:


```bash
go run . -host 0.0.0.0 -port 9000 server
```

## Testing with real-world MCP Clients

Once the server is started, assuming it's the default localhost:8080, you can try to add it to a popular MCP client:

```bash
claude mcp add -t http timezone http://localhost:8080
```

Once added, Claude Code will be able to discover and use the `cityTime` tool provided by this server.

In Claude Code:

```
> what's the timezone

⏺ I'll get the current time in a major US city for you.

⏺ timezone - cityTime (MCP)(city: "nyc")
  ⎿ The current time in New York City is 7:30:16 PM EDT on Wedn
    esday, July 23, 2025


⏺ The current timezone is EDT (Eastern Daylight Time), and it's
   7:30 PM on Wednesday, July 23, 2025.

> what timezones do you support?

⏺ The timezone tool supports three US cities:
  - NYC (Eastern Time)
  - SF (Pacific Time)
  - Boston (Eastern Time)
```
