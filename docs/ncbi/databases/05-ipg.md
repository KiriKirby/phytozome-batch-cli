# `ipg`

## Role

- official menu name: `Identical Protein Groups`
- primary result domain: `sequence-record` with strong group semantics
- current fit: good, but not identical to plain protein rows

## Why users choose it

- collapse redundant identical proteins
- inspect shared sequence instances across assemblies or submissions

## Retrieval plan

- `ESearch(db=ipg)`
- `ESummary` for screening
- only use `EFetch` later if a stable richer record format proves useful
- linked retrieval to protein/taxonomy/project context

## Important extractions

- IPG group identifier
- representative accession or display accession
- organism summary
- source assembly / project summary where available
- member counts or source counts when available

## Important jumps

- `ipg -> protein`
- `ipg -> taxonomy`
- `ipg -> bioproject`

## Table plan

Display:

- `search_term`, `search_type`, `label_name`, `protein_id`, `description`, `genome`, `member_count`

Detail/export emphasis:

- group identifier
- representative accession
- source record summary
- linked taxonomy and project context

## Special prompts

- make clear that this row is a group object, not one submitted protein record
