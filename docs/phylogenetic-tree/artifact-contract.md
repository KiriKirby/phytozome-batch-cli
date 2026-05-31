# Phylogenetic Tree Artifact Contract

## Purpose

Artifacts are the stable contract between Canvas, `mega-phgo-runtime`, snapshots, and the Reactree viewer service.

The contract avoids passing fragile in-memory structures between subsystems. It also avoids `.mao` as a PHgo protocol. Runtime execution is driven by JSON plus stable FASTA artifacts.

## Run Directory

Each tree refresh writes into a tree run directory under the application cache root.

Suggested path:

```text
.cache/tree/<canvas_session_id>/<run_id>/
```

Required files:

```text
input.fasta
input.meta.json
runtime-request.json
runtime-response.json
aligned.fasta
tree.nwk
runtime.stdout.txt
runtime.stderr.txt
runtime-summary.txt
run.manifest.json
viewer.payload.json
```

`runtime-request.json` is produced by PHgo. On a fresh computation, `runtime-response.json`, `aligned.fasta`, `tree.nwk`, and runtime logs are produced by `mega-phgo-runtime`.

When PHgo reuses an existing alignment/tree because only preview metadata changed, the new run directory still receives a complete `runtime-request.json` plus a `runtime-response.json` whose `runtime` field is `mega-phgo-runtime/reused`. This keeps snapshots and diagnostics complete without pretending the custom runtime was launched again.

Protein inputs may contain common FASTA terminal stop codons (`*`). The request and `input.fasta` keep the original selected sequences, while `mega-phgo-runtime` sanitizes protein/unknown non-codon sequences only for computation. Any cleanup is recorded in `runtime.log` with a `protein.sanitized` entry, and the aligned FASTA reflects the sanitized computable sequence set.

Reactree in-page visual state is persisted through the independent text `.pgv` PHgo Viewer Snapshot format. A `.pgv` stores the current `viewer.payload.json` data plus `viewer_state`, including Reactree layout/edit state and PHgo viewer-only panel state. Canvas `.pgo` snapshots still preserve the computation payload and panel state; browser-owned visual edits are round-tripped by exporting/opening `.pgv`.

## Stable Taxon IDs

Newick leaf names and aligned FASTA headers must use stable internal taxon IDs:

```text
PHGOT000001
PHGOT000002
PHGOT000003
```

User-facing labels live in metadata and page state. Stable IDs stay alphanumeric so Newick-to-metadata mapping is lossless.

## Input Metadata

`input.meta.json` contains one entry per selected Canvas row.

Required shape:

```json
{
  "schema_version": 1,
  "display_name_source": "label_name",
  "records": [
    {
      "taxon_id": "PHGOT000001",
      "display_name": "PAL1",
      "source_type": "keyword",
      "original_head": "original FASTA header",
      "sequence_kind": "protein",
      "canvas_item": "1",
      "canvas_row": 0,
      "table_values": {
        "source_type": "keyword",
        "head": "PAL1",
        "label_name": "PAL1",
        "phgo_alias": "PAL1; ATPAL1",
        "geneid": "AT2G37040",
        "transcript": "AT2G37040.1"
      }
    }
  ]
}
```

The old source-row column must not appear in `table_values`.

## Runtime Request

`runtime-request.json` is the only compute request protocol.

Required shape:

```json
{
  "schema_version": 1,
  "session_id": "canvas-session",
  "run_id": "20260528_153000_001",
  "sequence_kind": "protein",
  "settings": {
    "display_name_source": "label_name",
    "conversion_target": "protein",
    "conversion_action": "convert",
    "conversion_skip_unselect": true,
    "alignment_method": "muscle",
    "alignment_params": {},
    "tree_method": "neighbor_joining",
    "tree_params": {}
  },
  "records": [
    {
      "taxon_id": "PHGOT000001",
      "display_name": "PAL1",
      "sequence_kind": "protein"
    }
  ],
  "input_fasta": ">PHGOT000001\nMSEQUENCE...\n",
  "artifacts": {
    "base_dir": ".cache/tree/canvas-session/20260528_153000_001",
    "input_fasta": ".cache/tree/.../input.fasta",
    "metadata_json": ".cache/tree/.../input.meta.json",
    "aligned_fasta": ".cache/tree/.../aligned.fasta",
    "newick": ".cache/tree/.../tree.nwk",
    "summary": ".cache/tree/.../runtime-summary.txt",
    "runtime_log": ".cache/tree/.../runtime.log"
  }
}
```

The runtime must consume stable taxon IDs and must not reinterpret display labels as biological identifiers.

`conversion_target` is `protein` or `dna`. `conversion_action` is `convert` or `skip`. DNA-to-protein conversion is performed only by `mega-phgo-runtime`; PHgo does not translate sequences locally.

## Runtime Response

`runtime-response.json` records the completed runtime operation.

Required shape:

```json
{
  "schema_version": 1,
  "runtime": "mega-phgo-runtime",
  "completed_at": "2026-05-28T15:30:00+09:00",
  "skipped_records": [],
  "artifacts": {
    "aligned_fasta": ".cache/tree/.../aligned.fasta",
    "newick": ".cache/tree/.../tree.nwk",
    "summary": ".cache/tree/.../runtime-summary.txt",
    "runtime_log": ".cache/tree/.../runtime.log"
  }
}
```

If runtime execution fails after starting, the response may include `error_text`. PHgo should also preserve `runtime.stdout.txt` and `runtime.stderr.txt`.

When `error_text` is present, it is the primary failure message shown by PHgo. Missing `aligned.fasta` or `tree.nwk` should not replace the runtime-specific error text.

When the runtime cannot convert or use individual rows, it may return `skipped_records`. PHgo surfaces these through the common skip dialog; if the panel setting is enabled, the skipped row selections are also cleared before retrying.

After every new runtime execution or artifact reuse, PHgo validates `aligned.fasta` against the request before publishing the viewer payload. A wrong sequence count, unconverted nucleotide output in Protein mode, or protein output in DNA mode is a hard error with the artifact directory preserved for diagnosis.

Snapshot-restored artifacts are display/recovery state, not permission to skip the next compute. A saved `.pgo` may contain an old viewer payload and old alignment files from a previous runtime build. If the restored aligned FASTA already proves the payload has the wrong biological target, PHgo suppresses that stale payload instead of pushing it to Reactree. The first user-triggered Refresh after opening a Canvas snapshot must bypass artifact reuse entirely and run the current `mega-phgo-runtime` so stale historical files are replaced even when their old fingerprints still match.

When computation artifacts are reused, `runtime-response.json` uses:

```json
{
  "schema_version": 1,
  "runtime": "mega-phgo-runtime/reused",
  "artifacts": {
    "aligned_fasta": ".cache/tree/.../aligned.fasta",
    "newick": ".cache/tree/.../tree.nwk"
  }
}
```

This response means PHgo rewrote the viewer metadata and artifact contract, but did not rerun alignment or tree inference.

## Viewer Payload

`viewer.payload.json` is what the Reactree viewer consumes.

It contains:

- Newick tree where leaves are stable taxon IDs
- aligned FASTA with matching stable IDs when available
- metadata mapping stable IDs to `display_name`

The viewer reads final `display_name` values from metadata. It must not choose labels independently from Newick leaf names.

Reactree-facing labels may be sanitized at the viewer adapter boundary because Newick and FASTA headers cannot safely carry arbitrary punctuation or whitespace. That transformation is preview/export-side only: it must be deterministic, collision-safe, and applied identically to the Newick leaves and aligned FASTA headers. The stable taxon IDs and metadata remain the source of truth for snapshot restore and future rerendering.

## PHgo Viewer Snapshot

`.pgv` files are text JSON snapshots for the browser viewer.

Required shape:

```json
{
  "format": "phgo-viewer-snapshot",
  "schema_version": 1,
  "created_at": "2026-05-30T12:00:00Z",
  "producer": "phytozome-go tree viewer",
  "payload": {
    "schema_version": 1,
    "session_id": "canvas",
    "newick": "(PHGOT000001,PHGOT000002);",
    "metadata": {
      "schema_version": 1,
      "records": []
    }
  },
  "viewer_state": {
    "schema_version": 1,
    "reactree": {},
    "phgo": {}
  }
}
```

`payload` is the same model as `viewer.payload.json`. `viewer_state.reactree` records browser-owned visual state such as current tree topology after reroot/flip/swap/ladderize, layout, tree type, label mode, scale sliders, height, font/stroke settings, colors, collapsed clades and labels, search state, toolbar mode state, and zoom transform. `viewer_state.phgo` records PHgo viewer-only state such as the alignment split width and payload timestamp pairing.

The format is versioned independently from `.pgo`. Until the release contract is frozen, only the current schema version must be supported.

## Run Manifest

`run.manifest.json` records computation state and fingerprints.

Required artifact references:

```json
{
  "schema_version": 1,
  "input_fasta": "input.fasta",
  "metadata_json": "input.meta.json",
  "runtime_request": "runtime-request.json",
  "runtime_response": "runtime-response.json",
  "aligned_fasta": "aligned.fasta",
  "newick_path": "tree.nwk"
}
```

## Export Artifacts

Tree exports are written by the Reactree page. When generated, they should be stored under a run-local `exports/` directory if the viewer service supports persisted export files.

Suggested names:

```text
exports/<base_name>.svg
exports/<base_name>.png
exports/<base_name>.pdf
exports/<base_name>.nwk
exports/<base_name>.fasta
```

The viewer service owns SVG/PNG/PDF/Newick export generation. Canvas does not provide a separate tree export modal.

## Snapshot Integration

Session snapshots must preserve:

- latest tree tool panel state
- latest display-name values
- latest run manifest
- latest input metadata
- runtime request/response JSON
- aligned FASTA
- Newick tree output
- latest Reactree viewer payload
- generated export artifacts when the viewer later exposes persisted export files
- exported `.pgv` files when a user explicitly saves browser viewer visual state

Implemented snapshot shape:

- The Canvas module stores a `tree` object with the last tree panel state, last viewer payload, last run manifest, last run directory, last aligned FASTA, last Newick tree, and the computation fingerprints.
- The artifact manifest packs the last run directory's core files under `artifacts/tree/<session>/<run>/`, including `input.fasta`, `input.meta.json`, `runtime-request.json`, `runtime-response.json`, `aligned.fasta`, `tree.nwk`, `viewer.payload.json`, `run.manifest.json`, and runtime logs when they exist.
- Snapshot open restores packed files to their original run directory, restores the Canvas tree panel state, and keeps the last payload/plan in memory so reopening the tree panel can immediately push the previous tree to the local Reactree service.
- Snapshot save synchronizes the last tree payload metadata from the current Canvas table before packing the snapshot. If the user changed `display_name` or the display-name source after the last tree refresh, the snapshot records those current labels in `last_payload` and updates only the preview fingerprint. Alignment/tree fingerprints and runtime artifacts are not changed, and `mega-phgo-runtime` is not rerun during snapshot save.
- Snapshot restore rebuilds an in-memory tree run plan from the saved payload, manifest, and restored artifact files. If an older or partial snapshot has an empty `last_payload` but still contains `viewer.payload.json`, restore reads that artifact as a fallback so reopening the tree panel can still recover the rendered tree.
- Snapshot-restored run plans keep the requested Protein/DNA target from the manifest or panel state. The next explicit refresh always reruns `mega-phgo-runtime`; only later refreshes may reuse validated artifacts when the computation fingerprints match and the only change is label/render metadata.

Snapshot open should restore the Canvas page and allow the tree panel to reopen with the previous settings and previous rendered tree. It must not rerun `mega-phgo-runtime` during open or panel display, but the first explicit `Refresh tree` after snapshot open is always a full compute refresh.

Canvas/session `.pgo` snapshots can be restored as a complete workflow through `Explore -> Open session`, or imported as new Canvas items through Canvas Add canvas. Add rows stays FASTA/phgo FASTA only and must reject `.pgo` inputs with Add-canvas/Open-session guidance.
