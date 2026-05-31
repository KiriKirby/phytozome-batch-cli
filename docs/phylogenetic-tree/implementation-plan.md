# Phylogenetic Tree Implementation Plan

## Current Implementation Status

The current implementation has the main system-tree loop wired end to end:

- Canvas owns row selection, `display_name`, tree panel state, runtime settings, refresh, and snapshot persistence.
- `mega-phgo-runtime` is the only compute backend and is reached through `runtime-request.json`; MEGA-CC and `.mao` are not used in the PHgo workflow.
- Runtime availability is strict: both the executable probe and runtime-owned MUSCLE must pass from the application-local `mega-phgo-runtime` folder. Explicit runtime executable paths cannot bypass the same check.
- The Reactree viewer is a bundled Vite/React app served by the Go viewer service with empty initial state, payload endpoints, preview endpoint, and SSE live updates.
- Reactree labels are sanitized and deduplicated at the viewer boundary, with one deterministic label map shared by Newick and aligned FASTA.
- Canvas `.pgo` snapshots preserve tree panel state, last payload, run manifest, core tree artifacts, fingerprints, and restored in-memory tree plan state.
- Canvas Add canvas accepts `.pgo` snapshots as new Canvas items; Add rows rejects `.pgo` files and directs users to Add canvas or `Explore -> Open session`.

## Phase 1: Canvas Table Contract

- Remove the Canvas source-row column from display, import, metadata propagation, and tree-related options.
- Add `display_name` as the final Canvas table column.
- Reuse the existing Canvas rename/alias editing pattern for `display_name`.
- Add bulk display-name source application with fallback to original FASTA head/header.
- Add tests for display-name source application and source-row removal.

## Phase 2: Tree Panel Shell

- Add `Show tree tools` / `Hide tree tools` to Canvas.
- Add fixed-width right-side tree panel.
- Add tree-panel focus region and hidden focus shortcut hint.
- Add yellow accent styling for panel focus and controls.
- Add panel state preservation across collapse/expand.
- Start or reuse the viewer service when the panel opens.
- Open the browser page when the panel opens.

## Phase 3: Tree Panel Controls

- Add alignment method dropdown.
- Add method-specific runtime parameter panels.
- Add display-name source dropdown.
- Keep preview/layout/search/edit/export controls inside the Reactree page.
- Add tests for state round-trip and bulk display-name application.

## Phase 4: PHgo Runtime Runner

- Add `mega-phgo-runtime` executable and runtime-owned MUSCLE validation against the application-local `mega-phgo-runtime` folder only.
- Do not search `PATH` and do not use already installed MEGA/MEGA-CC folders.
- Bundle the full Windows `amd64` `mega-phgo-runtime` folder directly inside the release package root and validate it in place at runtime.
- Do not add runtime marker files or marker-based freshness checks.
- Keep current bundled runtime support Windows amd64 only. Linux/macOS must report unsupported until real PHgo runtime builds are explicitly reintroduced.
- Add runtime parameter registry for alignment and tree settings.
- Generate `runtime-request.json` from the current panel settings.
- Write stable-ID FASTA and metadata.
- Run runtime alignment.
- Run runtime tree inference.
- Read `runtime-response.json`.
- Parse or locate Newick output.
- Build run manifests and fingerprints.
- Add recompute/reuse logic.
- Add tests around fingerprints and command preparation.

## Phase 5: Reactree Viewer Service

- Create standalone viewer package under `tree-viewer`.
- Add local service health endpoint.
- Add session page route.
- Add empty initial preview.
- Embed the built Vite/React `reactreejs` app in the Go viewer service.
- Add payload and page-state update endpoints.
- Add live update through Server-Sent Events or WebSocket.
- Rely on Reactree's built-in layout, editing, search, and export UI as the primary viewer path.
- Add integration test with a small static Newick payload.

## Phase 6: Refresh Workflow

- Add panel-focused `Refresh tree` button and shortcut.
- Hide normal non-system Canvas buttons while tree panel is focused.
- On refresh, collect selected sequence-ready rows in visible order.
- Split refresh into compute and render phases.
- Recompute runtime artifacts on the first refresh, on the first refresh after opening a Canvas snapshot, when selected rows change, or when any non-label right-panel setting changes.
- Rerender only when the only change is `display_name` or display-name source.
- Push viewer updates without requiring browser refresh.
- Add progress and cancellation handling.

## Phase 7: Snapshot Integration

- Store tree panel state.
- Store latest display-name values.
- Store latest tree run manifest.
- Store all generated tree artifacts needed for reopening.
- Restore Canvas with tree state available.
- Reopen the viewer without recomputing when artifacts exist, but force a full compute pass on the first user-triggered refresh after snapshot open.

## Testing Priorities

- Canvas table schema excludes source-row and includes editable `display_name`.
- Display-name source dropdown never includes `display_name` or removed source-row.
- Changing selected rows triggers recomputation.
- Changing conversion, alignment, or tree parameters triggers recomputation.
- Changing display names or display-name source after a successful compute does not trigger the runtime.
- Viewer receives stable taxon IDs and metadata mapping.
- Viewer maps duplicate/sanitized display labels into unique Reactree-facing names and applies the same names to Newick and FASTA.
- Explicit and default runtime discovery both require the app-local PHgo runtime executable plus the runtime-owned MUSCLE binary.
- Session restore does not rerun the runtime by default, but the first explicit tree refresh after restore does.

Current validation commands:

```text
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build-tree-viewer.ps1
go test ./...
go vet ./...
go build ./...
```
