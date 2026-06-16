# `clinvar`

## Role

- official menu name: `ClinVar`
- primary result domain: `variant-clinical`
- current fit: very important for a deep NCBI rollout

## Why users choose it

- clinical variant interpretation
- review status and significance screening
- linked movement to gene, MedGen, GTR, dbVar, and literature

## Retrieval plan

- `ESearch(db=clinvar)`
- `ESummary` for main rows
- optional later `EFetch` only after exact detail fields are validated live
- current runtime state:
  - specialized summary row builder exists
  - automatic fallback already supports direct miss -> `gene -> clinvar`
  - reverse jump family `clinvar -> gene|medgen|gtr|dbvar` is indexed, and several of those are now active fallback sources for other target databases

## Important extractions

- ClinVar accession / variation ID
- variation name
- gene summary
- clinical significance
- review status
- condition
- variant type

## Important jumps

- `clinvar -> gene`
- `clinvar -> medgen`
- `clinvar -> gtr`
- `clinvar -> dbvar`

## Table plan

Display:

- `search_term`, `search_type`, `clinvar_accession`, `variation_name`, `gene_identifier`, `clinical_significance`, `review_status`, `condition`

Detail/export emphasis:

- variant type
- review/evidence notes
- linked disease and test context

## Special prompts

- if a row is outdated or superseded, show that as a clinical review-state prompt, not as a protein-style accession replacement only
