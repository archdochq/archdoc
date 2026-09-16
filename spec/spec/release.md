---
title: Release
includes: [ADR-0012]
---

# Release

Tagged versions build with goreleaser for linux, darwin and windows on amd64 and arm64. Archives are `tar.gz`, or `zip` for windows, named `archdoc_<version>_<os>_<arch>` with the binary `archdoc` at the archive root and `LICENSE` and `README.md` beside it. A `checksums.txt` of SHA-256 digests is published with them. A tag carrying a prerelease identifier publishes as a prerelease.

`archdoc --version` prints the tag. The version resolves in order: the ldflag goreleaser sets at build time; the module version from `runtime/debug.ReadBuildInfo`, which `go install ...@version` and `@latest` populate; `dev`.

`init` records the resolved version in the generated workflow as `ARCHDOC_VERSION`. A version that is not a release tag, including `dev` and a pseudo-version, is written as `latest` with a warning that the workflow is unpinned. The workflow downloads `archdoc_<version>_linux_amd64.tar.gz` and `checksums.txt` from the release into a temporary directory, verifies the archive against the checksums, and installs the binary. With `latest`, the tag is read from the redirect of the releases page's `latest` URL.

Asset names are fixed by `.goreleaser.yaml`. A test resolves the archive name, the archive format, the checksum filename and the binary name from that file and from the generated workflow independently and compares them.

The repository's own continuous integration runs `gofmt`, `go vet`, `go test ./...` and `goreleaser check` on push to `main` and on every pull request, and the release workflow runs the test suite before publishing.

The licence is the GNU Affero General Public License, version 3 or later, and its text ships in every archive.
