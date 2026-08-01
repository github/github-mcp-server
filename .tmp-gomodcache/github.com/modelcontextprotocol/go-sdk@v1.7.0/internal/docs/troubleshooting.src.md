# Troubleshooting

The Model Context Protocol is a complicated spec that leaves some room for
interpretation. Client and server SDKs can behave differently, or can be more
or less strict about their inputs. And of course, bugs happen.

When you encounter a problem using the Go SDK, these instructions can help
collect information that will be useful in debugging. Please try to provide
this information in a bug report, so that maintainers can more quickly
understand what's going wrong.

And most of all, please do [file bugs](https://github.com/modelcontextprotocol/go-sdk/issues/new?template=bug_report.md).

## Using the MCP inspector

To debug an MCP server, you can use the [MCP
inspector](https://modelcontextprotocol.io/legacy/tools/inspector). This is
useful for testing your server and verifying that it works with the typescript
SDK, as well as inspecting MCP traffic.

## Collecting MCP logs

For [stdio](protocol.md#stdio-transport) transport connections, you can also
inspect MCP traffic using a `LoggingTransport`:

%include ../../mcp/transport_example_test.go loggingtransport -

That example uses a `bytes.Buffer`, but you can also log to a file, or to
`os.Stderr`.

## Inspecting HTTP traffic

There are a couple different ways to investigate traffic to an HTTP transport
([streamable](protocol.md#streamable-transport) or legacy SSE).

The first is to use an HTTP middleware:

%include ../../mcp/streamable_example_test.go httpmiddleware -

The second is to use a general purpose tool to inspect http traffic, such as
[wireshark](https://www.wireshark.org/) or
[tcpdump](https://linux.die.net/man/8/tcpdump).
