# `nuccore`

## Role

- official menu name: `Nucleotide`
- primary result domain: `sequence-record`
- current fit: excellent for deep specialization

## Why users choose it

- nucleotide/mRNA/genomic record discovery
- CDS-derived protein recovery
- accession-driven sequence provenance
- source context behind protein/gene records

## Retrieval plan

- `ESearch(db=nuccore)`
- `ESummary` for screening
- `EFetch rettype=gb retmode=text` for CDS, qualifiers, and linked protein IDs
- optional `EFetch rettype=fasta` for nucleotide export if a future mode needs it

## Important extractions

- nucleotide accession.version
- title / definition
- organism / taxid
- length
- molecule type
- CDS blocks
- `/gene`, `/locus_tag`, `/product`
- `/protein_id`
- `/translation` when present
- genomic location summary

## Important jumps

- `nuccore -> gene`
- `nuccore -> protein`
- `nuccore -> taxonomy`
- `nuccore -> assembly`
- `nuccore -> biosample`
- `nuccore -> bioproject`
- `nuccore -> snp`
- `nuccore -> pubmed`

## Table plan

Display:

- `search_term`, `search_type`, `gene_locus`, `label_name`, `labelname_type`, `phgo_alias`, `gene_identifier`, `transcript`, `description`, `genome`, `location`

Detail/export emphasis:

- `ncbi_uid`
- `ncbi_accession`
- `ncbi_nucleotide_accession`
- `ncbi_protein_id`
- `ncbi_gene_id`
- `ncbi_gene_name`
- `ncbi_locus_tag`
- `ncbi_product`
- `ncbi_length`
- `ncbi_fallback_source`
- `ncbi_fasta_header`
- `ncbi_fasta`

## Special prompts

- if a CDS lacks `/translation` but has `/protein_id`, offer or automatically perform linked protein resolution
- do not claim protein-export parity for every nucleotide row
