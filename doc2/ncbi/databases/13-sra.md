# `sra`

## Role

- official menu name: `SRA`
- primary result domain: `sample-project`
- current fit: excellent metadata-first mode

## Why users choose it

- search sequencing runs and experiments
- inspect platform, library, and project/sample provenance

## Retrieval plan

- `ESearch(db=sra)`
- `ESummary` for row screening
- later optional `EFetch` or XML parsing only after exact high-value fields are confirmed

## Important extractions

- SRA accession
- title
- organism
- library strategy
- library source
- platform
- bioproject accession
- biosample accession
- study / experiment / run accession
- spots / bases / instrument model

## Important jumps

- `sra -> bioproject`
- `sra -> biosample`
- `sra -> assembly`
- `sra -> gds`

## Table plan

Display:

- `search_term`, `search_type`, `sra_accession`, `title`, `organism`, `library_strategy`, `library_source`, `platform`, `bioproject_accession`

Detail/export emphasis:

- biosample accession
- study / experiment / run accession
- spots / bases
- layout
- instrument model

## Special prompts

- SRA should remain metadata-first unless a future explicit run-download workflow is designed
