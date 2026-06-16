# `biocollections`

## Role

- official menu name: `Biocollections`
- primary result domain: `sample-project`
- current fit: good specialist provenance mode

## Why users choose it

- inspect collection and institution provenance behind biological materials

## Retrieval plan

- `ESearch(db=biocollections)`
- `ESummary` for main rows
- jump into related BioSample / Nuccore / Protein context

## Important extractions

- collection code
- institution
- description
- country / provenance notes
- related collection identifiers

## Important jumps

- `biocollections -> biosample`
- `biocollections -> nuccore`
- `biocollections -> protein`

## Table plan

Display:

- `search_term`, `search_type`, `collection_code`, `institution`, `description`, `country`, `status`

Detail/export emphasis:

- collection identifiers
- provenance notes
- linked biosample/sequence context

## Special prompts

- present as provenance metadata, not as a sequence search type
