# `snp`

## Role

- official menu name: `SNP`
- primary result domain: `variant-clinical`
- current fit: good

## Why users choose it

- rsID-centric searching
- variant-to-gene/ClinVar/dbVar linkage

## Retrieval plan

- `ESearch(db=snp)`
- `ESummary` for main rows
- optional later richer extraction only when it clearly improves review/export
- current runtime state:
  - specialized summary row builder exists
  - automatic fallback currently reaches `snp` through `dbvar -> snp` and `pubmed/pmc -> snp` jump definitions are indexed, but explicit runtime expansion is still partial

## Important extractions

- rsID
- chromosome / location summary
- gene context
- variant class
- clinical significance when present
- organism / taxonomy context

## Important jumps

- `snp -> clinvar`
- `snp -> dbvar`
- `snp -> gap`
- `snp -> biosample`
- `snp -> bioproject`

## Table plan

Display:

- `search_term`, `search_type`, `ncbi_rsid`, `gene_identifier`, `label_name`, `ncbi_variant_type`, `ncbi_clinical_significance`, `genome`

Detail/export emphasis:

- chromosomal position
- evidence and phenotype hints
- linked ClinVar/dbVar
- taxonomy ID, chromosome, and position columns are now explicit detail/report fields

## Special prompts

- no default `gene_locus`
- emphasize jump-to-ClinVar/dbVar choices
- if the row came from a linked fallback later, show `ncbi_link_*` provenance rather than pretending it was a native rsID direct hit
