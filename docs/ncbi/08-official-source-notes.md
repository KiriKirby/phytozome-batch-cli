# Official Source Notes

This document records which official NCBI sources should be trusted for which questions.

## Authority Split

### Use live `EInfo` as the authority for:

- current Entrez database token inventory
- current database menu names
- current database descriptions
- current indexed field inventory
- current linkname inventory
- current last-update timestamps per database

Reason:

- endpoint output is live
- it reflects current production indexing better than older prose summaries

### Use NCBI Bookshelf E-utilities chapters as the authority for:

- request semantics
- parameter meanings
- rate-limit policy
- History server behavior
- `retstart` / `retmax`
- GET versus POST guidance
- `idtype`, `sort`, `version=2.0`, `retmode`, `rettype`
- release-history caveats such as special handling for `genome`

Reason:

- these chapters define the supported API contract more explicitly than ad hoc page inspection

## Current Official-Date Notes

Observed official Bookshelf update markers:

- Quick Start chapter: October 24, 2018
- General Introduction chapter: November 17, 2022
- In-Depth parameters chapter: March 4, 2026

Practical rule:

- if the In-Depth chapter and an older chapter differ, prefer the newer In-Depth chapter for protocol details

## Important `EInfo` Endpoint-Shape Note

As of 2026-06-13:

- `einfo.fcgi?retmode=json` returns the live `dblist`
- `einfo.fcgi?retmode=json&version=2.0&db=<token>` returns richer `fieldlist` and `linklist`
- `einfo.fcgi?retmode=json&version=2.0` at the top level does not return the `dblist`; it returns empty `dbinfo`

Conclusion:

- use the plain JSON endpoint for current database inventory
- use `version=2.0` only for per-database capability discovery
- do not assume one top-level call can provide both current inventory and richer field/link metadata

## Current Inventory Discrepancy

As of 2026-06-13:

- the Bookshelf overview still contains prose stating that Entrez currently includes 38 databases
- live `EInfo` returns 39 database tokens

Conclusion:

- documentation and implementation should treat the live `EInfo` list as the current searchable type inventory
- the stale prose count in Bookshelf should be documented, not followed blindly

## Important Official Warnings To Preserve In Product Design

### Not all web-interface behaviors are plain `ESearch`

Official docs note that some web search features are not reproduced directly by `ESearch`.

Implication:

- do not promise that every NCBI website search behavior has an exact one-call E-utilities equivalent
- where relevant, route users to `ESpell`, `ECitMatch`, or a linked workflow instead

### Not all databases have equal `EFetch` parity

Official docs explicitly state that `EFetch` does not support all Entrez databases.

Implication:

- do not assume every search type can end in full-record fetch
- design some types as `ESearch + ESummary + linked navigation` first

### Sequence accession behavior differs from UID indexing

Official docs note that some newer sequence records are retrievable by accession in `EFetch` even when they are not available to `ESearch` or `ESummary` in the same way.

Implication:

- keep both accession-oriented and UID-oriented provenance fields
- do not collapse "requested id" and "indexed UID" into one field

### Link behavior should prefer explicit `linkname`

Official docs recommend explicit `linkname` use for efficient `ELink`.

Implication:

- future NCBI navigation actions should record exact `linkname` values where possible
- avoid open-ended "all links" retrieval in normal user flows unless the UI explicitly needs a link browser
