# `gtr`

## Role

- official menu name: `GTR`
- primary result domain: `variant-clinical`
- current fit: good and distinctive

## Why users choose it

- genetic test search
- linked movement to gene, MedGen, OMIM, and orgtrack-style organizational context

## Retrieval plan

- `ESearch(db=gtr)`
- `ESummary` for screening
- later optional `EFetch` only after test/lab detail fields are validated live
- current runtime state:
  - specialized summary row builder exists
  - automatic fallback already supports direct miss -> `clinvar -> gtr`, `medgen -> gtr`, and `omim -> gtr`

## Important extractions

- GTR accession
- test name
- condition
- gene summary
- lab / provider context

## Important jumps

- `gtr -> gene`
- `gtr -> medgen`
- `gtr -> omim`
- `gtr -> orgtrack`

## Table plan

Display:

- `search_term`, `search_type`, `gtr_accession`, `test_name`, `condition`, `gene_identifier`, `lab`

Detail/export emphasis:

- lab/provider
- method notes
- linked disease/gene references

## Special prompts

- GTR rows should foreground test/lab meaning, not only accession identity
