package github

import "slices"

// MCPAppsFeatureFlag is the feature flag name for MCP Apps (interactive UI forms).
const MCPAppsFeatureFlag = "remote_mcp_ui_apps"

// MCPAppsDisableFormDeferralFeatureFlag disables handing write-tool calls off
// to MCP App forms while preserving MCP Apps UI metadata and result views.
const MCPAppsDisableFormDeferralFeatureFlag = "mcp_apps_disable_form_deferral"

// FeatureFlagCSVOutput is the feature flag name for CSV output on list tools.
const FeatureFlagCSVOutput = "csv_output"

// FeatureFlagIFCLabels is the feature flag name for IFC security labels in tool results.
const FeatureFlagIFCLabels = "ifc_labels"

// FeatureFlagFileBlame is the feature flag name for the get_file_blame tool,
// which exposes git blame information for a file. It is gated so the extra tool
// is not advertised by default, keeping the tool surface small unless opted in.
const FeatureFlagFileBlame = "file_blame"

// FeatureFlagIssueDependencies is the feature flag name for the issue dependency
// tools (issue_dependency_read / issue_dependency_write), which read and edit an
// issue's blocked-by / blocking relationships. It is gated so these tools are not
// advertised in the default surface, keeping the fixed tool-schema cost small
// unless explicitly opted in.
const FeatureFlagIssueDependencies = "issue_dependencies"

// FeatureFlagDuplicateDetection is the feature flag name for the find_duplicate
// tool, which returns ranked duplicate candidates for an existing issue. It is
// gated so the extra tool is not advertised by default, and is deliberately
// excluded from insiders mode so duplicate detection is only ever an explicit
// opt-in.
const FeatureFlagDuplicateDetection = "duplicate_detection"

// FeatureFlagThreadResolutionReason exposes resolution reasons for Copilot review threads.
const FeatureFlagThreadResolutionReason = "thread_resolution_reason"

// AllowedFeatureFlags is the allowlist of feature flags that can be enabled
// by users via --features CLI flag or X-MCP-Features HTTP header.
// Only flags in this list are accepted; unknown flags are silently ignored.
// This is the single source of truth for which flags are user-controllable.
var AllowedFeatureFlags = []string{
	MCPAppsFeatureFlag,
	MCPAppsDisableFormDeferralFeatureFlag,
	FeatureFlagCSVOutput,
	FeatureFlagIFCLabels,
	FeatureFlagIssuesGranular,
	FeatureFlagPullRequestsGranular,
	FeatureFlagFileBlame,
	FeatureFlagIssueDependencies,
	FeatureFlagDuplicateDetection,
	FeatureFlagThreadResolutionReason,
}

// InsidersFeatureFlags is the list of feature flags that insiders mode enables.
// When insiders mode is active, all flags in this list are treated as enabled.
// This is the single source of truth for what "insiders" means in terms of
// feature flag expansion.
var InsidersFeatureFlags = []string{
	MCPAppsFeatureFlag,
	FeatureFlagCSVOutput,
	FeatureFlagFileBlame,
	FeatureFlagIssueDependencies,
}

// FeatureFlags defines runtime feature toggles that adjust tool behavior.
type FeatureFlags struct {
	LockdownMode bool
}

// ResolveFeatureFlags computes the effective set of enabled feature flags by:
//  1. Taking the user-supplied flags (from --features or X-MCP-Features) and
//     keeping only those present in AllowedFeatureFlags. Unknown or unsafe
//     flags from request input are silently dropped here.
//  2. If insiders mode is on, unioning in every flag from InsidersFeatureFlags.
//     Insiders is a server-controlled meta switch, so its expansion is NOT
//     re-validated against AllowedFeatureFlags.
//
// AllowedFeatureFlags and InsidersFeatureFlags are independent sets:
//   - A flag in AllowedFeatureFlags but not InsidersFeatureFlags is a regular
//     opt-in flag that insiders mode does not turn on automatically.
//   - A flag in InsidersFeatureFlags but not AllowedFeatureFlags is reachable
//     only through insiders mode and cannot be enabled by user input.
//
// Returns a set (map) for O(1) lookup by the feature checker.
func ResolveFeatureFlags(enabledFeatures []string, insidersMode bool) map[string]bool {
	effective := make(map[string]bool)
	for _, f := range enabledFeatures {
		if slices.Contains(AllowedFeatureFlags, f) {
			effective[f] = true
		}
	}
	if insidersMode {
		for _, f := range InsidersFeatureFlags {
			effective[f] = true
		}
	}
	return effective
}
gubonlucid-com:patch-9

docs/feature-flags.md

GUBON-EX (Enterprise Commercial Edition v1.0)
├── 00 CANONICAL STATUS
│   ├── Architecture Status: LOCKED[span_0](start_span)[span_0](end_span)
│   └── Production Certification Status: EVIDENCE-GATED (G01-G20)[span_1](start_span)[span_1](end_span)
├── 01 MARKET & BRAND
│   ├── Executive Summary: Enterprise Decision Operating Layer[span_2](start_span)[span_2](end_span)
│   ├── Commercial Model: Decision-as-a-Service (DaaS)[span_3](start_span)[span_3](end_span)
│   ├── Vision & Mission: Standardize decision execution into a reusable enterprise runtime[span_4](start_span)[span_4](end_span)
│   ├── Core Capabilities: Decision Runtime, Governance Kernel, MCP Runtime, Outcome Engine, Commercial Runtime[span_5](start_span)[span_5](end_span)
│   └── Value Proposition: Intelligence -> Decision -> Governance -> Execution -> Outcome -> Revenue -> Audit[span_6](start_span)[span_6](end_span)
├── 02 PRODUCT SYSTEM
│   ├── Decision Runtime: Request Ingestion, Schema Validation, Normalization, Deterministic Computation, Action Emission[span_7](start_span)[span_7](end_span)
│   ├── Governance Kernel: Auth, Authorization, Tenant Enforcement, Policy Evaluation, Risk Scoring, Idempotency, Audit[span_8](start_span)[span_8](end_span)
│   ├── MCP Runtime: Governed AI-Agent Integration, Tool Registry, Schema Validation, SEP-1865 Integration[span_9](start_span)[span_9](end_span)
│   ├── Outcome Engine: KPI Registration, Outcome Ingestion, Decision-to-Result Correlation[span_10](start_span)[span_10](end_span)
│   └── Commercial Runtime: Order, Payment, Webhook Verification, Entitlement, Service Activation, Revenue Ledger[span_11](start_span)[span_11](end_span)
├── 03 ENTERPRISE ARCHITECTURE
│   ├── Topology: Identity Boundary -> Gateway -> Runtimes -> Event Queue -> Data/Ledger -> Outcome Engine -> Control Plane[span_12](start_span)[span_12](end_span)
│   ├── Tech Stack: React, Node.js/TypeScript, Express, Redis/BullMQ, PostgreSQL/Prisma, Docker, OpenTelemetry[span_13](start_span)[span_13](end_span)
│   └── Identity Boundary: Principal, Tenant, Organization, Role, Permissions, RequestID[span_14](start_span)[span_14](end_span)
├── 04 DECISION & GOVERNANCE KERNEL
│   ├── Decision Lifecycle: INGEST -> VALIDATE -> NORMALIZE -> AUTHORIZE -> POLICY CHECK -> DETERMINISTIC EXECUTION -> RISK EVALUATION -> APPROVAL/ACTION -> OUTCOME EMISSION -> AUDIT[span_15](start_span)[span_15](end_span)
│   └── Risk Actions: AUTO_APPROVE, MANUAL_APPROVAL, MFA, BLOCK, ESCALATE[span_16](start_span)[span_16](end_span)
├── 05 GOVERNANCE KERNEL DEEP-DIVE
│   ├── Audit Ledger: Append-only, Cryptographic Chained Hashes [ H(n) = HASH(eventData(n) + H(n-1)) ][span_17](start_span)[span_17](end_span)
│   ├── Policy Enforcer: Versioned Policy Evaluation (ALLOW, DENY, REQUIRE_APPROVAL, REQUIRE_MFA, ESCALATE)[span_18](start_span)[span_18](end_span)
│   ├── Idempotency Guard: Deduplication across RequestID, TransactionID, PaymentID, WebhookEventID, JobID[span_19](start_span)[span_19](end_span)
│   ├── State Machine: Strict Transition Validation (currentState -> requestedState via actor/policy/event)[span_20](start_span)[span_20](end_span)
│   └── Recovery Engine: Retries, Backoff, Dead-Letter Queues, Compensating Transactions, State Reconciliation[span_21](start_span)[span_21](end_span)
├── 06 MCP RUNTIME
│   ├── Integration Boundary: AI Agent/LLM -> MCP Client -> Gateway -> Auth -> Tenant -> Authz -> Tool Registry -> Policy/Risk -> Decision Runtime[span_22](start_span)[span_22](end_span)
│   ├── Auth Patterns: OAuth 2.1, API Keys, Service Credentials, Signed Requests[span_23](start_span)[span_23](end_span)
│   └── Tool Registry: Tool ID, Schema, Required Scopes, Tenant Policy, Risk Class, Rate Limit[span_24](start_span)[span_24](end_span)
├── 07 OUTCOME ENGINE
│   ├── Data Model: DecisionID, TenantID, DecisionType, DecisionVersion, Action, Expected/Actual Outcome, Metric, Value[span_25](start_span)[span_25](end_span)
│   └── Feedback Loop: KPI Reporting, Risk Analysis, Controlled Calibration (No silent production mutations)[span_26](start_span)[span_26](end_span)
├── 08 COMMERCIAL ENGINE & SKU MATRIX
│   ├── Commercial Flow: Catalog -> SKU -> Pricing -> Order -> Payment -> Webhook -> Entitlement -> Service Activation -> Revenue Ledger[span_27](start_span)[span_27](end_span)
│   └── SKUs: EX-DEV (Developer), EX-PRO (Professional), EX-ENT (Enterprise), EX-GOV (Sovereign/Regulated)[span_28](start_span)[span_28](end_span)
├── 09 ORDER & PAYMENT ARCHITECTURE
│   ├── Canonical Order Flow: ORDER_CREATED -> CHECKOUT_CREATED -> PAYMENT_PENDING -> PAYMENT_CAPTURED -> WEBHOOK_RECEIVED -> WEBHOOK_VERIFIED -> PAYMENT_CONFIRMED -> ENTITLEMENT_PENDING -> ENTITLEMENT_GRANTED -> SERVICE_ACTIVE -> REVENUE_LEDGER_COMMITTED -> AUDIT_VERIFIED[span_29](start_span)[span_29](end_span)
│   ├── Webhook Verification: Header validation, raw body preservation, cryptographic signature check, idempotency check[span_30](start_span)[span_30](end_span)
│   └── Integrity Checks: Match Payment ID, Order ID, Amount, Currency, SKU, and Tenant ID[span_31](start_span)[span_31](end_span)
├── 10 TENANT & SECURITY ARCHITECTURE
│   ├── Tenant Isolation: Server-side enforced tenant boundaries across Application and DB (foreign keys, scoped queries)[span_32](start_span)[span_32](end_span)
│   ├── RBAC Roles: PLATFORM_ADMIN, TENANT_ADMIN, EXECUTIVE, OPERATOR, AUDITOR, ANALYST, SERVICE_ACCOUNT[span_33](start_span)[span_33](end_span)
│   └── Secrets Management: Deployment Secret Manager (DATABASE_URL, REDIS_URL, PAYPAL credentials, Keys, JWT secrets)[span_34](start_span)[span_34](end_span)
├── 11 PRODUCTION RELEASE GOVERNANCE (G01-G20)
│   ├── Code & Quality Gates: G01 Architecture, G02 Clean Build, G03 Type Safety, G04 Unit Tests (85%+), G05 Integration Tests, G06 API Contracts, G07 DB Migrations[span_35](start_span)[span_35](end_span)
│   ├── Operational Gates: G08 ACID Integrity, G09 Idempotency, G10 Auth Security, G11 Tenant Isolation, G12 Dependency Scan[span_36](start_span)[span_36](end_span)
│   ├── Commercial Gates: G13 Payment Integration, G14 Webhook Verification, G15 Entitlement Provisioning, G16 Queue & Worker Health[span_37](start_span)[span_37](end_span)
│   ├── Reliability Gates: G17 Disaster Recovery, G18 Observability, G19 Performance & Load (SLO validation)[span_38](start_span)[span_38](end_span)
│   └── Final Commercial Gate: G20 Real Revenue Loop Verification (End-to-End Real Transaction Evidence Package)[span_39](start_span)[span_39](end_span)
└── 12 MONOREPO STRUCTURE
    ├── apps/ (web, api, mcp-gateway, worker, scheduler, admin)[span_40](start_span)[span_40](end_span)
    ├── packages/ (decision-runtime, governance-kernel, commercial-runtime, outcome-engine, mcp-runtime, identity, tenant, rbac, payment, entitlement, revenue-ledger, audit-ledger, event-runtime, db, contracts, config, observability)[span_41](start_span)[span_41](end_span)
    ├── prisma/ (schema.prisma, migrations)[span_42](start_span)[span_42](end_span)
    ├── infra/ (docker, deployment, monitoring, ci)[span_43](start_span)[span_43](end_span)
    └── tests/ (unit, integration, security, payment, webhook, tenant-isolation, recovery, production)[span_44](start_span)[span_44](end_span)

GUBON-EX SOVEREIGN SERVER (Sovereign Edition v1.0)
├── 00 SOVEREIGN PRINCIPLES
│   ├── Compute Ownership: Dedicated CPU, RAM, Disk, Container Runtime control[span_0](start_span)[span_0](end_span)
│   ├── Data Ownership: Local isolated PostgreSQL, Audit Ledger, Decision History, Revenue Ledger[span_1](start_span)[span_1](end_span)
│   ├── Key Ownership: Zero external leakage of TLS, API Keys, JWT, Webhook secrets, MCP keys[span_2](start_span)[span_2](end_span)
│   ├── Runtime Ownership: Self-hosted Decision Engine, Governance Kernel, Workers, MCP Gateway[span_3](start_span)[span_3](end_span)
│   └── Lifecycle Ownership: Independent Deployment, Version Control, Rollback, Recovery Bundles[span_4](start_span)[span_4](end_span)
├── 01 SOVEREIGN ARCHITECTURE STACK (7 LAYERS)
│   ├── 01 Host OS: Hardened Enterprise Linux, UFW/IPTables, SSH Hardening, Auto Security Patching[span_5](start_span)[span_5](end_span)
│   ├── 02 Container Runtime: Docker Engine, Docker Compose Production Stack[span_6](start_span)[span_6](end_span)
│   ├── 03 GUBON Runtime: Sovereign Gateway, Decision Engine, Governance Kernel, MCP Gateway, Commercial Runtime, Workers[span_7](start_span)[span_7](end_span)
│   ├── 04 State Layer: Local PostgreSQL (Prisma ORM, Append-only Ledgers), Redis (BullMQ Queues)[span_8](start_span)[span_8](end_span)
│   ├── 05 Security Boundary: Secret Management, Local RBAC, Idempotency Guard, Cryptographic Hash-Chained Audit[span_9](start_span)[span_9](end_span)
│   ├── 06 Operations Layer: Structured JSON Logging, OpenTelemetry Metrics, Automated Local/Offsite Encrypted Backups[span_10](start_span)[span_10](end_span)
│   └── 07 Control Plane: Sovereign Status Management, Version Deployment Control, Audit Hash Verification[span_11](start_span)[span_11](end_span)
├── 02 NETWORK & SECURITY BOUNDARY
│   ├── Public Edge (Port 443): Reverse Proxy / Sovereign Gateway only (External API, Webhooks, Web presentation)[span_12](start_span)[span_12](end_span)
│   ├── Private Boundary (Internal Bridge Network): PostgreSQL, Redis, Internal Kernel APIs, Worker processes[span_13](start_span)[span_13](end_span)
│   └── Prohibited Exposure: No direct Internet exposure for DB, Cache, Queue workers, or Core Decision Engine[span_14](start_span)[span_14](end_span)
├── 03 RECOVERY & PORTABILITY (RECOVERY BUNDLE)
│   ├── Self-Contained Assets: docker-compose.yml, container images, migrations, env manifests[span_15](start_span)[span_15](end_span)
│   ├── Data Backups: Encrypted DB snapshots, Redis state dumps, Audit Hash chain exports[span_16](start_span)[span_16](end_span)
│   └── Recovery Execution: Single-command restoration pipeline for complete machine replacement[span_17](start_span)[span_17](end_span)
└── 04 SOVEREIGN VERIFICATION GATES (S01-S20)
    ├── S01-S04 Infrastructure: Host Security, Docker Operational, Network Boundary, Runtime Boot Check[span_18](start_span)[span_18](end_span)
    ├── S05-S08 State & Services: PostgreSQL Health, Redis Health, Worker Health, Sovereign Gateway Ingress[span_19](start_span)[span_19](end_span)
    ├── S09-S12 Security & Governance: MCP Boundary Isolation, Governance Kernel Validation, Hash-Chain Verification, Encrypted Backup Check[span_20](start_span)[span_20](end_span)
    ├── S13-S16 Reliability & Infra: Disaster Recovery Restoration Test, Local Monitoring, TLS Integrity, Rollback Test[span_21](start_span)[span_21](end_span)
    ├── S17-S19 Security & Commerce: Secret Isolation, Multi-Tenant Boundary Enforcement, Payment & Webhook Verification[span_22](start_span)[span_22](end_span)
    └── S20 Final Sovereign Gate: Sovereign Runtime E2E Real Flow Certification[span_23](start_span)[span_23](end_span)



# ==============================================================================
# GUBON-EX SINGLE-PASS VERCEL DEPLOYMENT & DECOUPLING PIPELINE
# Executing ZIP extraction, index normalization, assets separation, and production launch
# ==============================================================================

mkdir -p gubon-vercel && \
unzip -o "（閉環）S- SOVEREIGN PERSONAL DECISION INTELLIGENCE--.zip" -d gubon-vercel && \
cd gubon-vercel && \
find . -maxdepth 2 -type f -name "*.html" ! -name "index.html" -exec mv {} index.html \; && \
python3 - <<'PY'
from pathlib import Path
import re

p = Path("index.html")
if p.exists():
    s = p.read_text(encoding="utf-8")

    # Extract all inline CSS and JS blocks
    css_blocks = re.findall(r"<style[^>]*>(.*?)</style>", s, re.S | re.I)
    js_blocks = re.findall(r"<script[^>]*>(.*?)</script>", s, re.S | re.I)

    # Write decoupled asset files
    Path("style.css").write_text("\n\n".join(css_blocks), encoding="utf-8")
    Path("app.js").write_text("\n\n".join(js_blocks), encoding="utf-8")

    # Strip inline blocks from HTML document
    s = re.sub(r"<style[^>]*>.*?</style>", "", s, flags=re.S | re.I)
    s = re.sub(r"<script[^>]*>.*?</script>", "", s, flags=re.S | re.I)

    # Inject external bundle references
    if "</head>" in s:
        s = s.replace("</head>", '    <link rel="stylesheet" href="/style.css">\n</head>')
    else:
        s = '<link rel="stylesheet" href="/style.css">\n' + s

    if "</body>" in s:
        s = s.replace("</body>", '    <script src="/app.js"></script>\n</body>')
    else:
        s = s + '\n<script src="/app.js"></script>'

    p.write_text(s, encoding="utf-8")
PY
printf '{"cleanUrls":true,"trailingSlash":false}\n' > vercel.json && \
npx vercel --prod

