package github

import (
	"net/mail"
	"strconv"
	"strings"

	"github.com/google/go-github/v89/github"
)

func sanitizeOutputStringPtr(s *string) *string {
	if s == nil {
		return nil
	}
	sanitized := sanitizeOutputText(*s)
	return &sanitized
}

type protectedCommitTrailerAddress struct {
	placeholder string
	address     string
}

func sanitizeCommitMessage(message string) string {
	lines := strings.Split(message, "\n")
	protected := make([]protectedCommitTrailerAddress, 0)

	for i, line := range lines {
		address, ok := commitTrailerAddress(line)
		if !ok {
			continue
		}

		placeholder := commitTrailerAddressPlaceholder(message, len(protected))
		addressIndex := strings.LastIndex(line, address)
		lines[i] = line[:addressIndex] + placeholder + line[addressIndex+len(address):]
		protected = append(protected, protectedCommitTrailerAddress{
			placeholder: placeholder,
			address:     address,
		})
	}

	sanitized := sanitizeOutputText(strings.Join(lines, "\n"))
	for _, trailerAddress := range protected {
		sanitized = strings.ReplaceAll(sanitized, trailerAddress.placeholder, trailerAddress.address)
	}
	return sanitized
}

func sanitizeCommitMessagePtr(message *string) *string {
	if message == nil {
		return nil
	}
	sanitized := sanitizeCommitMessage(*message)
	return &sanitized
}

func commitTrailerAddress(line string) (string, bool) {
	colonIndex := strings.IndexByte(line, ':')
	if colonIndex <= 0 || !isCommitTrailerToken(line[:colonIndex]) {
		return "", false
	}

	value := strings.TrimSpace(line[colonIndex+1:])
	if !strings.HasSuffix(value, ">") {
		return "", false
	}
	openIndex := strings.LastIndexByte(value, '<')
	if openIndex < 0 || openIndex == len(value)-1 {
		return "", false
	}

	email := value[openIndex+1 : len(value)-1]
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != email {
		return "", false
	}
	return "<" + email + ">", true
}

func isCommitTrailerToken(token string) bool {
	if token == "" {
		return false
	}
	for i := range len(token) {
		c := token[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' {
			continue
		}
		return false
	}
	return true
}

func commitTrailerAddressPlaceholder(message string, index int) string {
	placeholder := "GITHUBMCPCOMMITTRAILEREMAIL" + strconv.Itoa(index) + "PLACEHOLDER"
	for strings.Contains(message, placeholder) {
		placeholder += "X"
	}
	return placeholder
}

func sanitizedIssueCopy(issue *github.Issue) *github.Issue {
	if issue == nil {
		return nil
	}
	issueCopy := *issue
	issueCopy.Title = sanitizeOutputStringPtr(issue.Title)
	issueCopy.Body = sanitizeOutputStringPtr(issue.Body)
	issueCopy.Labels = sanitizedLabelsCopy(issue.Labels)
	issueCopy.Milestone = sanitizedMilestoneCopy(issue.Milestone)
	issueCopy.Type = sanitizedIssueTypeCopy(issue.Type)
	return &issueCopy
}

func sanitizedSubIssueCopy(issue *github.SubIssue) *github.SubIssue {
	if issue == nil {
		return nil
	}
	issueCopy := *issue
	issueCopy.Title = sanitizeOutputStringPtr(issue.Title)
	issueCopy.Body = sanitizeOutputStringPtr(issue.Body)
	issueCopy.Labels = sanitizedLabelsCopy(issue.Labels)
	issueCopy.Milestone = sanitizedMilestoneCopy(issue.Milestone)
	issueCopy.Type = sanitizedIssueTypeCopy(issue.Type)
	return &issueCopy
}

func sanitizedSubIssuesCopy(issues []*github.SubIssue) []*github.SubIssue {
	if issues == nil {
		return nil
	}
	issuesCopy := make([]*github.SubIssue, len(issues))
	for i, issue := range issues {
		issuesCopy[i] = sanitizedSubIssueCopy(issue)
	}
	return issuesCopy
}

func sanitizedIssuesSearchResultCopy(result *github.IssuesSearchResult) *github.IssuesSearchResult {
	if result == nil {
		return nil
	}
	resultCopy := *result
	if result.Issues != nil {
		resultCopy.Issues = make([]*github.Issue, len(result.Issues))
		for i, issue := range result.Issues {
			resultCopy.Issues[i] = sanitizedIssueCopy(issue)
		}
	}
	return &resultCopy
}

func sanitizedLabelsCopy(labels []*github.Label) []*github.Label {
	if labels == nil {
		return nil
	}
	labelsCopy := make([]*github.Label, len(labels))
	for i, label := range labels {
		if label == nil {
			continue
		}
		labelCopy := *label
		labelCopy.Name = sanitizeOutputStringPtr(label.Name)
		labelCopy.Description = sanitizeOutputStringPtr(label.Description)
		labelsCopy[i] = &labelCopy
	}
	return labelsCopy
}

func sanitizedMilestoneCopy(milestone *github.Milestone) *github.Milestone {
	if milestone == nil {
		return nil
	}
	milestoneCopy := *milestone
	milestoneCopy.Title = sanitizeOutputStringPtr(milestone.Title)
	milestoneCopy.Description = sanitizeOutputStringPtr(milestone.Description)
	return &milestoneCopy
}

func sanitizedIssueTypeCopy(issueType *github.IssueType) *github.IssueType {
	if issueType == nil {
		return nil
	}
	issueTypeCopy := *issueType
	issueTypeCopy.Name = sanitizeOutputStringPtr(issueType.Name)
	issueTypeCopy.Description = sanitizeOutputStringPtr(issueType.Description)
	return &issueTypeCopy
}

func sanitizedIssueTypesCopy(issueTypes []*github.IssueType) []*github.IssueType {
	if issueTypes == nil {
		return nil
	}
	issueTypesCopy := make([]*github.IssueType, len(issueTypes))
	for i, issueType := range issueTypes {
		issueTypesCopy[i] = sanitizedIssueTypeCopy(issueType)
	}
	return issueTypesCopy
}

func sanitizedRepositoryCopy(repo *github.Repository) *github.Repository {
	if repo == nil {
		return nil
	}
	repoCopy := *repo
	repoCopy.Description = sanitizeOutputStringPtr(repo.Description)
	return &repoCopy
}

func sanitizedRepositoriesSearchResultCopy(result *github.RepositoriesSearchResult) *github.RepositoriesSearchResult {
	if result == nil {
		return nil
	}
	resultCopy := *result
	if result.Repositories != nil {
		resultCopy.Repositories = make([]*github.Repository, len(result.Repositories))
		for i, repo := range result.Repositories {
			resultCopy.Repositories[i] = sanitizedRepositoryCopy(repo)
		}
	}
	return &resultCopy
}

func sanitizedReleaseCopy(release *github.RepositoryRelease) *github.RepositoryRelease {
	if release == nil {
		return nil
	}
	releaseCopy := *release
	releaseCopy.Name = sanitizeOutputStringPtr(release.Name)
	releaseCopy.Body = sanitizeOutputStringPtr(release.Body)
	return &releaseCopy
}

func sanitizedCommitAuthorCopy(author *github.CommitAuthor) *github.CommitAuthor {
	if author == nil {
		return nil
	}
	authorCopy := *author
	authorCopy.Name = sanitizeOutputStringPtr(author.Name)
	return &authorCopy
}

func sanitizedCommitCopy(commit *github.Commit) *github.Commit {
	if commit == nil {
		return nil
	}
	commitCopy := *commit
	commitCopy.Message = sanitizeCommitMessagePtr(commit.Message)
	commitCopy.Author = sanitizedCommitAuthorCopy(commit.Author)
	commitCopy.Committer = sanitizedCommitAuthorCopy(commit.Committer)
	return &commitCopy
}

func sanitizedHeadCommitCopy(commit *github.HeadCommit) *github.HeadCommit {
	if commit == nil {
		return nil
	}
	commitCopy := *commit
	commitCopy.Message = sanitizeCommitMessagePtr(commit.Message)
	commitCopy.Author = sanitizedCommitAuthorCopy(commit.Author)
	commitCopy.Committer = sanitizedCommitAuthorCopy(commit.Committer)
	return &commitCopy
}

func sanitizedProjectV2TextContentCopy(content *github.ProjectV2TextContent) *github.ProjectV2TextContent {
	if content == nil {
		return nil
	}
	contentCopy := *content
	contentCopy.Raw = sanitizeOutputStringPtr(content.Raw)
	contentCopy.HTML = sanitizeOutputStringPtr(content.HTML)
	return &contentCopy
}

func sanitizedProjectV2FieldOptionCopy(option *github.ProjectV2FieldOption) *github.ProjectV2FieldOption {
	if option == nil {
		return nil
	}
	optionCopy := *option
	optionCopy.Name = sanitizedProjectV2TextContentCopy(option.Name)
	optionCopy.Description = sanitizedProjectV2TextContentCopy(option.Description)
	return &optionCopy
}

func sanitizedProjectV2FieldIterationCopy(iteration *github.ProjectV2FieldIteration) *github.ProjectV2FieldIteration {
	if iteration == nil {
		return nil
	}
	iterationCopy := *iteration
	iterationCopy.Title = sanitizedProjectV2TextContentCopy(iteration.Title)
	return &iterationCopy
}

func sanitizedProjectV2FieldConfigurationCopy(configuration *github.ProjectV2FieldConfiguration) *github.ProjectV2FieldConfiguration {
	if configuration == nil {
		return nil
	}
	configurationCopy := *configuration
	if configuration.Iterations != nil {
		configurationCopy.Iterations = make([]*github.ProjectV2FieldIteration, len(configuration.Iterations))
		for i, iteration := range configuration.Iterations {
			configurationCopy.Iterations[i] = sanitizedProjectV2FieldIterationCopy(iteration)
		}
	}
	return &configurationCopy
}

func sanitizedProjectV2FieldCopy(field *github.ProjectV2Field) *github.ProjectV2Field {
	if field == nil {
		return nil
	}
	fieldCopy := *field
	fieldCopy.Name = sanitizeOutputStringPtr(field.Name)
	if field.Options != nil {
		fieldCopy.Options = make([]*github.ProjectV2FieldOption, len(field.Options))
		for i, option := range field.Options {
			fieldCopy.Options[i] = sanitizedProjectV2FieldOptionCopy(option)
		}
	}
	fieldCopy.Configuration = sanitizedProjectV2FieldConfigurationCopy(field.Configuration)
	return &fieldCopy
}

func sanitizedProjectV2FieldsCopy(fields []*github.ProjectV2Field) []*github.ProjectV2Field {
	if fields == nil {
		return nil
	}
	fieldsCopy := make([]*github.ProjectV2Field, len(fields))
	for i, field := range fields {
		fieldsCopy[i] = sanitizedProjectV2FieldCopy(field)
	}
	return fieldsCopy
}

func sanitizedPullRequestCopy(pr *github.PullRequest) *github.PullRequest {
	if pr == nil {
		return nil
	}
	prCopy := *pr
	prCopy.Title = sanitizeOutputStringPtr(pr.Title)
	prCopy.Body = sanitizeOutputStringPtr(pr.Body)
	prCopy.Labels = sanitizedLabelsCopy(pr.Labels)
	prCopy.Milestone = sanitizedMilestoneCopy(pr.Milestone)
	prCopy.Head = sanitizedPullRequestBranchCopy(pr.Head)
	prCopy.Base = sanitizedPullRequestBranchCopy(pr.Base)
	return &prCopy
}

func sanitizedPullRequestBranchCopy(branch *github.PullRequestBranch) *github.PullRequestBranch {
	if branch == nil {
		return nil
	}
	branchCopy := *branch
	branchCopy.Repo = sanitizedRepositoryCopy(branch.Repo)
	return &branchCopy
}

func sanitizedPullRequestsCopy(prs []*github.PullRequest) []*github.PullRequest {
	if prs == nil {
		return nil
	}
	prsCopy := make([]*github.PullRequest, len(prs))
	for i, pr := range prs {
		prsCopy[i] = sanitizedPullRequestCopy(pr)
	}
	return prsCopy
}

func sanitizedPullRequestCommentCopy(comment *github.PullRequestComment) *github.PullRequestComment {
	if comment == nil {
		return nil
	}
	commentCopy := *comment
	commentCopy.Body = sanitizeOutputStringPtr(comment.Body)
	return &commentCopy
}

func sanitizedCombinedStatusCopy(status *github.CombinedStatus) *github.CombinedStatus {
	if status == nil {
		return nil
	}
	statusCopy := *status
	if status.Statuses != nil {
		statusCopy.Statuses = make([]*github.RepoStatus, len(status.Statuses))
		for i, repoStatus := range status.Statuses {
			statusCopy.Statuses[i] = sanitizedRepoStatusCopy(repoStatus)
		}
	}
	return &statusCopy
}

func sanitizedRepoStatusCopy(status *github.RepoStatus) *github.RepoStatus {
	if status == nil {
		return nil
	}
	statusCopy := *status
	statusCopy.Description = sanitizeOutputStringPtr(status.Description)
	return &statusCopy
}

func sanitizedWorkflowCopy(workflow *github.Workflow) *github.Workflow {
	if workflow == nil {
		return nil
	}
	workflowCopy := *workflow
	workflowCopy.Name = sanitizeOutputStringPtr(workflow.Name)
	return &workflowCopy
}

func sanitizedWorkflowsCopy(workflows *github.Workflows) *github.Workflows {
	if workflows == nil {
		return nil
	}
	workflowsCopy := *workflows
	if workflows.Workflows != nil {
		workflowsCopy.Workflows = make([]*github.Workflow, len(workflows.Workflows))
		for i, workflow := range workflows.Workflows {
			workflowsCopy.Workflows[i] = sanitizedWorkflowCopy(workflow)
		}
	}
	return &workflowsCopy
}

func sanitizedWorkflowRunCopy(run *github.WorkflowRun) *github.WorkflowRun {
	if run == nil {
		return nil
	}
	runCopy := *run
	runCopy.Name = sanitizeOutputStringPtr(run.Name)
	runCopy.DisplayTitle = sanitizeOutputStringPtr(run.DisplayTitle)
	runCopy.PullRequests = sanitizedPullRequestsCopy(run.PullRequests)
	runCopy.HeadCommit = sanitizedHeadCommitCopy(run.HeadCommit)
	runCopy.Repository = sanitizedRepositoryCopy(run.Repository)
	runCopy.HeadRepository = sanitizedRepositoryCopy(run.HeadRepository)
	return &runCopy
}

func sanitizedWorkflowRunsCopy(runs *github.WorkflowRuns) *github.WorkflowRuns {
	if runs == nil {
		return nil
	}
	runsCopy := *runs
	if runs.WorkflowRuns != nil {
		runsCopy.WorkflowRuns = make([]*github.WorkflowRun, len(runs.WorkflowRuns))
		for i, run := range runs.WorkflowRuns {
			runsCopy.WorkflowRuns[i] = sanitizedWorkflowRunCopy(run)
		}
	}
	return &runsCopy
}

func sanitizedWorkflowJobCopy(job *github.WorkflowJob) *github.WorkflowJob {
	if job == nil {
		return nil
	}
	jobCopy := *job
	jobCopy.Name = sanitizeOutputStringPtr(job.Name)
	jobCopy.WorkflowName = sanitizeOutputStringPtr(job.WorkflowName)
	if job.Steps != nil {
		jobCopy.Steps = make([]*github.TaskStep, len(job.Steps))
		for i, step := range job.Steps {
			if step == nil {
				continue
			}
			stepCopy := *step
			stepCopy.Name = sanitizeOutputStringPtr(step.Name)
			jobCopy.Steps[i] = &stepCopy
		}
	}
	return &jobCopy
}

func sanitizedWorkflowJobsCopy(jobs *github.Jobs) *github.Jobs {
	if jobs == nil {
		return nil
	}
	jobsCopy := *jobs
	if jobs.Jobs != nil {
		jobsCopy.Jobs = make([]*github.WorkflowJob, len(jobs.Jobs))
		for i, job := range jobs.Jobs {
			jobsCopy.Jobs[i] = sanitizedWorkflowJobCopy(job)
		}
	}
	return &jobsCopy
}

func sanitizedSecurityAdvisoryCopy(advisory *github.SecurityAdvisory) *github.SecurityAdvisory {
	if advisory == nil {
		return nil
	}
	advisoryCopy := *advisory
	advisoryCopy.Summary = sanitizeOutputStringPtr(advisory.Summary)
	advisoryCopy.Description = sanitizeOutputStringPtr(advisory.Description)
	advisoryCopy.PrivateFork = sanitizedRepositoryCopy(advisory.PrivateFork)
	advisoryCopy.CollaboratingTeams = sanitizedTeamsCopy(advisory.CollaboratingTeams)
	return &advisoryCopy
}

func sanitizedTeamsCopy(teams []*github.Team) []*github.Team {
	if teams == nil {
		return nil
	}
	teamsCopy := make([]*github.Team, len(teams))
	for i, team := range teams {
		teamsCopy[i] = sanitizedTeamCopy(team)
	}
	return teamsCopy
}

func sanitizedTeamCopy(team *github.Team) *github.Team {
	if team == nil {
		return nil
	}
	teamCopy := *team
	teamCopy.Name = sanitizeOutputStringPtr(team.Name)
	teamCopy.Description = sanitizeOutputStringPtr(team.Description)
	teamCopy.Parent = sanitizedTeamCopy(team.Parent)
	return &teamCopy
}

func sanitizedSecurityAdvisoriesCopy(advisories []*github.SecurityAdvisory) []*github.SecurityAdvisory {
	if advisories == nil {
		return nil
	}
	advisoriesCopy := make([]*github.SecurityAdvisory, len(advisories))
	for i, advisory := range advisories {
		advisoriesCopy[i] = sanitizedSecurityAdvisoryCopy(advisory)
	}
	return advisoriesCopy
}

func sanitizedGlobalSecurityAdvisoryCopy(advisory *github.GlobalSecurityAdvisory) *github.GlobalSecurityAdvisory {
	if advisory == nil {
		return nil
	}
	advisoryCopy := *advisory
	advisoryCopy.SecurityAdvisory = *sanitizedSecurityAdvisoryCopy(&advisory.SecurityAdvisory)
	return &advisoryCopy
}

func sanitizedGlobalSecurityAdvisoriesCopy(advisories []*github.GlobalSecurityAdvisory) []*github.GlobalSecurityAdvisory {
	if advisories == nil {
		return nil
	}
	advisoriesCopy := make([]*github.GlobalSecurityAdvisory, len(advisories))
	for i, advisory := range advisories {
		advisoriesCopy[i] = sanitizedGlobalSecurityAdvisoryCopy(advisory)
	}
	return advisoriesCopy
}

func sanitizedDependabotAlertCopy(alert *github.DependabotAlert) *github.DependabotAlert {
	if alert == nil {
		return nil
	}
	alertCopy := *alert
	alertCopy.DismissedComment = sanitizeOutputStringPtr(alert.DismissedComment)
	alertCopy.Repository = sanitizedRepositoryCopy(alert.Repository)
	if alert.SecurityAdvisory != nil {
		advisoryCopy := *alert.SecurityAdvisory
		advisoryCopy.Summary = sanitizeOutputStringPtr(alert.SecurityAdvisory.Summary)
		advisoryCopy.Description = sanitizeOutputStringPtr(alert.SecurityAdvisory.Description)
		alertCopy.SecurityAdvisory = &advisoryCopy
	}
	return &alertCopy
}

func sanitizedDependabotAlertsCopy(alerts []*github.DependabotAlert) []*github.DependabotAlert {
	if alerts == nil {
		return nil
	}
	alertsCopy := make([]*github.DependabotAlert, len(alerts))
	for i, alert := range alerts {
		alertsCopy[i] = sanitizedDependabotAlertCopy(alert)
	}
	return alertsCopy
}

func sanitizedCodeScanningAlertCopy(alert *github.Alert) *github.Alert {
	if alert == nil {
		return nil
	}
	alertCopy := *alert
	alertCopy.Repository = sanitizedRepositoryCopy(alert.Repository)
	alertCopy.RuleDescription = sanitizeOutputStringPtr(alert.RuleDescription)
	alertCopy.Rule = sanitizedCodeScanningRuleCopy(alert.Rule)
	alertCopy.DismissedComment = sanitizeOutputStringPtr(alert.DismissedComment)
	alertCopy.MostRecentInstance = sanitizedCodeScanningInstanceCopy(alert.MostRecentInstance)
	if alert.Instances != nil {
		alertCopy.Instances = make([]*github.MostRecentInstance, len(alert.Instances))
		for i, instance := range alert.Instances {
			alertCopy.Instances[i] = sanitizedCodeScanningInstanceCopy(instance)
		}
	}
	return &alertCopy
}

func sanitizedCodeScanningRuleCopy(rule *github.Rule) *github.Rule {
	if rule == nil {
		return nil
	}
	ruleCopy := *rule
	ruleCopy.Name = sanitizeOutputStringPtr(rule.Name)
	ruleCopy.Description = sanitizeOutputStringPtr(rule.Description)
	ruleCopy.FullDescription = sanitizeOutputStringPtr(rule.FullDescription)
	ruleCopy.Help = sanitizeOutputStringPtr(rule.Help)
	return &ruleCopy
}

func sanitizedCodeScanningAlertsCopy(alerts []*github.Alert) []*github.Alert {
	if alerts == nil {
		return nil
	}
	alertsCopy := make([]*github.Alert, len(alerts))
	for i, alert := range alerts {
		alertsCopy[i] = sanitizedCodeScanningAlertCopy(alert)
	}
	return alertsCopy
}

func sanitizedCodeScanningInstanceCopy(instance *github.MostRecentInstance) *github.MostRecentInstance {
	if instance == nil {
		return nil
	}
	instanceCopy := *instance
	if instance.Message != nil {
		messageCopy := *instance.Message
		messageCopy.Text = sanitizeOutputStringPtr(instance.Message.Text)
		instanceCopy.Message = &messageCopy
	}
	return &instanceCopy
}

func sanitizedSecretScanningAlertCopy(alert *github.SecretScanningAlert) *github.SecretScanningAlert {
	if alert == nil {
		return nil
	}
	alertCopy := *alert
	alertCopy.Repository = sanitizedRepositoryCopy(alert.Repository)
	alertCopy.ResolutionComment = sanitizeOutputStringPtr(alert.ResolutionComment)
	alertCopy.PushProtectionBypassRequestComment = sanitizeOutputStringPtr(alert.PushProtectionBypassRequestComment)
	alertCopy.PushProtectionBypassRequestReviewerComment = sanitizeOutputStringPtr(alert.PushProtectionBypassRequestReviewerComment)
	return &alertCopy
}

func sanitizedSecretScanningAlertsCopy(alerts []*github.SecretScanningAlert) []*github.SecretScanningAlert {
	if alerts == nil {
		return nil
	}
	alertsCopy := make([]*github.SecretScanningAlert, len(alerts))
	for i, alert := range alerts {
		alertsCopy[i] = sanitizedSecretScanningAlertCopy(alert)
	}
	return alertsCopy
}
