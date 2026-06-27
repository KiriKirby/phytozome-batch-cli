# Entrez Database Matrix

This document maps the current official Entrez databases to `phytozome GO` integration classes.

Source basis:

- official live `EInfo` database list from `https://eutils.ncbi.nlm.nih.gov/entrez/eutils/einfo.fcgi?retmode=json`
- official NCBI E-utilities Bookshelf documentation
- current code constraints in `internal/ncbi`, `internal/workflow`, `internal/tui`, and snapshot modules

Authority rule for this matrix:

- prefer live `EInfo` for the current database inventory
- prefer Bookshelf for stable protocol semantics and historical notes
- if they disagree on the current number of databases, treat live `EInfo` as the current source of truth and record the discrepancy explicitly

As of 2026-06-13, live `EInfo` returns 39 databases:

- `pubmed`
- `protein`
- `nuccore`
- `ipg`
- `nucleotide`
- `structure`
- `genome`
- `annotinfo`
- `assembly`
- `bioproject`
- `biosample`
- `blastdbinfo`
- `books`
- `cdd`
- `clinvar`
- `gap`
- `gapplus`
- `grasp`
- `dbvar`
- `gene`
- `gds`
- `geoprofiles`
- `medgen`
- `mesh`
- `nlmcatalog`
- `omim`
- `orgtrack`
- `pmc`
- `proteinclusters`
- `pcassay`
- `protfam`
- `pccompound`
- `pcsubstance`
- `seqannot`
- `snp`
- `sra`
- `taxonomy`
- `biocollections`
- `gtr`

Official-source discrepancy note:

- the current Bookshelf E-utilities overview still contains text stating that Entrez "currently includes 38 databases"
- live `EInfo` on 2026-06-13 returns 39 database tokens
- implementation planning should follow the live `EInfo` list while keeping the Bookshelf chapter as the authority for request semantics

## Integration Classes

### Class A: Direct keyword-row databases

These can map most naturally to the current `Keyword -> review table -> export/Canvas` workflow with database-specific columns and detail panes.

- `protein`
- `nuccore`
- `gene`
- `assembly`
- `biosample`
- `bioproject`
- `taxonomy`
- `snp`
- `clinvar`
- `dbvar`
- `sra`
- `gtr`
- `medgen`
- `omim`

Why:

- each has a primary record concept that can be rendered as one result row
- each has meaningful `ESearch -> ESummary -> EFetch` or `ESearch -> ESummary` retrieval paths
- each can be represented with a stable identifier plus summary metadata

Current product rule:

- this class is the current NCBI search-selector whitelist
- only these types should stay visible in the user-facing search UI for now

### Class B: Search-first but link-heavy databases

These are still suitable for the Keyword tab, but they should be treated as "record + linked expansion" workflows rather than simple single-record fetches.

- `protein`
- `nuccore`
- `gene`
- `assembly`
- `biosample`
- `bioproject`
- `taxonomy`
- `ipg`
- `proteinclusters`
- `protfam`
- `cdd`
- `snp`
- `clinvar`
- `dbvar`
- `sra`

Why:

- the user will often want linked records more than the source record itself
- `ELink` and History-server chaining should be first-class in engine design
- review UI should expect linked-navigation actions, not just static row export

### Class C: Searchable, but better modeled as literature/knowledge review pages

These fit Entrez technically, but they are not a good biological sequence-row analogue.

- `pubmed`
- `pmc`
- `books`
- `mesh`
- `nlmcatalog`
- `gds`
- `geoprofiles`

Why:

- record payloads are article, concept, catalog, GEO profile, or dataset summaries
- Canvas/tree/FASTA transfer is usually not meaningful
- export should emphasize citations, identifiers, URLs, and summaries rather than sequence-related columns

Current product rule:

- keep these indexed internally for future work, docs, and `ELink` graph planning
- do not expose them in the current user-facing NCBI search selector

### Class D: Searchable, but likely better as secondary/reference panels before full first-class integration

- `structure`
- `annotinfo`
- `blastdbinfo`
- `gap`
- `gapplus`
- `grasp`
- `orgtrack`
- `pcassay`
- `pccompound`
- `pcsubstance`
- `seqannot`
- `nucleotide`

Why:

- some are specialist/admin/index-oriented databases rather than common user-facing search targets
- some overlap strongly with better primary entry points
- some are likely useful mainly through links from other databases

Current product rule:

- keep these indexed internally
- do not expose them in the current user-facing NCBI search selector

## Current User-Facing Search-Type Families

Instead of exposing all 39 databases as flat choices, the current product exposes only the table-suitable whitelist grouped into search-type families.

### Sequence and annotation family

- Protein
- Nucleotide / Nuccore
- Gene

Best fit:

- current Keyword workflow
- row review
- FASTA or linked-sequence export
- Canvas transfer for sequence-bearing types where real sequence exists

### Genome and sample provenance family

- Assembly
- BioProject
- BioSample
- Taxonomy
- SRA

Best fit:

- metadata-centric review tables
- linked-navigation into sequence or gene records
- export as metadata tables rather than FASTA

### Variant and clinical family

- SNP
- ClinVar
- dbVar
- MedGen
- GTR
- OMIM

Best fit:

- evidence- and phenotype-centric review
- linked navigation between variant, gene, phenotype, and testing records
- no direct FASTA expectation for most types

### Literature and knowledge family

- currently hidden from the search selector
- retain as indexed/documented future capability only

### Specialist/support family

- currently hidden from the search selector
- retain as indexed/documented future capability only

## Current-Framework Fit Ranking

### Best immediate fit

- `gene`
- `nuccore`
- `assembly`
- `biosample`
- `bioproject`
- `taxonomy`
- `sra`

Reason:

- these can reuse the current `search rows -> review table -> export/details` pattern with minimal structural change

### Good fit, but needs linked or special payload handling

- `ipg`
- `proteinclusters`
- `protfam`
- `cdd`
- `snp`
- `clinvar`
- `dbvar`
- `medgen`
- `gtr`
- `omim`

Reason:

- review-table model still works
- row meaning, linked expansions, and details become more important than FASTA

### Poor fit for direct keyword-mode parity

- `pubmed`
- `pmc`
- `books`
- `mesh`
- `nlmcatalog`
- `gds`
- `geoprofiles`
- `structure`
- `blastdbinfo`
- `annotinfo`
- `grasp`
- `orgtrack`

Reason:

- they need different table priorities, actions, and export semantics
- some may belong better in `Explore` or a future `Reference` tab rather than plain biological keyword mode

## NCBI Naming Rule

Future UI labels should use human names that match NCBI menu naming, while internal IDs should continue to use the exact Entrez database token.

Examples:

- internal `protein`, label `Protein`
- internal `nuccore`, label `Nucleotide`
- internal `gds`, label `GEO DataSets`
- internal `protfam`, label `Protein Family Models`

Avoid inventing local aliases that obscure the official Entrez database token.
