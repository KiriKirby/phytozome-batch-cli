# `msaexpor` Test Plan

## Asset Contract Tests

Go asset tests cover the embedded viewer assets and vendored JalviewJS menu route.

Required checks:

- `jalview-bootstrap.html` loads the `msaexpor` asset or exposes the mount path needed by it
- `phgo-bridge.js` exposes the `msaexpor` launch bridge
- `phgo-bridge.js` writes `/assets/msaexpor/pdf.js` and `/assets/msaexpor/index.js` into the same-origin iframe document
- `phgo-bridge.js` waits for `iframe.contentWindow.PHGOmsaexpor.renderApp` before mounting the UI
- `phgo-bridge.js` injects `__PHGOJalviewBridgeAPI`, `__PHGO_SAVE_BLOB__`, and `__PHGO_MSAEXPOR_PARENT_WINDOW__` into the iframe
- `phgo-bridge.js` does not call parent-window `window.PHGOmsaexpor.renderApp` against iframe DOM
- vendored JalviewJS menu code contains `Export image...`
- vendored JalviewJS menu code routes PHgo MSA image export to the bridge instead of the legacy `ImageExporter` path
- forbidden old visible route strings are absent from the PHgo MSA export menu path

## Parser Unit Tests

Test the advanced DSL parser with:

- `>1,4/1,3\10,100`
- `>~\10,100/~,~,~`
- `>1,4/1,3\10,100/30,20,40`
- `>1,4/1,3\10,100/~,~,~`
- `>1,4/1,3\10,100/30,~,~`
- `>1,4/1,3\10,100/~,30,~`
- `>1,4/1,3\10,100/30,~,30`
- `>1,4/1,3\10,~`
- `>1,4/1,3\~,10`
- `>1,4/1,3\~,~`
- multiple non-empty lines
- blank lines
- internal token spaces are rejected
- comment syntax is rejected rather than ignored
- malformed missing `>`
- malformed missing `\`
- malformed multiple `\`
- malformed multiple allocation separators
- invalid range order
- range endpoint outside `[0, alignmentWidth]`
- duplicate PHgo row coordinate inside one DSL line
- repeated PHgo row coordinate across different DSL lines
- fixed allocation sum too small/large without `~`
- placeholder remainder that cannot divide evenly
- allocation `~` with zero remaining columns
- unresolved PHgo row coordinate
- `~` mixed with explicit PHgo row coordinates is rejected

## Layout Tests

Test layout planning with a small aligned FASTA:

- `>~\...` includes every current MSA row in normalized order
- model rows can be built from FASTA/state data even when `metadata.records` is unavailable
- row order follows DSL order
- automatic layout includes all exportable rows and full alignment width
- automatic layout produces exactly one major block when advanced layout is off
- half-open boundary ranges produce `end - start` columns
- `10,100` produces 90 columns
- `10,~` resolves to `[10, alignmentWidth)`
- `~,10` resolves to `[0,10)`
- `~,~` resolves to `[0, alignmentWidth)`
- `10,100/30,20,40` creates `[10,40)`, `[40,60)`, and `[60,100)`
- `10,100/~,30,~` creates `[10,40)`, `[40,70)`, and `[70,100)`
- unequal allocation widths align right-number columns to the longest block
- multiple DSL lines enter one uniform block flow

## Residue Number Tests

Test right-side residue number calculation:

- gaps do not increment residue position
- blanks do not increment residue position
- rows with different gap patterns produce different right-side numbers
- numbers are computed at each block end
- terminal stop marker `*` is ignored only when it is the final non-gap, non-whitespace character
- internal `*` is counted
- top alignment column labels remain MSA column numbers, not residue numbers

## Rendering Tests

Use bridge-contract tests and deterministic fake bridge outputs:

- `renderScene` calls `bridge.renderMSAExportScene(settings, layout)` exactly
- `renderScene` returns the bridge scene without invoking any PHgo-owned SVG renderer
- missing bridge fails with `MSA export requires the Jalview native render bridge`
- empty rows fail with `No MSA rows to export`
- zero aligned width fails with `No aligned MSA columns to export`
- bridge output exists and has expected width/height
- bridge SVG contains `data-msaexpor-renderer="jalview-vector"`
- bridge SVG contains vector residue/label elements and does not contain `<image href=` or `data:image/png`
- SVG and PDF export ignore Scale; PNG export alone uses Scale
- SVG/PDF output scenes have no page/background rectangle
- annotation text/panels are not emitted by the native export context
- checkbox glyphs or PHgo checkbox styling are not emitted by the native export context
- current selection/cursor markers are not emitted by the native export context
- `renderMSAExportScene` sets and restores `window.__PHGO_MSAEXPOR_RENDER_ACTIVE__`
- all vendored Jalview `IdCanvas` bundles return checkbox width `0` and no-op checkbox drawing while export is active
- all vendored Jalview `SeqCanvas` bundles suppress search highlights, cursor drawing, selection outlines, and annotation drawing while export is active
- export group outlines are based only on real alignment groups, not the current selection group
- group drawing is disabled when `showGroups` is off
- feature drawing is disabled when `showFeatures` is off
- automatic layout with no advanced script sends exactly one full-width block to the bridge

## Browser Tests

Use the in-app/browser path for release smoke tests and after changes to the bridge, SwingJS child-window registration, or save/export UI.

Required behavior:

- MSA page opens normally
- `File -> Export image...` opens a child window inside the MSA page
- the child window renders `Generate` and `Cancel` actions
- the child window is draggable
- the child window is resizable
- underlying MSA remains interactive when the child-window class is non-modal
- PHgo-owned Fluent-style content renders inside the child window
- input, select, textarea, checkbox, and button controls are interactable inside the same-origin iframe
- the iframe owns the `msaexpor` JavaScript runtime; Refresh, Generate, PDF conversion, PNG conversion, and Cancel must use iframe-local DOM APIs plus explicit parent handles
- the parent page owns Jalview native rendering and file-save delegation; the iframe must not use an implicit parent-window global except through the injected handles
- preview does not rerender automatically when settings change
- changing settings shows `Preview needs refresh`
- `Refresh preview` updates the SVG preview
- the preview region exposes horizontal and vertical scrolling when content exceeds the viewport
- `Generate` opens the PHgo save bridge or save picker when available
- `Generate` is disabled while an export is active to prevent duplicate file writes
- the save picker suggested filename defaults to the numbered page/payload title prefix, such as `1.1.svg`
- canceling the save picker reports `Export canceled.` without an error state
- missing save bridge and missing save picker report an in-window error instead of triggering browser download
- `Cancel` closes the child window without exporting
- exporting SVG writes a valid SVG
- exported SVG is vector and transparent, not a raster image embedded in SVG
- exporting PNG writes a non-empty PNG
- exporting PDF writes a non-empty PDF
- exported PDF is produced from the vector SVG path, not from a rasterized PNG scene
- invalid generated SVG causes PDF export to fail with a clear SVG/XML validation error

Automated browser-harness checks should additionally verify:

- `>~\10,100/~,~,~` updates the preview to three blocks
- `Generate` calls `showSaveFilePicker` with a numbered filename such as `1.1.svg`
- saved SVG blobs are non-empty and have `image/svg+xml` type
- `Cancel` closes the native SwingJS child window
- left labels are left-aligned and the grid starts after the longest exported left label plus a safety gap; no fixed-width cap may clip long labels
- opening another SwingJS child window does not leave the export iframe blank; the bridge remount watcher restores the iframe runtime if SwingJS replaces the host DOM

## Non-Mutation Tests

Verify export actions do not:

- call `/msa/apply`
- change `/msa/selection`
- change sequence content
- trigger tree recomputation
- alter runtime artifacts
