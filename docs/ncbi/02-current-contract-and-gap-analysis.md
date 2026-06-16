# Current Contract And Gap Analysis

This document describes what the codebase already guarantees for NCBI and what will break or become ambiguous when more Entrez types are added.

## What Exists Today

### Engine shape

- `internal/ncbi/ncbi.go` is now a multi-search-type Entrez client
- `internal/ncbi/searchtypes.go` defines the current NCBI search-type catalog and result-domain mapping
- `FetchSpeciesCandidates` now returns one synthetic candidate per user-exposed NCBI search type so the existing `DataSource` interface can carry the chosen subtype without a wider interface break
- `internal/searchengine/ncbiprotein/engine.go` classifies three search outcomes:
  - `NCBI protein accession`
  - `NCBI protein keyword`
  - `NCBI nucleotide fallback`
- non-protein NCBI search types already route through a shared generic `ESearch -> ESummary` path that normalizes row metadata into `ncbi_*` extra columns
- first-wave deep specialization is now in place for:
  - `gene`
  - `nuccore`
  - `nucleotide`
  - `assembly`
  - `bioproject`
  - `biosample`
  - `taxonomy`
  - `sra`
- those first-wave types no longer all share one naive generic row template; they now normalize into dedicated row shapes while still preserving `ncbi_*` raw source metadata
- the engine caches only successful non-empty row sets
- empty NCBI result sets are intentionally not cacheable successes

### Workflow shape

- NCBI is modeled as a `keyword/FASTA retrieval` source only
- NCBI is explicitly blocked as a BLAST target
- keyword review can trigger:
  - accession replacement handling from `replacedby`
  - auto-identification of `label_name`
  - auto-identification of `gene_locus`
- NCBI rows can feed later FASTA and Canvas operations because sequence payloads are stored inline

Important implemented nuance:

- this is now true only for genuinely sequence-backed NCBI rows
- first-wave non-protein metadata rows no longer pretend to be peptide-exportable by filling fake `SequenceID` / `ProteinID` values
- sequence exportability must be driven by real sequence payload or an explicit later resolver path, not by the source family being `ncbi`

### UI shape

- the main interface already treats NCBI as a database with `SearchTypes`
- only the curated table-suitable NCBI subset is now exposed in the NCBI Keyword tab search-type selector
- indexed but hidden types still exist internally for documentation, future rollout, and `ELink` graph planning
- current NCBI input grid shows:
  - `search term`
  - optional `symbol name` only for search types configured to show it
  - optional `gene locus` only for search types configured to show it

### Snapshot shape

- NCBI keyword snapshot state already preserves:
  - `EntrezDatabase`
  - `RecordType`
  - `EUtilitiesBaseURL`
  - `EngineSchema`
  - `Accessions`
  - `UIDs`
- snapshot `ResultDomain` for NCBI is now driven by the row result domain instead of being hardcoded to `sequence-record`
- snapshot `Extra` state now also stores the selected NCBI search-type ID

## Hidden Contracts Already Embedded In Code

### Search type is not the same thing as user-visible search-type selector

Today:

- the user-visible search type is only `Protein`
- the row-level `search_type` field can still become:
  - `NCBI protein accession`
  - `NCBI protein keyword`
  - `NCBI nucleotide fallback`
  - plus update annotations such as `(accepted NCBI update)`

Implication:

- future NCBI search-type design must distinguish:
  - requested database/search mode
  - engine-detected row class
  - fallback or update annotations

### NCBI rows already depend on source-specific extra fields

Current rows preserve many `ncbi_*` fields, including:

- accession and UID
- source db and status
- created / updated
- gene name, gene id, locus tag
- product
- `coded_by`
- `other_aliases`
- `other_designations`
- FASTA header and clean protein sequence
- selected static jump metadata for high-value first-wave `ELink` targets:
  - `ncbi_jump_targets`
  - `ncbi_jump_1_dbto`
  - `ncbi_jump_1_linkname`
  - `ncbi_jump_1_label`
  - and so on per indexed jump candidate

Implication:

- new NCBI database integrations should keep the same pattern: core shared row fields plus explicit `ncbi_*` extra columns
- do not collapse database-specific payload into vague generic text blobs

### The NCBI workflow already expects linked enrichment

Current protein rows derive values from multiple linked steps:

- `protein` search and summary
- `protein` FASTA fetch
- linked `gene` summary
- `nuccore` fallback CDS parsing
- linked protein reload from `/protein_id` when nucleotide CDS lacks `/translation`

Implication:

- future NCBI integrations should be designed as small pipelines, not single-endpoint wrappers

## Gaps Preventing Full Entrez Expansion

### Searchable subset versus indexed superset

The code now intentionally separates:

- the full indexed Entrez catalog in `internal/ncbi/searchtypes.go`
- the user-exposed searchable subset returned by `SearchableSearchTypes()` and `FetchSpeciesCandidates()`

This is now a product rule, not a temporary omission:

- literature / knowledge / GEO families are currently hidden from the search selector
- support/specialist families remain indexed but are not treated as ready keyword-table products

Implication:

- future NCBI work can continue enriching hidden types without re-exposing them prematurely
- "indexed" no longer means "user-facing searchable"

### Single-client / single-species assumption

The old one-candidate `ncbi_protein` assumption has now been replaced with synthetic search-type candidates.

Remaining gap:

- this is an architectural bridge, not the final UI/data model
- the app still conceptually treats these as species-like selections because the shared `DataSource` interface does not yet take a first-class search-type argument

Needed direction:

- keep database ID `ncbi`
- eventually make search type an explicit workflow argument instead of a synthetic candidate encoding
- reserve true species/taxonomy prefilters for future NCBI types that actually benefit from them

### Sequence-record bias

Current NCBI flow assumes:

- row may have a sequence
- row may export FASTA
- Canvas transfer may matter
- `gene_locus` is meaningful

Problem:

- many Entrez databases have no protein FASTA concept at all
- `gene_locus` is irrelevant or misleading for many database types

Needed direction:

- define NCBI result-domain families:
  - `sequence-record`
  - `genome-resource`
  - `sample-project`
  - `variant-clinical`
  - `literature-reference`
  - `specialist-reference`

### Protein-specific snapshot schema assumptions

Current snapshot NCBI state is good, but protein-centric.

Problem:

- future databases may need preserved values such as:
  - `WebEnv`
  - `QueryKey`
  - link pipeline steps
  - selected `linkname`
  - row primary IDs that are not accessions
  - fetch format/rettype

Needed direction:

- extend `keyword-source-state` rather than overloading the existing protein-only fields

### Column registry is database-level, not NCBI-search-type-level

Current prompt column registry now supports `ncbi:<result-domain>` synthetic keys in addition to the legacy `ncbi` bucket.

Current implemented refinement:

- first-wave specialized prompt keys now also exist for:
  - `ncbi:gene`
  - `ncbi:nuccore`
  - `ncbi:nucleotide`
  - `ncbi:assembly`
  - `ncbi:bioproject`
  - `ncbi:biosample`
  - `ncbi:taxonomy`
  - `ncbi:sra`

Remaining problem:

- `Protein`, `Gene`, `Assembly`, `ClinVar`, and `PubMed` do not share one sensible display-column list

Needed direction:

- keep result-domain-level fallback lists
- add deeper per-search-type column profiles where result domains are still too broad
- preserve one unified `ncbi_*` namespace for source columns

### Auto-identification logic is protein-oriented

Current NCBI auto label and locus flows assume:

- gene aliases and gene designation text are the first label/locus sources
- Phytozome is a useful secondary alias/locus fallback

Problem:

- that logic fits plant protein/gene rows
- it does not fit literature, sample, clinical, or project records

Needed direction:

- make auto-identification opt-in per NCBI search type
- define when `label_name` and `gene_locus` should be hidden, optional, auto-fillable, or not present at all

## Recommended Internal Split

The current `internal/ncbi` package should evolve into:

- shared E-utilities transport, throttling, retry, History-server, and `EInfo` capability helpers
- search-type-specific engines under `internal/searchengine/ncbi*`
- shared row-normalization helpers for `ncbi_*` metadata

Suggested families:

- `internal/searchengine/ncbiprotein`
- `internal/searchengine/ncbigene`
- `internal/searchengine/ncbinuccore`
- `internal/searchengine/ncbimetadata`
- `internal/searchengine/ncbiclinical`
- `internal/searchengine/ncbiliterature`

This keeps current protein behavior stable while allowing different row contracts per family.
