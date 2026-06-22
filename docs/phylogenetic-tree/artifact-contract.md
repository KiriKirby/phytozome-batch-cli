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

The request and `input.fasta` keep the original selected sequences. PHgo does not sanitize, trim, translate, reverse-translate, or repair protein/nucleotide content before runtime execution; the runtime hands the selected data to MEGA-derived alignment/tree components and surfaces their output or error text.

Reactree in-page visual state is persisted through the independent text `.pgv` PHgo Viewer Snapshot format. A `.pgv` stores the current `viewer.payload.json` data plus `viewer_state`, including Reactree layout/edit state and PHgo viewer-only panel state. Canvas `.pgo` snapshots preserve durable computation payloads, settings, tree operations, and MSA state, but deliberately drop pure UI-open state such as open menus, transient search text, current settings page, panel focus, scroll offsets, and browser viewport metadata.

## Stable Taxon IDs

Newick leaf names and aligned FASTA headers must use stable internal taxon IDs:

```text
PHGOT000001
PHGOT000002
PHGOT000003
```

User-facing labels live in metadata and page state. Stable IDs stay alphanumeric so Newick-to-metadata mapping is lossless. For tree rendering, `display_name` may include the optional PHgo coordinate prefix when the Canvas setting is enabled. For MSA rendering, `base_display_name` is Jalview's true sequence name and `display_prefix` is drawn separately in the left ID list; menus, rename dialogs, and Apply payloads use the base name. PHgo's Jalview left ID renderer ignores Jalview's right-align-ID and sequence-limit suffix settings so the visible list stays left-aligned and never gains generated `/start-end` suffixes that would diverge from PHgo `display_name`.

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
      "base_display_name": "PAL1",
      "display_prefix": "[1,1]",
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

`conversion_target` is a legacy field name kept for `.pgo` and runtime-request compatibility. Its value is the selected MEGA target data mode, `protein` or `dna`; it does not authorize PHgo-side conversion. PHgo does not translate, reverse-translate, repair, or locally skip biological sequence content. In DNA mode, PHgo may only substitute a row's sequence with a real embedded/resolved nucleotide/CDS sequence; otherwise MEGA runtime is the authority for success or failure.

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

When the runtime reports unusable individual rows through `skipped_records`, PHgo surfaces these through the common skip dialog; if the panel setting is enabled, the skipped row selections are also cleared before retrying.

Snapshot-restored artifacts are display/recovery state, not permission to skip the next compute. The first user-triggered Refresh after opening a Canvas snapshot must bypass artifact reuse entirely and run the current `mega-phgo-runtime` so stale historical files are replaced even when their old fingerprints still match.

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
  "schema_version": 3,
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
    "schema_version": 3,
    "reactree": {
      "schema_version": 3,
      "layout": "rectangular",
      "renderStyle": "phgo",
      "treeType": "phylogram",
      "labelMode": "bootstrap",
      "showAlignment": false,
      "hScale": 1,
      "vScale": 1,
      "fontScale": 1,
      "fontFamily": "system-ui, -apple-system, sans-serif",
      "strokeWidth": 1.5,
      "exportLongEdge": 4096,
      "transform": { "x": 0, "y": 0, "k": 1 }
    },
    "phgo": {
      "split_percent": 42,
      "payload_updated_at": "2026-05-30T12:00:00Z",
      "viewport": {
        "inner_width": 1200,
        "inner_height": 800,
        "device_pixel_ratio": 1
      }
    }
  }
}
```

`payload` is the same model as `viewer.payload.json`. `viewer_state.reactree` records browser-owned visual state such as current tree topology after reroot/flip/swap/ladderize, Office-style ribbon tab/search/menu state, layout, PHgo/MEGA render style, tree type, label mode, scale controls, alignment visibility, height, font/stroke settings, node/clade colors, collapsed clades and labels, search state, toolbar mode state, export long-edge size, and zoom transform. `hScale`, `vScale`, and circular `Size` are non-negative values with `0` allowed and no schema-level maximum. `exportLongEdge` is the selected fixed export long edge in pixels, currently defaulting to `4096`. `viewer_state.phgo` records PHgo viewer-only state such as alignment split width, payload timestamp pairing, and browser viewport metadata.

The format is versioned independently from `.pgo`. Current writers emit PGV schema v3 and viewer-state schema v3. Readers accept v1, v2, and v3 snapshots so existing `.pgv` files can still be opened, while newer fields are treated as optional when absent.

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

- latest durable tree tool settings
- latest display-name values
- latest run manifest
- latest input metadata
- runtime request/response JSON
- aligned FASTA
- Newick tree output
- latest Reactree viewer payload
- latest shared tree/MSA payload, row states, and durable Jalview state
- generated export artifacts when the viewer later exposes persisted export files
- exported `.pgv` files when a user explicitly saves browser viewer visual state

Implemented snapshot shape:

- The Canvas module stores a `tree` object with durable tree settings, last viewer payload, last run manifest, last run directory, last aligned FASTA, last Newick tree, and the computation fingerprints.
- The `canvas-msa-state-v1` module stores row states (`green`, `yellow`, `red`), the last shared tree/MSA payload and aligned FASTA, and Jalview-owned durable state such as groups, annotations, markers, settings, per-sequence descriptions, per-sequence saved names, and per-sequence text edits when available. Browser Jalview state is synchronized with PHgo through `GET`/`PUT /sessions/<id>/msa/state` so snapshot save reads the latest durable MSA state instead of relying on stale launch-time data. Checkbox toggles use a lightweight rows-only state save, while File > Save, File > Apply, and startup/layout synchronization save the full durable Jalview state. PHgo keeps one selected-row set for tree and MSA: green is selected for both, yellow is unchecked for both but marked as MSA-origin in PHgo, and red is ordinary unchecked.
- The artifact manifest packs the last run directory's core files under `artifacts/tree/<session>/<run>/`, including `input.fasta`, `input.meta.json`, `runtime-request.json`, `runtime-response.json`, `aligned.fasta`, `tree.nwk`, `viewer.payload.json`, `run.manifest.json`, and runtime logs when they exist.
- Snapshot open restores packed files to their original run directory, restores the Canvas tree settings, restores the MSA row/durable Jalview state, and keeps the last payload/plan in memory so reopening the tree panel can immediately push the previous tree and MSA payloads to the local viewer service.
- Snapshot save synchronizes the last tree payload metadata from the current Canvas table before packing the snapshot. If the user changed `display_name` or the display-name source after the last tree refresh, the snapshot records those current labels in `last_payload` and updates only the preview fingerprint. Alignment/tree fingerprints and runtime artifacts are not changed, and `mega-phgo-runtime` is not rerun during snapshot save.
- Snapshot restore rebuilds an in-memory tree run plan from the saved payload, manifest, and restored artifact files. If an older or partial snapshot has an empty `last_payload` but still contains `viewer.payload.json`, restore reads that artifact as a fallback so reopening the tree panel can still recover the rendered tree.
- Snapshot-restored run plans keep the requested Protein/DNA target from the manifest or panel state. The next explicit refresh always reruns `mega-phgo-runtime`; only later refreshes may reuse runtime artifacts when the computation fingerprints match and the only change is label/render metadata.

Snapshot open should restore the Canvas page and allow the tree panel to reopen with the previous settings and previous rendered tree. It must not rerun `mega-phgo-runtime` during open or panel display, but the first explicit `Refresh tree` after snapshot open is always a full compute refresh.

Canvas/session `.pgo` snapshots can be restored as a complete workflow through `Explore -> Open session`, or imported as new Canvas items through Canvas Add canvas. Add rows stays FASTA/phgo FASTA only and must reject `.pgo` inputs with Add-canvas/Open-session guidance.
