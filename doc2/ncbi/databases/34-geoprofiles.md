# `geoprofiles`

## Role

- official menu name: `GEO Profiles`
- primary result domain: `literature-reference`
- current fit: good but more detail-heavy than GDS

## Why users choose it

- expression-profile searching tied to genes and datasets

## Retrieval plan

- `ESearch(db=geoprofiles)`
- `ESummary` for screening
- current runtime state:
  - specialized summary row builder exists
  - automatic fallback already supports direct miss -> `gds -> geoprofiles`

## Important extractions

- GEO profile ID
- title
- gene identifier
- organism

## Important jumps

- `geoprofiles -> gds`
- `geoprofiles -> gene`
- `geoprofiles -> nuccore`

## Table plan

Display:

- `search_term`, `search_type`, `geo_profile_id`, `title`, `gene_identifier`, `organism`

Detail/export emphasis:

- linked GDS context
- gene linkage
- profile notes

## Special prompts

- if users arrive here from Gene, preserve that provenance visibly
