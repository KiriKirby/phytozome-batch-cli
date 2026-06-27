# `gene`

## Role

- official menu name: `Gene`
- primary result domain: `gene-record`
- current fit: one of the best direct fits for the current app

## Why users choose it

- gene symbol / locus-oriented searching
- strongest match for `label_name` and `gene_locus` workflows
- natural hub for variant, literature, and sequence jumps

## Retrieval plan

- `ESearch(db=gene)`
- `ESummary` as the default row builder
- optional richer `EFetch` or XML/text detail later
- linked sequence resolution through `ELink` to `protein` and `nuccore`

## Important extractions

- gene UID
- preferred name and nomenclature symbol
- description
- chromosome
- map location
- other aliases
- other designations
- organism / taxid
- summary
- linked nucleotide/protein accessions when later specialized

## Important jumps

- `gene -> protein`
- `gene -> nuccore`
- `gene -> clinvar`
- `gene -> dbvar`
- `gene -> medgen`
- `gene -> gtr`
- `gene -> omim`
- `gene -> cdd`
- `gene -> geoprofiles`
- `gene -> pubmed`

## Table plan

Display:

- `search_term`, `search_type`, `gene_locus`, `label_name`, `labelname_type`, `phgo_alias`, `gene_identifier`, `description`, `location`, `genome`

Detail/export emphasis:

- `symbols`
- `synonyms`
- `comments`
- `ncbi_uid`
- `ncbi_gene_id`
- `ncbi_gene_name`
- `ncbi_gene_description`
- `ncbi_other_aliases`
- `ncbi_other_designations`
- `gene_report_url`

## Special prompts

- gene rows should support jump-first actions to Protein, Nuccore, ClinVar, and MedGen
- sequence export should be explicit linked resolution, not implied inline
