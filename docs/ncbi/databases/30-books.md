# `books`

## Role

- official menu name: `Books`
- primary result domain: `literature-reference`
- current fit: useful knowledge-reference mode

## Why users choose it

- search NCBI Bookshelf chapters and knowledge objects
- support concept/gene/disease review with richer narrative references

## Retrieval plan

- `ESearch(db=books)`
- `ESummary` for screening
- current runtime state:
  - specialized summary row builder exists
  - automatic fallback already supports direct miss -> `medgen -> books` and `omim -> books`

## Important extractions

- book/chapter ID
- title
- source
- update date

## Important jumps

- `books -> gene`
- `books -> medgen`
- `books -> nlmcatalog`
- `books -> omim`
- `books -> pcassay`

## Table plan

Display:

- `search_term`, `search_type`, `book_id`, `title`, `source`, `last_update`

Detail/export emphasis:

- knowledge-source metadata
- linked gene/disease references

## Special prompts

- treat as reference support, not dataset support
