# Live EInfo Baseline (2026-06-13)

This document records the live `EInfo` baseline that should drive current NCBI search-type indexing.

Source method:

- database inventory from `https://eutils.ncbi.nlm.nih.gov/entrez/eutils/einfo.fcgi?retmode=json`
- per-database field/link metadata from `https://eutils.ncbi.nlm.nih.gov/entrez/eutils/einfo.fcgi?retmode=json&version=2.0&db=<token>`

Authority rule:

- use plain `retmode=json` for the live top-level `dblist`
- use `version=2.0` only for per-database `DbInfo` detail
- do not expect top-level `dblist` when `version=2.0` is present

Observed live behavior on 2026-06-13:

- plain `retmode=json` returns the 39-token `dblist`
- `version=2.0` returns richer `fieldlist` and `linklist`
- top-level `version=2.0` without `db=` returns empty `dbinfo`, not the database inventory

## Live inventory summary

| DB | Menu name | Count | Last update | Fields | Links |
| --- | --- | ---: | --- | ---: | ---: |
| `annotinfo` | AnnotInfo | 2,597 | 2026-06-12 15:02 | 14 | 0 |
| `assembly` | Assembly | 3,672,115 | 2026-06-12 17:04 | 51 | 10 |
| `biocollections` | Biocollections | 8,497 | 2024-05-15 09:56 | 16 | 3 |
| `bioproject` | BioProject | 1,056,341 | 2026-06-13 07:16 | 31 | 26 |
| `biosample` | BioSample | 57,248,480 | 2026-06-12 23:01 | 14 | 12 |
| `blastdbinfo` | BlastdbInfo | 3,693,354 | 2026-06-12 12:42 | 15 | 0 |
| `books` | Books | 1,321,962 | 2026-06-12 03:34 | 33 | 9 |
| `cdd` | Conserved Domains | 67,160 | 2025-07-02 19:21 | 16 | 13 |
| `clinvar` | ClinVar | 4,527,065 | 2026-06-07 19:23 | 47 | 11 |
| `dbvar` | dbVar | 8,669,169 | 2025-08-27 14:12 | 43 | 11 |
| `gap` | dbGaP | 363,717 | 2024-09-20 09:36 | 43 | 12 |
| `gapplus` | GaPPlus | 136,796 | 2017-09-29 04:56 | 16 | 0 |
| `gds` | GEO DataSets | 8,867,250 | 2026-06-12 19:27 | 31 | 10 |
| `gene` | Gene | 96,840,361 | 2026-06-13 08:40 | 36 | 33 |
| `genome` | Genome | 88,333 | 2025-07-08 11:32 | 30 | 9 |
| `geoprofiles` | GEO Profiles | 128,414,055 | 2024-02-20 03:42 | 26 | 9 |
| `grasp` | grasp | 7,862,970 | 2015-01-26 16:10 | 20 | 0 |
| `gtr` | GTR | 64,348 | 2026-06-13 01:14 | 65 | 4 |
| `ipg` | Identical Protein Groups | 1,127,939,485 | 2026-06-11 16:26 | 20 | 2 |
| `medgen` | MedGen | 236,473 | 2026-06-12 15:44 | 24 | 15 |
| `mesh` | MeSH | 355,789 | 2026-06-13 03:24 | 14 | 3 |
| `nlmcatalog` | NLM Catalog | 1,651,147 | 2026-06-13 06:42 | 42 | 1 |
| `nuccore` | Nucleotide | 730,271,728 | 2026-06-12 14:50 | 34 | 37 |
| `nucleotide` | Nucleotide | 730,271,728 | 2026-06-12 14:50 | 34 | 0 |
| `omim` | OMIM | 29,596 | 2026-06-13 03:18 | 22 | 18 |
| `orgtrack` | Orgtrack | 9,307 | 2026-06-13 01:04 | 37 | 3 |
| `pcassay` | PubChem BioAssay | 1,770,191 | 2026-06-12 08:50 | 41 | 33 |
| `pccompound` | PubChem Compound | 123,914,989 | 2026-06-12 17:10 | 41 | 28 |
| `pcsubstance` | PubChem Substance | 347,389,819 | 2026-06-13 10:49 | 21 | 19 |
| `pmc` | PMC | 12,279,255 | 2026-06-13 08:24 | 45 | 18 |
| `protein` | Protein | 1,613,939,775 | 2026-06-12 17:10 | 33 | 39 |
| `proteinclusters` | Protein Clusters | 1,137,329 | 2017-12-04 13:20 | 32 | 11 |
| `protfam` | Protein Family Models | 183,999 | 2026-06-12 23:42 | 24 | 5 |
| `pubmed` | PubMed | 40,704,785 | 2026-06-13 07:37 | 48 | 48 |
| `seqannot` | SeqAnnot | 520,071 | 2026-06-13 09:48 | 14 | 2 |
| `snp` | SNP | 1,197,210,835 | 2025-03-11 08:22 | 29 | 14 |
| `sra` | SRA | 44,896,144 | 2026-06-13 08:57 | 22 | 9 |
| `structure` | Structure | 254,922 | 2026-06-10 18:28 | 53 | 16 |
| `taxonomy` | Taxonomy | 2,977,575 | 2026-06-13 08:57 | 21 | 20 |

## Design implications

- `protein`, `nuccore`, `gene`, `pubmed`, and `pcassay` have large link graphs and should never be implemented as one-row-one-request detail flows.
- `gapplus`, `grasp`, `proteinclusters`, and `biocollections` show comparatively old `lastupdate` timestamps; product docs should avoid overpromising freshness-sensitive semantics without rechecking.
- `nucleotide` shares live counts with `nuccore` but exposes zero links in `EInfo`; this strengthens the design rule that `nuccore` should be the primary nucleotide-facing mode.
- `annotinfo` and `blastdbinfo` have no link graph in live `EInfo`; they belong in specialist/reference documentation first, not in the first user-facing rollout.
