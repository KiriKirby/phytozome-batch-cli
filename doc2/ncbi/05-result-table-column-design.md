# Result Table Column Design

This document plans result-table columns for future NCBI search types while staying compatible with the current result-table architecture.

## Current implementation scope rule

For the current product phase, only the following NCBI families should continue to receive user-facing search-table work:

- `protein`
- `gene`
- `nuccore`
- `assembly`
- `bioproject`
- `biosample`
- `taxonomy`
- `sra`
- `clinvar`
- `snp`
- `dbvar`
- `medgen`
- `gtr`
- `omim`

The following indexed families should not receive more user-facing search-table work right now:

- literature / knowledge / GEO:
  - `pubmed`, `pmc`, `books`, `mesh`, `nlmcatalog`, `gds`, `geoprofiles`
- support / specialist / future-only:
  - `nucleotide`, `ipg`, `proteinclusters`, `protfam`, `cdd`, `structure`, `annotinfo`, `blastdbinfo`, `gap`, `gapplus`, `grasp`, `orgtrack`, `seqannot`, `pcassay`, `pccompound`, `pcsubstance`, `genome`, `biocollections`

These hidden types can still keep:

- internal row builders
- link traversal targets
- documentation
- future `EFetch` / `ELink` planning

## Shared Design Rules

- keep one shared `ncbi_*` namespace for raw source columns
- keep display columns, detail columns, and export columns as separate explicit lists
- do not force all NCBI search types into one `ncbi` display schema
- only show columns in the main table that help fast screening
- keep richer provenance in detail/export columns

## Family A: Protein

### Current display baseline

Current display columns already fit well:

- `search_term`
- `search_type`
- `gene_locus`
- `label_name`
- `labelname_type`
- `phgo_alias`
- `protein_id`
- `gene_identifier`
- `description`
- `genome`

### Keep in detail/export

- `location`
- `comments`
- `auto_define`
- `gene_report_url`
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
- `ncbi_gene_description`
- `ncbi_other_aliases`
- `ncbi_other_designations`
- `ncbi_gene_locus_aliases`
- `ncbi_fasta_header`

## Family B: Nuccore / Nucleotide

### Display columns

- `search_term`
- `search_type`
- `label_name`
- `transcript`
- `gene_identifier`
- `description`
- `genome`
- `location`

### Detail/export extras

- `ncbi_uid`
- `ncbi_accession`
- `ncbi_nucleotide_accession`
- `ncbi_protein_id`
- `ncbi_gene_id`
- `ncbi_gene_name`
- `ncbi_locus_tag`
- `ncbi_product`
- `ncbi_fallback_source`
- `ncbi_length`
- `ncbi_created`
- `ncbi_updated`
- `ncbi_fasta_header`
- `ncbi_fasta`

Special note:

- if the row is a nucleotide record without a derived protein sequence, do not pretend it has normal protein-export parity
- current implementation now follows that rule:
  - summary-only `nuccore` / `nucleotide` rows use `TranscriptID` plus descriptive metadata
  - they do not auto-fill `SequenceID` / `ProteinID` unless a later dedicated sequence-resolution path is added

## Family C: Gene

### Display columns

- `search_term`
- `search_type`
- `gene_locus`
- `label_name`
- `labelname_type`
- `phgo_alias`
- `gene_identifier`
- `symbols`
- `description`
- `location`
- `genome`

### Detail/export extras

- `ncbi_uid`
- `ncbi_gene_id`
- `ncbi_gene_name`
- `ncbi_other_aliases`
- `ncbi_other_designations`
- `ncbi_gene_description`
- `ncbi_created`
- `ncbi_updated`
- `gene_report_url`

## Family D: Assembly

### Display columns

- `search_term`
- `search_type`
- `label_name`
- `bioproject_accession`
- `project_title`
- `organism`
- `project_type`
- `data_type`
- `status`

### Detail/export extras

- `submitter`
- `submission_date`
- `annotation_release`
- `biosample_accession`
- `bioproject_accession`
- `taxonomy_id`
- `ftp_path`
- `refseq_category`
- `genbank_accession`
- `ncbi_uid`

Rule:

- `gene_locus` should be hidden
- `label_name` is optional user/local annotation, not record-derived biology
- current runtime main table stays closer to the shared keyword-table grammar than the long-term idealized assembly-specific schema

## Family E: BioSample

### Display columns

- `search_term`
- `search_type`
- `label_name`
- `biosample_accession`
- `sample_name`
- `organism`
- `isolation_source`
- `host`
- `geo_loc_name`

### Detail/export extras

- `bioproject_accession`
- `taxonomy_id`
- `collection_date`
- `strain`
- `cultivar`
- `sex`
- `tissue`
- `developmental_stage`
- `attributes_json_or_flattened_fields`
- `ncbi_uid`

Current runtime note:

- the visible main table currently stays compact and uses the shared keyword-table grammar
- BioSample-specific fields are still primarily expressed through detail/export `ncbi_*` columns

## Family F: BioProject

### Display columns

- `search_term`
- `search_type`
- `label_name`
- `bioproject_accession`
- `project_title`
- `organism`
- `project_type`
- `data_type`
- `status`

### Detail/export extras

- `description`
- `submission_date`
- `relevance`
- `scope`
- `target_material`
- linked counts where available:
  - assemblies
  - biosamples
  - sra runs
- `ncbi_uid`

Current runtime note:

- the visible main table currently stays compact and uses the shared keyword-table grammar
- BioProject-specific fields are still primarily expressed through detail/export `ncbi_*` columns

## Family G: Taxonomy

### Display columns

- `search_term`
- `search_type`
- `tax_id`
- `scientific_name`
- `common_name`
- `rank`
- `lineage_summary`

### Detail/export extras

- `synonyms`
- `division`
- `genetic_code`
- `mitochondrial_code`
- `parent_tax_id`
- `full_lineage`
- `ncbi_uid`

Current runtime note:

- the visible main table currently stays compact and uses the shared keyword-table grammar
- richer taxonomy semantics remain in detail/export columns

## Family H: SRA

### Display columns

- `search_term`
- `search_type`
- `sra_accession`
- `title`
- `organism`
- `library_strategy`
- `library_source`
- `platform`
- `bioproject_accession`

### Detail/export extras

- `biosample_accession`
- `instrument_model`
- `spots`
- `bases`
- `layout`
- `study_accession`
- `experiment_accession`
- `run_accession`
- `submission_date`
- `ncbi_uid`

Current runtime note:

- the visible main table currently stays compact and uses the shared keyword-table grammar
- run/experiment/study granularity remains mostly in detail/export columns for now

## Family I: Variant / Clinical

### SNP display columns

- `search_term`
- `search_type`
- `rsid`
- `gene_identifier`
- `label_name`
- `variant_class`
- `clinical_significance`
- `genome`

### ClinVar display columns

- `search_term`
- `search_type`
- `clinvar_accession`
- `variation_name`
- `gene_identifier`
- `clinical_significance`
- `review_status`
- `condition`

### dbVar display columns

- `search_term`
- `search_type`
- `dbvar_accession`
- `variant_type`
- `gene_identifier`
- `phenotype`
- `clinical_assertion`

### MedGen / GTR / OMIM display columns

- `search_term`
- `search_type`
- primary accession
- preferred title
- condition / concept summary
- related gene summary

Rule:

- these should not show `gene_locus` unless a future type-specific use case proves it

Current rule:

- this family stays exposed in the search selector
- continue improving table/detail/export design here
- do not divert current product effort into literature/GEO table polishing until this family is substantially deeper

## Family J: Literature And Reference

### PubMed / PMC display columns

- `search_term`
- `search_type`
- `pmid_or_pmcid`
- `title`
- `journal`
- `pub_date`
- `authors_short`

### Books display columns

- `search_term`
- `search_type`
- `book_id`
- `title`
- `source`
- `last_update`

### MeSH display columns

- `search_term`
- `search_type`
- `mesh_id`
- `heading`
- `scope_note_short`

### GEO DataSets / GEO Profiles display columns

- `search_term`
- `search_type`
- `accession`
- `title`
- `organism`
- `platform_or_profile_class`

Rule:

- no `gene_locus`
- no FASTA columns
- details should emphasize abstracts, notes, identifiers, and URLs

## Canvas Eligibility Rule

Only rows with a real sequence payload or a defined supported sequence-resolution path should be Canvas-add eligible.

Likely eligible:

- `protein`
- selected `nuccore`
- selected `gene` via linked sequence resolution
- maybe `ipg`

Not generally eligible:

- `assembly`
- `biosample`
- `bioproject`
- `taxonomy`
- `sra`
- `pubmed`
- `pmc`
- `books`
- `mesh`
- `clinvar`
- `dbvar`
- `gtr`
- `medgen`
- `omim`

Do not let `database == ncbi` imply Canvas eligibility.

## First-Wave Implemented Reality

The current code now partially realizes the above design:

- `gene` has its own prompt/table schema bucket
- `nuccore` / `nucleotide` have their own prompt/table schema bucket
- `assembly`, `bioproject`, `biosample`, `taxonomy`, and `sra` now have dedicated row normalizers plus dedicated prompt/table schema buckets
- `clinvar`, `snp`, `dbvar`, `medgen`, `gtr`, and `omim` also now have dedicated row normalizers plus dedicated prompt/table schema buckets
- all 11 visible non-sequence table families now populate runtime `label_name` and `labelname_type` from record-native summary fields instead of leaving those rows mostly blank
- all 11 visible non-sequence table families now write a first practical layer of stable `ncbi_*` detail/export fields into `ExtraColumns`
- `biosample`, `sra`, `clinvar`, and `gtr` have now moved one step beyond plain summary-first shaping:
  - `biosample` parses summary attribute bags into normalized fields
  - `sra` parses summary XML fragments for study / experiment / run hierarchy
  - `clinvar` performs targeted `EFetch` XML enrichment for review/significance/condition/variant-type continuity
  - `gtr` performs targeted `EFetch` XML enrichment for condition/method/lab continuity
- the specialized prompt/table schemas for `biosample`, `sra`, `clinvar`, and `gtr` now expose selected `ncbi_*` columns directly in the visible main table instead of leaving them only in the extra-columns detail area
- the detail/report schemas are now also wider for the remaining visible NCBI table families, so runtime-extracted fields do not stay trapped only in raw extra metadata
  - examples:
  - `assembly`: `ncbi_assembly_accession`, `ncbi_assembly_name`, `ncbi_assembly_level`, `ncbi_assembly_status`, `ncbi_ftp_path`
  - `bioproject`: `ncbi_bioproject_accession`, `ncbi_project_title`, `ncbi_project_type`, `ncbi_project_data_type`
  - `biosample`: `ncbi_biosample_accession`, `ncbi_sample_name`, `ncbi_isolation_source`, `ncbi_host`, `ncbi_geo_loc_name`
  - `taxonomy`: `ncbi_taxonomy_id`, `ncbi_scientific_name`, `ncbi_common_name`, `ncbi_rank`, `ncbi_lineage_summary`
  - `sra`: `ncbi_sra_accession`, `ncbi_library_strategy`, `ncbi_library_source`, `ncbi_platform`, `ncbi_bioproject_accession`
  - `clinvar`: `ncbi_clinvar_accession`, `ncbi_clinical_significance`, `ncbi_review_status`, `ncbi_condition`
  - `snp`: `ncbi_rsid`, `ncbi_variant_type`, `ncbi_clinical_significance`, plus detail-time `ncbi_chromosome` / `ncbi_chrpos`
  - `dbvar`: `ncbi_dbvar_accession`, `ncbi_variant_type`, `ncbi_phenotype`, `ncbi_clinical_assertion`
  - `medgen`: `ncbi_medgen_id`, `ncbi_preferred_title`, `ncbi_condition_summary`, plus detail-time `ncbi_definition` / `ncbi_source`
  - `gtr`: `ncbi_gtr_accession`, `ncbi_test_name`, `ncbi_condition`, `ncbi_lab`
  - `omim`: `ncbi_omim_id`, `ncbi_omim_title`, `ncbi_condition_summary`, plus detail-time `ncbi_omim_text`
- replacement/update metadata is now part of the formal visible/detail/report schema instead of only internal row metadata:
  - `ncbi_replaced_by`
  - `ncbi_requested_accession`
  - `ncbi_replacement_accession`
  - `ncbi_replacement_decision`
- these buckets still rely on summary-first fields today, but they now stop inheriting old protein-shaped assumptions and already provide a usable detail/export metadata base for later specialized columns
