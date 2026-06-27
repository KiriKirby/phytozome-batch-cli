# ELink Jump Patterns

This document defines the linked-navigation graph that `phytozome GO` should eventually surface for NCBI.

## Design rule

- use explicit `linkname` whenever practical
- keep â€œopen link browserâ€?and â€œfollow default jumpâ€?separate
- preserve `dbfrom`, `dbto`, `linkname`, source IDs, and resolved target IDs in snapshot/export metadata when a linked workflow materially changes the result set

## Priority jump chains

### Protein-centered

- `protein -> gene`
- `protein -> nuccore`
- `protein -> cdd`
- `protein -> structure`
- `protein -> taxonomy`
- `protein -> bioproject`
- `protein -> proteinclusters`
- `protein -> protfam`
- `protein -> pubmed`

### Gene-centered

- `gene -> protein`
- `gene -> nuccore`
- `gene -> clinvar`
- `gene -> dbvar`
- `gene -> medgen`
- `gene -> gtr`
- `gene -> omim`
- `gene -> cdd`
- `gene -> geoprofiles`
- `gene -> pubmed`

### Assembly / project / sample / SRA

- `assembly -> bioproject`
- `assembly -> biosample`
- `assembly -> genome`
- `assembly -> nuccore`
- `bioproject -> assembly`
- `bioproject -> biosample`
- `bioproject -> sra`
- `biosample -> bioproject`
- `biosample -> assembly`
- `biosample -> dbvar`
- `sra -> bioproject`
- `sra -> biosample`
- `taxonomy -> assembly|bioproject|biosample|gene|nuccore|protein|sra`

### Variant / clinical

- `clinvar -> gene`
- `clinvar -> medgen`
- `clinvar -> gtr`
- `clinvar -> dbvar`
- `dbvar -> clinvar`
- `dbvar -> bioproject`
- `dbvar -> biosample`
- `snp -> clinvar`
- `snp -> dbvar`
- `medgen -> gene`
- `medgen -> clinvar`
- `medgen -> gtr`
- `medgen -> omim`
- `gtr -> gene`
- `gtr -> medgen`
- `gtr -> omim`
- `omim -> gene`
- `omim -> clinvar`
- `omim -> dbvar`
- `omim -> pubmed`

### Literature / reference

- `pubmed -> pmc`
- `pubmed -> gene`
- `pubmed -> protein`
- `pubmed -> bioproject`
- `pubmed -> biosample`
- `pubmed -> cdd`
- `pmc -> pubmed`
- `books -> gene`
- `books -> medgen`
- `books -> omim`
- `mesh -> medgen`
- `gds -> bioproject`
- `gds -> biosample`
- `geoprofiles -> gene`
- `geoprofiles -> gds`

### Chemical / assay

- `pcassay -> gene`
- `pcassay -> cdd`
- `pcassay -> pccompound`
- `pcassay -> pcsubstance`
- `pccompound -> gene`
- `pccompound -> mesh`
- `pccompound -> omim`
- `pccompound -> pcassay`
- `pcsubstance -> gene`
- `pcsubstance -> omim`
- `pcsubstance -> pcassay`

## UX rule for jump actions

- keep the current result row visible after a jump is chosen
- show the exact jump target in the new group `search_type`, for example:
  - `NCBI Gene -> ClinVar`
  - `NCBI Assembly -> BioProject`
  - `NCBI PubMed -> PMC`
- if multiple `linkname` values target the same `dbto`, show a jump-choice modal rather than silently choosing one
- if a jump returns zero rows, treat that as a real linked-search outcome and report the chosen `linkname`

## Current implemented first-wave state

The current code now executes an expanding subset of the above chains automatically when direct target-database search yields no rows.

### Currently active automatic fallback chains

- `gene -> protein`
  - active for protein retrieval fallback after direct `db=protein` and direct `db=nuccore` fallback fail
- `gene -> nuccore`
  - active for `nuccore` / `nucleotide` keyword fallback
- `gene -> clinvar`
  - active for `clinvar` keyword fallback
- `clinvar -> gene`
  - active for `gene` keyword fallback after direct `db=gene` misses
- `clinvar -> medgen`
  - active for `medgen` keyword fallback
- `clinvar -> gtr`
  - active for `gtr` keyword fallback
- `clinvar -> dbvar`
  - active for `dbvar` keyword fallback
- `assembly -> bioproject`
  - active for `bioproject` keyword fallback
- `biosample -> bioproject`
  - active for `bioproject` keyword fallback after the assembly chain
- `dbvar -> bioproject`
  - active as a later `bioproject` fallback path
- `gds -> bioproject`
  - active as a later `bioproject` fallback path
- `pubmed -> pmc`
  - active for `pmc` keyword fallback
- `pmc -> pubmed`
  - active for `pubmed` keyword fallback
- `medgen -> gene`
  - active for `gene` keyword fallback after earlier source families miss
- `dbvar -> clinvar`
  - active for `clinvar` keyword fallback after direct `db=clinvar` misses
- `snp -> dbvar`
  - active for `dbvar` keyword fallback after direct `db=dbvar` misses
- `gtr -> medgen`
  - active for `medgen` keyword fallback after earlier source families miss
- `omim -> medgen`
  - active as a later `medgen` fallback path
- `gtr -> omim`
  - active for `omim` keyword fallback
- `omim -> pubmed`
  - active as a later `pubmed` fallback path
- `books -> medgen`
  - active as a later `medgen` fallback path
- `gds -> geoprofiles`
  - active for `geoprofiles` keyword fallback
- `geoprofiles -> gene`
  - active as a later `gene` fallback path

### Implemented metadata behavior

For all implemented automatic chains:

- row `search_type` is rewritten to explicit forms such as `NCBI Gene -> ClinVar`
- row metadata now captures:
  - `ncbi_link_resolution=elink`
  - `ncbi_linked_from_db`
  - `ncbi_linked_to_db`
  - `ncbi_linked_from_search_type_id`
  - `ncbi_linked_to_search_type_id`
  - `ncbi_linkname`
  - `ncbi_link_source_ids`
  - `ncbi_link_target_ids`
  - `ncbi_link_source_keyword`

Static jump-preview metadata is also attached per search-type family even when a jump was not executed for the current row:

- `ncbi_jump_targets`
- `ncbi_jump_1_dbto`
- `ncbi_jump_1_linkname`
- `ncbi_jump_1_label`

This is intentionally different from executed-link provenance:

- `ncbi_jump_*` means "known available jump direction for this family"
- `ncbi_link_*` means "this row was actually produced through an executed linked fallback"

## Snapshot rule

If a future linked workflow becomes user-visible, snapshot state should preserve:

- original source database/search type
- jump source IDs
- chosen `linkname`
- target database
- target IDs or History-server state if used

Current implemented snapshot behavior now already preserves the executed linked-fallback summary inside the NCBI keyword source-state module:

- link resolution mode
- source and target database
- source and target search-type IDs
- chosen `linkname`
- source ID set
- target ID set
