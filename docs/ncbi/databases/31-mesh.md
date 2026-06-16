# `mesh`

## Role

- official menu name: `MeSH`
- primary result domain: `catalog-reference`
- current fit: useful controlled-vocabulary support mode

## Why users choose it

- concept/heading search
- future structured-query assistance for PubMed-like searching

## Retrieval plan

- `ESearch(db=mesh)`
- `ESummary` for rows
- current runtime state:
  - specialized summary row builder exists
  - `mesh -> medgen` jump is indexed and active as a MedGen fallback source

## Important extractions

- MeSH ID
- heading
- scope note short summary

## Important jumps

- `mesh -> gap`
- `mesh -> medgen`
- `mesh -> pccompound`

## Table plan

Display:

- `search_term`, `search_type`, `mesh_id`, `heading`, `scope_note_short`

Detail/export emphasis:

- wider concept notes
- linked disease/chemical contexts

## Special prompts

- no sequence actions
- future query-builder support is more important than row export complexity
