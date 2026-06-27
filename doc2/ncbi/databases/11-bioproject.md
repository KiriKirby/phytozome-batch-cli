# `bioproject`

## Role

- official menu name: `BioProject`
- primary result domain: `sample-project`
- current fit: excellent

## Why users choose it

- project-root search for later sample/assembly/SRA drilling
- metadata-centric review of study scope and project type

## Retrieval plan

- `ESearch(db=bioproject)`
- `ESummary` for main rows
- later optional `EFetch` if specific project metadata materially improves detail/export

## Important extractions

- BioProject accession
- project title
- organism
- project type
- data type / scope
- status
- linked resource counts where available

## Important jumps

- `bioproject -> assembly`
- `bioproject -> biosample`
- `bioproject -> sra`
- `bioproject -> gene`
- `bioproject -> protein`
- `bioproject -> pubmed`

## Table plan

Display:

- `search_term`, `search_type`, `label_name`, `bioproject_accession`, `project_title`, `organism`, `project_type`, `status`

Detail/export emphasis:

- scope
- target material
- description
- linked assembly/sample/SRA counts

## Special prompts

- project rows should support jump-first exploration rather than any sequence-export implication
