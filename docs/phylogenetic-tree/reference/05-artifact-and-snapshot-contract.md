# Artifacts And Snapshot Contract

The system-tree feature is auditable only if every run can be reconstructed
from preserved artifacts without guessing. This document defines the files and
JSON objects that connect Canvas, `mega-phgo-runtime`, the viewer, and `.pgo`
snapshots.

Primary sources:

- `internal/phylo/types.go`
- `internal/phylo/build.go`
- `internal/phylo/artifact.go`
- `internal/phylo/source_runtime.go`
- `internal/phylo/run.go`
- `internal/workflow/canvas.go`
- `internal/workflow/session_snapshot.go`
- `internal/sessionsnapshot/`
- `internal/viewersnapshot/`
- `tree-viewer/src/pgv.js`

## Run Directory Files

Every successful compute run should preserve:

| File | Owner | Purpose |
| --- | --- | --- |
| `input.fasta` | PHgo | Exact FASTA handed to MEGA runtime, using stable `PHGOT...` IDs |
| `input.meta.json` | PHgo | Display names, source rows, table values, row fingerprints |
| `runtime-request.json` | PHgo | Runtime API request |
| `runtime-response.json` | MEGA runtime | Runtime completion, skipped rows, artifact paths, error text |
| `aligned.fasta` | MEGA runtime | MEGA alignment output |
| `tree.nwk` | MEGA runtime | MEGA Newick output |
| `runtime-summary.txt` | MEGA runtime | Human-readable runtime summary |
| `runtime.log` | MEGA runtime | Runtime log |
| `runtime.stdout.txt` | PHgo | Captured stdout |
| `runtime.stderr.txt` | PHgo | Captured stderr |
| `viewer.payload.json` | PHgo | Browser viewer payload |
| `run.manifest.json` | PHgo | Settings, fingerprints, artifact names |

Partial run directories are allowed for diagnosis, but PHgo must not treat a
run as successful without the runtime response and required tree/alignment
artifacts.

Cancellation and failure handling:

- If the context is canceled before PHgo writes `runtime-request.json`, the run
  returns `context canceled` and does not create a request file.
- If cancellation happens while `mega-phgo-runtime` is executing, PHgo returns
  `context canceled` and keeps `runtime.stdout.txt` / `runtime.stderr.txt` for
  diagnosis.
- If the runtime exits nonzero but writes `runtime-response.json.error_text`,
  PHgo surfaces that exact MEGA/runtime error before checking for missing
  aligned FASTA or Newick files.

## Runtime Request

`runtime-request.json` contains:

- `schema_version`
- `session_id`
- `run_id`
- `created_at`
- `sequence_kind`
- normalized `settings`
- `records`
- `input_fasta`
- `artifacts`

The runtime receives normalized method names:

- protein ClustalW UI ID maps to runtime `clustalw`
- protein MUSCLE UI ID maps to runtime `muscle`
- DNA/codon IDs keep their runtime-specific method names
- tree methods map to `neighbor_joining`, `minimum_evolution`, `upgma`,
  `maximum_likelihood`, or `maximum_parsimony`

## Runtime Response

`runtime-response.json` contains:

- `schema_version`
- runtime name
- completion timestamp
- artifact paths
- optional `skipped_records`
- optional `error_text`

If `error_text` is nonempty, PHgo surfaces it before any missing-artifact
checks. This protects the real MEGA failure from being hidden by a secondary
Go-side message.

## Stable Taxon IDs And Labels

PHgo gives each selected record a stable runtime taxon ID:

```text
PHGOT000001
PHGOT000002
...
```

The runtime FASTA and Newick use those IDs. Display names live in metadata and
are applied only at the viewer adapter boundary. This keeps Newick escaping and
duplicate labels out of the MEGA computation path.

## Fingerprints

The run plan stores four fingerprints:

| Fingerprint | Includes | Excludes |
| --- | --- | --- |
| Input | selected row identity/order, source type, original head, sequence, table values except display name | viewer layout |
| Alignment | input fingerprint, target mode, alignment method, alignment parameters | display names |
| Tree | alignment fingerprint, tree method, tree parameters | display names |
| Preview | display-name source and display names | sequence/tree computation |

Display-name-only changes may refresh the viewer without rerunning the runtime.
Any row, sequence, target mode, alignment, or tree parameter change requires
compute refresh.

## Artifact Reuse

Reuse is valid only when:

- `run.manifest.json` exists,
- manifest schema is current,
- computation settings match,
- input/alignment/tree fingerprints match,
- `aligned.fasta` and `tree.nwk` exist,
- this is not the first user-triggered refresh after opening a `.pgo`.

Reuse still rewrites payload metadata so display names remain current.

## Canvas `.pgo` Snapshot Contract

A Canvas snapshot must preserve:

- durable tree settings
- target mode
- alignment method and parameters
- tree method and parameters
- display-name source and display names
- last viewer payload
- last run manifest
- last run ID and artifact directory reference
- `input.fasta`
- `input.meta.json`
- `runtime-request.json`
- `runtime-response.json`
- `aligned.fasta`
- `tree.nwk`
- `viewer.payload.json`
- `run.manifest.json`
- runtime logs when present
- MSA row states (`green`, `yellow`, `red`)
- last shared tree/MSA payload and aligned FASTA
- durable Jalview MSA state such as groups, annotations, markers, and settings when available

A Canvas snapshot must not preserve pure UI-open state such as expanded/focused
tool panels, current settings page, scroll offsets, browser viewport metadata,
transient search text, or open menu/ribbon state. PHgo uses one selected-row set
for tree and MSA. The yellow MSA row state means the row is unchecked for both
tree and MSA, and PHgo shows a yellow checkbox only because the exclusion came
from MSA Apply.

Opening a snapshot restores enough in-memory state to reopen the tree panel,
reopen the MSA view, save the snapshot again, and update preview metadata
without recomputing.

The first explicit `Refresh tree` after opening a snapshot must recompute
through `mega-phgo-runtime` even when restored fingerprints match.

## Viewer `.pgv` Snapshot Contract

The viewer snapshot preserves:

- producer name
- schema version
- viewer payload
- Reactree/browser-side state
- PHgo viewer state such as alignment split size and viewport metadata

`.pgv` is for the browser viewer, not the Canvas workflow. A `.pgo` preserves
workflow continuity; a `.pgv` preserves a viewer session.

Reader contract:

- Standalone tree-browser sessions read local `.pgv` files through
  `parseViewerSnapshot`; the Canvas viewer page does not expose a local `.pgv`
  open button.
- The reader rejects invalid JSON, unknown `format`, unsupported
  `schema_version`, and missing payload objects.
- Opening a `.pgv` restores `payload` and `viewer_state` in the browser only.
- While a `.pgv` is open, live session reload events are ignored so the restored
  state is not overwritten by the Canvas session. The user can return to the
  live session explicitly.
- The `.pgv` reader does not update `.pgo`, runtime manifests, Newick, aligned
  FASTA, or any MEGA computation artifact.

## Audit Requirements

- Snapshot tests must cover save/open with full artifacts and with
  `viewer.payload.json` fallback.
- Tests must verify first-refresh-after-snapshot forces compute.
- Tests must verify display-name-only snapshot sync changes preview metadata
  without changing alignment/tree fingerprints.
- Tests must verify runtime `error_text` is surfaced directly.
- Tests must verify missing runtime files report strict bundled-runtime errors.
- Tests must verify `.pgv` parse/reject/restore behavior and viewer asset sync
  after frontend changes.
- Tests must verify selected rows are not filtered by export-readiness helpers,
  normal FASTA export keeps its separate export-readiness filtering, empty
  selected records remain in `input.fasta`, and real source-resolution or
  source-construction errors are not swallowed into empty runtime records.
- Tests must verify viewer payload/state isolation between Canvas sessions and
  standalone `.nwk`/`.pgv` browser sessions.
