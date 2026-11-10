---
id: RFC-0017
title: Warnings on frozen documents can be acknowledged
status: proposed
created: 2026-10-31
decided:
depends: [ADR-0009, ADR-0023]
updates: [RFC-0006]
obsoletes: []
---

# RFC-0017: Warnings on frozen documents can be acknowledged

## Abstract

`archdoc.json` gains an optional `acknowledged` key naming frozen documents and, for each, the rules whose warnings on it have been seen and accepted. Lint leaves those warnings out of its output and out of `--strict-warnings`. A new rule, L22, reports an acknowledgement matching no warning from its rule on its document, or naming anything other than a frozen document or an L rule.

## Motivation

Under [ADR-0009](../adr/0009-a-lint-error-names-something-the-user-may-change.md) and [ADR-0023](../adr/0023-every-finding-no-permitted-edit-can-clear-is-a-warning.md) a fault inside a frozen document that no permitted edit could clear is a warning, and the document can never be edited to clear it. Lint reports it on every run, for as long as the repository exists.

A repository running `lint --strict-warnings` fails on such a warning for good. One that does not sees it printed among the warnings that are tasks, so a new warning, which may be one, reads as one more line of the same. Nothing records that someone has seen a fault and accepted it, and the document holding the fault cannot carry that record itself.

## Proposal

### Concepts

| Concept | Meaning |
|---|---|
| Acknowledgement | A frozen document, by path within `root`, and the rules whose warnings on it are accepted |
| Acknowledged warning | A warning from an acknowledged rule on its document while that document is frozen |
| Stale acknowledgement | An acknowledgement, or one rule in it, matching no warning lint would otherwise report |

### Components

| Component | Responsibility |
|---|---|
| `internal/config` | The `acknowledged` key: decoding, its default, and its shape |
| `internal/lint` | Leaving acknowledged warnings out of a run's findings, and L22 |
| `internal/reference` | `object` among the types the reference schema permits for a configuration key |
| `internal/export` | `acknowledged` in the exported configuration and the export schema. `config` publishes every setting `archdoc.json` holds, so it gains a sixth beside the five [RFC-0006](0006-the-export-publishes-what-it-parsed.md) names |
| `testdata/lint` | A fixture for L22 |
| Spec pages | `layout.md` for the key; `lint.md` for acknowledged warnings, their effect on `--strict-warnings`, and L22 |
| `agents/triage.md` | Acknowledging a warning on a frozen document as its remedy, and the L22 row generated from the registry |
| `PROCESS.md` | The key, and acknowledgements among what lint enforces |

### The key

```json
{
  "name": "Example",
  "acknowledged": {
    "adr/0003-storage-engine.md": ["L07", "L10"],
    "rfc/0012-retry-policy.md": ["L17"]
  }
}
```

| Rule | Detail |
|---|---|
| Optional | Absent, it is an empty object |
| Keys | Paths relative to `root`, slash-separated, as lint prints them |
| Values | A non-empty list of rule codes, each listed once |
| Shape | Anything other than an object of string lists is a configuration error |

Which paths name frozen documents and which codes name rules is not checked when the file is read: `internal/config` knows neither, and L22 reports both.

### Effect on lint

1. Every rule runs as before.
2. A finding is acknowledged when its severity is warning, its path is a key of `acknowledged`, its rule is listed under that key, and the document at that path is frozen.
3. An acknowledged finding is left out of the printed findings and the `--json` array.
4. `--strict-warnings` exits 2 on a warning only where it is not acknowledged.
5. L22 runs over the findings of step 1, before any is left out, and reports each stale acknowledgement.

An error is never acknowledged. A warning on a document that is not frozen is never acknowledged, since editing the document clears it.

### L22

**L22** Every acknowledgement names a warning. Reported at `archdoc.json`, its path relative to `root`, as an error, for an acknowledged path that is not a document, a document that is not frozen, a code that is not an L rule, and a rule that reports no warning on the document. Its remedy is to remove the entry from `acknowledged`.

An error, because `archdoc.json` is always editable. An acknowledgement matches on path and rule, so it accepts every warning that rule reports on the document. A rule left reporting none there, such as L17 once a link target is restored, leaves its acknowledgement stale, and L22 asks for it to be removed rather than letting it accept a different fault the same rule reports there later.

### Examples

An ADR at `adr/0003-storage-engine.md` is accepted on `main` with its `decided` date before `created`. L07 reports `adr/0003-storage-engine.md:6: warning: decided 2026-01-05 is before created 2026-02-01` on every run. With `"acknowledged": {"adr/0003-storage-engine.md": ["L07"]}` the warning is not printed and `--strict-warnings` passes. If it later gains a broken link, L17's warning is printed, because only L07 is acknowledged there.

A repository acknowledges L17 on an RFC at `rfc/0012-retry-policy.md` for a link to a file that was deleted. The file is restored, L17 no longer reports the link, and L22 reports `archdoc.json: error: acknowledged rule L17 reports no warning on rfc/0012-retry-policy.md: remove it from acknowledged`.

### Out of scope

- **A command to write acknowledgements.** The key is edited by hand; `lint --json` gives the path and rule an entry needs.
- **Listing the acknowledged warnings.** `archdoc.json` holds them, and L22 holds each to a warning lint still reports.

## Alternatives considered

The design rests on [ADR-0009](../adr/0009-a-lint-error-names-something-the-user-may-change.md) and [ADR-0023](../adr/0023-every-finding-no-permitted-edit-can-clear-is-a-warning.md), under which every fault in a frozen document that no permitted edit could clear is a warning, so a warning is the only finding an acknowledgement can need to accept.

- **One switch for every warning on a frozen document.** A frozen document's warnings are not all permanent: L17 reports a link in one whose target was deleted elsewhere, which restoring the target clears. One switch hides that task with the rest, and every later one.
- **Per rule, across every frozen document.** Accepting L07 on one document would hide an L07 warning arriving on any other.
- **Leaving the warnings printed and changing only `--strict-warnings`.** The run passes, and the output still mixes accepted faults with new ones.
- **Matching a finding's message.** An acknowledgement holding the message would separate two faults one rule reports on one document, and break when a release rewords the message.

No other alternatives were weighed.

## Backwards compatibility

The exported configuration always holds `acknowledged`, empty where `archdoc.json` names nothing. `archdoc reference` lists the key, L22, and `object` among the types a configuration key can have. Export schema v3 and `archdoc reference` both postdate v0.5.1, so neither addition reaches a released consumer. A repository using the key fails to load under an earlier release, whose `archdoc.json` decoder refuses an unknown key, so a CI pin older than the release adding it fails with a configuration error.

## Changelog

- 2026-11-08: `internal/reference`, `internal/export` and `PROCESS.md` are named in Components. The reference publishes each configuration key's type, the export publishes every setting `archdoc.json` holds, and `PROCESS.md` lists what lint enforces. Step 5 of the effect on lint says L22 reads the findings before any is left out, where it said after step 2. The examples name their documents by path, since [ADR-0003](../adr/0003-reverse-relationships-are-derived.md) and [RFC-0012](0012-rejection-rationale.md) are documents in this repository.
- 2026-11-09: L22 is stated as reporting an acknowledgement once its rule reports no warning on the document, since matching is on path and rule. A warning's rule is taken from `lint --json`, which the printed output does not carry. The claim that an acknowledgement cannot hide a fault that can be cleared is removed, and with it the Abstract's claim that none outlives the fault it accepted. L22's path is stated as relative to `root`. The statement in Backwards compatibility that nothing changes for a repository without the key is removed, since the exported configuration always holds `acknowledged` and `archdoc reference` lists the key. `updates` names [RFC-0006](0006-the-export-publishes-what-it-parsed.md), whose `config` is the file's five settings. The previous entry's date is corrected from 2026-11-04 to 2026-11-08, and the documents it mentions are linked.
- 2026-11-10: The Abstract names each case L22 reports. The `internal/export` component names `acknowledged` as a sixth setting beside the five [RFC-0006](0006-the-export-publishes-what-it-parsed.md) names. Backwards compatibility states that `archdoc reference` also lists L22 and the `object` type, and that it and export schema v3 both postdate v0.5.1.
