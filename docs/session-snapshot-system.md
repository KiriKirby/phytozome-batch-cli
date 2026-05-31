# Session Snapshot System (.pgo)

`phytozome GO` session snapshots are `.pgo` files. A `.pgo` file is a compressed workflow-freeze archive, not a raw memory dump. Its job is to reopen the program in the same user-facing state the user left behind, across databases, modes, and future features.

## Definition

A valid snapshot must preserve the information needed to continue the same workflow without re-running searches, BLAST, enrichment, or label inference unless the user explicitly asks for a new action later.

The preservation rule is strict:

- do not rely on recomputing anything that has already been generated once
- if a workflow has already produced rows, labels, filter decisions, reports, FASTA records, external-reference enrichments, or cached sequence/materialized blobs, the snapshot should preserve those generated artifacts directly instead of planning to rebuild them later
- if a cached artifact already exists in a non-text format, keep it in that original format when that is the simplest and most reliable preservation path
- snapshot design should prefer completeness and maintainability over file size

That means a snapshot must keep:

- the active database, mode, and result kind
- the selected species or release context
- the original query/input set, before any family merging or post-processing changes the visible table count
- all result rows currently known to the workflow
- row grouping, review mode, selection masks, filter flags, and filter settings
- query metadata, family BLAST settings, and user-edited labels
- table cursor, sort mode, list offset, and scroll position
- cached sequences and any other data needed by later export or detail actions
- previously generated artifacts and caches when later actions would otherwise depend on rebuilding them
- the application version and creation metadata that help explain how the snapshot was produced

A snapshot is complete when reopening it reproduces the same review and export choices the user would expect from the saved workflow state. Completeness is about recorded workflow inputs and decisions, not about freezing internal Go objects or UI widgets.

## File Layout

The `.pgo` file is a compressed archive with versioned XML modules:

- `manifest.xml` declares the format, format version, and module list.
- `modules/context-v2.xml` stores application version, database, mode, result kind, and creation metadata.
- `modules/keyword-result-v2.xml` stores keyword-like result pages, including TAIR family index results.
- `modules/keyword-source-state-v3.xml` stores keyword source-engine metadata, including NCBI Entrez database, record type, engine schema, accessions, and UIDs.
- `modules/blast-result-v2.xml` stores BLAST result pages, including original query-count metadata.
- `modules/keyword-review-state-v2.xml` stores keyword result-table UI state such as cursor, sort, and scroll position.
- `modules/blast-review-state-v2.xml` stores BLAST result-table UI state for both single-table and multi-table review.
- `modules/sequence-cache-v2.xml` stores peptide sequence data already needed by later FASTA export and detail views.
- `modules/export-settings-v2.xml` stores the latest export-setting state that should be restored on reopen.
- `modules/external-references-v2.xml` stores BLAST external-reference settings such as UniProt and InterPro toggles.
- `modules/handoff-state-v2.xml` stores transfer and rewind context needed for follow-up workflow continuity.
- `modules/artifact-manifest-v2.xml` lists explicitly selected generated artifacts packed under `artifacts/` in their original text or binary forms.
- `modules/runtime-cache-v2.xml` stores in-memory workflow caches that have already been computed and would otherwise be silently rebuilt, such as label lookup caches, query-source resolution caches, keyword-term row caches, UniProt and InterPro lookup caches, species-candidate caches, and protein-sequence hit/miss caches.

For the current `v2.3` implementation, artifact packing is intentionally narrow:

- only pack artifacts and cache payloads that are explicitly selected by the workflow as needed for later continuity
- store each packed file with an explicit restore target so snapshot open can rehydrate only the recorded state before workflow continuation begins
- do not blanket-pack the whole selected export directory, the default app-local `output/` directory, or the whole app-local `.cache/` tree

Each module has its own version. New application features should add a new module or a new module version when a behavior change would otherwise make the old meaning ambiguous.

## Operational Rules

- Opening a snapshot must not re-run searches, BLAST jobs, enrichment, or label inference.
- Opening a snapshot should not rebuild generated artifacts from source data if those artifacts were already available when the snapshot was written.
- Opening a snapshot should first restore only the explicitly recorded artifacts back to their original locations so later actions reuse frozen data directly instead of silently recomputing.
- The loader must reopen the saved review path directly, including multi-file BLAST review when the original input set had more than one query item.
- Family BLAST merging must never reduce a multi-input snapshot into single-input review just because the merged visible table count became one.
- Opening a snapshot should not perform network access unless the user later chooses an action that normally needs it and the required sequence data was not already stored.
- If a packed artifact cannot be restored, the open flow must surface that fact explicitly; fallback behavior is allowed only after an explicit restore failure, not as a silent default.

## Storage Policy

- Text-derived workflow structures may be stored in XML modules when that stays simplest to inspect and maintain.
- Generated raw payloads, caches, and non-text artifacts may be stored as separate archive members in their original formats, including binary blobs, when that avoids lossy translation or fragile reconstruction logic.
- File size is not a design driver for snapshot completeness. A larger but simpler and more faithful snapshot is preferred over a smaller snapshot that depends on reconstruction.

## UI Contract

Export settings include `Save session snapshot (.pgo)`. In single-table export this writes a snapshot for that workflow. In BLAST `Export all` this writes one snapshot for the whole reviewed input set, even when family merging collapses the visible tables.

The startup `Explore` section includes `Open session`. The input accepts:

- an absolute `.pgo` path
- a relative file name inside the default app-local `output/`
- either form with or without the `.pgo` extension

The open-session input is raw path input. A typed `.pgo` path is opened directly and is not reinterpreted as pasted text content.

Canvas Add canvas is the full import entry for FASTA/phgo FASTA text, FASTA/text files, and `.pgo` session snapshots. Add rows is FASTA/phgo FASTA only; if a `.pgo` path, dropped `.pgo` file, or output snapshot name is provided there, the UI must reject it and tell the user to use Add canvas or `Explore -> Open session`.

Canvas item titles follow a fixed source rule:

- result tables with a left sidebar preserve the original left-sidebar title
- result tables without a left sidebar, such as keyword, family, and single-file BLAST, use numeric titles
- multi-file BLAST is still a left-sidebar source after family merging; Canvas titles combine the original sidebar main title and family subtitle first line, for example `AT2G37040.1[PAL]`
- `.pgo` snapshot imports preserve saved left-sidebar titles when present; no-sidebar snapshot sources use the shortened snapshot filename
- FASTA/text files use the shortened source filename
- pasted FASTA text uses a numeric title

Canvas tree snapshots also preserve the system-tree tool panel state: the current page, display-name source, conversion target/action, skip-unselect setting, alignment method/parameters, and tree method/parameters. The default restored page is the conversion page, so reopened snapshots expose the Protein/DNA mode choice before alignment and tree settings. A restored tree payload may be shown immediately, but the first user-triggered `Refresh tree` after opening a Canvas snapshot must run a full `mega-phgo-runtime` compute pass before rendering; only later display-label-only changes are render-only.

## Current Version Rule

- The current unreleased snapshot format is `v2.3`.
- Because the software is not yet released, the code may refactor snapshot structure directly instead of carrying long-term compatibility burden for older draft formats.
- New modules should still keep their meanings explicit and versioned so future changes can evolve cleanly. The `keyword-source-state-v3` module is the current extension point for NCBI keyword-search source metadata beyond the protein-only first implementation.
- Store mode/database/result identifiers as plain strings so new workflows can be routed without creating a hardcoded tree.
