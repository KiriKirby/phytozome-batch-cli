# NCBI Integration Design Library

This directory is the source of truth for expanding `phytozome GO` NCBI support beyond the current protein-first implementation.

The goal of this library is not only to list NCBI databases, but to define:

- which Entrez search types fit the current `Keyword` interface directly and should stay exposed in the user-facing search-type selector
- which types require special execution, linked retrieval, or database-specific review behavior
- which NCBI-only mechanisms must be surfaced in workflow and snapshot design
- how result tables, detail views, exports, and Canvas transfers should be shaped for each supported type

## Current Baseline

The current codebase already has a non-trivial NCBI contract:

- `internal/ncbi` implements an E-utilities client
- `internal/searchengine/ncbiprotein` classifies `NCBI protein accession`, `NCBI protein keyword`, and `NCBI nucleotide fallback`
- `internal/workflow` handles NCBI replacement accessions, NCBI Gene-locus auto-identification, snapshot source metadata, and cached sequence reuse
- `internal/tui/main_interface.go` now models NCBI as a database with a curated table-suitable search-type subset exposed to users

That means future work must extend an existing architecture instead of replacing it with a generic "NCBI search" abstraction.

## Current search-entry rule

The codebase now distinguishes between:

- all indexed NCBI Entrez types kept internally for link traversal, future specialization, and documentation
- the smaller user-facing `searchable` subset that is actually exposed in the NCBI search-type selector

Current exposed NCBI search types are intentionally limited to table-suitable families that fit the existing keyword-result workflow:

- `protein`
- `gene`
- `nuccore`
- `assembly`
- `bioproject`
- `biosample`
- `taxonomy`
- `sra`
- `clinvar`
- `snp`
- `dbvar`
- `medgen`
- `gtr`
- `omim`

Currently indexed but hidden from the search selector:

- legacy or duplicate sequence/support entries such as `nucleotide`
- literature / knowledge / GEO entries such as `pubmed`, `pmc`, `books`, `mesh`, `nlmcatalog`, `gds`, `geoprofiles`
- specialist/support families such as `ipg`, `proteinclusters`, `protfam`, `cdd`, `structure`, `annotinfo`, `blastdbinfo`, `seqannot`, `gap`, `gapplus`, `grasp`, `orgtrack`, `pcassay`, `pccompound`, and `pcsubstance`

Hidden does not mean deleted:

- their Entrez DB tokens stay indexed
- link metadata and `ELink` planning can still reference them
- per-database documentation stays in this library

## Document Map

- [01-entrez-database-matrix.md](./01-entrez-database-matrix.md)
  Official Entrez database inventory, grouped by integration fit and workflow class.
- [02-current-contract-and-gap-analysis.md](./02-current-contract-and-gap-analysis.md)
  Current code behavior, hidden assumptions, and where future NCBI types will pressure the existing design.
- [03-eutilities-mechanics-and-ncbi-specific-risks.md](./03-eutilities-mechanics-and-ncbi-specific-risks.md)
  Official E-utilities mechanics, rate limits, History server, batching, replacement/update handling, and other NCBI-specific concerns.
- [04-keyword-ui-flow-planning.md](./04-keyword-ui-flow-planning.md)
  How each NCBI search type should map onto the current Keyword tab, pre-execution prompts, review flow, and navigation.
- [05-result-table-column-design.md](./05-result-table-column-design.md)
  Detailed result-table, detail-view, export, and Canvas column planning by NCBI type family.
- [06-implementation-roadmap.md](./06-implementation-roadmap.md)
  Recommended staged implementation order, engine split, tests, snapshot updates, and migration checkpoints.
- [07-database-profiles.md](./07-database-profiles.md)
  Per-database profiles for every currently indexed Entrez database, including result domain, special NCBI concerns, link/navigation expectations, and table-column planning.
- [08-official-source-notes.md](./08-official-source-notes.md)
  Notes about official source drift, Bookshelf-versus-live-`EInfo` differences, and rules for choosing the current authority.
- [09-live-einfo-2026-06-13-summary.md](./09-live-einfo-2026-06-13-summary.md)
  Live `EInfo` baseline, current field/link counts, and top-level-versus-`version=2.0` endpoint behavior notes.
- [10-database-family-deep-specialization.md](./10-database-family-deep-specialization.md)
  Deep specialization rules by NCBI database family.
- [11-elink-jump-patterns.md](./11-elink-jump-patterns.md)
  Planned `ELink` jump graph, jump UX, and snapshot requirements.
- [12-efetch-extraction-plans.md](./12-efetch-extraction-plans.md)
  Which families should use `EFetch`, how deeply, and for which extracted fields.
- [13-update-replacement-and-redirect-strategy.md](./13-update-replacement-and-redirect-strategy.md)
  Generalized replacement/update/jump prompt strategy beyond the current protein-only flow.
- [14-database-document-index.md](./14-database-document-index.md)
  File index for all per-database NCBI specialization documents.
- [databases/README.md](./databases/README.md)
  Per-database specialization directory.

## Current depth status

As of 2026-06-13, this library now contains:

- the live 39-database Entrez inventory baseline
- family-level specialization rules
- explicit `ELink` jump-graph planning
- explicit `EFetch` extraction planning
- update/replacement/redirect prompt strategy
- one specialization document for every currently indexed Entrez database token

## Scope Rule

This library is about NCBI data-source integration through official Entrez/E-utilities surfaces. It does not redefine:

- NCBI remote BLAST as a normal data-source search mode
- ad hoc HTML scraping of NCBI web pages as the primary retrieval path
- cross-database UI work that is unrelated to NCBI requirements

## Source Rule

When updating this directory:

- prefer official NCBI sources, especially E-utilities Bookshelf documentation and live `EInfo` metadata
- record when a conclusion is code-derived, source-derived, or inference-derived
- keep exact Entrez database names aligned with current `EInfo` output rather than informal aliases
