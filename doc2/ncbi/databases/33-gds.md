# `gds`

## Role

- official menu name: `GEO DataSets`
- primary result domain: `literature-reference`
- current fit: good GEO dataset mode

## Why users choose it

- search GEO datasets rather than article or profile rows
- dataset-level expression/experiment discovery

## Retrieval plan

- `ESearch(db=gds)`
- `ESummary` for rows
- current runtime state:
  - specialized summary row builder exists
  - `gds -> bioproject` and `gds -> geoprofiles` are active fallback paths

## Important extractions

- GDS accession
- title
- organism
- platform or dataset type

## Important jumps

- `gds -> bioproject`
- `gds -> biosample`
- `gds -> dbvar`
- `gds -> gds_similar`
- `gds -> geoprofiles`

## Table plan

Display:

- `search_term`, `search_type`, `gds_accession`, `title`, `organism`, `platform_or_dataset_type`

Detail/export emphasis:

- dataset notes
- linked sample/project/profile context

## Special prompts

- keep distinct from `geoprofiles`
