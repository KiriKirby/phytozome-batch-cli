# Database Family Deep Specialization

This document converts the 39 Entrez databases into implementation families with shared product rules.

## Family 1: Sequence-bearing search types

Databases:

- `protein`
- `nuccore`
- `gene` via linked sequence resolution

Shared rules:

- support sequence-aware review actions only when a real sequence payload or a reliable linked sequence path exists
- preserve both external accession and internal UID
- snapshot should keep sequence-capable provenance separately from plain summary metadata
- replacement/redirect prompts matter most here

## Family 2: Sequence-group and model search types

Databases:

- currently indexed internally, but hidden from the search selector:
  - `proteinclusters`
  - `protfam`
  - `cdd`

Shared rules:

- the primary row is a group/model/domain object, not one submitted protein
- row details should foreground representative/member relationships
- `ELink` expansion is often more important than `EFetch`
- FASTA export should be explicit and opt-in, never implied by `SourceDatabase=ncbi`

## Family 3: Genome, assembly, sample, and project provenance

Databases:

- `assembly`
- `bioproject`
- `biosample`
- `taxonomy`
- `sra`

Shared rules:

- metadata-first main tables
- strong linked navigation between project, sample, assembly, sequence, and taxonomy
- no `gene_locus` field by default
- attribute-heavy records should stay compact in the main table and expand in detail/export

Current product rule:

- this family remains in the user-facing NCBI search selector
- keep row shapes close to the existing keyword table grammar instead of inventing a wholly separate metadata-browser page for now
- current runtime state:
  - each of `assembly`, `bioproject`, `biosample`, `taxonomy`, and `sra` has its own summary row normalizer
  - each now fills record-native `label_name` / `labelname_type`
  - each now emits a first stable layer of family-specific `ncbi_*` fields for detail/export work
  - the main visible table is still intentionally compact, but `assembly`, `biosample`, `sra`, and `taxonomy` now already expose selected family-specific `ncbi_*` columns directly in the main table

## Family 4: Variant, clinical, and medical reference

Databases:

- `snp`
- `clinvar`
- `dbvar`
- `medgen`
- `gtr`
- `omim`

Shared rules:

- emphasize significance, review status, phenotype, condition, and evidence
- keep gene/protein links visible, but do not force sequence-first framing
- support “jump to related gene / ClinVar / MedGen / GTR / OMIM” actions as first-class review operations
- update/withdrawal/access-status prompts matter more than FASTA

Current product rule:

- this family remains in the user-facing NCBI search selector
- `gap`, `gapplus`, and `grasp` stay indexed internally but hidden for now
- current runtime state:
  - each of `snp`, `clinvar`, `dbvar`, `medgen`, `gtr`, and `omim` has its own summary row normalizer
  - each now fills record-native `label_name` / `labelname_type`
  - each now emits a first stable layer of family-specific `ncbi_*` significance / condition / accession fields
  - active `ELink` fallback chains now already connect important members of this family to each other and to `gene`
  - `clinvar` and `gtr` now also perform targeted XML `EFetch` enrichment
  - `snp`, `dbvar`, `medgen`, `gtr`, and `omim` now have specialized visible main-table `ncbi_*` columns rather than falling back entirely to generic keyword columns
  - `snp`, `dbvar`, `medgen`, and `omim` also now expose wider detail/report schemas so fields such as SNP position, MedGen definition/source, OMIM text, and replacement/update decisions are visible beyond raw row metadata

## Family 5: Literature, catalog, and GEO knowledge

Databases:

- `pubmed`
- `pmc`
- `books`
- `mesh`
- `nlmcatalog`
- `gds`
- `geoprofiles`

Shared rules:

- cite/reference export rather than sequence export
- compact article/dataset screening columns in the main table
- rich detail page sections for abstract/source/journal/dataset notes
- `ESpell`, `ECitMatch`, and large-result paging should be documented as future special support points

Current product rule:

- keep this entire family hidden from the current user-facing NCBI search selector
- do not spend current implementation time polishing them as search-table products
- retain indexing, docs, and link planning for future dedicated knowledge-mode work

## Family 6: Chemical and assay reference

Databases:

- `pcassay`
- `pccompound`
- `pcsubstance`

Shared rules:

- chemical- and assay-centric tables
- strong jump chains to genes, proteins, compounds, and substances
- do not reuse plant-sequence-oriented label heuristics

Current product rule:

- keep this family hidden from the current user-facing NCBI search selector

## Family 7: Specialist and system-support types

Databases:

- `annotinfo`
- `blastdbinfo`
- `orgtrack`

Shared rules:

- summary-first integration
- likely disabled for Canvas/FASTA permanently
- expose only after the UI can clearly signal “specialist metadata mode”

Current product rule:

- keep this family hidden from the current user-facing NCBI search selector

## Product consequence

The product should never again treat “NCBI” as one workflow shape. The correct level of specialization is:

1. source family = `ncbi`
2. result domain family
3. database token
4. optional row class / linked jump class inside that database
