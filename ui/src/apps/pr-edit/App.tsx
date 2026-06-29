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
  GitBranchIcon,
  LockIcon,
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

interface BranchItem {
  name: string;
  protected: boolean;
}

type ReviewerItem = { kind: "user" | "team"; id: string; text: string; avatar?: string; org?: string };
type PRState = "open" | "closed";

interface InitialPRState {
  title: string;
  body: string;
  state: PRState;
  base: string;
  draft: boolean;
  maintainerCanModify: boolean;
  reviewers: string[];
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null ? (value as Record<string, unknown>) : null;
}

function asString(value: unknown): string | undefined {
  return typeof value === "string" ? value : undefined;
}

function asBoolean(value: unknown): boolean | undefined {
  return typeof value === "boolean" ? value : undefined;
}

function asNumber(value: unknown): number | undefined {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value === "string") {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) return parsed;
  }
  return undefined;
}

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

function sameReviewerValues(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false;
  const sortedA = [...a].sort();
  const sortedB = [...b].sort();
  return sortedA.every((value, index) => value === sortedB[index]);
}

function parseUserReviewer(value: unknown): ReviewerItem | null {
  if (typeof value === "string") return reviewerFromValue(value);
  const record = asRecord(value);
  const login = asString(record?.login);
  if (!login) return null;
  return { kind: "user", id: login, text: login, avatar: asString(record?.avatar_url) };
}

function parseTeamReviewer(value: unknown, fallbackOrg: string): ReviewerItem | null {
  if (typeof value === "string") {
    if (value.includes("/")) return reviewerFromValue(value);
    return { kind: "team", id: `${fallbackOrg}/${value}`, text: `${fallbackOrg}/${value}`, org: fallbackOrg };
  }

  const record = asRecord(value);
  const slug = asString(record?.slug) || asString(record?.name);
  if (!slug) return null;

  const organization = asRecord(record?.organization);
  const org = asString(record?.org) || asString(organization?.login) || fallbackOrg;
  const id = org ? `${org}/${slug}` : slug;
  return { kind: "team", id, text: id, org };
}

function reviewersFromValues(values: unknown): ReviewerItem[] | undefined {
  if (!Array.isArray(values)) return undefined;
  return values
    .map((value) => (typeof value === "string" ? reviewerFromValue(value) : null))
    .filter((value): value is ReviewerItem => value !== null);
}

function extractRequestedReviewers(prData: Record<string, unknown>, owner: string): ReviewerItem[] {
  const requestedReviewers = Array.isArray(prData.requested_reviewers) ? prData.requested_reviewers : [];
  const requestedTeams = Array.isArray(prData.requested_teams) ? prData.requested_teams : [];
  return [
    ...requestedReviewers.map(parseUserReviewer).filter((value): value is ReviewerItem => value !== null),
    ...requestedTeams.map((team) => parseTeamReviewer(team, owner)).filter((value): value is ReviewerItem => value !== null),
  ];
}

function parsePRState(value: unknown): PRState {
  return value === "closed" ? "closed" : "open";
}

function buildInitialState(prData: Record<string, unknown>, owner: string): InitialPRState {
  const base = asRecord(prData.base);
  const requestedReviewers = extractRequestedReviewers(prData, owner);
  return {
    title: asString(prData.title) || "",
    body: asString(prData.body) || "",
    state: parsePRState(prData.state),
    base: asString(base?.ref) || "",
    draft: asBoolean(prData.draft) || false,
    maintainerCanModify: asBoolean(prData.maintainer_can_modify) || false,
    reviewers: requestedReviewers.map(reviewerValue),
  };
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
          Pull request updated successfully
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

function EditPRApp() {
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [prState, setPRState] = useState<PRState>("open");
  const [isDraft, setIsDraft] = useState(false);
  const [maintainerCanModify, setMaintainerCanModify] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successPR, setSuccessPR] = useState<PRResult | null>(null);
  const [initialValues, setInitialValues] = useState<InitialPRState | null>(null);
  const [isLoadingPR, setIsLoadingPR] = useState(false);
  const [submittedTitle, setSubmittedTitle] = useState("");

  const [availableBranches, setAvailableBranches] = useState<BranchItem[]>([]);
  const [baseBranch, setBaseBranch] = useState<string>("");
  const [branchesLoading, setBranchesLoading] = useState(false);
  const [baseFilter, setBaseFilter] = useState("");

  const [availableReviewers, setAvailableReviewers] = useState<ReviewerItem[]>([]);
  const [selectedReviewers, setSelectedReviewers] = useState<ReviewerItem[]>([]);
  const [reviewersLoading, setReviewersLoading] = useState(false);
  const [reviewersFilter, setReviewersFilter] = useState("");

  const { app, error: appError, toolInput, toolResult, callTool, hostContext, setModelContext, openLink } = useMcpApp({
    appName: "github-mcp-server-edit-pull-request",
  });

  const owner = (toolInput?.owner as string) || "";
  const repo = (toolInput?.repo as string) || "";
  const pullNumber = asNumber(toolInput?.pullNumber);

  // When the server updated the PR up-front instead of deferring to this form,
  // the host still renders this View and delivers the updated PR via
  // tool-result. Render that completed result as success so we never show an
  // edit form for changes already applied. The deferral sentinel
  // (awaiting_user_submission) returns null here, keeping the form for the
  // normal deferred flow. See github/copilot-mcp-core#1864.
  const resultPR = useMemo(() => completedToolResult<PRResult>(toolResult), [toolResult]);
  const shownPR = successPR ?? resultPR;

  useEffect(() => {
    setTitle("");
    setBody("");
    setPRState("open");
    setIsDraft(false);
    setMaintainerCanModify(false);
    setBaseBranch("");
    setSelectedReviewers([]);
    setAvailableBranches([]);
    setAvailableReviewers([]);
    setBaseFilter("");
    setReviewersFilter("");
    setInitialValues(null);
    setSuccessPR(null);
    setError(null);
    setSubmittedTitle("");
  }, [toolInput]);

  useEffect(() => {
    if (!app || !owner || !repo || !pullNumber) return;

    let cancelled = false;

    const loadPullRequest = async () => {
      setIsLoadingPR(true);
      try {
        const result = await callTool("pull_request_read", { method: "get", owner, repo, pullNumber });
        if (cancelled) return;

        if (result.isError) {
          const textContent = result.content?.find((c) => c.type === "text");
          const errorMessage = textContent && textContent.type === "text" ? textContent.text : "Failed to load pull request";
          setError(errorMessage);
          return;
        }

        const textContent = result.content?.find((c) => c.type === "text");
        if (!textContent || textContent.type !== "text" || !textContent.text) {
          setError("Pull request details were not returned");
          return;
        }

        const prData = JSON.parse(textContent.text) as Record<string, unknown>;
        const initialState = buildInitialState(prData, owner);
        const toolInputReviewers = reviewersFromValues(toolInput?.reviewers);

        setInitialValues(initialState);
        setTitle(asString(toolInput?.title) ?? initialState.title);
        setBody(asString(toolInput?.body) ?? initialState.body);
        setPRState(parsePRState(asString(toolInput?.state) ?? initialState.state));
        setIsDraft(asBoolean(toolInput?.draft) ?? initialState.draft);
        setBaseBranch(asString(toolInput?.base) ?? initialState.base);
        setMaintainerCanModify(asBoolean(toolInput?.maintainer_can_modify) ?? initialState.maintainerCanModify);
        setSelectedReviewers(toolInputReviewers ?? extractRequestedReviewers(prData, owner));
      } catch (e) {
        if (!cancelled) {
          setError(e instanceof Error ? e.message : "Failed to load pull request");
        }
      } finally {
        if (!cancelled) setIsLoadingPR(false);
      }
    };

    loadPullRequest();
    return () => {
      cancelled = true;
    };
  }, [app, callTool, owner, repo, pullNumber, toolInput]);

  useEffect(() => {
    if (!owner || !repo || !app) return;

    let cancelled = false;

    const loadBranches = async () => {
      setBranchesLoading(true);
      try {
        const result = await callTool("ui_get", { method: "branches", owner, repo });
        if (cancelled) return;
        if (result && !result.isError && result.content) {
          const textContent = result.content.find((c: { type: string }) => c.type === "text");
          if (textContent && "text" in textContent) {
            const data = JSON.parse(textContent.text as string);
            const branches = (data.branches || data || []).map(
              (b: { name: string; protected?: boolean }) => ({ name: b.name, protected: b.protected || false })
            );
            setAvailableBranches(branches);
            const defaultBranch = branches.find((b: BranchItem) => b.name === "main" || b.name === "master");
            if (defaultBranch) setBaseBranch((prev) => prev || defaultBranch.name);
          }
        }
      } catch (e) {
        console.error("Failed to load branches:", e);
      } finally {
        if (!cancelled) setBranchesLoading(false);
      }
    };

    const loadReviewers = async () => {
      setReviewersLoading(true);
      try {
        const result = await callTool("ui_get", { method: "reviewers", owner, repo });
        if (cancelled) return;
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
        if (!cancelled) setReviewersLoading(false);
      }
    };

    loadBranches();
    loadReviewers();

    return () => {
      cancelled = true;
    };
  }, [owner, repo, app, callTool]);

  useEffect(() => {
    if (availableReviewers.length === 0) return;
    setSelectedReviewers((prev) =>
      prev.map((reviewer) =>
        availableReviewers.find((available) => available.id === reviewer.id || available.text === reviewer.text) || reviewer
      )
    );
  }, [availableReviewers]);

  const filteredBaseBranches = useMemo(() => {
    if (!baseFilter.trim()) return availableBranches;
    return availableBranches.filter((branch) => branch.name.toLowerCase().includes(baseFilter.toLowerCase()));
  }, [availableBranches, baseFilter]);

  const filteredReviewers = useMemo(() => {
    if (!reviewersFilter.trim()) return availableReviewers;
    const lowerFilter = reviewersFilter.toLowerCase();
    return availableReviewers.filter((reviewer) =>
      reviewer.text.toLowerCase().includes(lowerFilter) || reviewer.id.toLowerCase().includes(lowerFilter)
    );
  }, [availableReviewers, reviewersFilter]);

  const handleSubmit = useCallback(async () => {
    if (!title.trim()) { setError("Title is required"); return; }
    if (!owner || !repo || !pullNumber) { setError("Pull request information not available"); return; }
    if (!baseBranch) { setError("Base branch is required"); return; }
    if (!initialValues) { setError("Pull request details are still loading"); return; }

    const selectedReviewerValues = selectedReviewers.map(reviewerValue);
    const params: Record<string, unknown> = { owner, repo, pullNumber, _ui_submitted: true };

    if (title.trim() !== initialValues.title) params.title = title.trim();
    if (body !== initialValues.body) params.body = body;
    if (prState !== initialValues.state) params.state = prState;
    if (baseBranch !== initialValues.base) params.base = baseBranch;
    if (isDraft !== initialValues.draft) params.draft = isDraft;
    if (maintainerCanModify !== initialValues.maintainerCanModify) params.maintainer_can_modify = maintainerCanModify;
    if (!sameReviewerValues(selectedReviewerValues, initialValues.reviewers)) params.reviewers = selectedReviewerValues;

    const hasChanges = Object.keys(params).some((key) => !["owner", "repo", "pullNumber", "_ui_submitted"].includes(key));
    if (!hasChanges) {
      setError("No changes to update");
      return;
    }

    setIsSubmitting(true);
    setError(null);
    setSubmittedTitle(title);

    try {
      const result = await callTool("update_pull_request", params);

      if (result.isError) {
        const textContent = result.content?.find((c) => c.type === "text");
        const errorMessage = textContent && textContent.type === "text" ? textContent.text : "Failed to update pull request";
        setError(errorMessage);
      } else {
        const textContent = result.content?.find((c) => c.type === "text");
        if (textContent && textContent.type === "text" && textContent.text) {
          const prData = JSON.parse(textContent.text);
          setSuccessPR(prData);
          void setModelContext({
            structuredContent: prData,
            content: [
              {
                type: "text",
                text: `Pull request #${pullNumber} in ${owner}/${repo} was updated by the user via the edit-pull-request view.`,
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
  }, [title, body, owner, repo, pullNumber, baseBranch, initialValues, selectedReviewers, prState, isDraft, maintainerCanModify, callTool, setModelContext]);

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

  if (toolInput === null) {
    return (
      <AppProvider hostContext={hostContext}>
        <Box display="flex" alignItems="center" justifyContent="center" p={4}>
          <Spinner size="medium" />
        </Box>
      </AppProvider>
    );
  }

  if (!owner || !repo || !pullNumber) {
    return (
      <AppProvider hostContext={hostContext}>
        <Flash variant="danger">Pull request owner, repo, and pullNumber are required.</Flash>
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
          <Box sx={{ color: "open.fg", flexShrink: 0 }}>
            <GitPullRequestIcon size={16} />
          </Box>
          <Box sx={{ minWidth: 0 }}>
            <Text sx={{ fontWeight: "semibold" }}>#{pullNumber} · {owner}/{repo}</Text>
            {title && (
              <Text as="div" sx={{ color: "fg.muted", fontSize: 0, mt: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                {title}
              </Text>
            )}
          </Box>
        </Box>

        {error && <Flash variant="danger" sx={{ mb: 3 }}>{error}</Flash>}

        {isLoadingPR && !initialValues ? (
          <Box display="flex" alignItems="center" justifyContent="center" p={4}>
            <Spinner size="medium" />
            <Text sx={{ color: "fg.muted", ml: 2 }}>Loading pull request...</Text>
          </Box>
        ) : (
          <>
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

            <Box sx={{ mb: 3 }}>
              <Text as="label" sx={{ fontWeight: "semibold", fontSize: 1, display: "block", mb: 2 }}>
                Description
              </Text>
              <MarkdownEditor value={body} onChange={setBody} placeholder="Add a description..." />
            </Box>

            <Box sx={{ mb: 3, flex: "1 1 240px", minWidth: 0 }}>
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

            <Box display="flex" justifyContent="space-between" alignItems="flex-end" flexWrap="wrap" gap={3} mb={3}>
              <Box>
                <Text sx={{ fontSize: 0, color: "fg.muted", mb: 1, display: "block" }}>state</Text>
                <ButtonGroup>
                  <Button size="small" variant={prState === "open" ? "primary" : "default"} onClick={() => setPRState("open")}>
                    Open
                  </Button>
                  <Button size="small" variant={prState === "closed" ? "primary" : "default"} onClick={() => setPRState("closed")}>
                    Closed
                  </Button>
                </ButtonGroup>
              </Box>

              <Box as="label" display="flex" alignItems="center" sx={{ cursor: "pointer", gap: 2, mb: "6px" }}>
                <Checkbox checked={isDraft} onChange={(e) => setIsDraft(e.target.checked)} />
                <Text sx={{ fontSize: 1, color: "fg.muted" }}>Mark as draft</Text>
              </Box>
            </Box>

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

            <Box display="flex" justifyContent="space-between" alignItems="flex-end" flexWrap="wrap" gap={3}>
              <Box as="label" display="flex" alignItems="center" sx={{ cursor: "pointer", gap: 2, mt: 1 }}>
                <Checkbox checked={maintainerCanModify} onChange={(e) => setMaintainerCanModify(e.target.checked)} />
                <Text sx={{ fontSize: 1, color: "fg.muted" }}>Allow maintainer edits</Text>
              </Box>

              <Button
                variant="primary"
                onClick={handleSubmit}
                disabled={isSubmitting || !initialValues || !title.trim() || !baseBranch}
              >
                {isSubmitting ? (
                  <><Spinner size="small" sx={{ mr: 1 }} />Updating...</>
                ) : (
                  "Update pull request"
                )}
              </Button>
            </Box>
          </>
        )}
      </Box>
    </AppProvider>
  );
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <EditPRApp />
  </StrictMode>
);
