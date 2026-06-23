# `msaexpor` Architecture

## Purpose

`msaexpor` replaces Jalview's current MSA image export workflow with a PHgo-owned settings UI and layout/export pipeline that renders through Jalview's current MSA drawing code.

The existing Jalview image path is centered on painting the current `AlignmentPanel` through `ImageExporter` and `ImageMaker`. In the vendored JalviewJS source this path appears around `AlignFrame.createPNG`, `AlignFrame.createSVG`, `AlignmentPanel.makeAlignmentImage`, `AlignmentPanel.printUnwrapped`, `AlignmentPanel.printWrappedAlignment`, `ScalePanel.drawScale`, `IdCanvas.drawIds`, and `SeqCanvas.drawPanelForPrinting`.

PHgo's export requirements are different from Jalview's visible menu path, but the residue, feature, group, font, and colour rendering must match the current Jalview view. Therefore the primary `msaexpor` renderer reuses Jalview's native SwingJS drawing primitives (`IdCanvas.drawIds` and `SeqCanvas.drawPanel`) on an offscreen `BufferedImage`, with PHgo export settings applied as a temporary bridge context.

## Forbidden Export Strategy

`msaexpor` must not export by taking screenshots of the page, an internal frame, or the currently visible Jalview alignment panel.

Screenshot or visible panel-paint export is forbidden because it can include:

- temporary cursor or selection rectangles
- active highlight frames
- PHgo left-list checkboxes
- scrollbars
- menus or hover state
- window chrome
- annotation panels
- current viewport clipping
- stale offscreen image-cache artifacts

The export must be generated from explicit settings and Jalview's current model/renderers instead. Offscreen `BufferedImage` rendering is allowed only through the controlled `msaexpor` bridge path because it suppresses forbidden UI state and does not capture browser/window pixels.

## Export Content Boundary

The exported figure contains only the `msaexpor` alignment board:

- selected sequence labels and optional PHgo coordinate prefix
- optional length ratio and length percent text
- residues/gaps for selected rows and MSA columns
- optional alignment column numbers
- optional right-side per-row residue end numbers
- optional Jalview/PHgo group rendering
- optional sequence feature rendering

The exported figure excludes:

- Jalview annotations by default and in the initial contract
- PHgo row checkboxes
- current Jalview selection/cursor boxes
- MSA page menus/toolbars
- scrollbars
- child-window chrome
- PHgo toast/status messages

## Current Code Integration Points

`internal/phylo/viewer.go` already provides the main server-side data surfaces:

- `GET /sessions/<id>/payload`
  Returns `ViewerPayload`, including `aligned_fasta` and `metadata.records`.
- `GET /sessions/<id>/aligned.fasta`
  Returns Jalview-facing aligned FASTA with PHgo display-name headers.
- `GET /sessions/<id>/msa/selection`
  Returns row state plus display names, display prefixes, canvas coordinates, and metadata order.
- `GET /sessions/<id>/msa/state`
  Returns bridge-saved MSA state, including rows, sequences, groups, features, annotations, markers, and settings when available.
- `PUT /sessions/<id>/msa/state`
  Persists MSA state for snapshots. `msaexpor` must not call this endpoint for ordinary export actions.

`internal/phylo/viewer_assets/assets/jalviewjs/phgo-bridge.js` already provides browser-side helpers:

- `collectMSAState(trigger, options)`
- `collectMSASequences(frame)`
- `collectMSAGroups(frame)`
- `collectMSAFeatures(frame)`
- `saveMSAStateNow(...)`
- current session discovery through `window.__PHGOJalviewState`
- selected row lookup through `window.__PHGOMSASelection`

`msaexpor` must reuse these surfaces and Jalview renderer objects instead of scraping the displayed DOM.

The required primary render bridge is:

```javascript
window.__PHGOJalviewBridgeAPI.renderMSAExportScene(settings, layout)
```

It returns a scene object with `{ svg, width, height, source: "jalview-native" }`. The SVG embeds the offscreen Jalview-rendered PNG as an `<image>` element. This is intentional: in this path SVG and PDF are publication/export containers for Jalview-identical raster content, not independent vector reimplementations of Jalview residue/feature/group styling.

## Ownership

### Jalview/SwingJS Owns

- `File` menu placement
- internal child-window registration
- child-window drag, resize, z-order, focus, and close behavior
- preserving normal interaction with the underlying alignment when the child window is non-modal

### PHgo Bridge Owns

- replacing the old `Export image` action with `Export image...`
- opening or focusing the `msaexpor` child window
- mounting a PHgo DOM host inside the child window content area
- placing a same-origin iframe inside that host so browser-native form controls are isolated from SwingJS parent-page event handling
- passing session and current MSA data to `msaexpor`

### `msaexpor` Owns

- settings UI
- validation
- parsing advanced group DSL
- resolving export rows and column blocks
- requesting the Jalview-native export scene through the bridge
- failing explicitly when the Jalview-native render bridge is unavailable
- saving the bridge-produced container SVG
- deriving PNG and PDF from the generated SVG
- saving exported files

`msaexpor` is currently implemented as PHgo-owned browser JavaScript and CSS mounted inside a same-origin iframe within the Jalview/SwingJS child-window host. The outer native child window remains Jalview/SwingJS-managed for drag, resize, z-order, focus, and close behavior. The iframe exists to keep input, select, textarea, and button interactions from being intercepted by SwingJS page-level event handlers.

The iframe is also the JavaScript runtime boundary for the export UI. The bridge writes a complete iframe document that loads `/assets/msaexpor/pdf.js` and `/assets/msaexpor/index.js` inside the iframe, waits for `iframe.contentWindow.PHGOmsaexpor.renderApp`, and only then mounts the UI. The parent Jalview page injects only three controlled handles into that iframe:

- `__PHGOJalviewBridgeAPI`, used to call Jalview's native offscreen renderer
- `__PHGO_SAVE_BLOB__`, used only when the parent page already has a real PHgo save bridge
- `__PHGO_MSAEXPOR_PARENT_WINDOW__`, used for the parent document title and native child-window close path

The parent page must not execute `PHGOmsaexpor.renderApp` directly against iframe DOM. Running parent-realm functions against iframe DOM makes `window`, `document`, `Blob`, `Image`, `URL`, PDF export, save picker, and cancel behavior ambiguous and can leave Refresh/Generate buttons wired to the wrong browser realm.

When no real parent save bridge exists, the iframe must call its own `showSaveFilePicker` directly from the Generate button handler. The parent page must not wrap its own `showSaveFilePicker` as an iframe save bridge, because browsers require the file picker to be opened in the same user-activation chain as the clicked iframe button.

The controls intentionally follow Fluent UI interaction/layout conventions and are restyled to the MSA palette, but the implementation does not require a React runtime inside the MSA page. If the UI is later migrated to React + Fluent components, it must preserve the same exported globals, iframe mount/event-isolation contract, settings contract, renderer contract, and tests.

## State Model

`msaexpor` state is export UI state. It is not alignment state.

Opening, editing settings, previewing, or exporting must not:

- alter sequence content
- toggle MSA row selection
- alter groups/features
- call MSA Apply
- trigger tree recomputation
- update runtime artifacts

If `msaexpor` UI defaults are persisted later, they must be stored under a dedicated `msaexpor` viewer-state key and must remain separate from biological MSA state.

## Asset Strategy

The MSA page currently loads PHgo bridge assets directly from the vendored viewer assets. `msaexpor` must be bundled under the same local asset tree so it stays offline-capable.

## No Fallback Renderer

`msaexpor` must not include, call, or preserve any PHgo-owned residue/grid/group/feature renderer as an alternate output path. The only valid render function is:

```javascript
window.__PHGOJalviewBridgeAPI.renderMSAExportScene(settings, layout)
```

During this call the bridge must set `window.__PHGO_MSAEXPOR_RENDER_ACTIVE__ = true` on the Jalview page and restore its previous value in `finally`. Vendored Jalview `IdCanvas` and `SeqCanvas` patches use that flag as the authoritative export context. The flag is intentionally independent of iframe globals so the actual loaded SwingJS core bundle can suppress export-only UI consistently.

The export context has these hard effects:

- PHgo checkbox column width is `0`
- PHgo checkbox drawing is skipped even if row selection state exists
- current selection group fill/outline is skipped
- cursor and search-result highlights are skipped
- wrapped and unwrapped annotation drawing is skipped
- real Jalview alignment groups and feature overlays remain available and are controlled only by `showGroups` and `showFeatures`

This keeps the renderer native to Jalview while ensuring the output is the MSA board itself, not a screenshot of the interactive UI.

If that function is missing, throws, or cannot access Jalview's native drawing objects, preview and export must show an explicit error. They must not switch to a DOM screenshot, canvas screenshot, old Jalview visible-panel print path, generated PHgo SVG grid, browser download workaround, or any other silent substitute.

The same rule applies to the window shell: the export UI must be mounted inside a real Jalview/SwingJS child window. If the native child-window content host cannot be found, opening the export window fails instead of creating a PHgo-managed fake window.

Target asset shape:

```text
internal/phylo/viewer_assets/assets/msaexpor/
  index.js
  pdf.js
  style.css
```

The build path must use local repository or vendored dependencies. The implementation must not depend on a remote CDN or runtime download. `pdf.js` is built as a single browser-global IIFE from `tree-viewer/src/msaexpor-pdf.js` through `tree-viewer/vite.msaexpor.config.js`; it must expose `window.PHGOmsaexporPDF.exportPDFBlob` and must not require ESM module loading from the MSA bootstrap page.
