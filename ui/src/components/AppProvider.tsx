import { ThemeProvider, BaseStyles, Box } from "@primer/react";
import type { ReactNode } from "react";
import { useEffect } from "react";

interface AppProviderProps {
  children: ReactNode;
}

export function AppProvider({ children }: AppProviderProps) {
  useEffect(() => {
    // Set up theme data attributes for proper Primer theming
    const prefersDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
    const colorMode = prefersDark ? "dark" : "light";
    document.body.setAttribute("data-color-mode", colorMode);
    document.body.setAttribute("data-light-theme", "light");
    document.body.setAttribute("data-dark-theme", "dark");
  }, []);

  return (
    <ThemeProvider colorMode="auto">
      <BaseStyles>
        <Box p={3}>{children}</Box>
      </BaseStyles>
    </ThemeProvider>
  );
}
