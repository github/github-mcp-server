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
} from "@primer/react";
import {
  IssueOpenedIcon,
  CheckCircleIcon,
} from "@primer/octicons-react";
import { AppProvider } from "../../components/AppProvider";
import { useMcpApp } from "../../hooks/useMcpApp";
import { MarkdownEditor } from "../../components/MarkdownEditor";

interface IssueResult {
  ID?: string;
  number?: number;
  title?: string;
  body?: string;
  url?: string;
  html_url?: string;
  URL?: string;
}

function SuccessView({
  issue,
  owner,
  repo,
  submittedTitle,
  isUpdate,
}: {
  issue: IssueResult;
  owner: string;
  repo: string;
  submittedTitle: string;
  isUpdate: boolean;
}) {
  const issueUrl = issue.html_url || issue.url || issue.URL || "#";

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
          {isUpdate ? "Issue updated successfully" : "Issue created successfully"}
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
          <IssueOpenedIcon size={16} />
        </Box>
        <Box sx={{ minWidth: 0 }}>
          <a
            href={issueUrl}
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
            {issue.title || submittedTitle}
            {issue.number && (
              <Text sx={{ color: "fg.muted", fontWeight: "normal", ml: 1 }}>
                #{issue.number}
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

function CreateIssueApp() {
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successIssue, setSuccessIssue] = useState<IssueResult | null>(null);

  const { app, error: appError, toolInput, callTool } = useMcpApp({
    appName: "github-mcp-server-issue-write",
  });

  const method = (toolInput?.method as string) || "create";
  const issueNumber = toolInput?.issue_number as number | undefined;
  const isUpdateMode = method === "update" && issueNumber !== undefined;
  const owner = (toolInput?.owner as string) || "";
  const repo = (toolInput?.repo as string) || "";

  // Pre-fill from toolInput
  useEffect(() => {
    if (toolInput?.title) setTitle(toolInput.title as string);
    if (toolInput?.body) setBody(toolInput.body as string);
  }, [toolInput]);

  const handleSubmit = useCallback(async () => {
    if (!title.trim()) {
      setError("Title is required");
      return;
    }
    if (!owner || !repo) {
      setError("Repository information not available");
      return;
    }

    setIsSubmitting(true);
    setError(null);

    try {
      const params: Record<string, unknown> = {
        method: isUpdateMode ? "update" : "create",
        owner,
        repo,
        title: title.trim(),
        body: body.trim(),
        _ui_submitted: true
      };

      if (isUpdateMode && issueNumber) {
        params.issue_number = issueNumber;
      }

      const result = await callTool("issue_write", params);

      if (result.isError) {
        const textContent = result.content?.find(
          (c: { type: string }) => c.type === "text"
        );
        setError(
          (textContent as { text?: string })?.text || "Failed to create issue"
        );
      } else {
        const textContent = result.content?.find(
          (c: { type: string }) => c.type === "text"
        );
        if (textContent && "text" in textContent) {
          try {
            const issueData = JSON.parse(textContent.text as string);
            setSuccessIssue(issueData);
          } catch {
            setSuccessIssue({ title, body });
          }
        }
      }
    } catch (e) {
      setError(`Error: ${e instanceof Error ? e.message : String(e)}`);
    } finally {
      setIsSubmitting(false);
    }
  }, [title, body, owner, repo, isUpdateMode, issueNumber, callTool]);

  if (appError) {
    return (
      <Flash variant="danger" sx={{ m: 2 }}>
        Connection error: {appError.message}
      </Flash>
    );
  }

  if (!app) {
    return (
      <Box display="flex" alignItems="center" justifyContent="center" p={4}>
        <Spinner size="medium" />
      </Box>
    );
  }

  if (successIssue) {
    return (
      <SuccessView
        issue={successIssue}
        owner={owner}
        repo={repo}
        submittedTitle={title}
        isUpdate={isUpdateMode}
      />
    );
  }

  return (
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
          <IssueOpenedIcon size={16} />
        </Box>
        <Text sx={{ fontWeight: "semibold", whiteSpace: "nowrap" }}>
          {isUpdateMode ? `Update issue #${issueNumber}` : "New issue"}
        </Text>
        <Text sx={{ color: "fg.muted", fontSize: 0, ml: 1 }}>
          {owner}/{repo}
        </Text>
      </Box>

      {/* Error banner */}
      {error && (
        <Flash variant="danger" sx={{ mb: 3 }}>
          {error}
        </Flash>
      )}

      {/* Title */}
      <FormControl sx={{ mb: 3 }}>
        <FormControl.Label sx={{ fontWeight: "semibold" }}>
          Title
        </FormControl.Label>
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
        <Text
          as="label"
          sx={{ fontWeight: "semibold", fontSize: 1, display: "block", mb: 2 }}
        >
          Description
        </Text>
        <MarkdownEditor
          value={body}
          onChange={setBody}
          placeholder="Add a description..."
        />
      </Box>

      {/* Submit button */}
      <Box display="flex" justifyContent="flex-end" gap={2}>
        <Button
          variant="primary"
          onClick={handleSubmit}
          disabled={isSubmitting || !title.trim()}
        >
          {isSubmitting ? (
            <>
              <Spinner size="small" sx={{ mr: 1 }} />
              {isUpdateMode ? "Updating..." : "Creating..."}
            </>
          ) : (
            isUpdateMode ? "Update issue" : "Create issue"
          )}
        </Button>
      </Box>
    </Box>
  );
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <AppProvider>
      <CreateIssueApp />
    </AppProvider>
  </StrictMode>
);
