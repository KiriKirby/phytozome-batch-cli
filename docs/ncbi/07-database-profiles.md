# Entrez Database Profiles

This document gives a per-database integration profile for every current Entrez database token returned by live `EInfo`.

Profile fields:

- `Official label`
- `Primary result domain`
- `Current-framework fit`
- `Important NCBI-specific concerns`
- `Expected linked navigation`
- `Suggested main table columns`

Result-domain vocabulary used here:

- `sequence-record`
- `sequence-group`
- `genome-resource`
- `sample-project`
- `variant-clinical`
- `literature-reference`
- `chemical-bioassay`
- `specialist-reference`
- `system-index`

## Sequence And Gene-Centric Databases

### `protein`

- Official label: `Protein`
- Primary result domain: `sequence-record`
- Current-framework fit: first-class; already implemented
- Important NCBI-specific concerns:
  - supports accession- and UID-oriented retrieval
  - `replacedby` handling matters
  - sequence rows may require linked `gene` and `nuccore` fetches
  - protein records can link to CDD, Gene, Taxonomy, OMIM, Structure, BioProject, PubMed, and assays
- Expected linked navigation:
  - `gene`, `nuccore`, `taxonomy`, `cdd`, `structure`, `bioproject`, `pubmed`, `proteinclusters`, `protfam`, `omim`
- Suggested main table columns:
  - `search_term`, `search_type`, `gene_locus`, `label_name`, `labelname_type`, `phgo_alias`, `protein_id`, `gene_identifier`, `description`, `genome`

### `nuccore`

- Official label: `Nucleotide`
- Primary result domain: `sequence-record`
- Current-framework fit: excellent
- Important NCBI-specific concerns:
  - can contain CDS, mRNA, genomic, TSA, WGS, organelle, and many other sequence classes
  - may need `seq_start`, `seq_stop`, `strand`, and `complexity` for future partial fetches
  - not every row should imply protein export parity
- Expected linked navigation:
  - `gene`, `protein`, `taxonomy`, `assembly`, `biosample`, `bioproject`, `snp`, `pubmed`
- Suggested main table columns:
  - `search_term`, `search_type`, `gene_locus`, `label_name`, `labelname_type`, `phgo_alias`, `gene_identifier`, `transcript`, `description`, `genome`, `location`

### `nucleotide`

- Official label: `Nucleotide`
- Primary result domain: `sequence-record`
- Current-framework fit: medium
- Important NCBI-specific concerns:
  - overlaps conceptually with `nuccore`
  - should not be exposed without a very explicit reason for why a user should choose it over `nuccore`
  - documentation and UI must explain the distinction before implementation
- Expected linked navigation:
  - similar to `nuccore`, but treat it as a compatibility/specialist entry until proven necessary
- Suggested main table columns:
  - same baseline as `nuccore` if exposed at all

### `gene`

- Official label: `Gene`
- Primary result domain: `sequence-record`
- Current-framework fit: excellent
- Important NCBI-specific concerns:
  - `label_name` / `gene_locus` logic maps naturally here
  - many valuable linked outputs come through `ELink`, especially `clinvar`, `dbvar`, `medgen`, `gtr`, `omim`, `nuccore`, `geoprofiles`, and `cdd`
  - sequence resolution is often indirect rather than inline
- Expected linked navigation:
  - `nuccore`, `protein`, `clinvar`, `dbvar`, `medgen`, `gtr`, `omim`, `cdd`, `genome`, `geoprofiles`, `books`
- Suggested main table columns:
  - `search_term`, `search_type`, `gene_locus`, `label_name`, `labelname_type`, `phgo_alias`, `gene_identifier`, `description`, `location`, `genome`

### `ipg`

- Official label: `Identical Protein Groups`
- Primary result domain: `sequence-group`
- Current-framework fit: good
- Important NCBI-specific concerns:
  - records represent grouped identical proteins rather than one ordinary protein record
  - detail view should emphasize group membership and reference/source composition
  - sequence export may still be meaningful, but group semantics must stay visible
- Expected linked navigation:
  - `protein`, `gene`, `nuccore`, `taxonomy`
- Suggested main table columns:
  - `search_term`, `search_type`, `group_id`, `label_name`, `protein_id`, `gene_identifier`, `description`, `genome`, `member_count`

### `proteinclusters`

- Official label: `Protein Clusters`
- Primary result domain: `sequence-group`
- Current-framework fit: good
- Important NCBI-specific concerns:
  - cluster identity and member count matter more than single-sequence fields
  - later exports should preserve cluster identifiers and representative/member views
- Expected linked navigation:
  - `protein`, `gene`, `taxonomy`, `pubmed`
- Suggested main table columns:
  - `search_term`, `search_type`, `cluster_id`, `label_name`, `description`, `genome`, `member_count`

### `protfam`

- Official label: `Protein Family Models`
- Primary result domain: `sequence-group`
- Current-framework fit: good
- Important NCBI-specific concerns:
  - represents family/model concepts, not individual submitted proteins
  - row actions should emphasize related proteins and model scope
- Expected linked navigation:
  - `protein`, `proteinclusters`, `cdd`, `pubmed`
- Suggested main table columns:
  - `search_term`, `search_type`, `family_model_id`, `label_name`, `description`, `model_type`, `member_count`

### `cdd`

- Official label: `Conserved Domains`
- Primary result domain: `sequence-group`
- Current-framework fit: good
- Important NCBI-specific concerns:
  - rows should foreground domain/model identity, not organism
  - highly link-centric to proteins and genes
- Expected linked navigation:
  - `protein`, `gene`, `structure`, `pubmed`
- Suggested main table columns:
  - `search_term`, `search_type`, `cdd_id`, `label_name`, `description`, `source`, `domain_class`

## Genome / Sample / Project / Taxonomy Databases

### `assembly`

- Official label: `Assembly`
- Primary result domain: `genome-resource`
- Current-framework fit: excellent
- Important NCBI-specific concerns:
  - assembly accessions, assembly levels, RefSeq/GenBank category, and FTP locations matter
  - rows often need links to BioProject, BioSample, Taxonomy, and Genome
- Expected linked navigation:
  - `bioproject`, `biosample`, `taxonomy`, `genome`, `nuccore`, `gene`
- Suggested main table columns:
  - `search_term`, `search_type`, `label_name`, `assembly_accession`, `assembly_name`, `organism`, `assembly_level`, `status`

### `genome`

- Official label: `Genome`
- Primary result domain: `genome-resource`
- Current-framework fit: medium
- Important NCBI-specific concerns:
  - official docs note this database was redesigned; records correspond to species rather than an individual chromosome
  - official Bookshelf notes that `EFetch` no longer supports `db=genome`
  - summary- and link-oriented support matters more than full-record fetch
- Expected linked navigation:
  - `assembly`, `gene`, `protein`, `nuccore`, `taxonomy`
- Suggested main table columns:
  - `search_term`, `search_type`, `genome_id`, `organism`, `label_name`, `description`, `status`

### `bioproject`

- Official label: `BioProject`
- Primary result domain: `sample-project`
- Current-framework fit: excellent
- Important NCBI-specific concerns:
  - project title, project type, scope, and linked resource counts matter
  - often the right root record for sample/assembly/SRA traversal
- Expected linked navigation:
  - `biosample`, `assembly`, `sra`, `gene`, `protein`, `pubmed`
- Suggested main table columns:
  - `search_term`, `search_type`, `label_name`, `bioproject_accession`, `project_title`, `organism`, `project_type`, `status`

### `biosample`

- Official label: `BioSample`
- Primary result domain: `sample-project`
- Current-framework fit: excellent
- Important NCBI-specific concerns:
  - attribute bags are extensive and irregular
  - table should stay compact; flatten only the most screening-relevant attributes
  - full attribute detail belongs in row details/export
- Expected linked navigation:
  - `bioproject`, `assembly`, `sra`, `taxonomy`
- Suggested main table columns:
  - `search_term`, `search_type`, `label_name`, `biosample_accession`, `sample_name`, `organism`, `isolation_source`, `host`, `geo_loc_name`

### `sra`

- Official label: `SRA`
- Primary result domain: `sample-project`
- Current-framework fit: excellent
- Important NCBI-specific concerns:
  - run/experiment/study/sample/project accessions all matter
  - layout, platform, library strategy, and spot/base counts are screening-critical
  - generally metadata-first, not sequence-row-first
- Expected linked navigation:
  - `biosample`, `bioproject`, `taxonomy`, sometimes `assembly` or publication records
- Suggested main table columns:
  - `search_term`, `search_type`, `sra_accession`, `title`, `organism`, `library_strategy`, `library_source`, `platform`, `bioproject_accession`

### `taxonomy`

- Official label: `Taxonomy`
- Primary result domain: `genome-resource`
- Current-framework fit: excellent
- Important NCBI-specific concerns:
  - hierarchy and lineage are primary
  - many later NCBI search types may want taxonomy as a prefilter or linked drill-down
- Expected linked navigation:
  - `gene`, `protein`, `nuccore`, `assembly`, `biosample`, `bioproject`, `sra`
- Suggested main table columns:
  - `search_term`, `search_type`, `tax_id`, `scientific_name`, `common_name`, `rank`, `lineage_summary`

### `biocollections`

- Official label: `BioCollections`
- Primary result domain: `sample-project`
- Current-framework fit: good
- Important NCBI-specific concerns:
  - collection/institution metadata matters more than biological record structure
  - useful as provenance support rather than direct sequence discovery
- Expected linked navigation:
  - `biosample`, `taxonomy`
- Suggested main table columns:
  - `search_term`, `search_type`, `collection_code`, `institution`, `description`, `country`, `status`

## Variant / Clinical / Medical Databases

### `snp`

- Official label: `SNP`
- Primary result domain: `variant-clinical`
- Current-framework fit: good
- Important NCBI-specific concerns:
  - rsID-centered rows
  - gene, clinical significance, and taxonomy context may exist but not uniformly
  - should not inherit protein-style `gene_locus` assumptions by default
- Expected linked navigation:
  - `gene`, `clinvar`, `dbvar`, `pubmed`, `taxonomy`
- Suggested main table columns:
  - `search_term`, `search_type`, `rsid`, `gene_identifier`, `label_name`, `variant_class`, `clinical_significance`, `genome`

### `clinvar`

- Official label: `ClinVar`
- Primary result domain: `variant-clinical`
- Current-framework fit: good
- Important NCBI-specific concerns:
  - review status and clinical significance are screening-critical
  - conditions and related genes should be visible without opening every row
  - official Bookshelf notes EFetch support exists
- Expected linked navigation:
  - `gene`, `medgen`, `snp`, `dbvar`, `pubmed`
- Suggested main table columns:
  - `search_term`, `search_type`, `clinvar_accession`, `variation_name`, `gene_identifier`, `clinical_significance`, `review_status`, `condition`

### `dbvar`

- Official label: `dbVar`
- Primary result domain: `variant-clinical`
- Current-framework fit: good
- Important NCBI-specific concerns:
  - structural-variant framing matters
  - phenotype/evidence fields should be detail/export oriented when too wide for table screening
- Expected linked navigation:
  - `gene`, `clinvar`, `pubmed`, `taxonomy`
- Suggested main table columns:
  - `search_term`, `search_type`, `dbvar_accession`, `variant_type`, `gene_identifier`, `phenotype`, `clinical_assertion`

### `medgen`

- Official label: `MedGen`
- Primary result domain: `variant-clinical`
- Current-framework fit: good
- Important NCBI-specific concerns:
  - concept/condition-centric rather than sequence-centric
  - best used for linked explanation around genes and variants
- Expected linked navigation:
  - `gene`, `clinvar`, `gtr`, `omim`, `pubmed`
- Suggested main table columns:
  - `search_term`, `search_type`, `medgen_id`, `preferred_title`, `condition_summary`, `related_gene_summary`

### `gtr`

- Official label: `GTR`
- Primary result domain: `variant-clinical`
- Current-framework fit: good
- Important NCBI-specific concerns:
  - test, lab, and condition relationships matter
  - official Bookshelf notes EFetch support exists
- Expected linked navigation:
  - `gene`, `medgen`, `clinvar`, `omim`
- Suggested main table columns:
  - `search_term`, `search_type`, `gtr_accession`, `test_name`, `condition`, `gene_identifier`, `lab`

### `omim`

- Official label: `OMIM`
- Primary result domain: `variant-clinical`
- Current-framework fit: good
- Important NCBI-specific concerns:
  - disease/gene knowledge object, not direct sequence record
  - linked navigation is central
- Expected linked navigation:
  - `gene`, `protein`, `medgen`, `gtr`, `pubmed`
- Suggested main table columns:
  - `search_term`, `search_type`, `omim_id`, `title`, `condition_summary`, `related_gene_summary`

## Literature / Reference / Knowledge Databases

### `pubmed`

- Official label: `PubMed`
- Primary result domain: `literature-reference`
- Current-framework fit: medium
- Important NCBI-specific concerns:
  - web PubMed has behaviors not reproduced exactly by plain ESearch
  - 10,000-result ESearch ceiling is a real design constraint
  - `ECitMatch` and `ESpell` are especially relevant here
- Expected linked navigation:
  - `pmc`, `gene`, `protein`, `clinvar`, `mesh`
- Suggested main table columns:
  - `search_term`, `search_type`, `pmid`, `title`, `journal`, `pub_date`, `authors_short`

### `pmc`

- Official label: `PMC`
- Primary result domain: `literature-reference`
- Current-framework fit: medium
- Important NCBI-specific concerns:
  - article/full-text centric
  - citation and availability relationships matter more than biological export
- Expected linked navigation:
  - `pubmed`, sometimes gene/protein links through article associations
- Suggested main table columns:
  - `search_term`, `search_type`, `pmcid`, `title`, `journal`, `pub_date`, `authors_short`

### `books`

- Official label: `Books`
- Primary result domain: `literature-reference`
- Current-framework fit: medium
- Important NCBI-specific concerns:
  - chapter/book/knowledge object rather than dataset record
  - should emphasize title/source/update metadata
- Expected linked navigation:
  - `pubmed`, internal NCBI knowledge references
- Suggested main table columns:
  - `search_term`, `search_type`, `book_id`, `title`, `source`, `last_update`

### `mesh`

- Official label: `MeSH`
- Primary result domain: `literature-reference`
- Current-framework fit: medium
- Important NCBI-specific concerns:
  - concept/thesaurus object
  - should support later structured-query building rather than FASTA-style export expectations
- Expected linked navigation:
  - `pubmed`
- Suggested main table columns:
  - `search_term`, `search_type`, `mesh_id`, `heading`, `scope_note_short`

### `nlmcatalog`

- Official label: `NLM Catalog`
- Primary result domain: `literature-reference`
- Current-framework fit: medium
- Important NCBI-specific concerns:
  - journal/catalog record, not research-data row
  - likely most useful as metadata export and citation support
- Expected linked navigation:
  - `pubmed`
- Suggested main table columns:
  - `search_term`, `search_type`, `catalog_id`, `title`, `publisher`, `issn`, `subject_summary`

### `gds`

- Official label: `GEO DataSets`
- Primary result domain: `literature-reference`
- Current-framework fit: medium
- Important NCBI-specific concerns:
  - dataset-level GEO object
  - should stay distinct from `geoprofiles`
- Expected linked navigation:
  - `geoprofiles`, `pubmed`, sometimes gene references
- Suggested main table columns:
  - `search_term`, `search_type`, `gds_accession`, `title`, `organism`, `platform_or_dataset_type`

### `geoprofiles`

- Official label: `GEO Profiles`
- Primary result domain: `literature-reference`
- Current-framework fit: medium
- Important NCBI-specific concerns:
  - profile-level GEO object rather than dataset-level object
  - detail views will matter more than compact table semantics
- Expected linked navigation:
  - `gds`, `gene`, `pubmed`
- Suggested main table columns:
  - `search_term`, `search_type`, `geo_profile_id`, `title`, `gene_identifier`, `organism`

## Structure / Chemical / Specialist Databases

### `structure`

- Official label: `Structure`
- Primary result domain: `specialist-reference`
- Current-framework fit: low-to-medium
- Important NCBI-specific concerns:
  - structure identity and experiment matter more than keyword alias logic
  - direct tree/Canvas/FASTA assumptions are weak
- Expected linked navigation:
  - `protein`, `cdd`, `pubmed`
- Suggested main table columns:
  - `search_term`, `search_type`, `structure_id`, `title`, `molecule_summary`, `method`, `organism`

### `pcassay`

- Official label: `PubChem BioAssay`
- Primary result domain: `chemical-bioassay`
- Current-framework fit: low-to-medium
- Important NCBI-specific concerns:
  - assay target/result concepts are primary
  - highly link-driven from genes/proteins/chemicals
- Expected linked navigation:
  - `protein`, `gene`, `pccompound`, `pcsubstance`, `pubmed`
- Suggested main table columns:
  - `search_term`, `search_type`, `assay_id`, `title`, `target_summary`, `assay_type`, `status`

### `pccompound`

- Official label: `PubChem Compound`
- Primary result domain: `chemical-bioassay`
- Current-framework fit: low-to-medium
- Important NCBI-specific concerns:
  - chemical-centric; should not inherit biology-first column assumptions
- Expected linked navigation:
  - `pcassay`, `pcsubstance`, `protein`, `gene`
- Suggested main table columns:
  - `search_term`, `search_type`, `cid`, `compound_name`, `formula`, `mass`

### `pcsubstance`

- Official label: `PubChem Substance`
- Primary result domain: `chemical-bioassay`
- Current-framework fit: low-to-medium
- Important NCBI-specific concerns:
  - substance/submission context matters
- Expected linked navigation:
  - `pccompound`, `pcassay`
- Suggested main table columns:
  - `search_term`, `search_type`, `sid`, `substance_name`, `source`, `status`

## System / Specialist / Support Databases

### `annotinfo`

- Official label: `AnnotInfo`
- Primary result domain: `system-index`
- Current-framework fit: low
- Important NCBI-specific concerns:
  - more system/reference-like than end-user biological search
  - should not be front-line exposed until use cases are explicit
- Expected linked navigation:
  - likely sequence/genome annotation references
- Suggested main table columns:
  - `search_term`, `search_type`, `annotation_id`, `title`, `description`

### `blastdbinfo`

- Official label: `BLAST DB Information`
- Primary result domain: `system-index`
- Current-framework fit: low
- Important NCBI-specific concerns:
  - index/admin-like metadata
  - especially easy to confuse with normal BLAST workflows
  - should not be presented as a biological record search unless there is a strong feature need
- Expected linked navigation:
  - limited; mainly informational
- Suggested main table columns:
  - `search_term`, `search_type`, `db_name`, `description`, `content_type`, `updated_at`

### `gap`

- Official label: `dbGaP`
- Primary result domain: `specialist-reference`
- Current-framework fit: low
- Important NCBI-specific concerns:
  - access and policy constraints matter
  - should surface availability/access-status notes clearly
- Expected linked navigation:
  - `pubmed`, `gtr`, possibly project/sample relationships
- Suggested main table columns:
  - `search_term`, `search_type`, `gap_accession`, `title`, `study_type`, `access_status`

### `gapplus`

- Official label: `dbGaP Plus`
- Primary result domain: `specialist-reference`
- Current-framework fit: low
- Important NCBI-specific concerns:
  - treat separately from `gap`
  - must not silently merge these tokens in implementation or documentation
- Expected linked navigation:
  - similar family to `gap`
- Suggested main table columns:
  - `search_term`, `search_type`, `gapplus_accession`, `title`, `study_type`, `access_status`

### `grasp`

- Official label: `GRASP`
- Primary result domain: `specialist-reference`
- Current-framework fit: low
- Important NCBI-specific concerns:
  - specialist resource; likely better as linked drill-down first
- Expected linked navigation:
  - variant/gene/phenotype-related references
- Suggested main table columns:
  - `search_term`, `search_type`, `grasp_id`, `title`, `trait_summary`

### `orgtrack`

- Official label: `OrgTrack`
- Primary result domain: `system-index`
- Current-framework fit: low
- Important NCBI-specific concerns:
  - administrative/monitoring flavor
  - not a normal biological result-table fit
- Expected linked navigation:
  - minimal or specialist-only
- Suggested main table columns:
  - `search_term`, `search_type`, `record_id`, `title`, `status`

### `seqannot`

- Official label: `SeqAnnot`
- Primary result domain: `specialist-reference`
- Current-framework fit: low
- Important NCBI-specific concerns:
  - annotation-centric and likely best consumed through linked sequence context
- Expected linked navigation:
  - `nuccore`, `gene`, `protein`
- Suggested main table columns:
  - `search_term`, `search_type`, `annotation_accession`, `title`, `description`, `organism`

## Remaining Notes

### `pubmed`, `pmc`, and large-result paging

- official docs note practical search retrieval limits
- `PubMed` and `PMC` need explicit design for result slicing and user explanation when queries exceed simple page retrieval

### `genome` and `EFetch`

- official Bookshelf release notes state that `EFetch` no longer supports retrievals from `db=genome`
- `genome` should therefore be planned as a summary/link-centric search type rather than a full-record fetch type

### `clinvar`, `gtr`, `bioproject`, `biosample`, `sra`

- official Bookshelf release notes explicitly mention EFetch support additions over time
- nevertheless, implementation should still verify exact current `rettype` / `retmode` combinations before shipping each type
