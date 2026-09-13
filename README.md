# ArchDoc

A CLI that manages a specification repository: a directory of Markdown documents
with YAML front matter, a fixed lifecycle, and cross-references the tool derives
rather than asks you to maintain.

There are four kinds of document. An **RFC** proposes how something should work
and gets a spec page once it is built. An **ADR** records a decision that
constrains designs without being a thing the spec describes. The **spec**
describes how the project works right now and is always current. A **ref** holds
research and background, and is never normative.

RFCs and ADRs are frozen once they reach a terminal status. To change what an
accepted document says you write a new one that updates or obsoletes it, and the
tool enforces that: eighteen lint rules, a generated index of every derived
relationship, and a CI workflow it scaffolds for you.

## Getting started

```
go install github.com/ollieread/archdoc/cmd/archdoc@latest

mkdir myproject-spec && cd myproject-spec
archdoc init
archdoc new rfc "The first design"
archdoc lint
```

`archdoc init` scaffolds the repository, writes `PROCESS.md` describing the
process it enforces, and generates a GitHub workflow that runs `archdoc lint`
and `archdoc index --check` on every push.

## Documents

- `docs/ARCHDOC.md` specifies what the tool does.
- `internal/template/PROCESS.md` specifies the process it enforces, and is
  authoritative where the two disagree. It is also the copy shipped inside the
  binary and written by `archdoc init`.
- `docs/DECISIONS.md` records why the tool works the way it does, grouped by
  subject, including the alternatives that were tried and rejected.
- `docs/TESTING.md` covers how the suite is built and how to run it.

## Licence

GNU AGPLv3. The full text is in `LICENSE`.

Use it however you like, including inside a company and including commercially.
The obligation only lands if you redistribute a modified archdoc, or run one as
a network service: then you must publish your source under the same licence.
Running it on your own documents creates no obligation at all.

`archdoc init --license=mit` scaffolds an MIT licence for the spec repository
you are creating. That covers your documents and is unrelated to ArchDoc's own
licence.
