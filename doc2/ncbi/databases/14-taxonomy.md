# `taxonomy`

## Role

- official menu name: `Taxonomy`
- primary result domain: `taxonomy-reference`
- current fit: excellent

## Why users choose it

- taxonomy search and lineage review
- prefilter or jump root for many other NCBI database families

## Retrieval plan

- `ESearch(db=taxonomy)`
- `ESummary` for main rows
- no need to force `EFetch` early unless a richer lineage payload is needed

## Important extractions

- tax ID
- scientific name
- common name
- rank
- lineage summary
- division
- parent tax ID

## Important jumps

- `taxonomy -> assembly`
- `taxonomy -> bioproject`
- `taxonomy -> biosample`
- `taxonomy -> books`
- `taxonomy -> gene`
- `taxonomy -> protein`
- `taxonomy -> nuccore`
- `taxonomy -> sra`

## Table plan

Display:

- `search_term`, `search_type`, `ncbi_taxonomy_id`, `label_name`, `ncbi_common_name`, `ncbi_rank`, `ncbi_lineage_summary`

Detail/export emphasis:

- full lineage
- division
- genetic code
- parent tax ID
- scientific name as an explicit detail/export field even when `label_name` already carries it

## Special prompts

- taxonomy is a strong candidate for future NCBI prefilter UX, but that should be distinct from ordinary keyword searching
