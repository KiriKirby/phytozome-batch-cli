# `msaexpor` Implementation Plan

## Implementation Status

The first implementation is in place under `internal/phylo/viewer_assets/assets/msaexpor/`.

Implemented and covered by automated tests:

- PHgo-owned `msaexpor` browser module and MSA-themed CSS.
- `File -> Export image...` route into the PHgo bridge.
- Jalview/SwingJS child-window creation and DOM host mounting.
- data model loading from payload, MSA selection, saved state, live bridge state, and FASTA/state default rows when metadata records are unavailable.
- strict advanced layout DSL parser.
- automatic full-width single-block layout.
- Jalview-native offscreen render bridge as the only renderer.
- SVG and memory-safe PNG export.
- PDF export through the single-file `PHGOmsaexporPDF` bundle.
- live Jalview/SwingJS rendering for residue cell colors, residue text colors, group colors, feature colors, labels, and grid geometry.
- manual SVG preview refresh with a dirty-state indicator and a scrollable preview region.
- same-origin iframe isolation inside the SwingJS child-window content area so browser-native form controls remain interactive while the outer Jalview window manager still owns drag, resize, z-order, and close behavior.
- save picker integration with sanitized numbered default filenames.
- `Refresh preview`, `Generate`, and `Cancel` actions, with `Generate` disabled during an active export to prevent duplicate writes.
- explicit empty-input errors for missing rows or missing aligned columns.
- PDF SVG/XML validation before handing the scene to `svg2pdf.js`.
- Node parser/rendering tests and Go asset contract tests.

Release-signoff validation guidance:

- Browser smoke testing has been exercised against a real JalviewJS MSA page: `File -> Export image...` opens the `msaexpor` UI inside a native SwingJS `JInternalFrame`, the frame can be dragged through the native title bar, native resize handles are present, the UI shows `Generate` and `Cancel`, default format/scale are SVG/2x, and the preview renders a non-empty Jalview-native vector SVG scene.
- Save-picker suggested filenames and abort behavior are covered by JavaScript unit tests because the in-app browser validation context does not allow overriding `window.showSaveFilePicker`.
- Do one large-alignment PDF visual review before release candidates that include renderer or PDF dependency changes.
- A future React + Fluent rewrite is optional only; it is not required for this implementation to be complete. If done later, it must preserve the same child-window, settings, renderer, export, and test contracts.

## Phase 1: Asset and Module Shell

- Add a PHgo-owned `msaexpor` browser module under local viewer assets.
- Add a build/copy path so `msaexpor` is bundled into `internal/phylo/viewer_assets`.
- Add a single-file browser-global PDF dependency bundle.
- Keep the module offline-capable.

## Phase 2: Menu Replacement

- Patch the vendored JalviewJS menu path so `File -> Export image` becomes `Export image...`.
- Bypass `AlignFrame.createPNG`, `AlignFrame.createSVG`, and related Jalview `ImageExporter` paths for this PHgo MSA menu item.
- Route the menu action into `window.__PHGOJalviewBridgeAPI.openMSAExportImageWindow()`.
- Preserve the old Jalview internals only as hidden upstream code, not as the visible PHgo MSA image export workflow.

## Phase 3: Native Child Window Registration

- Open or focus one `msaexpor` child window per MSA page.
- Register through the existing Jalview/SwingJS child-window framework.
- Let SwingJS manage drag, resize, z-order, focus, and close behavior.
- Mount a PHgo DOM host inside the window content area.
- Mount a same-origin iframe inside that host and render the PHgo-owned Fluent-style UI inside the iframe. This isolates input/select/textarea events from SwingJS parent-page event capture while preserving the native child-window frame.
- A future React + Fluent rewrite must keep the same iframe mount and bridge contract unless it proves an equally robust event-isolation strategy.

Required bridge surface:

```javascript
window.__PHGOJalviewBridgeAPI.openMSAExportImageWindow()
```

The function:

- resolves the current MSA session
- ensures current MSA state is available
- creates or focuses the child window
- mounts or updates the PHgo export root

Vendored Jalview menu code must call `openMSAExportImageWindowSafe()` rather than the raw opener. The safe entry records bridge debug state and shows an in-page toast beginning with `MSA export window failed:` if SwingJS frame construction, asset loading, or model loading fails.

## Phase 4: Data Collection

Build one normalized export model from:

- `GET /sessions/<id>/payload`
- `GET /sessions/<id>/msa/selection`
- `GET /sessions/<id>/msa/state`
- `collectMSAState("msaexpor-open", { full: true })` when current in-memory edits are needed

The model must include:

- sequence rows in current Jalview/MSA visual order when live state is available
- taxon id
- PHgo coordinate values
- display name
- display prefix
- aligned sequence string
- row state
- groups
- features
- alignment width

Do not read rendered residue text or colors from screen pixels. The final scene must be produced by the native render bridge rather than by PHgo reconstructing Jalview style data. `msaexpor` must not collect per-residue color arrays or a PHgo-owned style snapshot for export, because that duplicates Jalview's renderer and adds O(rows * columns) work before the real native render.

If `payload.metadata.records` is absent or empty, row construction must still proceed from live/saved `state.sequences` and then from `payload.aligned_fasta`. Empty export input is an explicit error: renderers must report `No MSA rows to export.` instead of producing a blank image.

## Phase 5: Settings UI

Implement the initial settings from [Settings Contract](./settings-contract.md):

- format selector: SVG, PNG, PDF
- scale selector: 1x, 2x, 5x, 10x
- cell width input
- cell height input
- show PHgo coordinates
- show length ratio
- show length percent
- show alignment column numbers
- column number interval
- show right residue numbers
- show groups
- show features
- advanced layout switch
- advanced layout text area

The current UI uses PHgo-owned Fluent-style controls and applies PHgo/MSA-flavored theme overrides for colors and corner radius. The controls are hosted in a same-origin iframe so native browser select/input/textarea interactions are not intercepted by SwingJS global handlers.

## Phase 6: DSL Parser

Implement the DSL in [Group DSL](./group-dsl.md).

Parser output must be a structured layout plan:

```text
LayoutPlan
  blocks[]
    sourceLine
    rows[]
    columnStartBoundary
    columnEndBoundary
    visibleColumnCount
    alignmentWidthForNumbering
```

Parser behavior must be testable without a browser.

## Phase 7: Jalview-Native Renderer

Implement the bridge renderer:

- accepts normalized settings and resolved layout blocks
- temporarily sets Jalview viewport cell geometry for export
- emits left labels as SVG text using the same PHgo/Jalview label rules
- emits residue blocks as SVG cell rectangles and text using Jalview's active renderer state
- emits top alignment column labels and right residue numbers in the same vector scene
- disables PHgo row checkboxes, selection/cursor UI, annotations, and unrelated window/page UI
- gates group and feature rendering according to settings
- restores viewport state in `finally`

The bridge returns the canonical transparent vector SVG. There is no screenshot, DOM capture, or raster-image-in-SVG fallback.

## Phase 8: PNG and PDF Export

Mirror the tree viewer's save/export approach:

- SVG: save the bridge-generated vector SVG string
- PNG: rasterize the bridge-generated vector SVG to canvas at selected scale
- PDF: convert the bridge-generated vector SVG using the bundled PDF path
- save picker: sanitize suggested filenames, default to the numbered title prefix when available, and close partial writable handles in `finally`

The implementation may share or duplicate only small browser-save helpers from `tree-viewer/src/pgv.js`. `msaexpor` must not depend on tree-specific snapshot logic.

If neither the PHgo save bridge nor `showSaveFilePicker` is available, saving fails explicitly. The module must not silently downgrade to an `<a download>` browser-download path.

## Phase 9: Preview

The preview renders the same Jalview-native vector SVG scene in a scrollable preview region inside the child window.

Preview updates are manual:

- initial model load does not render the preview automatically; it marks the preview dirty and waits for `Refresh preview`
- editing any setting marks the preview dirty and shows `Preview needs refresh`
- `Refresh preview` regenerates the preview
- changing final format or export scale must not switch the preview renderer to PNG/PDF or use the final export scale
- preview rendering uses the logical Jalview-native vector scene for responsiveness
- final `Generate` always regenerates the canonical Jalview-native vector SVG scene from current settings before saving

Preview viewport clipping and scrollbars may depend on child-window size. The generated SVG scene, PNG dimensions, PDF page size, block wrapping, label content, and cell geometry must not depend on child-window size.

PNG export must use `canvas.toBlob()` rather than `canvas.toDataURL()` so large exports do not duplicate the entire raster as a base64 string before saving.

## Phase 10: Validation and Error UX

Validate before export:

- aligned FASTA exists
- selected rows exist
- exportable MSA rows exist
- aligned MSA columns exist
- cell dimensions are positive
- column interval is positive when enabled
- DSL parses when advanced layout is enabled
- DSL row coordinates resolve
- column ranges are within alignment width
- resolved allocation counts match range widths

Errors must be shown inside the `msaexpor` window, not as browser alerts.

Save cancellation is not an error. If the user cancels the picker, the window reports `Export canceled.` and leaves the MSA state unchanged.
