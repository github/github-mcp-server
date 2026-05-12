import { Box, Text } from "@primer/react";

export function FeedbackFooter() {
  return (
    <Box
      display="flex"
      justifyContent="center"
      mt={2}
    >
      <Text sx={{ color: "fg.subtle", fontSize: 0, textAlign: "center" }}>
        Help us improve MCP Apps support in the GitHub MCP Server
        <br />
        github.com/github/github-mcp-server/issues/new?template=insiders-feedback.md
      </Text>
    </Box>
  );
}
