import { StrictMode, useState, useCallback, useEffect, useMemo } from "react";
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
  CounterLabel,
  Label,
} from "@primer/react";
import {
  GitPullRequestIcon,
  CheckCircleIcon,
  RepoIcon,
  LockIcon,
  GitBranchIcon,
  TriangleDownIcon,
  PersonIcon,
  PeopleIcon,
} from "@primer/octicons-react";
import { AppProvider } from "../../components/AppProvider";
import { useMcpApp } from "../../hooks/useMcpApp";
import { completedToolResult } from "../../lib/toolResult";
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

type ReviewerItem = { kind: "user" | "team"; id: string; text: string; avatar?: string; org?: string };

function reviewerFromValue(value: string): ReviewerItem {
  if (value.includes("/")) {
    const [org, slug] = value.split("/", 2);
    return { kind: "team", id: `${org}/${slug}`, text: `${org}/${slug}`, org };
  }
  return { kind: "user", id: value, text: value };
}

function reviewerValue(reviewer: ReviewerItem): string {
  return reviewer.kind === "team" ? reviewer.id : reviewer.text;
}

function SuccessView({
  pr,
  owner,
  repo,
  submittedTitle,
  openLink,
}: {
  pr: PRResult;
  owner: string;
  repo: string;
  submittedTitle: string;
  openLink: (url: string) => Promise<void>;
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
            onClick={(e) => {
              // MCP Apps run in a sandboxed iframe where a plain anchor may be
              // blocked, so route the click through the host's open-link
              // capability (falls back to window.open).
              e.preventDefault();
              if (prUrl === "#") return;
              void openLink(prUrl);
            }}
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
  const [availableReviewers, setAvailableReviewers] = useState<ReviewerItem[]>([]);
  const [selectedReviewers, setSelectedReviewers] = useState<ReviewerItem[]>([]);
  const [reviewersLoading, setReviewersLoading] = useState(false);
  const [reviewersFilter, setReviewersFilter] = useState("");

  // Repository state
  const [selectedRepo, setSelectedRepo] = useState<RepositoryItem | null>(null);
  const [repoSearchResults, setRepoSearchResults] = useState<RepositoryItem[]>([]);
  const [repoSearchLoading, setRepoSearchLoading] = useState(false);
  const [repoFilter, setRepoFilter] = useState("");

  const { app, error: appError, toolInput, toolResult, callTool, hostContext, setModelContext, openLink } = useMcpApp({
    appName: "github-mcp-server-create-pull-request",
  });

  const owner = selectedRepo?.owner || (toolInput?.owner as string) || "";
  const repo = selectedRepo?.name || (toolInput?.repo as string) || "";
  const [submittedTitle, setSubmittedTitle] = useState("");

  // When the server executed up-front instead of deferring to this form (e.g.
  // the agent passed show_ui=false or parameters the form can't represent), the
  // host still renders this View and delivers the created PR via tool-result.
  // Treat that completed result as a success so we never show a "Create pull
  // request" form for a PR that already exists. The deferral sentinel
  // (awaiting_user_submission) returns null here, keeping the form for the
  // normal deferred flow. See github/copilot-mcp-core#1864.
  const resultPR = useMemo(() => completedToolResult<PRResult>(toolResult), [toolResult]);
  const shownPR = successPR ?? resultPR;

  // Reset all transient form/result state when toolInput changes (new invocation).
  // Without this, the SuccessView from a previous submit stays visible and stale
  // form values bleed through because the prefill effect below only sets when
  // toolInput has truthy values and never clears. The repo is re-initialized from
  // the new invocation here (rather than in a separate effect) so it isn't wiped
  // by this reset.
  useEffect(() => {
    setTitle("");
    setBody("");
    setHeadBranch("");
    setBaseBranch("");
    setIsDraft(false);
    setMaintainerCanModify(true);
    setSuccessPR(null);
    setError(null);
    setSubmittedTitle("");
    // Clear branch list and filters so a new invocation doesn't briefly show stale
    // branches from the previous repo (or allow selecting invalid options) before the
    // new repo's ui_get branches call resolves.
    setAvailableBranches([]);
    setBaseFilter("");
    setHeadFilter("");
    setAvailableReviewers([]);
    setSelectedReviewers([]);
    setReviewersFilter("");
    if (toolInput?.owner && toolInput?.repo) {
      setSelectedRepo({
        id: `${toolInput.owner}/${toolInput.repo}`,
        owner: toolInput.owner as string,
        name: toolInput.repo as string,
        fullName: `${toolInput.owner}/${toolInput.repo}`,
        isPrivate: false,
      });
    } else {
      setSelectedRepo(null);
    }
  }, [toolInput]);

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
    if (Array.isArray(toolInput?.reviewers)) {
      setSelectedReviewers((toolInput.reviewers as string[]).map(reviewerFromValue));
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
          const textContent = result.content.find((c) => c.type === "text");
          if (textContent && textContent.type === "text" && textContent.text) {
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

  // Load branches and reviewers when repo is selected
  useEffect(() => {
    if (!owner || !repo || !app) return;

    const loadBranches = async () => {
      setBranchesLoading(true);
      try {
        const result = await callTool("ui_get", { method: "branches", owner, repo });
        if (result && !result.isError && result.content) {
          const textContent = result.content.find((c: { type: string }) => c.type === "text");
          if (textContent && "text" in textContent) {
            const data = JSON.parse(textContent.text as string);
            const branches = (data.branches || data || []).map(
              (b: { name: string; protected?: boolean }) => ({ name: b.name, protected: b.protected || false })
            );
            setAvailableBranches(branches);
            if (branches.length > 0) {
              const defaultBranch = branches.find((b: BranchItem) => b.name === 'main' || b.name === 'master');
              // Functional update so a base branch already prefilled from
              // toolInput.base (or chosen by the user) isn't overwritten by a
              // stale closure value captured before the request resolved.
              if (defaultBranch) setBaseBranch((prev) => prev || defaultBranch.name);
            }
          }
        }
      } catch (e) {
        console.error("Failed to load branches:", e);
      } finally {
        setBranchesLoading(false);
      }
    };

    const loadReviewers = async () => {
      setReviewersLoading(true);
      try {
        const result = await callTool("ui_get", { method: "reviewers", owner, repo });
        if (result && !result.isError && result.content) {
          const textContent = result.content.find((c: { type: string }) => c.type === "text");
          if (textContent && "text" in textContent) {
            const data = JSON.parse(textContent.text as string);
            const users = (data.users || []).map(
              (u: { login: string; avatar_url?: string }) => ({
                kind: "user" as const,
                id: u.login,
                text: u.login,
                avatar: u.avatar_url,
              })
            );
            const teams = (data.teams || []).map(
              (t: { slug: string; name?: string; org: string }) => ({
                kind: "team" as const,
                id: `${t.org}/${t.slug}`,
                text: `${t.org}/${t.slug}`,
                org: t.org,
              })
            );
            setAvailableReviewers([...users, ...teams]);
          }
        }
      } catch (e) {
        console.error("Failed to load reviewers:", e);
      } finally {
        setReviewersLoading(false);
      }
    };

    loadBranches();
    loadReviewers();
  }, [owner, repo, app, callTool]);

  useEffect(() => {
    if (availableReviewers.length === 0) return;
    setSelectedReviewers((prev) =>
      prev.map((reviewer) =>
        availableReviewers.find((available) => available.id === reviewer.id || available.text === reviewer.text) || reviewer
      )
    );
  }, [availableReviewers]);

  // Filters
  const filteredBaseBranches = useMemo(() => {
    if (!baseFilter.trim()) return availableBranches;
    return availableBranches.filter((b) => b.name.toLowerCase().includes(baseFilter.toLowerCase()));
  }, [availableBranches, baseFilter]);

  const filteredHeadBranches = useMemo(() => {
    if (!headFilter.trim()) return availableBranches;
    return availableBranches.filter((b) => b.name.toLowerCase().includes(headFilter.toLowerCase()));
  }, [availableBranches, headFilter]);

  const filteredReviewers = useMemo(() => {
    if (!reviewersFilter.trim()) return availableReviewers;
    const lowerFilter = reviewersFilter.toLowerCase();
    return availableReviewers.filter((reviewer) =>
      reviewer.text.toLowerCase().includes(lowerFilter) || reviewer.id.toLowerCase().includes(lowerFilter)
    );
  }, [availableReviewers, reviewersFilter]);

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
        ...(toolInput as Record<string, unknown> | undefined),
        owner, repo,
        title: title.trim(),
        body: body.trim(),
        head: headBranch,
        base: baseBranch,
        draft: isDraft,
        maintainer_can_modify: maintainerCanModify,
        reviewers: selectedReviewers.map(reviewerValue),
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
          // Push the new PR into the model context so subsequent agent
          // turns can reference it (MCP Apps 2026-01-26 ui/update-model-context).
          void setModelContext({
            structuredContent: prData,
            content: [
              {
                type: "text",
                text: `A new pull request was created in ${owner}/${repo} by the user via the create-pull-request view.`,
              },
            ],
          });
        }
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "An error occurred");
    } finally {
      setIsSubmitting(false);
    }
  }, [title, body, owner, repo, baseBranch, headBranch, isDraft, maintainerCanModify, selectedReviewers, toolInput, callTool, setModelContext]);

  if (shownPR) {
    return (
      <AppProvider hostContext={hostContext}>
        <SuccessView pr={shownPR} owner={owner} repo={repo} submittedTitle={submittedTitle} openLink={openLink} />
      </AppProvider>
    );
  }

  if (!app && !appError) {
    return (
      <AppProvider hostContext={hostContext}>
        <Box display="flex" alignItems="center" justifyContent="center" p={4}>
          <Spinner size="medium" />
        </Box>
      </AppProvider>
    );
  }

  if (appError) {
    return (
      <AppProvider hostContext={hostContext}>
        <Flash variant="danger">{appError.message}</Flash>
      </AppProvider>
    );
  }

  return (
    <AppProvider hostContext={hostContext}>
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
          sx={{ minWidth: 0, overflow: "hidden" }}
        >
          <Box sx={{ minWidth: 0, maxWidth: "100%" }}>
            <ActionMenu>
              <ActionMenu.Button
                size="small"
                leadingVisual={selectedRepo?.isPrivate ? LockIcon : RepoIcon}
                sx={{ maxWidth: "100%", overflow: "hidden", "& > span:last-child": { overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" } }}
              >
                {selectedRepo ? selectedRepo.fullName : "Select repository"}
              </ActionMenu.Button>
            <ActionMenu.Overlay width="medium">
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
                        setAvailableReviewers([]);
                        setSelectedReviewers([]);
                        setReviewersFilter("");
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
        </Box>

        {/* Branch selectors */}
        <Box display="flex" gap={2} mb={3} alignItems="flex-end" sx={{ minWidth: 0, flexWrap: "wrap" }}>
          <Box sx={{ flex: "1 1 120px", minWidth: 0 }}>
            <Text sx={{ fontSize: 0, color: "fg.muted", mb: 1, display: "block" }}>base</Text>
            <ActionMenu>
              <ActionMenu.Button size="small" leadingVisual={GitBranchIcon} sx={{ width: "100%", "& > span": { overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" } }}>
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

          <Text sx={{ color: "fg.muted", pb: 1, px: 1, flexShrink: 0 }}>←</Text>

          <Box sx={{ flex: "1 1 120px", minWidth: 0 }}>
            <Text sx={{ fontSize: 0, color: "fg.muted", mb: 1, display: "block" }}>compare</Text>
            <ActionMenu>
              <ActionMenu.Button size="small" leadingVisual={GitBranchIcon} sx={{ width: "100%", "& > span": { overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" } }}>
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

        {/* Reviewers */}
        <Box sx={{ mb: 3 }}>
          <Text sx={{ fontSize: 0, color: "fg.muted", mb: 1, display: "block" }}>reviewers</Text>
          <ActionMenu>
            <ActionMenu.Button size="small" leadingVisual={PersonIcon} sx={{ minWidth: 160 }}>
              {selectedReviewers.length === 0 ? (
                "No reviewers"
              ) : (
                <>
                  Reviewers
                  <CounterLabel sx={{ ml: 1 }}>{selectedReviewers.length}</CounterLabel>
                </>
              )}
            </ActionMenu.Button>
            <ActionMenu.Overlay width="medium">
              <Box p={2} borderBottomWidth={1} borderBottomStyle="solid" borderBottomColor="border.default">
                <TextInput
                  placeholder="Search reviewers"
                  value={reviewersFilter}
                  onChange={(e) => setReviewersFilter(e.target.value)}
                  size="small"
                  block
                />
              </Box>
              <ActionList selectionVariant="multiple">
                {reviewersLoading ? (
                  <ActionList.Item disabled><Spinner size="small" /> Loading...</ActionList.Item>
                ) : filteredReviewers.length === 0 ? (
                  <ActionList.Item disabled>No reviewers available</ActionList.Item>
                ) : (
                  filteredReviewers.map((reviewer) => (
                    <ActionList.Item
                      key={reviewer.id}
                      selected={selectedReviewers.some((r) => r.id === reviewer.id)}
                      onSelect={() => {
                        setSelectedReviewers((prev) =>
                          prev.some((r) => r.id === reviewer.id)
                            ? prev.filter((r) => r.id !== reviewer.id)
                            : [...prev, reviewer]
                        );
                      }}
                    >
                      <ActionList.LeadingVisual>
                        {reviewer.kind === "user" ? (
                          reviewer.avatar ? (
                            <img
                              src={reviewer.avatar}
                              alt=""
                              width={16}
                              height={16}
                              style={{ borderRadius: "50%", display: "block" }}
                            />
                          ) : (
                            <PersonIcon />
                          )
                        ) : (
                          <PeopleIcon />
                        )}
                      </ActionList.LeadingVisual>
                      {reviewer.text}
                    </ActionList.Item>
                  ))
                )}
              </ActionList>
            </ActionMenu.Overlay>
          </ActionMenu>
          {selectedReviewers.length > 0 && (
            <Box display="flex" gap={1} mt={2} flexWrap="wrap">
              {selectedReviewers.map((reviewer) => (
                <Label
                  key={reviewer.id}
                  sx={{
                    backgroundColor: "canvas.inset",
                    color: "fg.muted",
                    borderColor: "border.default",
                  }}
                >
                  {reviewer.text}
                </Label>
              ))}
            </Box>
          )}
        </Box>

        {/* Options and Submit */}
        <Box display="flex" justifyContent="space-between" alignItems="flex-end" flexWrap="wrap" gap={3}>
          <Box as="label" display="flex" alignItems="center" sx={{ cursor: "pointer", gap: 2, mt: 1 }}>
            <Checkbox checked={maintainerCanModify} onChange={(e) => setMaintainerCanModify(e.target.checked)} />
            <Text sx={{ fontSize: 1, color: "fg.muted" }}>Allow maintainer edits</Text>
          </Box>

          <ButtonGroup>
            <Button
              variant="primary"
              onClick={handleSubmit}
              disabled={isSubmitting || !owner || !repo || !baseBranch || !headBranch}
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
                  disabled={isSubmitting || !owner || !repo || !baseBranch || !headBranch}
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
