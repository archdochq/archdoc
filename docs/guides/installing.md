# Installing ArchDoc

ArchDoc is a single binary called `archdoc`. Install it in any one of the ways below, then check it with `archdoc --version`.

ArchDoc also needs git on `PATH`. Whether a document is frozen is read from the branch your repository is configured with, so the commands that need to know, such as `lint`, `changes`, `link` and the transition commands, refuse to run without it.

## Homebrew

On macOS or Linux:

```sh
brew install archdochq/tap/archdoc
```

The cask lives in the `archdochq/homebrew-tap` repository and installs the latest stable release. Pre-releases are not published to the tap; install one from the releases page or with the install script instead.

The macOS binaries are not notarised. The tap clears the quarantine attribute from the binary after installing it, so Gatekeeper does not stop it from running.

To upgrade:

```sh
brew upgrade archdoc
```

## The install script

Every release attaches `install.sh`, a POSIX shell script that installs that release on Linux or macOS. The copy attached to the latest stable release is always at the same URL:

```sh
curl -fsSL https://github.com/archdochq/archdoc/releases/latest/download/install.sh | sh
```

Running the script again installs over the binary already there, which is how you upgrade with it.

### What it does

1. Works out the release to install: `ARCHDOC_VERSION` if you set it, otherwise the release the script was attached to.
2. Maps the operating system to `linux` or `darwin` and the architecture to `amd64` (from `x86_64` or `amd64`) or `arm64` (from `aarch64` or `arm64`). Anything else is refused, naming the platform.
3. Downloads the release archive for your platform and the release's `checksums.txt` into a temporary directory, with `curl`, or `wget` where `curl` is missing.
4. Checks the archive's SHA-256 digest against its line in `checksums.txt`, with `sha256sum`, or `shasum -a 256` where that is missing. An archive with no line, or with a digest that does not match, stops the script before anything is installed.
5. Extracts `archdoc` and copies it, mode `0755`, to a temporary name in the install directory, then renames it to `archdoc`. A binary already there is replaced whole or not at all.
6. Prints the release and the path it installed, and a warning when the install directory is not on `PATH`.

On success it prints a line such as:

```text
Installed archdoc v0.6.0 at /usr/local/bin/archdoc
```

Whether it succeeds, fails or is interrupted, the script removes its temporary directory and any temporary file it left in the install directory.

### Where it installs

The first of these that applies:

1. `ARCHDOC_INSTALL_DIR`, if set.
2. `/usr/local/bin`, if you can write to it.
3. `$HOME/.local/bin`, if `HOME` is set.

The directory is created if it is missing. If none of them applies, the script stops and asks you to set `ARCHDOC_INSTALL_DIR`.

If the `archdoc` in the install directory is a symbolic link, the script refuses to replace it. A package manager such as Homebrew installs `archdoc` as a link, and the binary it points at belongs to that package manager. Upgrade it there, or install somewhere else with `ARCHDOC_INSTALL_DIR`.

### Variables

| Variable | Effect |
|---|---|
| `ARCHDOC_VERSION` | The release to install, as a tag such as `v0.6.0`, in place of the one the script was attached to |
| `ARCHDOC_INSTALL_DIR` | The directory to install into |
| `ARCHDOC_DOWNLOAD_URL` | The base URL of the release's assets, in place of `https://github.com/archdochq/archdoc/releases/download/<tag>` |

Set a variable on the `sh` side of the pipe:

```sh
curl -fsSL https://github.com/archdochq/archdoc/releases/latest/download/install.sh | ARCHDOC_VERSION=v0.6.0 sh
curl -fsSL https://github.com/archdochq/archdoc/releases/latest/download/install.sh | ARCHDOC_INSTALL_DIR="$HOME/bin" sh
```

The same command works in a Dockerfile or a CI job on a Linux image, as long as the image has `curl` or `wget`, `tar`, and `sha256sum` or `shasum`.

There is no install script for Windows. Use the zip archive or `go install`, below.

## Release archives

Each release on the [releases page](https://github.com/archdochq/archdoc/releases) attaches an archive per platform, for Linux, macOS and Windows on `amd64` and `arm64`. They are named `archdoc_<version>_<os>_<arch>`, where `<version>` is the tag without its leading `v`:

```text
archdoc_0.6.0_darwin_amd64.tar.gz
archdoc_0.6.0_darwin_arm64.tar.gz
archdoc_0.6.0_linux_amd64.tar.gz
archdoc_0.6.0_linux_arm64.tar.gz
archdoc_0.6.0_windows_amd64.zip
archdoc_0.6.0_windows_arm64.zip
archdoc_0.6.0_docs.tar.gz
install.sh
checksums.txt
```

Linux and macOS archives are `tar.gz`; Windows archives are `zip`. Each holds the binary at its root, with `LICENSE` and `README.md` beside it.

`archdoc_<version>_docs.tar.gz` holds the release's documentation: these guides, `PROCESS.md`, the command and rule reference, and the published schemas. `checksums.txt` lists the SHA-256 digest of every file the release attaches. Check the archive you downloaded before extracting it. On Linux:

```sh
grep ' archdoc_0.6.0_linux_amd64.tar.gz$' checksums.txt | sha256sum -c
tar -xzf archdoc_0.6.0_linux_amd64.tar.gz archdoc
```

On macOS, use `shasum -a 256 -c` in place of `sha256sum -c`. A matching archive prints `<archive>: OK`.

Then move `archdoc` into a directory on your `PATH`.

On Windows, download the zip archive for your architecture, compare the output of `Get-FileHash <archive> -Algorithm SHA256` in PowerShell with the archive's line in `checksums.txt`, extract the zip, and put the directory holding the binary on your `PATH`.

## With Go

```sh
go install archdoc.dev/cmd/archdoc@latest
```

This builds from source and works on any platform Go supports, Windows included. The module requires Go 1.26.2 or later. Replace `@latest` with a tag, such as `@v0.6.0`, to install a particular release. Releases made before the module path became `archdoc.dev` can only be installed under their old path, `github.com/archdochq/archdoc/cmd/archdoc`.

`go install` writes the binary to `$(go env GOPATH)/bin`, which is often not on `PATH`. If `archdoc` is not found afterwards, add that directory to `PATH`.

## In GitHub Actions

The `archdochq/setup` action installs ArchDoc in a GitHub Actions job and puts it on `PATH`, on Linux, macOS and Windows runners:

```yaml
- uses: archdochq/setup@v1
  with:
    version: v0.6.0
- run: archdoc lint
```

`version` defaults to `latest`. Pin it, so the rules your repository is held to do not change underneath it. The action verifies the archive against the release's `checksums.txt` before installing it, and gives the installed tag and directory as its `version` and `path` outputs. Its repository is [archdochq/setup](https://github.com/archdochq/setup).

You do not need it for the workflow `archdoc init` writes. That workflow uses `archdochq/lint`, which installs ArchDoc through `archdochq/setup` itself. [Checking in CI](ci.md) covers both.

## Checking the install

```sh
archdoc --version
```

A release binary, whether from Homebrew, the install script or an archive, prints its tag, such as `v0.6.0`. A binary from `go install` prints the module version it was built from. A binary built with `go build` from a checkout prints the tag at a tagged commit, a pseudo-version such as `v0.5.2-0.20261026120114-a9241aa1ac5f` at any other, with `+dirty` appended where the checkout has uncommitted changes, and `dev` where the build records no version control information, as with `-buildvcs=false` or a tree without `.git`.

`archdoc init` pins the workflow it writes to this version when it is a release tag. Anything else, a pseudo-version or `dev`, is written to the workflow as `latest`, with a warning that the workflow is unpinned.

Once it is installed, [Five minutes with ArchDoc](five-minutes.md) walks through a first repository.
