# E-utilities Mechanics And NCBI-Specific Risks

This document captures official NCBI mechanics that must shape implementation, not just error handling.

## Official Usage Rules

Source basis: NCBI Entrez Programming Utilities Help.

Current official-document dates that matter:

- E-utilities Quick Start: last updated October 24, 2018
- General Introduction chapter: last updated November 17, 2022
- In-Depth parameters chapter: last updated March 4, 2026

When these chapters disagree with live endpoint behavior, prefer:

- live endpoint output for the current database list and current field/link inventory
- the newest Bookshelf chapter for request semantics

- without an API key, NCBI recommends no more than 3 requests per second per site/IP
- with an API key, a site can post up to 10 requests per second by default
- higher rates require direct NCBI approval
- for large jobs, NCBI recommends weekends or 9:00 PM to 5:00 AM US Eastern on weekdays
- `tool` and `email` should be sent in requests and registered with NCBI if high-volume software may trigger enforcement

Current code already enforces the 3/10 request-per-second rule globally across goroutines. Future work must not bypass that throttle.

Important live implementation reminder:

- the current repository fallback key string is usable only when sent in lowercase
- the same key string sent in uppercase currently receives a live NCBI `400` response with `error="API key not wellformed"` and `type="api-key-fake"`
- product behavior must therefore normalize configured keys to lowercase before request use
- fallback to the no-key rate still matters for genuinely invalid/revoked keys
- documentation should still treat `NCBI_API_KEY` as a deployment secret, not as something to rely on as a hardcoded public constant

## E-utility Roles

The practical NCBI pipeline surface is:

- `EInfo`
  - discover databases
  - discover fields
  - discover link names
  - discover truncatable/rangeable fields
- `ESearch`
  - search and retrieve UID lists
  - return counts
  - sort result sets
  - page large sets with `retstart` / `retmax`
  - emit accession IDs instead of GI for sequence DBs via `idtype=acc`
- `ESummary`
  - retrieve compact record summaries
  - supports JSON
  - supports versioned XML
- `EFetch`
  - retrieve full records in database-specific formats
  - supports batching with `retstart` / `retmax`
- `ELink`
  - retrieve linked records across databases
  - retrieve related records in the same database
  - can post linked outputs to History server
- `EPost`
  - upload explicit UID lists to History server
- `EGQuery`
  - count a query across all Entrez databases
- `ESpell`
  - spell suggestions
- `ECitMatch`
  - PubMed citation-matching API

## Core Mechanisms Future NCBI Integration Must Support

### 1. Dynamic capability discovery through `EInfo`

Future code should not hardcode all fields, links, or sort assumptions.

Use `EInfo` to discover:

- valid database names
- field list and field descriptions
- whether fields are truncatable
- whether fields are rangeable
- link names for `ELink`

Why it matters:

- different databases support different fields and link sets
- future NCBI changes should be absorbed by capability refresh instead of code archaeology
- the Bookshelf overview can lag the live inventory, while `EInfo` reflects the current indexed database set

### 2. History server pipelines

NCBI History server is not optional for serious batch work.

Use cases:

- large result sets beyond one response page
- multi-step `ESearch -> ELink -> ESummary/EFetch`
- filtered subsets from a prior posted set
- explicit uploaded UID lists

Key values to preserve when a workflow depends on them:

- `WebEnv`
- `query_key`
- source `db`
- `linkname` when output came from `ELink`

If a future search type depends on History-server state, snapshot design should preserve it explicitly when it improves reopen fidelity.

### 3. Batching

Official docs note practical retrieval batching:

- `ESearch` output paging with `retstart` / `retmax`
- `ESummary` and `EFetch` paging with `retstart` / `retmax`
- `EPost` for large explicit input sets
- `ELink` POST for large id lists or repeated `id=` preservation cases

Rule for implementation:

- do not create one HTTP request per row when batchable endpoints exist
- do not fetch full records before summaries prove a row is worth keeping
- prefer:
  - `ESearch` once
  - `ESummary` in batches
  - `EFetch` only for selected rows, detail views, or sequence-required flows

### 4. Official accession-versus-UID behavior

NCBI sequence databases accept accession.version for many operations, but UID logic still exists.

Implications:

- preserve both accession-like external IDs and internal UIDs when available
- do not assume every record class has the same primary identifier
- some ESummary/ELink behavior still revolves around UID indexing
- `EFetch` can retrieve sequence records by accession.version even when indexing or linking differs

### 5. Database-specific search fields

`field=` and `[fieldtag]` search behavior is database specific.

Implications:

- advanced search UI must be generated per search type, not shared blindly
- a future "simple query" mode can stay raw-text
- a future "structured query" mode must read field metadata from `EInfo`

### 6. Sorting is database-specific

`ESearch sort=` values vary by database.

Implications:

- review ordering must distinguish:
  - raw Entrez order
  - explicit server-side sort
  - client-side table sort
- do not pretend one universal NCBI sort menu exists

## NCBI-Specific Product Risks To Surface In UI/Docs

### Replacement and record updates

Current protein flow already handles `replacedby`.

Future generalization:

- any NCBI type with replacement/merged/withdrawn concepts should surface row status clearly
- review should never silently overwrite the originally requested identifier
- keep original request, replacement target, and user decision visible in row metadata

### Search-count inflation versus useful-row yield

Some databases return many hits but few user-useful rows after filtering or linked fetch.

Implication:

- progress UI should distinguish:
  - search hit count
  - summary rows fetched
  - usable rows retained

### Cross-database ambiguity

NCBI has overlapping concepts:

- `nuccore` versus `nucleotide`
- `gene` versus `protein` linked gene metadata
- `assembly` versus `genome`
- `gds` versus `geoprofiles`
- `gap` versus `gapplus`

Implication:

- documentation and UI labels must explain why a user would choose one over the other
- internal search-type IDs must remain exact Entrez DB names

### Record types with no meaningful FASTA

Many future NCBI search types should not expose FASTA export or Canvas transfer.

Implication:

- later actions must be driven by result-domain capability, not only by source database name `ncbi`

### Record-by-record link expansion cost

Linked enrichment can explode request counts.

Implication:

- use batch `ELink` where possible
- deduplicate linked target IDs before `ESummary` / `EFetch`
- cache linked summaries and fetches per identifier
- for rare one-to-one-preserving cases, keep explicit mapping structures instead of sacrificing provenance

### POST versus GET

Official NCBI docs explicitly recommend POST for long queries or large ID lists.

Implication:

- future NCBI infrastructure should support both GET and POST transport centrally
- do not duplicate ad hoc POST logic in each search-type engine

## Recommended Mandatory Engine Features Before Large NCBI Expansion

- shared `EInfo` cache with refresh policy
- shared History-server helper
- shared GET/POST request builder
- shared global throttle and retry policy
- shared batch scheduler for `ESummary`, `EFetch`, and `ELink`
- shared row provenance fields:
  - source database
  - requested search type
  - engine row class
  - linked-from database and linkname when relevant
  - request identifier and final resolved identifier

## Current implemented first-wave state

The current codebase now partially realizes the above roadmap:

- first-wave summary-specialized row builders exist for `gene`, `nuccore`, `nucleotide`, `assembly`, `bioproject`, `biosample`, `taxonomy`, and `sra`
- those builders already preserve static jump-target metadata for the most important planned `ELink` paths
- actual `ELink` execution is now partially active for selected fallback chains:
  - `gene -> protein`
  - `gene -> nuccore`
  - `gene -> clinvar`
  - `assembly -> bioproject`
  - `biosample -> bioproject`
  - `pubmed -> pmc`
- those chains currently act as automatic fallback search paths plus provenance capture, not yet as a full standalone jump browser UI
