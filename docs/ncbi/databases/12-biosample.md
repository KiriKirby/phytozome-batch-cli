# `biosample`

## Role

- official menu name: `BioSample`
- primary result domain: `sample-project`
- current fit: excellent

## Why users choose it

- sample provenance
- source material and environmental context
- drill-down into project, assembly, or run data

## Retrieval plan

- `ESearch(db=biosample)`
- `ESummary` for main rows
- selectively add richer attribute extraction later

## Important extractions

- BioSample accession
- sample name/title
- organism
- core attributes such as isolation source, host, geo location, strain, cultivar, tissue, collection date
- linked BioProject and Assembly context

## Important jumps

- `biosample -> bioproject`
- `biosample -> assembly`
- `biosample -> dbvar`
- `biosample -> taxonomy`

## Table plan

Display:

- `search_term`, `search_type`, `label_name`, `biosample_accession`, `sample_name`, `organism`, `isolation_source`, `host`, `geo_loc_name`

Detail/export emphasis:

- attribute bag / flattened key fields
- collection date
- strain / cultivar / tissue / developmental stage
- linked project and assembly

## Special prompts

- keep the main table narrow even if detail/export exposes many attributes
