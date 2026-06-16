# `pubmed`

## Role

- official menu name: `PubMed`
- primary result domain: `literature-reference`
- current fit: valuable, but clearly different from sequence-style search modes

## Why users choose it

- article/citation search around genes, pathways, diseases, or datasets
- jump root into PMC and linked biological entities

## Retrieval plan

- `ESearch(db=pubmed)`
- `ESummary` for article screening
- future optional `ESpell` and `ECitMatch`
- paging strategy must explicitly respect large-result limits
- current runtime state:
  - specialized summary row builder exists
  - automatic fallback already supports direct miss -> `pmc -> pubmed`, plus later-source fallbacks from `clinvar`, `dbvar`, `medgen`, `omim`, `gds`, `geoprofiles`, and `books`

## Important extractions

- PMID
- title
- journal
- publication date
- short author list
- citation support fields

## Important jumps

- `pubmed -> pmc`
- `pubmed -> assembly`
- `pubmed -> bioproject`
- `pubmed -> biosample`
- `pubmed -> cdd`
- `pubmed -> gene`
- `pubmed -> protein`

## Table plan

Display:

- `search_term`, `search_type`, `pmid`, `title`, `journal`, `pub_date`, `authors_short`

Detail/export emphasis:

- citation details
- abstract-like summary when available
- linked biological targets

## Special prompts

- no FASTA/Canvas
- when result counts are huge, explain NCBI paging limits and slicing explicitly
