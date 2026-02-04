import { StrictMode, useState, useCallback, useEffect, useMemo } from "react";
import { createRoot } from "react-dom/client";
import {
  Box,
  Text,
  TextInput,
  Button,
  Flash,
  Link,
  Spinner,
  FormControl,
  ActionMenu,
  ActionList,
  Checkbox,
} from "@primer/react";
import {
  GitPullRequestIcon,
  CheckCircleIcon,
  RepoIcon,
  LockIcon,
  GitBranchIcon,
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

interface RepositoryItem {
  id: string;
  owner: string;
  name: string;
  fullName: string;
  isPrivate: boolean;
}

interface BranchItem {
  name: string;
  protected: boolean;
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
        <Box sx={{ color: "open.fg", flexShrink: 0, mt: "2px" }}>
          <GitPullRequestIcon size={16} />
        </Box>
        <Box sx={{ minWidth: 0 }}>
          <Link
            href={prUrl}
            target="_blank"
            rel="noopener noreferrer"
            sx={{
              fontWeight: "semibold",
              fontSize: 1,
              display: "block",
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
            }}
          >
            {pr.title || submittedTitle}
            {pr.number && (
              <Text sx={{ color: "fg.muted", fontWeight: "normal", ml: 1 }}>
                #{pr.number}
              </Text>
            )}
          </Link>
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

  // Branch state
  const [availableBranches, setAvailableBranches] = useState<BranchItem[]>([]);
  const [baseBranch, setBaseBranch] = useState<string>("");
  const [headBranch, setHeadBranch] = useState<string>("");
  const [branchesLoading, setBranchesLoading] = useState(false);
  const [baseFilter, setBaseFilter] = useState("");
  const [headFilter, setHeadFilter] = useState("");

  // Options
  const [isDraft, setIsDraft] = useState(false);
  const [maintainerCanModify, setMaintainerCanModify] = useState(true);

  // Repository state
  const [selectedRepo, setSelectedRepo] = useState<RepositoryItem | null>(null);
  const [repoSearchResults, setRepoSearchResults] = useState<RepositoryItem[]>([]);
  const [repoSearchLoading, setRepoSearchLoading] = useState(false);
  const [repoFilter, setRepoFilter] = useState("");

  const { app, error: appError, toolInput, callTool } = useMcpApp({
    appName: "github-mcp-server-create-pull-request",
  });

  const owner = selectedRepo?.owner || (toolInput?.owner as string) || "";
  const repo = selectedRepo?.name || (toolInput?.repo as string) || "";
  const [submittedTitle, setSubmittedTitle] = useState("");

  // Initialize selectedRepo from toolInput
  useEffect(() => {
    if (toolInput?.owner && toolInput?.repo && !selectedRepo) {
      setSelectedRepo({
        id: `${toolInput.owner}/${toolInput.repo}`,
        owner: toolInput.owner as string,
        name: toolInput.repo as string,
        fullName: `${toolInput.owner}/${toolInput.repo}`,
        isPrivate: false,
      });
    }
  }, [toolInput, selectedRepo]);

  // Pre-fill from toolInput
  useEffect(() => {
    if (toolInput?.title) setTitle(toolInput.title as string);
    if (toolInput?.body) setBody(toolInput.body as string);
    if (toolInput?.head) setHeadBranch(toolInput.head as string);
    if (toolInput?.base) setBaseBranch(toolInput.base as string);
    if (toolInput?.draft) setIsDraft(toolInput.draft as boolean);
    if (toolInput?.maintainer_can_modify !== undefined) {
      setMaintainerCanModify(toolInput.maintainer_can_modify as boolean);
    }
  }, [toolInput]);

  // Search repositories
  useEffect(() => {
    if (!app || !repoFilter.trim()) {
      setRepoSearchResults([]);
      return;
    }

    const searchRepos = async () => {
      setRepoSearchLoading(true);
      try {
        const result = await callTool("search_repositories", { query: repoFilter, perPage: 10 });
        if (result && !result.isError && result.content) {
          const textContent = result.content.find((c: { type: string }) => c.type === "text");
          if (textContent?.text) {
            const data = JSON.parse(textContent.text);
            const repos = (data.repositories || data.items || []).map(
              (r: { id?: number; owner?: { login?: string } | string; name?: string; full_name?: string; private?: boolean }) => ({
                id: String(r.id || r.full_name),
                owner: typeof r.owner === 'string' ? r.owner : r.owner?.login || r.full_name?.split('/')[0] || '',
                name: r.name || '',
                fullName: r.full_name || '',
                isPrivate: r.private || false,
              })
            );
            setRepoSearchResults(repos);
          }
        }
      } catch (e) {
        console.error("Failed to search repositories:", e);
      } finally {
        setRepoSearchLoading(false);
      }
    };

    const debounce = setTimeout(searchRepos, 300);
    return () => clearTimeout(debounce);
  }, [app, callTool, repoFilter]);

  // Load branches, labels, reviewers, milestones when repo is selected
  useEffect(() => {
    if (!owner || !repo || !app) return;

    const loadBranches = async () => {
      setBranchesLoading(true);
      try {
        const result = await callTool("list_branches", { owner, repo, perPage: 100 });
        if (result && !result.isError && result.content) {
          const textContent = result.content.find((c: { type: string }) => c.type === "text");
          if (textContent && "text" in textContent) {
            const data = JSON.parse(textContent.text as string);
            const branches = (data.branches || data || []).map(
              (b: { name: string; protected?: boolean }) => ({ name: b.name, protected: b.protected || false })
            );
            setAvailableBranches(branches);
            if (!baseBranch && branches.length > 0) {
              const defaultBranch = branches.find((b: BranchItem) => b.name === 'main' || b.name === 'master');
              if (defaultBranch) setBaseBranch(defaultBranch.name);
            }
          }
        }
      } catch (e) {
        console.error("Failed to load branches:", e);
      } finally {
        setBranchesLoading(false);
      }
    };

    loadBranches();
  }, [owner, repo, app, callTool, baseBranch]);

  // Filters
  const filteredBaseBranches = useMemo(() => {
    if (!baseFilter.trim()) return availableBranches;
    return availableBranches.filter((b) => b.name.toLowerCase().includes(baseFilter.toLowerCase()));
  }, [availableBranches, baseFilter]);

  const filteredHeadBranches = useMemo(() => {
    if (!headFilter.trim()) return availableBranches;
    return availableBranches.filter((b) => b.name.toLowerCase().includes(headFilter.toLowerCase()));
  }, [availableBranches, headFilter]);

  const handleSubmit = useCallback(async () => {
    if (!title.trim()) { setError("Title is required"); return; }
    if (!owner || !repo) { setError("Repository information not available"); return; }
    if (!baseBranch) { setError("Base branch is required"); return; }
    if (!headBranch) { setError("Head branch is required"); return; }
    if (baseBranch === headBranch) { setError("Base and head branches cannot be the same"); return; }

    setIsSubmitting(true);
    setError(null);
    setSubmittedTitle(title);

    try {
      const result = await callTool("create_pull_request", {
        owner, repo,
        title: title.trim(),
        body: body.trim(),
        head: headBranch,
        base: baseBranch,
        draft: isDraft,
        maintainer_can_modify: maintainerCanModify
      });

      if (result.isError) {
        const errorText = result.content?.find((c: { type: string }) => c.type === "text");
        setError(errorText?.text || "Failed to create pull request");
      } else {
        const textContent = result.content?.find((c: { type: string }) => c.type === "text");
        if (textContent?.text) {
          const prData = JSON.parse(textContent.text);
          setSuccessPR(prData);
        }
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "An error occurred");
    } finally {
      setIsSubmitting(false);
    }
  }, [title, body, owner, repo, baseBranch, headBranch, isDraft, maintainerCanModify, callTool]);

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
        {/* Repository picker */}
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
          <ActionMenu>
            <ActionMenu.Button
              size="small"
              leadingVisual={selectedRepo?.isPrivate ? LockIcon : RepoIcon}
            >
              {selectedRepo ? selectedRepo.fullName : "Select repository"}
            </ActionMenu.Button>
            <ActionMenu.Overlay width="large">
              <ActionList selectionVariant="single">
                <Box px={3} py={2}>
                  <TextInput
                    placeholder="Search repositories..."
                    value={repoFilter}
                    onChange={(e) => setRepoFilter(e.target.value)}
                    sx={{ width: "100%" }}
                    size="small"
                    autoFocus
                  />
                </Box>
                <ActionList.Divider />
                {repoSearchLoading ? (
                  <Box display="flex" justifyContent="center" p={3}>
                    <Spinner size="small" />
                  </Box>
                ) : repoSearchResults.length > 0 ? (
                  repoSearchResults.map((r) => (
                    <ActionList.Item
                      key={r.id}
                      selected={selectedRepo?.id === r.id}
                      onSelect={() => {
                        setSelectedRepo(r);
                        setRepoFilter("");
                        setAvailableBranches([]);
                        setBaseBranch("");
                        setHeadBranch("");
                      }}
                    >
                      <ActionList.LeadingVisual>
                        {r.isPrivate ? <LockIcon /> : <RepoIcon />}
                      </ActionList.LeadingVisual>
                      {r.fullName}
                    </ActionList.Item>
                  ))
                ) : selectedRepo ? (
                  <ActionList.Item key={selectedRepo.id} selected onSelect={() => setRepoFilter("")}>
                    <ActionList.LeadingVisual>
                      {selectedRepo.isPrivate ? <LockIcon /> : <RepoIcon />}
                    </ActionList.LeadingVisual>
                    {selectedRepo.fullName}
                  </ActionList.Item>
                ) : (
                  <Box px={3} py={2}>
                    <Text sx={{ color: "fg.muted", fontSize: 1 }}>Type to search repositories...</Text>
                  </Box>
                )}
              </ActionList>
            </ActionMenu.Overlay>
          </ActionMenu>
        </Box>

        {/* Branch selectors */}
        <Box display="flex" gap={3} mb={3} alignItems="flex-end">
          <Box flex={1}>
            <Text sx={{ fontSize: 0, color: "fg.muted", mb: 1, display: "block" }}>base</Text>
            <ActionMenu>
              <ActionMenu.Button size="small" leadingVisual={GitBranchIcon} sx={{ width: "100%" }}>
                {baseBranch || "Select base"}
              </ActionMenu.Button>
              <ActionMenu.Overlay width="medium">
                <ActionList selectionVariant="single">
                  <Box p={2}>
                    <TextInput
                      placeholder="Filter branches..."
                      value={baseFilter}
                      onChange={(e) => setBaseFilter(e.target.value)}
                      size="small"
                      block
                    />
                  </Box>
                  <ActionList.Divider />
                  {branchesLoading ? (
                    <ActionList.Item disabled><Spinner size="small" /> Loading...</ActionList.Item>
                  ) : filteredBaseBranches.length === 0 ? (
                    <ActionList.Item disabled>No branches found</ActionList.Item>
                  ) : (
                    filteredBaseBranches.map((branch) => (
                      <ActionList.Item
                        key={branch.name}
                        selected={baseBranch === branch.name}
                        onSelect={() => { setBaseBranch(branch.name); setBaseFilter(""); }}
                      >
                        {branch.name}
                        {branch.protected && <ActionList.TrailingVisual><LockIcon size={12} /></ActionList.TrailingVisual>}
                      </ActionList.Item>
                    ))
                  )}
                </ActionList>
              </ActionMenu.Overlay>
            </ActionMenu>
          </Box>

          <Text sx={{ color: "fg.muted", pb: 1, px: 1 }}>←</Text>

          <Box flex={1}>
            <Text sx={{ fontSize: 0, color: "fg.muted", mb: 1, display: "block" }}>compare</Text>
            <ActionMenu>
              <ActionMenu.Button size="small" leadingVisual={GitBranchIcon} sx={{ width: "100%" }}>
                {headBranch || "Select head"}
              </ActionMenu.Button>
              <ActionMenu.Overlay width="medium">
                <ActionList selectionVariant="single">
                  <Box p={2}>
                    <TextInput
                      placeholder="Filter branches..."
                      value={headFilter}
                      onChange={(e) => setHeadFilter(e.target.value)}
                      size="small"
                      block
                    />
                  </Box>
                  <ActionList.Divider />
                  {branchesLoading ? (
                    <ActionList.Item disabled><Spinner size="small" /> Loading...</ActionList.Item>
                  ) : filteredHeadBranches.length === 0 ? (
                    <ActionList.Item disabled>No branches found</ActionList.Item>
                  ) : (
                    filteredHeadBranches.map((branch) => (
                      <ActionList.Item
                        key={branch.name}
                        selected={headBranch === branch.name}
                        onSelect={() => { setHeadBranch(branch.name); setHeadFilter(""); }}
                      >
                        {branch.name}
                      </ActionList.Item>
                    ))
                  )}
                </ActionList>
              </ActionMenu.Overlay>
            </ActionMenu>
          </Box>
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

        {/* Options */}
        <Box mb={3} display="flex" gap={4}>
          <FormControl>
            <Checkbox checked={isDraft} onChange={(e) => setIsDraft(e.target.checked)} />
            <FormControl.Label sx={{ fontWeight: "normal", ml: 1 }}>Create as draft</FormControl.Label>
          </FormControl>
          <FormControl>
            <Checkbox checked={maintainerCanModify} onChange={(e) => setMaintainerCanModify(e.target.checked)} />
            <FormControl.Label sx={{ fontWeight: "normal", ml: 1 }}>Allow maintainer edits</FormControl.Label>
          </FormControl>
        </Box>

        {/* Submit button */}
        <Box display="flex" justifyContent="flex-end" gap={2}>
          <Button
            variant="primary"
            onClick={handleSubmit}
            disabled={isSubmitting || !owner || !repo || !baseBranch || !headBranch}
          >
            {isSubmitting ? (
              <><Spinner size="small" sx={{ mr: 1 }} />Creating...</>
            ) : isDraft ? (
              "Create draft pull request"
            ) : (
              "Create pull request"
            )}
          </Button>
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
