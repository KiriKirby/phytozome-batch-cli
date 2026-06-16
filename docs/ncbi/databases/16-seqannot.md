# `seqannot`

## Role

- official menu name: `SeqAnnot`
- primary result domain: `annotation-record`
- current fit: specialist but sequence-adjacent

## Why users choose it

- inspect annotation records attached to sequence contexts

## Retrieval plan

- `ESearch(db=seqannot)`
- `ESummary` for main rows
- linked jumps into `nuccore` and `bioproject`

## Important extractions

- annotation accession
- title
- description
- linked sequence/project context

## Important jumps

- `seqannot -> nuccore`
- `seqannot -> bioproject`

## Table plan

Display:

- `search_term`, `search_type`, `annotation_accession`, `title`, `description`, `organism`

Detail/export emphasis:

- linked sequence and project identifiers

## Special prompts

- keep this out of sequence-export actions by default
