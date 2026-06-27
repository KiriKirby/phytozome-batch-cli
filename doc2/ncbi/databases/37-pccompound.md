# `pccompound`

## Role

- official menu name: `PubChem Compound`
- primary result domain: `chemical-bioassay`
- current fit: specialist chemical mode

## Why users choose it

- search compounds linked to genes, assays, or disease references

## Retrieval plan

- `ESearch(db=pccompound)`
- `ESummary` for rows

## Important extractions

- CID
- compound name
- formula
- mass

## Important jumps

- `pccompound -> gene`
- `pccompound -> mesh`
- `pccompound -> nuccore`
- `pccompound -> omim`
- `pccompound -> pcassay`

## Table plan

Display:

- `search_term`, `search_type`, `cid`, `compound_name`, `formula`, `mass`

Detail/export emphasis:

- linked assay/gene/disease references
- richer compound metadata if later needed

## Special prompts

- keep the UI language chemical-centric, not annotation-centric
