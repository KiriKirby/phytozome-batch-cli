# `gapplus`

## Role

- official menu name: `GaPPlus`
- primary result domain: `variant-clinical`
- current fit: specialist/legacy support mode

## Why users choose it

- explicit dbGaP Plus searches where the user knows this token matters

## Retrieval plan

- `ESearch(db=gapplus)`
- `ESummary` only until stronger use cases are proven
- no live `ELink` graph in current `EInfo`

## Important extractions

- accession
- title
- study/source hints
- access/status cues

## Table plan

Display:

- `search_term`, `search_type`, `gapplus_accession`, `title`, `study_type`, `access_status`

Detail/export emphasis:

- raw summary metadata

## Special prompts

- never silently merge this into ordinary `gap`
- keep the token distinction explicit in docs and UI
