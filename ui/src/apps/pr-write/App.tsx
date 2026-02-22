import { StrictMode, useState, useCallback, useEffect } from "react";
import { createRoot } from "react-dom/client";
import {
  Box,
  Text,
  TextInput,
  Button,
  Flash,
  Spinner,
  FormControl,
  ActionMenu,
  ActionList,
  Checkbox,
  ButtonGroup,
} from "@primer/react";
import {
  GitPullRequestIcon,
  CheckCircleIcon,
  TriangleDownIcon,
} from "@primer/octicons-react";
import { AppProvider } from "../../components/AppProvider";
import { useMcpApp } from "../../hooks/useMcpApp";
import { MarkdownEditor } from "../../components/MarkdownEditor";

interface PRResult {
  ID?: string;
  number?: number;
  title?: string;
  url?: string;
  html_url?: string;
  URL?: string;
}

function SuccessView({
  pr,
  owner,
  repo,
  submittedTitle,
}: {
  pr: PRResult;
  owner: string;
  repo: string;
  submittedTitle: string;
}) {
  const prUrl = pr.html_url || pr.url || pr.URL || "#";

  return (
    <Box
      borderWidth={1}
      borderStyle="solid"
      borderColor="border.default"
      borderRadius={2}
      bg="canvas.subtle"
      p={3}
    >
      <Box
        display="flex"
        alignItems="center"
        mb={3}
        pb={3}
        borderBottomWidth={1}
        borderBottomStyle="solid"
        borderBottomColor="border.default"
      >
        <Box sx={{ color: "success.fg", flexShrink: 0, mr: 2 }}>
          <CheckCircleIcon size={16} />
        </Box>
        <Text sx={{ fontWeight: "semibold" }}>
          Pull request created successfully
        </Text>
      </Box>

      <Box
        display="flex"
        alignItems="flex-start"
        gap={2}
        p={3}
        bg="canvas.subtle"
        borderRadius={2}
        borderWidth={1}
        borderStyle="solid"
        borderColor="border.default"
      >
        <Box sx={{ color: "open.fg", flexShrink: 0, mt: "2px", mr: 1 }}>
          <GitPullRequestIcon size={16} />
        </Box>
        <Box sx={{ minWidth: 0 }}>
          <a
            href={prUrl}
            target="_blank"
            rel="noopener noreferrer"
            style={{
              fontWeight: 600,
              fontSize: "14px",
              display: "block",
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
              color: "var(--fgColor-accent, var(--color-accent-fg))",
              textDecoration: "none",
            }}
          >
            {pr.title || submittedTitle}
            {pr.number && (
              <Text sx={{ color: "fg.muted", fontWeight: "normal", ml: 1 }}>
                #{pr.number}
              </Text>
            )}
          </a>
          <Text sx={{ color: "fg.muted", fontSize: 0 }}>
            {owner}/{repo}
          </Text>
        </Box>
      </Box>
    </Box>
  );
}

function CreatePRApp() {
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successPR, setSuccessPR] = useState<PRResult | null>(null);

  const [isDraft, setIsDraft] = useState(false);
  const [maintainerCanModify, setMaintainerCanModify] = useState(true);

  const { app, error: appError, toolInput, callTool } = useMcpApp({
    appName: "github-mcp-server-create-pull-request",
  });

  const owner = (toolInput?.owner as string) || "";
  const repo = (toolInput?.repo as string) || "";
  const head = (toolInput?.head as string) || "";
  const base = (toolInput?.base as string) || "";
  const [submittedTitle, setSubmittedTitle] = useState("");

  // Pre-fill from toolInput
  useEffect(() => {
    if (toolInput?.title) setTitle(toolInput.title as string);
    if (toolInput?.body) setBody(toolInput.body as string);
    if (toolInput?.draft) setIsDraft(toolInput.draft as boolean);
    if (toolInput?.maintainer_can_modify !== undefined) {
      setMaintainerCanModify(toolInput.maintainer_can_modify as boolean);
    }
  }, [toolInput]);

  const handleSubmit = useCallback(async () => {
    if (!title.trim()) { setError("Title is required"); return; }
    if (!owner || !repo) { setError("Repository information not available"); return; }

    setIsSubmitting(true);
    setError(null);
    setSubmittedTitle(title);

    try {
      const result = await callTool("create_pull_request", {
        owner, repo,
        title: title.trim(),
        body: body.trim(),
        head,
        base,
        draft: isDraft,
        maintainer_can_modify: maintainerCanModify,
        _ui_submitted: true
      });

      if (result.isError) {
        const errorText = result.content?.find((c) => c.type === "text");
        const errorMessage = errorText && errorText.type === "text" ? errorText.text : "Failed to create pull request";
        setError(errorMessage);
      } else {
        const textContent = result.content?.find((c) => c.type === "text");
        if (textContent && textContent.type === "text" && textContent.text) {
          const prData = JSON.parse(textContent.text);
          setSuccessPR(prData);
        }
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "An error occurred");
    } finally {
      setIsSubmitting(false);
    }
  }, [title, body, owner, repo, head, base, isDraft, maintainerCanModify, callTool]);

  if (successPR) {
    return (
      <AppProvider>
        <SuccessView pr={successPR} owner={owner} repo={repo} submittedTitle={submittedTitle} />
      </AppProvider>
    );
  }

  if (!app && !appError) {
    return (
      <AppProvider>
        <Box display="flex" alignItems="center" justifyContent="center" p={4}>
          <Spinner size="medium" />
        </Box>
      </AppProvider>
    );
  }

  if (appError) {
    return (
      <AppProvider>
        <Flash variant="danger">{appError.message}</Flash>
      </AppProvider>
    );
  }

  return (
    <AppProvider>
      <Box
        borderWidth={1}
        borderStyle="solid"
        borderColor="border.default"
        borderRadius={2}
        bg="canvas.subtle"
        p={3}
      >
        {/* Header */}
        <Box
          display="flex"
          alignItems="center"
          gap={2}
          mb={3}
          pb={2}
          borderBottomWidth={1}
          borderBottomStyle="solid"
          borderBottomColor="border.default"
        >
          <Box sx={{ color: "fg.default", flexShrink: 0, display: "flex", mr: 1 }}>
            <GitPullRequestIcon size={16} />
          </Box>
          <Text sx={{ fontWeight: "semibold", whiteSpace: "nowrap" }}>New pull request</Text>
          <Text sx={{ color: "fg.muted", fontSize: 0, ml: 1 }}>
            {owner}/{repo}
          </Text>
          {head && base && (
            <Text sx={{ color: "fg.muted", fontSize: 0 }}>
              {base} ← {head}
            </Text>
          )}
        </Box>

        {/* Error banner */}
        {error && <Flash variant="danger" sx={{ mb: 3 }}>{error}</Flash>}

        {/* Title */}
        <FormControl sx={{ mb: 3 }}>
          <FormControl.Label sx={{ fontWeight: "semibold" }}>Title</FormControl.Label>
          <TextInput
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="Title"
            block
            contrast
          />
        </FormControl>

        {/* Description */}
        <Box sx={{ mb: 3 }}>
          <Text as="label" sx={{ fontWeight: "semibold", fontSize: 1, display: "block", mb: 2 }}>
            Description
          </Text>
          <MarkdownEditor value={body} onChange={setBody} placeholder="Add a description..." />
        </Box>

        {/* Options and Submit */}
        <Box display="flex" justifyContent="space-between" alignItems="flex-end" flexWrap="wrap" gap={3}>
          <Box as="label" display="flex" alignItems="center" sx={{ cursor: "pointer", gap: 2 }}>
            <Checkbox checked={maintainerCanModify} onChange={(e) => setMaintainerCanModify(e.target.checked)} />
            <Text sx={{ fontSize: 1, color: "fg.muted" }}>Allow maintainer edits</Text>
          </Box>

          <ButtonGroup>
            <Button
              variant="primary"
              onClick={handleSubmit}
              disabled={isSubmitting || !owner || !repo}
            >
              {isSubmitting ? (
                <><Spinner size="small" sx={{ mr: 1 }} />Creating...</>
              ) : isDraft ? (
                "Draft pull request"
              ) : (
                "Create pull request"
              )}
            </Button>
            <ActionMenu>
              <ActionMenu.Anchor>
                <Button
                  variant="primary"
                  disabled={isSubmitting || !owner || !repo}
                  sx={{ px: 2 }}
                  aria-label="Select pull request type"
                >
                  <TriangleDownIcon />
                </Button>
              </ActionMenu.Anchor>
              <ActionMenu.Overlay width="medium">
                <ActionList selectionVariant="single">
                  <ActionList.Item selected={!isDraft} onSelect={() => setIsDraft(false)}>
                    <ActionList.LeadingVisual>
                      <GitPullRequestIcon />
                    </ActionList.LeadingVisual>
                    Create pull request
                    <ActionList.Description variant="block">
                      Open a pull request that is ready for review
                    </ActionList.Description>
                  </ActionList.Item>
                  <ActionList.Item selected={isDraft} onSelect={() => setIsDraft(true)}>
                    <ActionList.LeadingVisual>
                      <GitPullRequestIcon />
                    </ActionList.LeadingVisual>
                    Create draft pull request
                    <ActionList.Description variant="block">
                      Cannot be merged until marked ready for review
                    </ActionList.Description>
                  </ActionList.Item>
                </ActionList>
              </ActionMenu.Overlay>
            </ActionMenu>
          </ButtonGroup>
        </Box>
      </Box>
    </AppProvider>
  );
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <CreatePRApp />
  </StrictMode>
);
