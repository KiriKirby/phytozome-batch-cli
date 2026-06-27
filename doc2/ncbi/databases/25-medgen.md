# `medgen`

## Role

- official menu name: `MedGen`
- primary result domain: `variant-clinical`
- current fit: very useful concept layer around genes and variants

## Why users choose it

- disease/condition concept search
- linked movement to genes, ClinVar, GTR, and OMIM

## Retrieval plan

- `ESearch(db=medgen)`
- `ESummary` for main rows
- optional richer fetch only if it clearly improves concept detail
- current runtime state:
  - specialized summary row builder exists
  - automatic fallback already supports direct miss -> `clinvar -> medgen`, `gtr -> medgen`, `omim -> medgen`, `mesh -> medgen`, and `books -> medgen`

## Important extractions

- MedGen accession / concept ID
- preferred title
- condition summary
- related gene summary

## Important jumps

- `medgen -> books`
- `medgen -> clinvar`
- `medgen -> gap`
- `medgen -> gene`
- `medgen -> gtr`
- `medgen -> omim`

## Table plan

Display:

- `search_term`, `search_type`, `medgen_id`, `preferred_title`, `condition_summary`, `related_gene_summary`

Detail/export emphasis:

- linked disease/gene/test references
- concept notes
- `definition` and `source` are now explicit detail/report fields in addition to the compact condition summary columns

## Special prompts

- present as a concept record, not as a sequence or project row
