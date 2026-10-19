---
id: RFC-0014
title: Lint rules and the command reference are published as data
status: proposed
created: 2026-10-19
decided:
depends: [ADR-0005, ADR-0011, ADR-0019, ADR-0022]
updates: []
obsoletes: []
---

# RFC-0014: Lint rules and the command reference are published as data

## Abstract

Each lint rule in the registry gains a title, a summary, a remedy, the release that introduced it and the releases that changed it, as data. A hidden command, `archdoc reference`, prints the commands, the rules, the front matter of each type, the configuration, the exit codes and the export schema as one JSON document, described by a published schema of its own, and the triage guide's table of rules is generated from the registry.

## Motivation

Anything that displays or works with the details of ArchDoc, its commands and their flags, its lint rules, the front matter of each type, its configuration and its exit codes, needs them as they stand in the release it describes, and needs them to follow each release without being written again by hand. A site documenting the tool, such as archdoc.dev, can generate a page for each command and each rule from them. A rule today carries only its code and its function. What a rule means and how to clear it are written as prose in the triage guide, beside the registry rather than from it, and the commands, the front matter keys, the configuration and the exit codes are described only in the help text, the code and the spec pages. Nothing a build can read holds them together.

## Proposal

### Concepts

| Concept | Meaning |
|---|---|
| Rule entry | A rule in the registry: its code and check, with its title, summary, remedy, `since` and changes |
| `since` | The first tagged release carrying the rule, pre-releases included |
| Change | A release that changed what a rule reports, with a one-line account of the change |
| Reference | The JSON document `archdoc reference` prints |
| Reference schema | The JSON Schema describing the reference, embedded in the binary and published |

### Components

| Component | Responsibility |
|---|---|
| `internal/lint` | `Rule` gains `Title`, `Summary`, `Remedy`, `Since` and `Changes`, filled for every rule in `Rules`, and the test of `Since` and `Changes` against the tag being released |
| `internal/repo` | The keys of each type's front matter as data: each key's value type, whether it must be present, whether it may be empty, and the values of an enum. The parser and L01 read it, which moves into it the keys L01 requires to carry a value, and L01's message names the statuses from it rather than listing them |
| `internal/config` | Each key of `archdoc.json` as data: its JSON type, whether it is required, and its default where it has one. `Load` applies the defaults from it |
| `internal/export` | The descriptions of `name`, `root` and `strict` in the `config` definition of `schema: 3`, written in full, since the reference takes every key's description from that definition |
| `internal/reference` | The reference's types, its embedded schema, and the test comparing the schema's declared properties against the types by reflection. It imports nothing that imports cobra |
| `cmd/archdoc` | The hidden `reference` command, which walks the command tree into the reference's types; the exit codes as data beside the constants that hold them |
| `internal/template` | The triage guide's table of rules, generated from the registry |
| Published schemas | `schema/reference/v1.json`, generated from the embedded schema, and `schema/export/v3.json`, regenerated for its `config` descriptions |
| Release | `.github/workflows/release.yml` passes the tag to the test step |
| Specification pages | `commands.md`: `reference`, and the exit codes as the reference gives them. `lint.md`: what a rule entry holds. `release.md`: the test of `since` against the tag. `architecture.md`: `internal/reference` in the package tree, and `schema/` holding the reference schema beside the export's |

### Rule entries

| Field | Content |
|---|---|
| `Title` | A few words naming what the rule checks, such as `Front matter matches the schema` |
| `Summary` | One line saying what a finding means, as inline Markdown. The triage guide's "What it means" column |
| `Remedy` | How a finding is cleared, as inline Markdown. The triage guide's "Remedy" column, guide pointers included |
| `Since` | The tag of the first release carrying the rule |
| `Changes` | Each later release that changed what the rule reports, as `{Release, Summary}`, in release order |

| Rule | Detail |
|---|---|
| Existing text | Each `Summary` and `Remedy` starts as the triage guide's row for that rule as it stands, so the generated table is unchanged |
| Severity | Not a field. A rule's severity depends on the document it reports, as `lint.md` gives it, and a summary says "(warning)" where the triage guide does |
| Existing releases | `Since` is `v0.1.0-alpha.1` for L01 to L18 and `v0.5.0` for L19. L20 and L21 name the release carrying [RFC-0008](0008-questions.md) and [RFC-0009](0009-changelog.md) |
| History | `Changes` starts empty for every rule and records changes made once this design is built. Changes made before it are not recorded |
| Unreleased | A rule added, or changed, between releases names the next tag to be cut, a pre-release included |
| Removal | A rule removed from the registry is removed from the reference with it |

### Checking `since` against the release

The release workflow's test step sets `ARCHDOC_RELEASE_TAG` to the tag being released. A test in `internal/lint` refuses any `Since`, or change, naming a release later than that tag, comparing versions by their numbers and treating a pre-release as earlier than the release it precedes. Without the variable the test is skipped, so it runs in the release workflow only. A failure stops the workflow before goreleaser publishes, leaving the tag pushed and unpublished, to be deleted and cut again once the registry is corrected. The test does not check that an earlier release carried a rule its `Since` names.

### The triage guide

The table under "Rules and remedies" in `agents/triage.md` is generated from the registry, one row per rule in code order, as `Rule | What it means | Remedy`. A test compares the table in the guide with the registry, and `go test ./internal/template -update-triage` rewrites it, as `-update-schema` rewrites the published export schema. The guide stays a static file, so `archdoc agents`, `archdoc update` and the skills the release publishes copy it as they do now.

### `archdoc reference`

Prints the reference to stdout and exits. `--schema` prints the reference schema instead. Needs no repository. Hidden from `archdoc --help` and from shell completion.

```json
{
  "schema": 1,
  "version": "v0.6.0",
  "command": {
    "path": "archdoc",
    "use": "archdoc",
    "short": "Manage a specification repository",
    "long": "",
    "flags": [
      {"name": "version", "shorthand": "v", "type": "bool", "default": "false", "usage": "version for archdoc"}
    ],
    "global_flags": [
      {"name": "chdir", "shorthand": "C", "type": "string", "default": "",
       "usage": "run as if archdoc had been started in this directory"},
      "…"
    ],
    "commands": [
      {
        "path": "archdoc new",
        "use": "new <rfc|adr|ref> <title>",
        "short": "Create the next-numbered document of a type",
        "long": "",
        "flags": [
          {"name": "backfill", "shorthand": "", "type": "bool", "default": "false",
           "usage": "create the document already terminal, recording a decision taken earlier"}
        ],
        "global_flags": [],
        "commands": []
      }
    ]
  },
  "rules": [
    {"code": "L13", "title": "…", "summary": "…", "remedy": "…", "since": "v0.1.0-alpha.1", "changes": []}
  ],
  "front_matter": {
    "rfc": [
      {"key": "status", "type": "enum", "required": true, "may_be_empty": false,
       "values": ["draft", "proposed", "accepted", "rejected", "withdrawn"]},
      {"key": "decided", "type": "date", "required": true, "may_be_empty": true, "values": []}
    ]
  },
  "config": [
    {"key": "ref_stale_days", "type": "integer", "required": false, "default": 180,
     "description": "How old a ref's verified date may be. 0 disables the check."}
  ],
  "exit_codes": [
    {"code": 0, "meaning": "…"}, {"code": 1, "meaning": "…"}, {"code": 2, "meaning": "…"}
  ],
  "export_schema": {}
}
```

| Field | Content |
|---|---|
| `schema` | `1`, the reference schema's number |
| `version` | The version `archdoc --version` prints |
| `command` | The root command. Each command carries its full path, its `use` line, its short and long help, its own flags, the persistent flags it defines, and its subcommands in the order help lists them. Hidden commands, `help` and `completion` are left out, and so is `--help`, which every command has. The walk registers the version flag first, which cobra otherwise adds only when the root command runs |
| Arguments | Described by `use` alone. A command that prompts for a missing argument accepts fewer than `use` shows |
| Flags | `type` and `default` as the flag library gives them: a type name and the default as help prints it, both strings |
| `rules` | Every rule entry, in code order |
| `front_matter` | For each type that has front matter, its keys, each with its value type (`string`, `date`, `list` or `enum`), whether it must be present, whether it may be present with no value, and an enum's values. Absent arrays are empty |
| `config` | Every key `archdoc.json` holds, with its JSON type, whether it is required, its default in that type where it has one, and its description from the `config` definition of the embedded export schema |
| `exit_codes` | `0`, `1` and `2`, each with its meaning as `commands.md` gives it |
| `export_schema` | The embedded export schema, as `export --schema` prints it. Embedded rather than named by its `$id`, because the schema can gain optional fields without its number changing, and only the copy the release embeds is the one that release emits |

Every array is an array and never `null`. `commands.md` gives exit code `2` as returned when lint reports an error, or a warning under `--strict-warnings`, when a `--check` finds a file to change, when `link` reports an error, and when `link --suggest` without a terminal or `--apply` prints a suggestion, and the reference gives each code the meaning `commands.md` does.

### The reference schema

The reference schema is embedded in the binary, printed by `reference --schema`, and published at `schema/reference/v1.json` in the source repository, generated from the embedded copy and compared with it byte for byte, as the export's is. Its `$id` is `https://archdoc.dev/schema/reference/v1.json`, following the identification [ADR-0022](../adr/0022-the-export-schemas-are-identified-by-archdoc-dev-urls.md) gives the export's schemas from v3, and the site serves it at its `$id`. The schema's own `description` states the stability rule below, as the export schema's does. A test compares the schema's declared properties against the reference's types by reflection.

The reference's `schema` number follows the rule [ADR-0019](../adr/0019-export-stability.md) sets for the export's: before ArchDoc 1.0, a change that removes a field, or changes a field's type or meaning, moves it to the next integer, at most once between two tagged releases, pre-releases included; a number a tagged release has emitted is never given to another shape, and each released schema stays published; from 1.0 the shape only gains optional fields.

### Dependencies

`cobra/doc` imports `go-md2man`, which [ADR-0005](../adr/0005-the-dependency-list-is-closed.md) does not admit, and writes Markdown, man pages, ReST and YAML rather than this shape. The command tree is walked by the command itself, and under [ADR-0011](../adr/0011-only-the-command-package-imports-cobra.md) only `cmd/archdoc` imports cobra, so the walk lives there and fills the types `internal/reference` defines. Versions are compared by code in the test, not by a module.

### Out of scope

- **`lint --explain`.** A command printing a rule's entry at the terminal reads the same registry, and is a design of its own.
- **A rule's URL in lint's output.** It waits on a page for each rule existing at a stable address, so that no released binary prints a URL that answers with an error.
- **The docs bundle a release attaches.** It would carry the reference, and is a design of its own.

## Alternatives considered

The design rests on [ADR-0005](../adr/0005-the-dependency-list-is-closed.md), under which `cobra/doc` is not added; on [ADR-0011](../adr/0011-only-the-command-package-imports-cobra.md), under which the command tree is walked in `cmd/archdoc`; on [ADR-0019](../adr/0019-export-stability.md), whose rule for the export's number the reference's follows; and on [ADR-0022](../adr/0022-the-export-schemas-are-identified-by-archdoc-dev-urls.md), whose identification of the export's schemas the reference schema's follows.

- **Keeping the reference a contract between this repository and the tools that read it.** Tested against its own types and versioned by its number alone, with no published schema. Lost because a published schema lets any tool read the reference against a stated shape, under the same promise as the export.
- **Recording the changes made to each rule in releases already tagged.** Lost because each would be reconstructed from the code at each tag, and a wrong entry would be published on the rule's page.

## Backwards compatibility

Nothing a repository holds breaks. `archdoc reference` is new and hidden, and lint's findings, its JSON and its exit codes are unchanged. The triage guide's table is generated to the text it holds now, so `archdoc update` finds no difference in it.

`schema: 3` is not yet released, and its `config` definition gains descriptions only, which change no field's type or meaning. Moving the front matter keys and L01's keys that must carry a value into one table in `internal/repo` changes no finding, and the existing tests of L01 hold. `commands.md`'s account of exit code `2` is completed to what the commands already do.

A rule added later without a title, summary, remedy or `since` fails a test requiring each on every rule entry. The tests gain the comparison of the triage table with the registry, the reference schema's reflection test and byte comparison with its published copy, and the test of `since` against the tag being released.

## Open questions

## Changelog
