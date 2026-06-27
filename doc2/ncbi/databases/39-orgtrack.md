# `orgtrack`

## Role

- official menu name: `Orgtrack`
- primary result domain: `catalog-reference` / system-support
- current fit: low, but still worth documenting fully

## Why users choose it

- organizational tracking / registry-style metadata searches

## Retrieval plan

- `ESearch(db=orgtrack)`
- `ESummary` for rows

## Important extractions

- record ID
- title
- status
- organization metadata

## Important jumps

- `orgtrack -> gene`
- `orgtrack -> gtr`
- `orgtrack -> medgen`

## Table plan

Display:

- `search_term`, `search_type`, `record_id`, `title`, `status`

Detail/export emphasis:

- linked GTR/MedGen context
- organization metadata

## Special prompts

- label clearly as specialist support metadata
