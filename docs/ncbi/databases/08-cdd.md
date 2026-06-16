# `cdd`

## Role

- official menu name: `Conserved Domains`
- primary result domain: `sequence-group`
- current fit: strong domain/reference fit

## Why users choose it

- domain-centric searching
- conservation checks around proteins and genes
- jump hub between domain, structure, protein, and literature

## Retrieval plan

- `ESearch(db=cdd)`
- `ESummary` for main rows
- linked navigation is usually more important than `EFetch`

## Important extractions

- CDD accession / UID
- domain/model title
- source database
- domain class or model class
- related family/superfamily cues

## Important jumps

- `cdd -> protein`
- `cdd -> gene`
- `cdd -> structure`
- `cdd -> pubmed`

## Table plan

Display:

- `search_term`, `search_type`, `cdd_id`, `label_name`, `description`, `source`, `domain_class`

Detail/export emphasis:

- linked proteins
- linked structures
- domain/family relationships

## Special prompts

- if the user arrived from Protein or Gene, preserve the source-jump context visibly in `search_type`
