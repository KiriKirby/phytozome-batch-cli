# Keyword UI Flow Planning

This document maps future NCBI search types onto the current main-interface Keyword framework.

## Current Framework Constraints

The current Keyword tab already supports:

- database selector
- search-type selector
- optional species selector
- grid columns that vary by database/search type
- pre-execution auto-identification
- result review and export

So the expansion strategy should be:

- keep `database = ncbi`
- expose only the curated NCBI `search type` subset that already fits table review well
- vary species requirement, input columns, and post-search actions by search type family

Current exposed family set:

- sequence / gene-like:
  - `protein`, `gene`, `nuccore`
- genome / project / sample / taxonomy:
  - `assembly`, `bioproject`, `biosample`, `taxonomy`, `sra`
- variant / clinical / medical:
  - `clinvar`, `snp`, `dbvar`, `medgen`, `gtr`, `omim`

Current hidden family set:

- literature / knowledge / GEO:
  - `pubmed`, `pmc`, `books`, `mesh`, `nlmcatalog`, `gds`, `geoprofiles`
- support / specialist:
  - `ipg`, `proteinclusters`, `protfam`, `cdd`, `structure`, `annotinfo`, `blastdbinfo`, `seqannot`, `gap`, `gapplus`, `grasp`, `orgtrack`, `pcassay`, `pccompound`, `pcsubstance`

## Search-Type Families And UI Fit

### Family 1: Sequence-bearing search types

- `protein`
- `nuccore`
- `gene`
- `ipg`
- `proteinclusters`
- `protfam`
- `cdd`

Recommended fit:

- stay in current Keyword tab
- support review-table results directly
- enable downstream sequence-aware actions only where real sequence payload can be produced

Suggested input columns:

- always:
  - `search term`
  - `symbol name`
- optional by type:
  - `gene locus`
  - `organism`
  - `molecule class`

Species selector:

- `protein`: hidden by default, optional future taxon filter mode
- `nuccore`: optional future species/taxonomy filter
- `gene`: likely useful
- `ipg`: optional
- `proteinclusters` / `protfam` / `cdd`: usually not required, but taxon filtering may still be useful later

### Family 2: Genome/sample/project metadata types

- `assembly`
- `bioproject`
- `biosample`
- `taxonomy`
- `genome`
- `biocollections`
- `sra`

Recommended fit:

- still use Keyword tab
- hide `gene locus`
- often hide or downgrade `symbol name` depending on user intent
- support metadata review rows and linked-navigation actions

Suggested input columns:

- `search term`
- optional `symbol name` only when used as a local alias for the query itself, not as a biological label extracted from the result

Species selector:

- often useful
- if shown, interpret as a prefilter, not as a required exact organism target

### Family 3: Variant/clinical types

- `snp`
- `clinvar`
- `dbvar`
- `medgen`
- `gtr`
- `omim`

Recommended fit:

- use Keyword tab for search
- hide `gene locus` by default
- hide auto gene-locus inference entirely
- allow optional local `symbol name` only as user annotation, not required biological row output

Suggested input columns:

- `search term`
- optional `symbol name`

Species selector:

- mostly hidden
- `snp` may benefit from taxonomy filtering later

### Family 4: Literature/reference types

- `pubmed`
- `pmc`
- `books`
- `mesh`
- `nlmcatalog`
- `gds`
- `geoprofiles`

Recommended fit:

- do not expose in the user-facing search selector for the current product phase
- should not imitate sequence workflow
- hide `gene locus`
- usually hide result-derived `symbol name`
- treat optional `symbol name` only as a user note if kept at all

Suggested input columns:

- `search term` only

Species selector:

- hidden in most cases

### Family 5: Specialist/support types

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

Recommended fit:

- keep them out of the user-facing search selector for the current product phase
- expose only after their per-type review semantics are documented

## Pre-Execution Auto-Identification Rules

Auto-identification should become NCBI-search-type-specific.

### Keep current protein-style auto-identification

- `protein`
- `gene`
- `nuccore` when row-level gene fields are present

Use:

- result aliases/designations first
- Phytozome fallback only for plant/gene-style label enrichment

### Disable `gene_locus` auto-identification

- `assembly`
- `biosample`
- `bioproject`
- `taxonomy`
- `sra`
- `pubmed`
- `pmc`
- `books`
- `mesh`
- `nlmcatalog`
- `gds`
- `geoprofiles`
- `clinvar`
- `dbvar`
- `medgen`
- `gtr`
- `omim`

### Restrict `label_name` behavior

For non-sequence and non-gene-like search types:

- `label_name` should mean user annotation only, not inferred biological symbol
- auto-identification should be off unless a strong per-type rule exists

## Review Actions By Family

### Sequence-oriented

- `View`
- `Export`
- `FASTA`
- `Add to Canvas`
- linked navigation:
  - `Gene`
  - `Nucleotide`
  - `Taxonomy`
  - `CDD`
  - `Protein cluster`

### Metadata-oriented

- `View`
- `Export`
- linked navigation
- no FASTA unless the type can materialize sequence through a defined link step

### Literature/reference-oriented

- `View`
- `Export`
- `Open NCBI page`
- linked navigation to sequence/gene/project only when available
- no Canvas/tree transfer

## Result-Domain Routing Rule

Future NCBI search types should set an explicit result domain before review:

- `sequence-record`
- `sequence-group`
- `genome-resource`
- `sample-project`
- `variant-clinical`
- `literature-reference`
- `specialist-reference`

This domain should control:

- visible columns
- detail tabs
- allowed exports
- Canvas eligibility
- auto-identification behavior
- snapshot metadata

Do not keep deriving all of this from `database == ncbi`.
