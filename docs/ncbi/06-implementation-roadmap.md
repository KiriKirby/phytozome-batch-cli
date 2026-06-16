# Implementation Roadmap

This document recommends a practical implementation order for full NCBI expansion.

## Guiding Principle

Do not try to expose all 39 Entrez databases at once. First stabilize the shared NCBI substrate, then onboard the database families that best fit the current interface.

## Stage 0: Shared Infrastructure First

Before adding many new search types, build or refactor:

- shared `EInfo` discovery cache
- shared GET/POST transport helper
- shared History-server helper
- shared batch `ESummary` / `EFetch` / `ELink` runner
- shared result-domain typing
- shared NCBI per-search-type column profile registry
- shared snapshot extension for NCBI source-state beyond protein-only assumptions

This stage reduces later duplication more than any early UI work.

### Stage 0 current status

Already completed in code:

- NCBI search-type catalog under `internal/ncbi/searchtypes.go`
- synthetic per-search-type NCBI candidates to carry subtype choice through the existing source interface
- main-interface NCBI search-type selector expanded from one protein type to the indexed Entrez set
- protein-specialized path preserved as-is
- generic non-protein `ESearch -> ESummary` row path added as the first shared metadata substrate
- prompt/report/snapshot routing upgraded from plain `ncbi` to `ncbi:<result-domain>` where available

Still remaining inside Stage 0:

- live `EInfo` discovery cache instead of static compile-time catalog only
- shared POST / History-server helper reuse for large non-protein result sets
- shared `ELink` traversal helpers
- deeper per-search-type column plans inside the broad result-domain families

## Stage 1: Best-fit direct expansions

Implement first:

- `gene`
- `nuccore`
- `assembly`
- `biosample`
- `bioproject`
- `taxonomy`
- `sra`

Why:

- these fit the current Keyword tab well
- they exercise multiple result domains
- they establish the patterns needed for later clinical and literature types

### Stage 1 current status

Already completed in runtime:

- first-wave direct row specialization for `gene`, `nuccore`, `nucleotide`, `assembly`, `bioproject`, `biosample`, `taxonomy`, and `sra`
- per-search-type prompt/detail/export routing for those databases instead of one broad generic NCBI table
- non-sequence metadata rows no longer pretend to be protein-exportable by filling fake `SequenceID` / `ProteinID`

## Stage 2: Link-heavy biological reference types

Implement next:

- `ipg`
- `proteinclusters`
- `protfam`
- `cdd`

Why:

- they are biologically relevant to the existing app
- they benefit heavily from `ELink`
- they prepare the codebase for more graph-like NCBI traversal

## Stage 3: Variant and clinical family

Implement next:

- `snp`
- `clinvar`
- `dbvar`
- `medgen`
- `gtr`
- `omim`

Why:

- these require clear non-sequence result-domain handling
- they stress the "NCBI is not always FASTA" rule

### Stage 3 current status

Already completed in runtime:

- first-wave summary-specialized row builders for `snp`, `clinvar`, `dbvar`, `medgen`, `gtr`, and `omim`
- automatic `ELink` fallback execution now exists for a meaningful subset of this family:
  - `gene -> clinvar`
  - `clinvar -> gene`
  - `clinvar -> medgen`
  - `clinvar -> gtr`
  - `clinvar -> dbvar`
  - `medgen -> gene`
  - `gtr -> medgen`
  - `gtr -> omim`
  - `dbvar -> clinvar`
  - `dbvar -> bioproject`
  - `snp -> dbvar`
  - `omim -> medgen`
  - `omim -> pubmed`
- snapshot source-state now preserves linked fallback provenance, not only the final target row IDs

Still remaining:

- deeper `EFetch`-driven extraction where a target database exposes meaningfully richer payloads than `ESummary`; this is still especially relevant for structural-variant placement details and some multi-source condition normalization
- explicit UI jump actions where multiple target `linkname` values remain important choices rather than silent fallback order

## Stage 4: Literature and knowledge family

Implement after the above:

- `pubmed`
- `pmc`
- `books`
- `mesh`
- `nlmcatalog`
- `gds`
- `geoprofiles`

Why:

- these are valuable, but they fit the current export/Canvas assumptions least naturally

### Stage 4 current status

Already completed in runtime:

- first-wave summary-specialized row builders for `pubmed`, `pmc`, `books`, `mesh`, `gds`, and `geoprofiles`
- automatic `ELink` fallback execution now exists for:
  - `pubmed -> pmc`
  - `pmc -> pubmed`
  - `pubmed -> medgen`
  - `pubmed -> omim`
  - `pmc -> bioproject`
  - `books -> medgen`
  - `gds -> bioproject`
  - `gds -> geoprofiles`
  - `geoprofiles -> gene`
  - `geoprofiles -> pubmed`

Still remaining:

- richer literature/detail flattening for author lists, journal/citation fields, and future abstract/full-text specific extraction
- explicit UI distinctions between article-search rows, concept-reference rows, and GEO dataset/profile rows

## Stage 5: Specialist/support databases

Only after clear product need:

- `structure`
- `annotinfo`
- `blastdbinfo`
- `gap`
- `gapplus`
- `grasp`
- `orgtrack`
- `seqannot`
- `pcassay`
- `pccompound`
- `pcsubstance`

## Recommended Package Split

- `internal/ncbi`
  - shared transport
  - throttle
  - retry
  - `EInfo`
  - History server
  - cross-cutting helpers
- `internal/searchengine/ncbiprotein`
- `internal/searchengine/ncbigene`
- `internal/searchengine/ncbinuccore`
- `internal/searchengine/ncbimetadata`
- `internal/searchengine/ncbiclinical`
- `internal/searchengine/ncbiliterature`

## Snapshot Work Required For Every New NCBI Type

For each added search type, update in the same change:

- keyword source-state schema/content
- result-domain persistence
- row extra-column preservation
- sequence cache preservation when relevant
- linked-record provenance preservation when relevant
- reopen tests for review state and later export/detail behavior

Per the repository rules, NCBI feature work is not complete until snapshot behavior is updated deliberately.

## Test Strategy

### Unit tests

- `EInfo` capability parsing
- `ESearch` query building
- `ESummary` normalization per type
- `ELink` pipeline helpers
- row normalization and result-domain classification
- snapshot source-state persistence

### Controlled integration tests

- mocked `ESearch -> ESummary -> EFetch`
- mocked `ESearch -> ELink -> ESummary`
- replacement/update status propagation where applicable
- paging and batch behavior

### Optional live tests

- gated by environment variables
- one smoke test per major family, not exhaustive matrix runs
- verify current official endpoint behavior, especially JSON shape drift and linkname assumptions

## UI Rollout Rule

Expose NCBI search types in the main interface only after:

- engine contract exists
- column profile exists
- snapshot behavior exists
- result actions are defined
- unsupported actions are explicitly disabled

Do not expose search-type choices that still collapse into generic "view raw text" or "no export semantics yet" behavior unless they are clearly marked as limited.
