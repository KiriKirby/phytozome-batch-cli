# `annotinfo`

## Role

- official menu name: `AnnotInfo`
- primary result domain: `annotation-record`
- current fit: specialist/system-support

## Why users choose it

- annotation index / support metadata, not ordinary end-user biological searching

## Retrieval plan

- `ESearch(db=annotinfo)`
- `ESummary` only
- no `ELink` graph is exposed in live `EInfo`

## Important extractions

- annotation identifier
- title
- description
- organism / annotation context if present

## Table plan

Display:

- `search_term`, `search_type`, `annotation_id`, `title`, `description`

Detail/export emphasis:

- raw summary metadata
- any annotation-source context

## Special prompts

- do not expose as a front-line search type without clear “specialist metadata” wording
