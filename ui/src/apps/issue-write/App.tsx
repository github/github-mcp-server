import { StrictMode, useState, useCallback, useEffect, useMemo, useRef } from "react";
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
  CounterLabel,
  ActionMenu,
  ActionList,
  Label,
} from "@primer/react";
import {
  IssueOpenedIcon,
  CheckCircleIcon,
  TagIcon,
  PersonIcon,
  RepoIcon,
  MilestoneIcon,
  LockIcon,
  TriangleDownIcon,
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

interface LabelItem {
  id: string;
  text: string;
  color: string;
}

interface AssigneeItem {
  id: string;
  text: string;
}

interface MilestoneItem {
  id: string;
  number: number;
  text: string;
  description: string;
}

interface IssueTypeItem {
  id: string;
  text: string;
}

interface RepositoryItem {
  id: string;
  owner: string;
  name: string;
  fullName: string;
  isPrivate: boolean;
}

// Calculate text color based on background luminance
function getContrastColor(hexColor: string): string {
  const r = parseInt(hexColor.substring(0, 2), 16);
  const g = parseInt(hexColor.substring(2, 4), 16);
  const b = parseInt(hexColor.substring(4, 6), 16);
  const luminance = (0.299 * r + 0.587 * g + 0.114 * b) / 255;
  return luminance > 0.5 ? "#000000" : "#ffffff";
}

function SuccessView({
  issue,
  owner,
  repo,
  submittedTitle,
  submittedLabels,
  isUpdate,
}: {
  issue: IssueResult;
  owner: string;
  repo: string;
  submittedTitle: string;
  submittedLabels: LabelItem[];
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
        <Box sx={{ color: "open.fg", flexShrink: 0, mt: "2px" }}>
          <IssueOpenedIcon size={16} />
        </Box>
        <Box sx={{ minWidth: 0 }}>
          <Link
            href={issueUrl}
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
            {issue.title || submittedTitle}
            {issue.number && (
              <Text sx={{ color: "fg.muted", fontWeight: "normal", ml: 1 }}>
                #{issue.number}
              </Text>
            )}
          </Link>
          <Text sx={{ color: "fg.muted", fontSize: 0 }}>
            {owner}/{repo}
          </Text>
          {submittedLabels.length > 0 && (
            <Box display="flex" gap={1} mt={2} flexWrap="wrap">
              {submittedLabels.map((label) => (
                <Label
                  key={label.id}
                  sx={{
                    backgroundColor: `#${label.color}`,
                    color: getContrastColor(label.color),
                    borderColor: `#${label.color}`,
                  }}
                >
                  {label.text}
                </Label>
              ))}
            </Box>
          )}
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

  // Labels state
  const [availableLabels, setAvailableLabels] = useState<LabelItem[]>([]);
  const [selectedLabels, setSelectedLabels] = useState<LabelItem[]>([]);
  const [labelsLoading, setLabelsLoading] = useState(false);
  const [labelsFilter, setLabelsFilter] = useState("");

  // Assignees state
  const [availableAssignees, setAvailableAssignees] = useState<AssigneeItem[]>([]);
  const [selectedAssignees, setSelectedAssignees] = useState<AssigneeItem[]>([]);
  const [assigneesLoading, setAssigneesLoading] = useState(false);
  const [assigneesFilter, setAssigneesFilter] = useState("");

  // Milestones state
  const [availableMilestones, setAvailableMilestones] = useState<MilestoneItem[]>([]);
  const [selectedMilestone, setSelectedMilestone] = useState<MilestoneItem | null>(null);
  const [milestonesLoading, setMilestonesLoading] = useState(false);

  // Issue types state
  const [availableIssueTypes, setAvailableIssueTypes] = useState<IssueTypeItem[]>([]);
  const [selectedIssueType, setSelectedIssueType] = useState<IssueTypeItem | null>(null);
  const [issueTypesLoading, setIssueTypesLoading] = useState(false);

  // Repository state
  const [selectedRepo, setSelectedRepo] = useState<RepositoryItem | null>(null);
  const [repoSearchResults, setRepoSearchResults] = useState<RepositoryItem[]>([]);
  const [repoSearchLoading, setRepoSearchLoading] = useState(false);
  const [repoFilter, setRepoFilter] = useState("");

  const { app, error: appError, toolInput, callTool } = useMcpApp({
    appName: "github-mcp-server-issue-write",
  });

  // Get method and issue_number from toolInput
  const method = (toolInput?.method as string) || "create";
  const issueNumber = toolInput?.issue_number as number | undefined;
  const isUpdateMode = method === "update" && issueNumber !== undefined;

  // Initialize from toolInput or selected repo
  const owner = selectedRepo?.owner || (toolInput?.owner as string) || "";
  const repo = selectedRepo?.name || (toolInput?.repo as string) || "";

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

  // Search repositories when filter changes
  useEffect(() => {
    if (!app || !repoFilter.trim()) {
      setRepoSearchResults([]);
      return;
    }

    const searchRepos = async () => {
      setRepoSearchLoading(true);
      try {
        console.log("Searching repositories with query:", repoFilter);
        const result = await callTool("search_repositories", {
          query: repoFilter,
          perPage: 10,
        });
        console.log("Repository search result:", result);
        if (result && !result.isError && result.content) {
          const textContent = result.content.find(
            (c: { type: string }) => c.type === "text"
          );
          if (textContent?.text) {
            const data = JSON.parse(textContent.text);
            console.log("Parsed repository data:", data);
            const repos = (data.repositories || data.items || []).map(
              (r: { id?: number; owner?: { login?: string } | string; name?: string; full_name?: string; private?: boolean }) => ({
                id: String(r.id || r.full_name),
                owner: typeof r.owner === 'string' ? r.owner : r.owner?.login || '',
                name: r.name || '',
                fullName: r.full_name || `${typeof r.owner === 'string' ? r.owner : r.owner?.login}/${r.name}`,
                isPrivate: r.private || false,
              })
            );
            console.log("Mapped repos:", repos);
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

  // Load labels, assignees, milestones, and issue types when owner/repo available
  useEffect(() => {
    if (!owner || !repo || !app) return;

    const loadLabels = async () => {
      setLabelsLoading(true);
      try {
        const result = await callTool("list_label", { owner, repo });
        console.log("Labels result:", result);
        if (result && !result.isError && result.content) {
          const textContent = result.content.find(
            (c: { type: string }) => c.type === "text"
          );
          if (textContent && "text" in textContent) {
            const data = JSON.parse(textContent.text as string);
            const labels = (data.labels || []).map(
              (l: { name: string; color: string; id: string }) => ({
                id: l.id || l.name,
                text: l.name,
                color: l.color,
              })
            );
            setAvailableLabels(labels);
          }
        }
      } catch (e) {
        console.error("Failed to load labels:", e);
      } finally {
        setLabelsLoading(false);
      }
    };

    const loadAssignees = async () => {
      setAssigneesLoading(true);
      try {
        const result = await callTool("list_assignees", { owner, repo });
        console.log("Assignees result:", result);
        if (result && !result.isError && result.content) {
          const textContent = result.content.find(
            (c: { type: string }) => c.type === "text"
          );
          if (textContent && "text" in textContent) {
            const data = JSON.parse(textContent.text as string);
            const assignees = (data.assignees || []).map(
              (a: { login: string }) => ({
                id: a.login,
                text: a.login,
              })
            );
            setAvailableAssignees(assignees);
          }
        }
      } catch (e) {
        console.error("Failed to load assignees:", e);
      } finally {
        setAssigneesLoading(false);
      }
    };

    const loadMilestones = async () => {
      setMilestonesLoading(true);
      try {
        const result = await callTool("list_milestones", { owner, repo });
        console.log("Milestones result:", result);
        if (result && !result.isError && result.content) {
          const textContent = result.content.find(
            (c: { type: string }) => c.type === "text"
          );
          if (textContent && "text" in textContent) {
            const data = JSON.parse(textContent.text as string);
            const milestones = (data.milestones || []).map(
              (m: { number: number; title: string; description: string }) => ({
                id: String(m.number),
                number: m.number,
                text: m.title,
                description: m.description || "",
              })
            );
            setAvailableMilestones(milestones);
          }
        }
      } catch (e) {
        console.error("Failed to load milestones:", e);
      } finally {
        setMilestonesLoading(false);
      }
    };

    const loadIssueTypes = async () => {
      setIssueTypesLoading(true);
      try {
        const result = await callTool("list_issue_types", { owner });
        console.log("Issue types result:", result);
        if (result && !result.isError && result.content) {
          const textContent = result.content.find(
            (c: { type: string }) => c.type === "text"
          );
          if (textContent && "text" in textContent) {
            const data = JSON.parse(textContent.text as string);
            // list_issue_types returns array directly or wrapped in issue_types/types
            const typesArray = Array.isArray(data) ? data : (data.issue_types || data.types || []);
            const types = typesArray.map(
              (t: { id: number; name: string; description?: string } | string) => {
                if (typeof t === "string") {
                  return { id: t, text: t };
                }
                return { id: String(t.id || t.name), text: t.name };
              }
            );
            setAvailableIssueTypes(types);
          }
        }
      } catch (e) {
        // Issue types may not be available for all repos/orgs
        console.debug("Issue types not available:", e);
      } finally {
        setIssueTypesLoading(false);
      }
    };

    loadLabels();
    loadAssignees();
    loadMilestones();
    loadIssueTypes();
  }, [owner, repo, app, callTool]);

  // Track which prefill fields have been applied to avoid re-applying after user edits
  const prefillApplied = useRef<{
    title: boolean;
    body: boolean;
    labels: boolean;
    assignees: boolean;
    milestone: boolean;
    type: boolean;
  }>({ title: false, body: false, labels: false, assignees: false, milestone: false, type: false });

  // Store existing issue data for matching when available lists load
  interface ExistingIssueData {
    labels: string[];
    assignees: string[];
    milestoneNumber: number | null;
    issueType: string | null;
  }
  const [existingIssueData, setExistingIssueData] = useState<ExistingIssueData | null>(null);

  // Reset prefill tracking when toolInput changes (new invocation)
  useEffect(() => {
    prefillApplied.current = { title: false, body: false, labels: false, assignees: false, milestone: false, type: false };
    setExistingIssueData(null);
  }, [toolInput]);

  // Load existing issue data when in update mode
  useEffect(() => {
    if (!isUpdateMode || !owner || !repo || !issueNumber || !app || existingIssueData !== null) {
      return;
    }

    const loadExistingIssue = async () => {
      try {
        console.log("Loading existing issue:", owner, repo, issueNumber);
        const result = await callTool("issue_read", {
          method: "get",
          owner,
          repo,
          issue_number: issueNumber,
        });

        if (result && !result.isError && result.content) {
          const textContent = result.content.find(
            (c: { type: string }) => c.type === "text"
          );
          if (textContent?.text) {
            const issueData = JSON.parse(textContent.text);
            console.log("Loaded issue data:", issueData);

            // Pre-fill title and body immediately
            if (issueData.title && !prefillApplied.current.title) {
              setTitle(issueData.title);
              prefillApplied.current.title = true;
            }
            if (issueData.body && !prefillApplied.current.body) {
              setBody(issueData.body);
              prefillApplied.current.body = true;
            }

            // Extract data for deferred matching when available lists load
            const labelNames = (issueData.labels || [])
              .map((l: { name?: string } | string) => typeof l === 'string' ? l : l.name)
              .filter(Boolean) as string[];
            
            const assigneeLogins = (issueData.assignees || [])
              .map((a: { login?: string } | string) => typeof a === 'string' ? a : a.login)
              .filter(Boolean) as string[];
            
            const milestoneNumber = issueData.milestone 
              ? (typeof issueData.milestone === 'object' ? issueData.milestone.number : issueData.milestone)
              : null;
            
            const issueType = issueData.issue_type?.name || issueData.type || null;

            setExistingIssueData({ labels: labelNames, assignees: assigneeLogins, milestoneNumber, issueType });
          }
        }
      } catch (e) {
        console.error("Error loading existing issue:", e);
      }
    };

    loadExistingIssue();
  }, [isUpdateMode, owner, repo, issueNumber, app, callTool, existingIssueData]);

  // Apply existing labels when available labels load
  useEffect(() => {
    if (!existingIssueData?.labels.length || !availableLabels.length || prefillApplied.current.labels) return;
    const matched = availableLabels.filter((l) => existingIssueData.labels.includes(l.text));
    if (matched.length > 0) {
      setSelectedLabels(matched);
      prefillApplied.current.labels = true;
    }
  }, [existingIssueData, availableLabels]);

  // Apply existing assignees when available assignees load
  useEffect(() => {
    if (!existingIssueData?.assignees.length || !availableAssignees.length || prefillApplied.current.assignees) return;
    const matched = availableAssignees.filter((a) => existingIssueData.assignees.includes(a.text));
    if (matched.length > 0) {
      setSelectedAssignees(matched);
      prefillApplied.current.assignees = true;
    }
  }, [existingIssueData, availableAssignees]);

  // Apply existing milestone when available milestones load
  useEffect(() => {
    if (!existingIssueData?.milestoneNumber || !availableMilestones.length || prefillApplied.current.milestone) return;
    const matched = availableMilestones.find((m) => m.number === existingIssueData.milestoneNumber);
    if (matched) {
      setSelectedMilestone(matched);
      prefillApplied.current.milestone = true;
    }
  }, [existingIssueData, availableMilestones]);

  // Apply existing issue type when available issue types load
  useEffect(() => {
    if (!existingIssueData?.issueType || !availableIssueTypes.length || prefillApplied.current.type) return;
    const matched = availableIssueTypes.find((t) => t.text === existingIssueData.issueType);
    if (matched) {
      setSelectedIssueType(matched);
      prefillApplied.current.type = true;
    }
  }, [existingIssueData, availableIssueTypes]);

  // Pre-fill title and body immediately (don't wait for data loading)
  useEffect(() => {
    if (toolInput?.title && !prefillApplied.current.title) {
      setTitle(toolInput.title as string);
      prefillApplied.current.title = true;
    }
    if (toolInput?.body && !prefillApplied.current.body) {
      setBody(toolInput.body as string);
      prefillApplied.current.body = true;
    }
  }, [toolInput]);

  // Pre-fill labels once available data is loaded
  useEffect(() => {
    if (
      toolInput?.labels &&
      Array.isArray(toolInput.labels) &&
      availableLabels.length > 0 &&
      !prefillApplied.current.labels
    ) {
      const prefillLabels = availableLabels.filter((l) =>
        (toolInput.labels as string[]).includes(l.text)
      );
      if (prefillLabels.length > 0) {
        setSelectedLabels(prefillLabels);
        prefillApplied.current.labels = true;
      }
    }
  }, [toolInput, availableLabels]);

  // Pre-fill assignees once available data is loaded
  useEffect(() => {
    if (
      toolInput?.assignees &&
      Array.isArray(toolInput.assignees) &&
      availableAssignees.length > 0 &&
      !prefillApplied.current.assignees
    ) {
      const prefillAssignees = availableAssignees.filter((a) =>
        (toolInput.assignees as string[]).includes(a.text)
      );
      if (prefillAssignees.length > 0) {
        setSelectedAssignees(prefillAssignees);
        prefillApplied.current.assignees = true;
      }
    }
  }, [toolInput, availableAssignees]);

  // Pre-fill milestone once available data is loaded
  useEffect(() => {
    if (
      toolInput?.milestone &&
      availableMilestones.length > 0 &&
      !prefillApplied.current.milestone
    ) {
      const milestone = availableMilestones.find(
        (m) => m.number === Number(toolInput.milestone)
      );
      if (milestone) {
        setSelectedMilestone(milestone);
        prefillApplied.current.milestone = true;
      }
    }
  }, [toolInput, availableMilestones]);

  // Pre-fill issue type once available data is loaded
  useEffect(() => {
    if (
      toolInput?.type &&
      availableIssueTypes.length > 0 &&
      !prefillApplied.current.type
    ) {
      const issueType = availableIssueTypes.find(
        (t) => t.text === toolInput.type
      );
      if (issueType) {
        setSelectedIssueType(issueType);
        prefillApplied.current.type = true;
      }
    }
  }, [toolInput, availableIssueTypes]);

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
        show_ui: false, // Execute the action directly, don't show UI again
      };

      if (isUpdateMode && issueNumber) {
        params.issue_number = issueNumber;
      }

      if (selectedLabels.length > 0) {
        params.labels = selectedLabels.map((l) => l.text);
      }
      if (selectedAssignees.length > 0) {
        params.assignees = selectedAssignees.map((a) => a.text);
      }
      if (selectedMilestone) {
        params.milestone = selectedMilestone.number;
      }
      if (selectedIssueType) {
        params.type = selectedIssueType.text;
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
  }, [title, body, owner, repo, selectedLabels, selectedAssignees, selectedMilestone, selectedIssueType, callTool]);

  // Filtered items for dropdowns
  const filteredLabels = useMemo(() => {
    if (!labelsFilter) return availableLabels;
    const lowerFilter = labelsFilter.toLowerCase();
    return availableLabels.filter((l) =>
      l.text.toLowerCase().includes(lowerFilter)
    );
  }, [availableLabels, labelsFilter]);

  const filteredAssignees = useMemo(() => {
    if (!assigneesFilter) return availableAssignees;
    const lowerFilter = assigneesFilter.toLowerCase();
    return availableAssignees.filter((a) =>
      a.text.toLowerCase().includes(lowerFilter)
    );
  }, [availableAssignees, assigneesFilter]);

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
        submittedLabels={selectedLabels}
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
                  onChange={(e) => {
                    console.log("Repo filter changed:", e.target.value);
                    setRepoFilter(e.target.value);
                  }}
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
                      // Clear metadata when switching repos
                      setAvailableLabels([]);
                      setSelectedLabels([]);
                      setAvailableAssignees([]);
                      setSelectedAssignees([]);
                      setAvailableMilestones([]);
                      setSelectedMilestone(null);
                      setAvailableIssueTypes([]);
                      setSelectedIssueType(null);
                    }}
                  >
                    <ActionList.LeadingVisual>
                      {r.isPrivate ? <LockIcon /> : <RepoIcon />}
                    </ActionList.LeadingVisual>
                    {r.fullName}
                  </ActionList.Item>
                ))
              ) : selectedRepo ? (
                <ActionList.Item
                  key={selectedRepo.id}
                  selected
                  onSelect={() => setRepoFilter("")}
                >
                  <ActionList.LeadingVisual>
                    {selectedRepo.isPrivate ? <LockIcon /> : <RepoIcon />}
                  </ActionList.LeadingVisual>
                  {selectedRepo.fullName}
                </ActionList.Item>
              ) : (
                <Box px={3} py={2}>
                  <Text sx={{ color: "fg.muted", fontSize: 1 }}>
                    Type to search repositories...
                  </Text>
                </Box>
              )}
            </ActionList>
          </ActionMenu.Overlay>
        </ActionMenu>
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

      {/* Metadata section */}
      <Box display="flex" gap={1} mb={3} sx={{ flexWrap: "nowrap", overflow: "hidden" }}>
        {/* Labels dropdown */}
        <ActionMenu>
          <ActionMenu.Button size="small" leadingVisual={TagIcon}>
            Labels
            {selectedLabels.length > 0 && (
              <CounterLabel sx={{ ml: 1 }}>{selectedLabels.length}</CounterLabel>
            )}
          </ActionMenu.Button>
          <ActionMenu.Overlay width="medium">
            <Box p={2} borderBottomWidth={1} borderBottomStyle="solid" borderBottomColor="border.default">
              <TextInput
                placeholder="Filter labels"
                value={labelsFilter}
                onChange={(e) => setLabelsFilter(e.target.value)}
                size="small"
                block
              />
            </Box>
            <ActionList selectionVariant="multiple">
              {labelsLoading ? (
                <ActionList.Item disabled>
                  <Spinner size="small" /> Loading...
                </ActionList.Item>
              ) : filteredLabels.length === 0 ? (
                <ActionList.Item disabled>No labels available</ActionList.Item>
              ) : (
                filteredLabels.map((label) => (
                  <ActionList.Item
                    key={label.id}
                    selected={selectedLabels.some((l) => l.id === label.id)}
                    onSelect={() => {
                      setSelectedLabels((prev) =>
                        prev.some((l) => l.id === label.id)
                          ? prev.filter((l) => l.id !== label.id)
                          : [...prev, label]
                      );
                    }}
                  >
                    <ActionList.LeadingVisual>
                      <Box
                        sx={{
                          width: 14,
                          height: 14,
                          borderRadius: "50%",
                          backgroundColor: `#${label.color}`,
                        }}
                      />
                    </ActionList.LeadingVisual>
                    {label.text}
                  </ActionList.Item>
                ))
              )}
            </ActionList>
          </ActionMenu.Overlay>
        </ActionMenu>

        {/* Assignees dropdown */}
        <ActionMenu>
          <ActionMenu.Button size="small" leadingVisual={PersonIcon}>
            Assignees
            {selectedAssignees.length > 0 && (
              <CounterLabel sx={{ ml: 1 }}>{selectedAssignees.length}</CounterLabel>
            )}
          </ActionMenu.Button>
          <ActionMenu.Overlay width="medium">
            <Box p={2} borderBottomWidth={1} borderBottomStyle="solid" borderBottomColor="border.default">
              <TextInput
                placeholder="Search people"
                value={assigneesFilter}
                onChange={(e) => setAssigneesFilter(e.target.value)}
                size="small"
                block
              />
            </Box>
            <ActionList selectionVariant="multiple">
              {assigneesLoading ? (
                <ActionList.Item disabled>
                  <Spinner size="small" /> Loading...
                </ActionList.Item>
              ) : filteredAssignees.length === 0 ? (
                <ActionList.Item disabled>No assignees available</ActionList.Item>
              ) : (
                filteredAssignees.map((assignee) => (
                  <ActionList.Item
                    key={assignee.id}
                    selected={selectedAssignees.some((a) => a.id === assignee.id)}
                    onSelect={() => {
                      setSelectedAssignees((prev) =>
                        prev.some((a) => a.id === assignee.id)
                          ? prev.filter((a) => a.id !== assignee.id)
                          : [...prev, assignee]
                      );
                    }}
                  >
                    {assignee.text}
                  </ActionList.Item>
                ))
              )}
            </ActionList>
          </ActionMenu.Overlay>
        </ActionMenu>

        {/* Milestones dropdown */}
        <ActionMenu>
          <ActionMenu.Button size="small" leadingVisual={MilestoneIcon}>
            {selectedMilestone ? selectedMilestone.text : "Milestone"}
          </ActionMenu.Button>
          <ActionMenu.Overlay width="medium">
            <ActionList selectionVariant="single">
              {milestonesLoading ? (
                <ActionList.Item disabled>
                  <Spinner size="small" /> Loading...
                </ActionList.Item>
              ) : availableMilestones.length === 0 ? (
                <ActionList.Item disabled>No milestones</ActionList.Item>
              ) : (
                <>
                  {selectedMilestone && (
                    <ActionList.Item
                      onSelect={() => setSelectedMilestone(null)}
                    >
                      Clear selection
                    </ActionList.Item>
                  )}
                  {availableMilestones.map((milestone) => (
                    <ActionList.Item
                      key={milestone.id}
                      selected={selectedMilestone?.id === milestone.id}
                      onSelect={() => setSelectedMilestone(milestone)}
                    >
                      {milestone.text}
                      {milestone.description && (
                        <ActionList.Description>
                          {milestone.description}
                        </ActionList.Description>
                      )}
                    </ActionList.Item>
                  ))}
                </>
              )}
            </ActionList>
          </ActionMenu.Overlay>
        </ActionMenu>

        {/* Issue Types dropdown */}
        <ActionMenu>
          <ActionMenu.Button size="small" leadingVisual={IssueOpenedIcon}>
            {selectedIssueType ? selectedIssueType.text : "Type"}
          </ActionMenu.Button>
          <ActionMenu.Overlay width="medium">
            <ActionList selectionVariant="single">
              {issueTypesLoading ? (
                <ActionList.Item disabled>
                  <Spinner size="small" /> Loading...
                </ActionList.Item>
              ) : availableIssueTypes.length === 0 ? (
                <ActionList.Item disabled>No issue types</ActionList.Item>
              ) : (
                <>
                  {selectedIssueType && (
                    <ActionList.Item
                      onSelect={() => setSelectedIssueType(null)}
                    >
                      Clear selection
                    </ActionList.Item>
                  )}
                  {availableIssueTypes.map((type) => (
                    <ActionList.Item
                      key={type.id}
                      selected={selectedIssueType?.id === type.id}
                      onSelect={() => setSelectedIssueType(type)}
                    >
                      {type.text}
                    </ActionList.Item>
                  ))}
                </>
              )}
            </ActionList>
          </ActionMenu.Overlay>
        </ActionMenu>
      </Box>

      {/* Selected labels display */}
      {selectedLabels.length > 0 && (
        <Box display="flex" gap={1} mb={3} flexWrap="wrap">
          {selectedLabels.map((label) => (
            <Label
              key={label.id}
              sx={{
                backgroundColor: `#${label.color}`,
                color: getContrastColor(label.color),
                borderColor: `#${label.color}`,
              }}
            >
              {label.text}
            </Label>
          ))}
        </Box>
      )}

      {/* Selected metadata display */}
      {(selectedAssignees.length > 0 || selectedMilestone) && (
        <Box mb={3} sx={{ fontSize: 0, color: "fg.muted" }}>
          {selectedAssignees.length > 0 && (
            <Text as="div">
              Assigned to: {selectedAssignees.map((a) => a.text).join(", ")}
            </Text>
          )}
          {selectedMilestone && (
            <Text as="div">Milestone: {selectedMilestone.text}</Text>
          )}
        </Box>
      )}

      {/* Submit button */}
      <Button
        variant="primary"
        onClick={handleSubmit}
        disabled={isSubmitting || !title.trim()}
        sx={{ width: "100%" }}
      >
        {isSubmitting ? (
          <>
            <Spinner size="small" sx={{ mr: 2 }} />
            {isUpdateMode ? "Updating..." : "Creating..."}
          </>
        ) : (
          isUpdateMode ? "Update issue" : "Create issue"
        )}
      </Button>
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
