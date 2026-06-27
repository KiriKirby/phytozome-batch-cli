# `pcsubstance`

## Role

- official menu name: `PubChem Substance`
- primary result domain: `chemical-bioassay`
- current fit: specialist chemical/submission mode

## Why users choose it

- search substance and submission-oriented PubChem records

## Retrieval plan

- `ESearch(db=pcsubstance)`
- `ESummary` for rows

## Important extractions

- SID
- substance name
- source
- status

## Important jumps

- `pcsubstance -> books`
- `pcsubstance -> gene`
- `pcsubstance -> nuccore`
- `pcsubstance -> omim`
- `pcsubstance -> pcassay`

## Table plan

Display:

- `search_term`, `search_type`, `sid`, `substance_name`, `source`, `status`

Detail/export emphasis:

- submission/source context
- linked assay/gene/disease references

## Special prompts

- do not conflate SID with CID
