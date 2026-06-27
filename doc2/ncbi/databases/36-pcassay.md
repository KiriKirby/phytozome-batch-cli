# `pcassay`

## Role

- official menu name: `PubChem BioAssay`
- primary result domain: `chemical-bioassay`
- current fit: strong specialist linked-reference mode

## Why users choose it

- assay-centric searching tied to genes, proteins, domains, and compounds

## Retrieval plan

- `ESearch(db=pcassay)`
- `ESummary` for main rows
- heavy use of `ELink`

## Important extractions

- assay ID
- title
- target summary
- assay type
- status

## Important jumps

- `pcassay -> books`
- `pcassay -> cdd`
- `pcassay -> gene`
- `pcassay -> pccompound`
- `pcassay -> pcsubstance`

## Table plan

Display:

- `search_term`, `search_type`, `assay_id`, `title`, `target_summary`, `assay_type`, `status`

Detail/export emphasis:

- target details
- linked compounds/substances
- linked gene/domain references

## Special prompts

- clearly separate assay target semantics from sequence target semantics
