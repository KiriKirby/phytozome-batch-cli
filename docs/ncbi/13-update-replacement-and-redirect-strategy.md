# Update, Replacement, And Redirect Strategy

This document generalizes the current NCBI protein replacement logic to the full Entrez family.

## Current proven rule

For `protein`, the workflow already treats `replacedby` as a user-visible decision, not a silent overwrite.

That rule should become the NCBI-wide baseline.

## Prompt categories

### Category 1: hard replacement

Use when the source record clearly reports a replacement target such as:

- `replacedby`
- discontinued/merged accession with a newer accession

Prompt:

- show original requested identifier
- show replacement identifier
- ask whether to keep old, use new, or skip

Persist:

- `ncbi_requested_accession`
- `ncbi_replacement_accession`
- `ncbi_replacement_decision`

Current implemented state:

- the workflow is now shared across visible NCBI families rather than being treated as protein-only
- replacement detection now recognizes non-protein identifiers such as:
  - `ncbi_clinvar_accession`
  - `ncbi_gtr_accession`
  - `ncbi_dbvar_accession`
  - `ncbi_bioproject_accession`
  - `ncbi_biosample_accession`
  - `ncbi_assembly_accession`
  - `ncbi_omim_id`
  - `ncbi_medgen_id`
  - `ncbi_rsid`
- first-wave non-protein search types preserve `replacedby`-style summary metadata in `ncbi_*` extra fields when the source summary exposes it
- replacement/update visibility is no longer protein-only at the schema layer:
  - NCBI replacement-related fields have registered column metadata plus report/detail visibility
  - visible NCBI table families can surface:
    - `ncbi_replaced_by`
    - `ncbi_requested_accession`
    - `ncbi_replacement_accession`
    - `ncbi_replacement_decision`
  - accepted and kept-old decisions are also reflected back into `search_type`
  - this means update state is no longer buried exclusively inside raw extra JSON-like metadata
- important API-key-specific correction: the current live default key works only when normalized to lowercase; uppercase form is rejected by live NCBI as malformed, so runtime now lowercases the key before requests

### Category 2: linked better target

Use when the current row is valid, but a linked target is more actionable:

- `nuccore` CDS row with only `/protein_id`
- `gene` row where the user likely wants linked `protein` or `nuccore`
- `PubMed` row with a `PMC` full-text jump

Prompt:

- frame this as a jump choice, not as a replacement

Persist:

- jump source
- jump target
- selected `linkname`

Current implemented state:

- first-wave `gene`, `assembly`, `bioproject`, `biosample`, `taxonomy`, and `sra` rows now already carry static planned jump metadata such as:
  - `ncbi_jump_targets`
  - `ncbi_jump_1_dbto`
  - `ncbi_jump_1_linkname`
  - `ncbi_jump_1_label`
- this supports future jump UI/report/snapshot work without losing the intended link graph per database
- some jump categories are now no longer only planned metadata: a growing subset of `ELink` chains executes automatically when direct target-database search returns zero rows, and those rows persist executed-link provenance in:
  - `ncbi_link_resolution`
  - `ncbi_linked_from_db`
  - `ncbi_linked_to_db`
  - `ncbi_linked_from_search_type_id`
  - `ncbi_linked_to_search_type_id`
  - `ncbi_linkname`
  - `ncbi_link_source_ids`
  - `ncbi_link_target_ids`

### Category 3: access / policy redirect

Use when the record exists but direct use is restricted or secondary:

- `gap`
- selected controlled-access clinical resources

Prompt:

- explain that the row is real but access/export may be limited

## Table visibility rule

If replacement or redirect was accepted, reflect it in visible metadata:

- `search_type`
- row detail
- extra columns

Do not bury the decision only inside raw extra fields.

## Snapshot rule

Snapshots should preserve both:

- what the user asked for originally
- what the user chose to continue with

This is especially important for reopened reviews, report generation, and later jump re-execution.

Current implemented snapshot behavior already preserves the executed NCBI linked-fallback summary inside `keyword-source-state`, so reopened sessions do not have to guess whether the visible rows came from a direct search or an automatic `ELink` bridge.
