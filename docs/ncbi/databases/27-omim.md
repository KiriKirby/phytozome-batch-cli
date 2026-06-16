# `omim`

## Role

- official menu name: `OMIM`
- primary result domain: `variant-clinical`
- current fit: important linked knowledge layer

## Why users choose it

- disease/gene knowledge-object searching
- jump hub to gene, ClinVar, dbVar, MedGen, and literature

## Retrieval plan

- `ESearch(db=omim)`
- `ESummary` for rows
- later optional richer extraction only if it improves summary-to-detail continuity
- current runtime state:
  - specialized summary row builder exists
  - automatic fallback already supports direct miss -> `clinvar -> omim`, `medgen -> omim`, `gtr -> omim`, `dbvar -> omim`, `geoprofiles -> omim`, and `pubmed -> omim`

## Important extractions

- OMIM ID
- title
- condition summary
- related gene summary

## Important jumps

- `omim -> biosample`
- `omim -> books`
- `omim -> clinvar`
- `omim -> dbvar`
- `omim -> gene`
- `omim -> medgen`
- `omim -> protein`
- `omim -> pubmed`

## Table plan

Display:

- `search_term`, `search_type`, `omim_id`, `title`, `condition_summary`, `related_gene_summary`

Detail/export emphasis:

- linked gene/variant/literature relationships
- condition notes
- OMIM text is now an explicit detail/report field rather than remaining only implicit background text

## Special prompts

- if the row was reached from Gene or ClinVar, preserve that jump provenance visibly
