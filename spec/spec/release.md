---
title: Release
includes: [ADR-0012, ADR-0013, ADR-0015]
---

# Release

Tagged versions build with goreleaser for linux, darwin and windows on amd64 and arm64. Archives are `tar.gz`, or `zip` for windows, named `archdoc_<version>_<os>_<arch>` with the binary `archdoc` at the archive root and `LICENSE` and `README.md` beside it. A `checksums.txt` of SHA-256 digests is published with them. A tag carrying a prerelease identifier publishes as a prerelease.

`archdoc --version` prints the tag. The version resolves in order: the ldflag goreleaser sets at build time; the module version from `runtime/debug.ReadBuildInfo`, which `go install ...@version` and `@latest` populate; `dev`.

`init` records the resolved version in the generated workflow as the `version` input of `archdochq/lint`. A version that is not a release tag, including `dev` and a pseudo-version in any of the three shapes Go produces, is written as `latest` with a warning that the workflow is unpinned. `archdochq/setup` downloads the archive for the runner's platform and `checksums.txt` from the release into a temporary directory, verifies the archive against the checksums, and puts the binary on `PATH`. With `latest`, the tag is read from the redirect of the releases page's `latest` URL.

Asset names are fixed by `.goreleaser.yaml` and reconstructed by `archdochq/setup`, which the scaffolded workflow reaches through `archdochq/lint`. A test renders the archive name, the archive format, the checksum filename and the binary name from that file and compares them against the names the action expects. The action's own repository checks the other side, by downloading a real release on every run.

The repository's own continuous integration runs `gofmt`, `go vet`, `go test ./...` and `goreleaser check` on push to `main` and on every pull request, and the release workflow runs the test suite before publishing.

The specification under `spec/` is checked by a second workflow, which builds `archdoc` from the commit under test rather than downloading a release. It lints through `archdochq/lint` with no `version` input, which leaves the binary just built on `PATH` in place and annotates each finding on the line that caused it, then runs `archdoc index --check` against the same binary. It carries no version pin.

ArchDoc is published from the `archdochq` organisation. The repository and the Homebrew tap were published under `ollieread` before that, and neither of those names is reused.

The licence is the GNU Affero General Public License, version 3 or later, and its text ships in every archive.
