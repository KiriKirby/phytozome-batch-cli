# Phylogenetic Tree System Architecture

## Purpose

The phylogenetic tree system lets users build and review trees from Canvas rows without leaving the Canvas workflow.

Canvas remains the place where rows are selected, renamed, and organized. The tree system adds a right-side tool panel for analysis and data-preparation controls, then delegates browser-facing tree display, interaction, and graphical export to an external viewer service built around Reactree.js.

## Ownership Model

### Canvas TUI

Canvas owns:

- selected tree input rows
- display-name editing
- the right-side tree tool panel
- focus and shortcut behavior
- `mega-phgo-runtime` parameter selection
- tree refresh requests
- session snapshot integration

Canvas does not own:

- SVG tree rendering
- PNG/PDF rendering
- browser-side tree interaction
- Reactree.js state internals
- layout controls
- search controls
- node editing controls
- export controls

### Tree Analysis Runner

The tree analysis runner owns:

- writing stable runtime input FASTA
- writing `runtime-request.json`
- launching `mega-phgo-runtime`
- collecting `runtime-response.json`, aligned FASTA, Newick, and logs
- deciding whether alignment/tree computation can be reused for a refresh

The runner must use `mega-phgo-runtime` only for alignment and tree inference. It must not call MEGA-CC or generate `.mao` files as the primary protocol.

Runtime discovery is intentionally strict. PHgo must only use the application-local `mega-phgo-runtime` folder next to the running application. It must not search `PATH`, use an already installed MEGA/MEGA-CC runtime, or accept a system-managed runtime location. If that folder, its `mega-phgo-runtime` executable, or its runtime-owned MUSCLE binary is missing, Canvas should offer the PHgo release asset download/extract flow and populate that exact folder.

The runtime check happens when the user expands the tree tool panel and again before a refresh launches computation. Opening the panel therefore confirms that the local runtime is available, starts/reuses the viewer service, and opens the browser, but it still does not run alignment or tree inference until the user explicitly chooses `Refresh tree`.

### Tree Viewer Service

The tree viewer service owns:

- serving the local browser page
- loading `tree.nwk`, aligned FASTA, metadata, and preview configuration
- rendering the tree through Reactree.js
- live-refreshing the browser view when Canvas pushes new tree data
- exposing Reactree.js' built-in layout, search, editing, and export controls
- preserving viewer-side style and interaction state when needed

The viewer service may be distributed as a bundled Node/TypeScript application or as another bundled local process, but it must remain a separate subsystem with an explicit API.

## Process Model

The first implementation should use three logical processes:

```text
phytozome-go TUI
  -> mega-phgo-runtime subprocess
  -> phgo-tree-viewer local service
       -> user browser
```

The TUI starts the tree viewer service when the tree tool panel is opened. It opens the browser to the viewer page every time the panel is expanded. The viewer page may initially be empty.

`mega-phgo-runtime` is launched only after the user chooses `Refresh tree`.

## Lifecycle

1. User opens Canvas.
2. User clicks `Show tree tools` or presses the matching shortcut.
3. Canvas opens a fixed-width right-side tree tool panel and preserves all panel settings.
4. Canvas verifies the application-local `mega-phgo-runtime` folder, prompting to download/extract it if missing.
5. Canvas starts or reuses the local viewer service.
6. Canvas opens the browser to the viewer preview URL.
7. Viewer shows an empty preview until the first refresh.
8. User selects rows and edits display names.
9. User activates `Refresh tree`.
10. Canvas rechecks the runtime folder, then builds input artifacts from selected rows and display names.
11. Runner decides whether runtime alignment/tree computation is required.
12. Runner writes or reuses alignment/tree artifacts.
13. Canvas pushes the latest payload to the viewer service.
14. Viewer updates the browser automatically when possible.
14. Viewer-side export stays inside the Reactree page.

## Non-Goals

- Do not render tree diagrams inside the terminal.
- Do not embed MEGA GUI widgets or source-code components.
- Do not make Reactree.js computation-aware; it receives tree artifacts, not raw unaligned workflow state.
- Do not preserve the old Canvas source-row column in the tree workflow.

## Snapshot Rule

Tree snapshots must preserve generated artifacts and settings instead of planning to recompute them.

The snapshot contract must include:

- selected Canvas row identity at refresh time
- display-name values used at refresh time
- runtime alignment method and parameter values
- runtime tree method and parameter values
- input FASTA
- aligned output
- Newick tree output
- viewer page state
