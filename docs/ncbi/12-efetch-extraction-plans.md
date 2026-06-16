# EFetch Extraction Plans

This document defines which NCBI database families should use `EFetch`, and what should be extracted when they do.

## Design rule

- `ESummary` is the default row-construction surface
- `EFetch` is for richer detail, export, and special parsing
- do not call `EFetch` for every row in the first pass

## Strong `EFetch` families

### `protein`

Use `EFetch` for:

- FASTA
- GenPept / GenBank-like flatfile parsing
- replacement-aware rich detail when summaries are too thin

Extract:

- sequence
- header
- product
- `coded_by`
- `db_xref` GeneID
- gene/locus tag

### `nuccore`

Use `EFetch` for:

- GenBank text
- CDS parsing
- `/translation`
- `/protein_id`
- feature/qualifier extraction for detail/export

Extract:

- nucleotide accession
- CDS blocks
- organism
- gene / locus_tag / product
- linked protein IDs
- inline translation when present

### `gene`

Prefer summary first, but support `EFetch` or XML/text detail later for:

- richer description
- genomic context
- accession relationships

### `clinvar`, `gtr`, `bioproject`, `biosample`, `sra`

Official docs note `EFetch` support exists or has existed for these families. Use `EFetch` only after:

- the exact `rettype`/`retmode` combination is verified live
- the extracted fields clearly improve detail/export beyond summary JSON

Current runtime note:

- these families now already expose a more useful summary-first layer through normalized main-table fields plus stable family-specific `ncbi_*` extra columns
- the next `EFetch` step should therefore target fields that summary clearly does not carry well, rather than duplicating summary-accessible identifiers/titles/status text

Practical next extraction priorities:

- `clinvar`
  - richer review/evidence payload
  - variant-type / assertion-detail continuity
  - trait/condition structure more reliable than summary string flattening
- `gtr`
  - test/lab/provider structure
  - method and condition detail beyond current summary strings
- `bioproject`
  - project description / scope / target material
  - linked resource counts when summary is sparse
- `biosample`
  - structured attribute bag flattening
  - reliable attribute-to-column extraction for isolation source / host / geo / strain / tissue
- `sra`
  - study / experiment / run hierarchy
  - spots / bases / layout / instrument fields
  - XML-heavy payload handling strategy

Current implemented state:

- `biosample`
  - summary `sampledata` is now flattened into normalized `ncbi_biosample_attr_*` fields
  - key fields such as isolation source, host, geo location, collection date, strain/cultivar/tissue/stage now prefer parsed attribute-bag values when summary top-level keys are absent
- `sra`
  - summary `expxml` and `runs` are now parsed for first practical hierarchy fields
  - current runtime extracts and normalizes:
    - study accession
    - experiment accession
    - run accession
    - biosample accession
    - layout
    - platform
    - instrument model
    - spots / bases
- `clinvar`
  - runtime now performs targeted `EFetch(db=clinvar, rettype=clinvarset, retmode=xml)` enrichment after summary rows are built
  - current XML enrichment writes back:
    - review status
    - clinical significance
    - condition / trait summary
    - variant type
- `gtr`
  - runtime now performs targeted `EFetch(db=gtr, rettype=gtracc, retmode=xml)` enrichment after summary rows are built
  - current XML enrichment writes back:
    - condition
    - method
    - lab
    - test-name confirmation

## Summary-first families

Treat these as summary-first until a stronger reason appears:

- `assembly`
- `genome`
- `taxonomy`
- `medgen`
- `omim`
- `pubmed`
- `pmc`
- `books`
- `mesh`
- `nlmcatalog`
- `proteinclusters`
- `protfam`
- `cdd`
- `annotinfo`
- `blastdbinfo`
- `orgtrack`

## Known official caution

Bookshelf release notes explicitly state that `EFetch` no longer supports `db=genome`.

Product rule:

- `genome` should stay `ESummary + ELink` driven unless live official docs say otherwise

## Extraction staging rule

When a database is specialized deeply, define three extraction layers:

1. main-table extraction
2. detail-page extraction
3. export-only extraction

This prevents `EFetch` from turning normal review into a latency-heavy full-record loader.
