# `genome`

## Role

- official menu name: `Genome`
- primary result domain: `genome-resource`
- current fit: useful but summary/link-centric

## Why users choose it

- broad genome-level entry point
- species-level genome overview before dropping into assembly or sequence records

## Retrieval plan

- `ESearch(db=genome)`
- `ESummary` as the primary row builder
- no dependence on `EFetch`, because official docs note `db=genome` no longer has normal `EFetch` support

## Important extractions

- genome ID
- organism
- title / summary
- status
- linked assembly and sequence context

## Important jumps

- `genome -> assembly`
- `genome -> bioproject`
- `genome -> gene`
- `genome -> nuccore`

## Table plan

Display:

- `search_term`, `search_type`, `genome_id`, `organism`, `label_name`, `description`, `status`

Detail/export emphasis:

- summary text
- linked assembly information
- linked sequence context

## Special prompts

- never promise full-record fetch parity here unless live official docs change
