# `protein`

## Role

- official menu name: `Protein`
- primary result domain: `sequence-record`
- current fit: first-class and already partially specialized in code

## Why users choose it

- direct accession lookup
- protein-centric keyword search
- best entry point for peptide FASTA export
- strongest fit for current Canvas/sequence downstream actions

## Retrieval plan

- `ESearch(db=protein)`
- `ESummary` for screening rows
- `EFetch rettype=fasta`
- `EFetch rettype=gb` or GenPept-style flatfile parsing
- optional linked `gene` summary enrichment
- fallback to `nuccore` only when protein search yields no usable FASTA

## Important extractions

- accession.version
- UID
- title / definition
- organism and taxid
- length
- status / `replacedby`
- product
- `coded_by`
- GeneID, gene symbol, locus tag
- clean FASTA header and sequence

## Important jumps

- `protein -> gene`
- `protein -> nuccore`
- `protein -> cdd`
- `protein -> structure`
- `protein -> taxonomy`
- `protein -> bioproject`
- `protein -> proteinclusters`
- `protein -> protfam`
- `protein -> pubmed`

## Table plan

Display:

- `search_term`, `search_type`, `gene_locus`, `label_name`, `labelname_type`, `phgo_alias`, `protein_id`, `gene_identifier`, `description`, `genome`

Detail/export emphasis:

- `location`
- `ncbi_uid`
- `ncbi_accession`
- `ncbi_status`
- `ncbi_replaced_by`
- `ncbi_created`
- `ncbi_updated`
- `ncbi_length`
- `ncbi_gene_id`
- `ncbi_gene_name`
- `ncbi_locus_tag`
- `ncbi_product`
- `ncbi_coded_by`
- `ncbi_fasta_header`
- `ncbi_fasta`

## Special prompts

- replacement prompt when `replacedby` is populated
- linked jump prompt when a user wants Gene, CDD, or source nucleotide context
- never silently overwrite the originally requested accession
