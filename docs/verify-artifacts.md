# Verifying published artifacts

## Container images: cosign v2 and v3

Images published to `ghcr.io/github/github-mcp-server` by
`.github/workflows/docker-publish.yml` are signed keylessly using GitHub Actions
OIDC and Sigstore Fulcio, with signing metadata recorded in the public Rekor
transparency log. Verify the image digest, the exact workflow certificate
identity, and the issuer before running the image.

The publisher uses **cosign v3.1.3 in legacy compatibility mode**. Signatures
remain in `sha256-<digest>.sig` tags in the image repository, with the legacy
cosign payload and Rekor v1 verification material. We do not publish the new
protobuf-based Sigstore bundles or OCI 1.1 referring signature artifacts during
this transition.

Use a security-patched cosign client: v2.6.5 or v3.1.3 (or a later compatible
patch). Both fix [GHSA-fx35-mq7g-6g98](https://github.com/sigstore/cosign/security/advisories/GHSA-fx35-mq7g-6g98).
Existing v2 verification commands do not need new flags. Cosign v3.1.3 image
verification probes for new bundles and falls back to legacy signatures when
none are found. The v3 command below explicitly selects
`--new-bundle-format=false` rather than relying on that auto-detection; it also
works for signatures published before the publisher upgrade.

For a release image, replace both placeholders below with the digest obtained
from your trusted release/deployment configuration and its full Git tag (for
example, `vX.Y.Z`, not the Docker alias `X.Y.Z`). Use the multi-platform image
index digest, which is what the publishing workflow signs, rather than an
individual platform manifest digest. Do not verify a mutable tag and then pull
that tag: verify and run the same digest.

```sh
IMAGE='ghcr.io/github/github-mcp-server@sha256:<image-index-digest>'
REF='refs/tags/vX.Y.Z'
IDENTITY="https://github.com/github/github-mcp-server/.github/workflows/docker-publish.yml@${REF}"
ISSUER='https://token.actions.githubusercontent.com'
```

With **cosign v2.6.5**:

```sh
cosign verify \
  --certificate-identity "${IDENTITY}" \
  --certificate-oidc-issuer "${ISSUER}" \
  "${IMAGE}"
```

With **cosign v3.1.3**:

```sh
cosign verify \
  --new-bundle-format=false \
  --certificate-identity "${IDENTITY}" \
  --certificate-oidc-issuer "${ISSUER}" \
  "${IMAGE}"
```

For a main-branch, nightly, or manually published main-branch image, use
`REF='refs/heads/main'` instead. Images from `next` use
`REF='refs/heads/next'`. Other manually selected refs need their exact ref.
Pull-request builds are not published or signed by this workflow.
Do not broaden the certificate identity to accept arbitrary workflows or refs,
and do not bypass certificate or transparency-log verification.

### Why the publisher opts out of the v3 defaults

Cosign v3 changes both signature format and signing-service discovery:

| Publisher flag | Compatibility behavior |
| --- | --- |
| `--new-bundle-format=false` | Keeps the legacy cosign image payload and verification material instead of the protobuf-based Sigstore bundle (serialized as JSON). |
| `--use-signing-config=false` | Keeps the legacy service defaults, including Rekor v1, instead of fetching signing-service URLs from TUF. This is needed as well as the format flag: v3 rejects legacy image signing with its default signing config unless a local bundle output is supplied. |
| `--registry-referrers-mode=legacy` | Explicitly keeps `.sig` tag storage instead of opting into OCI 1.1 referrers for legacy signatures. |

The workflow still requests an ephemeral Fulcio certificate from GitHub Actions
OIDC and uploads to Rekor; it does not disable transparency logging or certificate
checks. `--yes` accepts the public transparency-log disclosure, including for
private repositories. Do not reuse this workflow for private artifacts without
reviewing that disclosure.

The signing compatibility flags remain supported in v3.1.3, although
`--new-bundle-format` is deprecated. Image verification selects the legacy path
with `--new-bundle-format=false`; `cosign verify` does not accept the signing
flag `--registry-referrers-mode`. The v3 default new-format path stores a
Sigstore bundle as an OCI referring artifact (using the OCI referrers fallback
tag when the registry does not support the referrers API);
changing only the registry mode does not restore the legacy payload.
V2 clients before v2.6 cannot verify those new image bundles; v2.6.x supports
them with `--new-bundle-format=true`, while v3 expects them by default.
V2.6.5 image verification also auto-detects new bundles, but consumers using
older clients or relying on `.sig` tags must not assume that support.
No new-format signature is currently promised by this repository.

This is a compatibility bridge, not a permanent commitment to legacy storage.
A future format migration must announce consumer changes, validate registry
referrer support and verification tooling, and consider dual signing before
removing `.sig` signatures. It must also account for Rekor v1 service availability.
See the upstream [v3 announcement](https://blog.sigstore.dev/cosign-3-0-available/)
and [v3.0.0 changelog](https://github.com/sigstore/cosign/blob/v3.0.0/CHANGELOG.md).

## Release archives: GitHub artifact attestations

The GoReleaser workflow (`.github/workflows/goreleaser.yml`) builds release
archives and checksums, then uses `actions/attest-build-provenance` for
`dist/*.tar.gz`, `dist/*.zip`, and `dist/*.txt`. Neither that workflow nor
`.goreleaser.yaml` uses cosign, and releases do not provide cosign
`sign-blob` signatures or bundle files. Their attestation format is unaffected
by the cosign upgrade.

Using a current GitHub CLI, download the desired archive from a trusted release
tag and verify its build provenance:

```sh
TAG='vX.Y.Z'
ARCHIVE='github-mcp-server_Linux_x86_64.tar.gz'
gh release download "${TAG}" \
  --repo github/github-mcp-server \
  --pattern "${ARCHIVE}"
gh attestation verify "${ARCHIVE}" \
  --repo github/github-mcp-server \
  --signer-workflow github/github-mcp-server/.github/workflows/goreleaser.yml \
  --source-ref "refs/tags/${TAG}"
```

Choose the archive name for your OS and architecture; Windows archives use
`.zip`. Verification fails if no matching attestation exists, for example for
an older release that predates artifact attestations. A downloaded checksum
file alone is not proof of publisher identity.
