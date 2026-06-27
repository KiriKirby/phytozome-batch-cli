# `dbvar`

## Role

- official menu name: `dbVar`
- primary result domain: `variant-clinical`
- current fit: good

## Why users choose it

- structural variant discovery
- project/sample/ClinVar-linked variation review

## Retrieval plan

- `ESearch(db=dbvar)`
- `ESummary` first
- later optional richer extraction for phenotype/assertion fields
- current runtime state:
  - specialized summary row builder exists
  - automatic fallback already supports direct miss -> `clinvar -> dbvar`, `snp -> dbvar`, and `gds -> dbvar`

## Important extractions

- dbVar accession
- variant type
- phenotype summary
- related gene summary
- clinical assertion summary

## Important jumps

- `dbvar -> bioproject`
- `dbvar -> biosample`
- `dbvar -> clinvar`
- `dbvar -> gap`
- `dbvar -> gds`

## Table plan

Display:

- `search_term`, `search_type`, `dbvar_accession`, `variant_type`, `gene_identifier`, `phenotype`, `clinical_assertion`

Detail/export emphasis:

- project/sample provenance
- linked ClinVar
- phenotype/evidence notes
- replacement/update decision fields when a source record is superseded

## Special prompts

- clearly distinguish structural-variant semantics from SNP semantics
