# `structure`

## Role

- official menu name: `Structure`
- primary result domain: `sequence-record` / specialist-reference
- current fit: specialist but biologically meaningful

## Why users choose it

- structure-centric lookup connected to proteins and domains

## Retrieval plan

- `ESearch(db=structure)`
- `ESummary` first
- later optional richer structure-specific detail extraction if the UI gains value from it

## Important extractions

- structure ID
- title
- molecule summary
- method
- organism

## Important jumps

- `structure -> cdd`
- `structure -> gene`
- `structure -> protein`
- `structure -> mmdb`

## Table plan

Display:

- `search_term`, `search_type`, `structure_id`, `title`, `molecule_summary`, `method`, `organism`

Detail/export emphasis:

- linked protein/domain references
- structure method/context

## Special prompts

- no default Canvas/FASTA assumptions
