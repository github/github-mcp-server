import { ThemeProvider, BaseStyles, Box } from "@primer/react";
import type { ReactNode, CSSProperties } from "react";
import { useEffect, useMemo, useState } from "react";
import type { McpUiHostContext } from "@modelcontextprotocol/ext-apps";
import { FeedbackFooter } from "./FeedbackFooter";

interface AppProviderProps {
  children: ReactNode;
  hostContext?: McpUiHostContext;
}

export function AppProvider({ children, hostContext }: AppProviderProps) {
  const hostTheme = hostContext?.theme;
  const hostVariables = hostContext?.styles?.variables;
  const [isMobile, setIsMobile] = useState(() => {
    if (typeof window === "undefined") return false;
    return window.matchMedia("(max-width: 768px)").matches;
  });

  useEffect(() => {
    if (typeof window === "undefined") return;

    const mediaQuery = window.matchMedia("(max-width: 768px)");
    const handleChange = (event: MediaQueryListEvent) => setIsMobile(event.matches);

    setIsMobile(mediaQuery.matches);
    mediaQuery.addEventListener("change", handleChange);

    return () => {
      mediaQuery.removeEventListener("change", handleChange);
    };
  }, []);

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
    document.body.setAttribute("data-device-type", isMobile ? "mobile" : "desktop");
  }, [hostTheme, isMobile]);

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
        <Box
          p={isMobile ? 2 : 3}
          style={{
            ...styleVars,
            maxWidth: "100%",
            boxSizing: "border-box",
            WebkitTapHighlightColor: "transparent",
          }}
        >
          {children}
          <FeedbackFooter />
        </Box>
      </BaseStyles>
    </ThemeProvider>
  );
}
