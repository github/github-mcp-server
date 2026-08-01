# Contributing to the Go MCP SDK

Thank you for your interest in contributing! The Go SDK needs active
contributions to keep up with changes in the MCP spec, fix bugs, and accommodate
new and emerging use-cases. We welcome all forms of contribution, from filing
and reviewing issues, to contributing fixes, to proposing and implementing new
features.

As described in the [design document](https://github.com/modelcontextprotocol/go-sdk/blob/main/design/design.md), it is important for
the MCP SDK to remain idiomatic, future-proof, and extensible. The process
described here is intended to ensure that the SDK evolves safely and
transparently, while adhering to these goals.

%toc


## Development setup

This module can be built and tested using the standard Go toolchain. Run `go
test ./...` to run all its tests.

To test changes to this module against another module that uses the SDK, we
recommend using a [`go.work` file](https://go.dev/doc/tutorial/workspaces) to
define a multi-module workspace. For example, if your directory contains a
`project` directory containing your project, and a `go-sdk` directory
containing the SDK, run:

```sh
go work init ./project ./go-sdk
```

### Conformance tests

The SDK includes a script to run the official MCP conformance tests against the
SDK's conformance server:

```sh
./scripts/conformance.sh
```

By default, results are cleaned up after the script runs. To save results to a
specific directory:

```sh
./scripts/conformance.sh --result_dir ./conformance-results
```

To run against a local checkout of the
[conformance repo](https://github.com/modelcontextprotocol/conformance) instead
of the latest npm release:

```sh
./scripts/conformance.sh --conformance_repo ~/src/conformance
```

Note: you must run `npm install` in the conformance repo first.

Run `./scripts/conformance.sh --help` for more options.

## Filing issues

This project uses the [GitHub issue
tracker](https://github.com/modelcontextprotocol/go-sdk/issues) for issues. The
process for filing bugs and proposals is described below.

TODO(rfindley): describe a process for asking general questions in the public
MCP discord server.

### Bugs

Please [report
bugs](https://github.com/modelcontextprotocol/go-sdk/issues/new). If the SDK is
not working as you expected, it is likely due to a bug or inadequate
documentation, and reporting an issue will help us address this shortcoming.

When reporting a bug, make sure to answer these five questions:

1. What did you do?
2. What did you see?
3. What did you expect to see?
4. What version of the Go MCP SDK are you using?
5. What version of Go are you using (`go version`)?

### Proposals

A proposal is an issue that proposes a new API for the SDK, or a change to the
signature or behavior of an existing API. Proposals should be labeled with the
'proposal' label, and require an explicit approval from a maintainer before
being accepted (indicated by the 'proposal-accepted' label). Proposals must
remain open for at least a week to allow discussion before being accepted or
declined by a maintainer.

Proposals that are straightforward and uncontroversial may be approved based on
discussion on the issue tracker or in a GitHub Discussion. However, proposals
that are deemed to be sufficiently unclear or complicated may be deferred to a
regular Working Group meeting (see 'Governance' below).

This process is similar to the [Go proposal
process](https://github.com/golang/proposal), but is necessarily lighter weight
to accommodate the greater rate of change expected for the SDK.

### Design discussion

For open ended design discussion (anything that doesn't fall into the issue
categories above), use [GitHub
Discussions](https://github.com/modelcontextprotocol/go-sdk/discussions).
Ideally, each discussion should be focused on one aspect of the design. For
example: Tool Binding and Session APIs would be two separate discussions.
When discussions reach a consensus, they should be promoted into proposals.

## Contributing code

The project uses GitHub pull requests (PRs) to review changes.

Any significant change should be associated with a GitHub issue. Issues that
are deemed to be good opportunities for contribution are be labeled ['Help
Wanted'](https://github.com/modelcontextprotocol/go-sdk/issues?q=is%3Aissue%20state%3Aopen%20label%3A%22help%20wanted%22).
If you want to work on such an issue, please first comment on the issue to say
that you're interested in contributing. For issues _not_ labeled 'Help Wanted',
it is recommended that you ask (and wait for confirmation) on the issue before
contributing, to avoid duplication of effort or wasted work. For nontrivial
changes that _don't_ relate to an existing issue, please file an issue first.

Changes should be high quality and well tested, and should generally follow the
[Google Go style guide](https://google.github.io/styleguide/go/). Commit
messages should follow the [format used by the Go
project](https://go.dev/wiki/CommitMessage).

Unless otherwise noted, the Go source files are distributed under the license
found in the LICENSE file. New contributions are licensed under Apache 2.0. All
Go files in the SDK should have a copyright header following the format below:

```go
// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.
```

### Adding and updating dependencies

In general, the SDK tries to use as few dependencies as possible. Each new
dependency is a potential source for bugs, churn, and conflicts for our users.
Therefore, we require a [proposal](#proposals) for any new module dependency,
including upgrading an existing module to a new major version. New dependencies
should be evaluated for their stability and security, and should be
well-established in the Go ecosystem.

In general, dependencies should be for internal use by the SDK implementation,
or for testing. Do not include types from dependencies in the SDK API.

On the other hand, updating existing dependencies can be done at any time
without a proposal, as long as their major version does not change. Prefer to
update dependencies immediately following a release of the SDK, to allow as
much time as possible to find issues with the new version.

After any change to dependencies, run govulncheck to check them for
vulnerabilities.

```
go run golang.org/x/vuln/cmd/govulncheck@latest
```

### Updating the README

The top-level `README.md` file is generated from `internal/readme/README.src.md`
and should not be edited directly. To update the README:

1. Make your changes to `internal/readme/README.src.md`
2. Run `go generate ./internal/readme` from the repository root to regenerate `README.md`
3. Commit both files together

The CI system will automatically check that the README is up-to-date by running
`go generate ./internal/readme` and verifying no changes result. If you see a CI failure about the
README being out of sync, follow the steps above to regenerate it.

## Timeouts

If a contributor hasn't responded to issue questions or PR comments in two weeks,
the issue or PR may be closed. It can be reopened when the contributor can resume
work.

## Code of conduct

This project follows the [Go Community Code of Conduct](https://go.dev/conduct).
If you encounter a conduct-related issue, please mail conduct@golang.org.

## Governance

Initially, the Go SDK repository will be administered by the Go team and
Anthropic, and they will be the approvers (the set of people able to merge PRs
to the SDK), also referred to as the 'Working Group'. The policies here are
also intended to satisfy necessary constraints of the Go team's participation
in the project. This may change in the future: see 'Ongoing Evaluation' below.

### Working Group meetings

On a regular basis, the Working Group will host a virtual meeting to discuss
outstanding proposals and other changes to the SDK. These meetings and their
agendas will be announced in advance, and open to all. The meetings will be
recorded, and recordings and meeting notes will be made available afterward.
(TODO: decide on a mechanism for tracking these meetings--likely a GitHub
issue.)

This process is similar to the [Go Tools
call](https://go.dev/wiki/golang-tools), though it is expected that meetings
will at least initially occur on a more frequent basis.

### Discord

Discord (either the public or private Anthropic discord servers) should only be
used for logistical coordination or answering questions. For transparency and
durability, design discussion and decisions should occur in GitHub issues,
GitHub discussions, or public steering meetings.

### Antitrust considerations

The goal of this repository is to provide a robust and complete Go
implementation of the model context protocol, in an open and transparent
manner, without bias toward specific integration paths or providers. To that
end, the model context protocol organization's [antitrust
policy](https://github.com/modelcontextprotocol/modelcontextprotocol/blob/main/ANTITRUST.md)
applies to all participation in this project.
