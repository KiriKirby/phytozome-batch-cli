# `pmc`

## Role

- official menu name: `PMC`
- primary result domain: `literature-reference`
- current fit: good companion to PubMed

## Why users choose it

- article/full-text oriented discovery
- linked movement to PubMed and biological references

## Retrieval plan

- `ESearch(db=pmc)`
- `ESummary` for rows
- detail page can later emphasize full-text availability or article identifiers
- current runtime state:
  - specialized summary row builder exists
  - automatic fallback already supports direct miss -> `pubmed -> pmc`

## Important extractions

- PMCID
- title
- journal
- publication date
- short author list

## Important jumps

- `pmc -> pubmed`
- `pmc -> bioproject`
- `pmc -> cdd`
- `pmc -> clinvar`
- `pmc -> gap`

## Table plan

Display:

- `search_term`, `search_type`, `pmcid`, `title`, `journal`, `pub_date`, `authors_short`

Detail/export emphasis:

- related PubMed IDs
- linked biological references

## Special prompts

- frame PMC as full-text/corpus context, not as a duplicate of PubMed
