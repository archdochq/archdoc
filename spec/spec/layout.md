---
title: Layout
includes: [RFC-0001, RFC-0007, RFC-0010, RFC-0011, RFC-0013, ADR-0017]
---

# Layout

A specification repository is a directory containing `archdoc.json` and, under `root`, the document directories. `group/` is optional, and `init` does not create it.

## With `root` of `.`

```
<repo>/
├── archdoc.json
├── README.md
├── PROCESS.md
├── INDEX.md              generated
├── GLOSSARY.md           generated
├── LICENSE               optional
├── AGENTS.md             optional
├── agents/               optional
├── .github/workflows/archdoc-lint.yml
├── rfc/
├── adr/
├── spec/
├── ref/
├── term/
└── group/                optional
```

## With `root` of `docs`, inside a code repository

```
<repo>/
├── .github/workflows/archdoc-lint.yml
├── archdoc.json          root: "docs"
├── README.md             the code repository's own
└── docs/
    ├── README.md
    ├── PROCESS.md
    ├── INDEX.md          generated
    ├── GLOSSARY.md       generated
    ├── rfc/
    ├── adr/
    ├── spec/
    ├── ref/
    ├── term/
    └── group/            optional
```

## With `archdoc.json` below the repository root, as scaffolded before ADR-0017

```
<repo>/
├── .github/workflows/archdoc-lint.yml    working-directory: spec
├── README.md             the code repository's own
└── spec/
    ├── archdoc.json      root: "."
    ├── README.md
    ├── PROCESS.md
    ├── INDEX.md          generated
    ├── GLOSSARY.md       generated
    ├── rfc/
    ├── adr/
    ├── spec/
    ├── ref/
    └── term/
```

`init` does not write this shape. It writes `archdoc.json` at the git repository root, with `root` naming the path down to the directory it was run in, so the second arrangement above is what running `init` inside `docs/` produces. A repository scaffolded before that keeps this one, and works unchanged: every command finds the configuration by walking up, and the workflow's `working-directory` is the path from the repository root to the directory holding it.

`root` is relative to `archdoc.json` and defaults to `.`. The document directories, `README.md`, `PROCESS.md`, `INDEX.md`, `GLOSSARY.md`, `LICENSE`, `AGENTS.md` and `agents/` are all resolved under it. `root` names the directory holding `archdoc.json` or one below it; an absolute `root`, or one leading outside that directory, is a configuration error. The check is applied to the spelling and again to the resolved path.

`.github/workflows/archdoc-lint.yml` is written at the root of the git repository, never under `root`. GitHub runs workflows from there only. Where there is no git repository it is written in the current directory.

`README.md`, `PROCESS.md`, `INDEX.md`, `AGENTS.md` and the files under `agents/` are not documents and no lint rule examines them.

## Configuration

```json
{
  "name": "TheGamePanel",
  "branch": "main",
  "root": ".",
  "strict": true,
  "ref_stale_days": 180
}
```

- `name`: the project name, used in the README and INDEX headings.
- `branch`: the branch frozen documents are compared against.
- `root`: as above.
- `strict`: `true` keeps the transition graph as PROCESS.md gives it; `false` additionally permits `draft → accepted` and `draft → rejected`.
- `ref_stale_days`: refs whose `verified` date is older than this produce a lint warning, except a ref carrying `as_of`. `0` disables the check; a negative value is a configuration error.

Unknown keys are an error. A missing key takes the default shown, except `name`, which is required. The file is decoded onto a struct already holding the defaults, and the decoder must reach the end of the file.

## Templates

Embedded under `internal/template`: `rfc.md`, `adr.md`, `ref.md`, `PROCESS.md`, `README.md`, `AGENTS.md`, `archdoc-lint.yml`, `LICENSE.mit`, and the guides under `agents/`. Rendered templates substitute `.ID`, `.Title`, `.Date`, `.Name`, `.Version`, `.Status`, `.Decided`, `.Backfilled`, `.AsOf`, `.Year` and `.WorkingDirectory` as applicable, through `text/template`. `AGENTS.md`, `PROCESS.md` and everything under `agents/` ship verbatim. Only `agents/` sits outside the parse glob, so a brace there is literal text; every other embedded `.md` file is parsed as a template whether or not it carries substitutions, and one that does not parse fails the build.

One template function is registered: `yaml`, which encodes a value as a YAML scalar, quoting and escaping only where the encoding requires it. Front matter uses `title: {{ yaml .Title }}`. The H1 uses `.Title` raw and is read back by splitting on the first `": "`.

Each guide under `agents/` carries `name` and `description` front matter and is a valid skill file as written. The same files are written into a repository by `init --agents`, refreshed by `agents` and `update`, and published by each stable release to `skills/<name>/SKILL.md` in `archdochq/skills`, copied byte for byte by `plugin/build.go`. A test checks each guide's front matter against the Agent Skills rules: only the keys the specification defines, a `name` of 1 to 64 lowercase letters, digits and single inner hyphens that no other guide declares, and a `description` of 1 to 1024 characters that is not whitespace alone. Each guide cites `PROCESS.md` by section name, and a test resolves every citation against the headings present.
