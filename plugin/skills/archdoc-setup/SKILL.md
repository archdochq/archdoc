---
name: archdoc-setup
description: Use before documenting a project with ArchDoc, to install the binary if it is missing, locate or create the specification repository, and establish the two-repository working arrangement.
---

# Setting up

Do this once, before anything else. It establishes where the spec repository is and how to run
commands against it from wherever you are working.

## Two repositories

Documenting a project usually spans two:

- **The code repository.** Where you are working, and where the *evidence* is: git history,
  merged pull requests, changelogs, issue threads, the code itself.
- **The specification repository.** Where documents are written. ArchDoc runs against this one.

The evidence lives in the first and the output goes in the second, which is why you work from
the code repository and write across. Backfilling requires sources, and the sources are not in
the spec repository. (PROCESS.md, Backfilling)

## Is archdoc installed?

```
archdoc --version
```

If that fails, install it. No Go toolchain needed for the first option:

```
# a release binary, from https://github.com/archdochq/archdoc/releases
# or, with Go:
go install github.com/archdochq/archdoc/cmd/archdoc@latest
```

`go install` puts it in `$(go env GOPATH)/bin`, which is often not on PATH. If the command is
still not found after installing, that is why.

## Where is the spec repository?

It is the directory containing `archdoc.json`, or any directory below it. Look in the usual
places relative to the code repository:

```
ls ../*/archdoc.json ./archdoc.json ./spec/archdoc.json ./docs/archdoc.json 2>/dev/null
```

A sibling directory is the common arrangement: `thegamepanel/panel` alongside
`thegamepanel/spec`. Ask rather than guess if nothing turns up and the project may have one
elsewhere.

## Running commands across

`archdoc` finds its configuration by walking **up** from the working directory, so from a
sibling code repository it finds nothing. Use `-C`:

```
archdoc -C ../spec lint
archdoc -C ../spec new rfc "Retry policy"
```

Do this rather than `cd`. Each shell command may start fresh, so a `cd` has to be repeated
every time and is easy to lose track of. `-C` states the target on every invocation, which is
also what makes the transcript readable later.

Record the path once and use it consistently for the rest of the session.

## If there is no spec repository

Creating one is a decision for the project, not something to do unasked. Confirm first, then:

```
mkdir ../spec && cd ../spec && git init
archdoc init --agents
```

`--agents` writes the guides into the new repository so anyone working there afterwards has
them. Answer the prompts, or pass `--name`, `--branch` and `--no-interaction`.

Commit the scaffold before writing any documents. The frozen-document rule compares against the
branch, and with no commits there is nothing to compare against. (PROCESS.md, Workflow)

## Then

Read `survey.md` to work out what needs documenting before writing anything.
