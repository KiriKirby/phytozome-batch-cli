# `protfam`

## Role

- official menu name: `Protein Family Models`
- primary result domain: `sequence-group`
- current fit: good for model/family review, not direct sequence export

## Why users choose it

- inspect protein family models
- move from a family concept to its related genes/proteins/domains

## Retrieval plan

- `ESearch(db=protfam)`
- `ESummary` for screening
- linked jumps to CDD, protein, gene, literature, and structure

## Important extractions

- family/model accession
- model title
- model type
- related-domain or related-family summary
- linked member summary

## Important jumps

- `protfam -> cdd`
- `protfam -> gene`
- `protfam -> protein`
- `protfam -> pubmed`
- `protfam -> structure`

## Table plan

Display:

- `search_term`, `search_type`, `family_model_id`, `label_name`, `description`, `model_type`, `member_count`

Detail/export emphasis:

- related proteins
- linked domains
- literature support
- structure support

## Special prompts

- present these as model objects
- keep sequence actions behind explicit linked-navigation choices
