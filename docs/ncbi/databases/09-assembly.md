# `assembly`

## Role

- official menu name: `Assembly`
- primary result domain: `genome-resource`
- current fit: excellent metadata-first search type

## Why users choose it

- search genome assemblies directly
- inspect assembly accession, level, and status
- move into linked project/sample/sequence context

## Retrieval plan

- `ESearch(db=assembly)`
- `ESummary` for main rows
- later optional `EFetch` only if it yields richer release/export fields worth the cost

## Important extractions

- assembly accession
- assembly name
- organism
- assembly level
- RefSeq category / status
- submitter
- release/submission dates
- linked BioProject / BioSample hints
- FTP or download path when surfaced

## Important jumps

- `assembly -> bioproject`
- `assembly -> biosample`
- `assembly -> genome`
- `assembly -> nuccore`
- `assembly -> gene`

## Table plan

Display:

- `search_term`, `search_type`, `ncbi_assembly_accession`, `label_name`, `genome`, `ncbi_assembly_level`, `ncbi_assembly_status`, `ncbi_bioproject_accession`

Detail/export emphasis:

- submitter
- submission date
- RefSeq category
- linked project/sample accessions
- FTP path
- taxonomy ID
- replacement/update visibility when `replacedby` exists

## Special prompts

- no `gene_locus`
- do not offer FASTA/Canvas by default
