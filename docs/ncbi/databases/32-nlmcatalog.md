# `nlmcatalog`

## Role

- official menu name: `NLM Catalog`
- primary result domain: `catalog-reference`
- current fit: catalog/journal metadata mode

## Why users choose it

- journal/source searching behind biomedical literature

## Retrieval plan

- `ESearch(db=nlmcatalog)`
- `ESummary` for screening rows

## Important extractions

- catalog ID
- title
- publisher
- ISSN
- subject summary

## Important jumps

- `nlmcatalog -> books`

## Table plan

Display:

- `search_term`, `search_type`, `catalog_id`, `title`, `publisher`, `issn`, `subject_summary`

Detail/export emphasis:

- catalog metadata
- source/journal detail

## Special prompts

- keep the UI language clearly catalog-oriented
