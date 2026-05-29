import { ThemeProvider, BaseStyles, Box } from "@primer/react";
import type { ReactNode, CSSProperties } from "react";
import { useEffect, useMemo } from "react";
import type { McpUiHostContext } from "@modelcontextprotocol/ext-apps";
import { FeedbackFooter } from "./FeedbackFooter";

interface AppProviderProps {
  children: ReactNode;
  hostContext?: McpUiHostContext;
}

export function AppProvider({ children, hostContext }: AppProviderProps) {
  const hostTheme = hostContext?.theme;
  const hostVariables = hostContext?.styles?.variables;

  useEffect(() => {
    // Prefer the host-supplied theme; fall back to the OS preference.
    const colorMode =
      hostTheme === "light" || hostTheme === "dark"
        ? hostTheme
        : window.matchMedia("(prefers-color-scheme: dark)").matches
          ? "dark"
          : "light";
    document.body.setAttribute("data-color-mode", colorMode);
    document.body.setAttribute("data-light-theme", "light");
    document.body.setAttribute("data-dark-theme", "dark");
  }, [hostTheme]);

  // Project the host's standardized CSS variables onto the root so child
  // components can consume them via `var(--color-...)`. We rely on Primer's
  // own defaults when the host does not supply variables.
  const styleVars = useMemo<CSSProperties | undefined>(() => {
    if (!hostVariables) return undefined;
    const out: Record<string, string> = {};
    for (const [key, value] of Object.entries(hostVariables)) {
      if (typeof value === "string") out[key] = value;
    }
    return out as CSSProperties;
  }, [hostVariables]);

  const colorMode =
    hostTheme === "light" || hostTheme === "dark" ? hostTheme : "auto";

  return (
    <ThemeProvider colorMode={colorMode}>
      <BaseStyles>
        <Box p={3} style={styleVars}>
          {children}
          <FeedbackFooter />
        </Box>
      </BaseStyles>
    </ThemeProvider>
  );
}
