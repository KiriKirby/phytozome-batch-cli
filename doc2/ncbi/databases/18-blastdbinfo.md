# `blastdbinfo`

## Role

- official menu name: `BlastdbInfo`
- primary result domain: `annotation-record` / system-support
- current fit: low as a normal end-user biological search

## Why users choose it

- inspect BLAST database metadata, not biological primary records

## Retrieval plan

- `ESearch(db=blastdbinfo)`
- `ESummary` only
- no live `ELink` graph in `EInfo`

## Important extractions

- BLAST database name
- title/description
- content type
- update/build metadata

## Table plan

Display:

- `search_term`, `search_type`, `db_name`, `description`, `content_type`, `updated_at`

Detail/export emphasis:

- raw database metadata
- build/update details

## Special prompts

- keep clearly separate from the app's BLAST execution workflows
