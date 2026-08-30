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
# GitHub Pre-release License Terms

## GitHub Pre-release License Terms

These terms apply to the pre-release software made available to you by GitHub. To the extent there is a conflict between these terms and any other Agreement you have with us, these terms govern.

## 1. Pre-Release Software.

  The software provided is a pre-release version. “Pre-release” means software, online services, and additional products and features that are not yet generally available, such as private preview, public preview, early access, technical preview, or similar versions.

  Pre-release software may not operate correctly. It may delete your data, corrupt your data, or have other bugs. It also may not work the way a final commercial version of the software will.

  GitHub may change or discontinue pre-release software at any time, for any reason, without notice to you. GitHub may change the software for the final commercial version, or may not release a commercial version at all. GitHub is not obligated to provide to you any maintenance, technical support, or updates for the software.

## 2. Installation and Use Rights.

  a. **General**. GitHub grants you a limited right to use a non-production instance of the software for evaluation and testing. This means you may use any number of copies of the software to evaluate its functionality and internally develop and test your applications, including deployment of the software within your internal corporate network for evaluation but not external distribution. You may also use the software in internally demonstrating your applications, but may not use the pre-release software in any active production environment, including any use to process live customer data.

  b. **Inclusion of Third-Party Components.** The software may include third-party components with separate legal notices or governed by other agreements, as may be described in a license file accompanying the software.

  c. **Optional Extensions.** The software may give you the option to download other GitHub and third-party software packages. Any third-party software packages are provided for your convenience only, and are governed by any applicable agreements between you and the third party. GitHub is not responsible or liable for any third-party software.

## 3. Scope of License.

  GitHub reserves all rights not expressly granted to you in these terms, including retaining ownership of all aspects of the pre-release software as well as all related intellectual property rights.

  Unless applicable law gives you more rights despite this limitation, you may use the software only as expressly permitted in this Agreement. In doing so, you must comply with any technical limitations in the software that only allow you to use it in certain ways. You may not:

  a. work around any technical limitations in the software;

  b. reverse engineer, decompile, or disassemble the software, or otherwise attempt to derive the source code for the software, except to the extent required by applicable third party licensing terms governing use of certain open source components that may be included in the software;

  c. remove, minimize, block, or modify any notices of GitHub or its suppliers in the software;

  d. share, publish, or lease the software;

  e. provide the software as a stand-alone offering or combine it with any of your applications for others to use;

  f. transfer the software or these terms to any third party; or

  g. use the software to create or propagate malware, or in any way that is against the law

## 4. Data Collection and Usage.

  a. **Consent to Data Collection.** The pre-release software may collect telemetry information about you and your use of the software, and send that information to GitHub. Subject to the limitations in Section 4(b) below, GitHub may use this information to provide services, to improve our products and services, or for any other purpose permitted under the GitHub Data Protection Agreement or GitHub Privacy Statement. Your use of the pre-release software operates as your consent to these practices.

  b. **Use of Collected Data.**
  * GitHub will use collected data for analytics and measurement to understand how our pre-release software and related products are used.
  * The software will collect data and usage information about events generated when interacting with it. These events help us analyze and measure performance and features used. This usage information is used by GitHub and may be shared with affiliates and other third parties to help deliver, develop, evaluate, and improve the software and related products.
  * We analyze data to ensure the pre-release software is working as intended, to evaluate the safety, reliability, and user experience of the software, and to investigate and detect potential abuse.
  * We may combine the information we collect from the pre-release software with other data.
  * For pre-release software that uses AI:
      * You retain ownership of the code that you input to the software.
      * GitHub does not own the output sent to you by the software.
      * GitHub will not use Copilot Business or Copilot Enterprise Inputs or the Outputs generated to train AI language models, unless you have instructed us in writing to do so.

  c. **Processing of Personal Data.** GitHub is the data controller in relation to the Personal Data processed in connection with the pre-release software.

  d. **Data Collection by You.** There may be some features in the pre-release software that enable you and GitHub to collect data from users of your applications. If you use these features, you must comply with all applicable laws on data collection, including providing appropriate notices to users of your applications as well as a copy of GitHub’s Privacy Statement. You can learn more about data collection and use in the software documentation.

  e. **Revocation of Consent to Data Collection.** You may revoke your consent to data collection by the prerelease software by contacting GitHub and requesting removal from the technical preview. Please note that, if you request removal from the preview, you will no longer be able to use the pre-release software.

## 5. Updates.

  You may obtain updates to the pre-release software only from GitHub or GitHub-authorized sources. The software may install automatic updates and download and install them for you. You agree to these automatic updates without any additional notice. Software updates may not include or support all existing software features, platforms, services, or peripheral devices. These updates are generally meant to improve and evolve the software, but they may also change or disable any part of the software, including potentially removing features and services, or revoking support for certain platforms or hardware.

## 6. Time-Bound Software.

  Your use of the pre-release software will end upon any of _(i)_ commercial release of the software, or _(ii)_ at the discretion of GitHub to discontinue the support or development of the software, or _(iii)_ termination of the technical preview by either party. You may not be able to access data used in the software when it stops running.

## 7. Feedback.

  If you give feedback about the pre-release software to GitHub, you give to GitHub the right to use, share, and commercialize your feedback in any way and for any purpose, without payment to you. You agree that you will not give feedback that is subject to any license that would require GitHub to license its software or documentation to third parties if we included your feedback in them.

## 8. Communications.

  By using pre-release software, you agree to be contacted by GitHub and Microsoft regarding your participation in the technical preview, including email request(s) for feedback about the software.

## 9. No Warranties.

You bear the sole risk of using the pre-release software.

  The pre-release software is licensed “as is” without any warranty of any kind or sort, whether such warranty would be express, implied, or statutory. To the extent permitted under your local laws, GitHub and Microsoft disclaim all warranties in the pre-release software, including implied warranties of merchantability, fitness for a particular purpose, title, quiet enjoyment, accuracy, course of dealing, usage of trade, and non-infringement.

  Neither GitHub nor Microsoft give any express warranties, guarantees, or commitments about the pre-release software or its quality, reliability, availability, security, or function. The software may contain errors, may delete or corrupt your data, and may have defects or other bugs.

## 10. Defense of Third Party Claims.

  If your Agreement provides for the defense of third party claims, that provision will apply to your use of the pre-release software and the outputs you receive from it. For software that uses artificial intelligence, you must have complied with (a) the Acceptable Use Policies in your Agreement, and (b) the [Microsoft Enterprise AI Services Code of Conduct](https://aka.ms/AIcode), and (c) the [Microsoft Customer Copyright Commitment Required Mitigations](https://aka.ms/AIfilters).

## 11. No Uptime Guarantees.

  The pre-release software is not subject to an uptime guarantee or similar service level agreement. The software may be unavailable or stop working entirely at any time for any reason.

## 12. Limitation of Liability.

  GitHub’s maximum liability for any claim related to your use of the pre-release software is limited to direct damages up to five hundred dollars ($500.00 USD). This limit will not apply to the defense obligations in Section 10.

## 13. Compliance with Export Restrictions.

  You must comply with all domestic and international export laws and regulations that apply to the pre-release software, including any applicable restrictions on destinations, end users, and end use.

## 14. Confidentiality.

  The pre-release software is non-public, confidential information of GitHub. Your use is subject to the confidentiality obligations between you and GitHub in the Agreement.

  Please do not _(i)_ disclose or share the software with anyone who is not subject to these terms and a non-disclosure agreement; _(ii)_ post or allow others to post any photos or videos of the pre-release software on or via any online platform, including personal social media websites; or _(iii)_ describe or discuss any part of the software on or via any online platform, unless given advance and express permission by GitHub to do so.

## 15. Nature of Terms for Microsoft Customers.

  If you license GitHub through Microsoft, these terms shall be considered an amendment to the Microsoft Product Terms for GitHub Offerings for the duration of your use of the pre-release software.content/site-policy/github-terms/github-pre-release-license-terms.md
