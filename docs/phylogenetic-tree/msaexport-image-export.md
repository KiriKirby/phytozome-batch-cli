# MSA Image Export Rewrite (`msaexpor`)

## Purpose

This document defines the PHgo rewrite of MSA image export inside the JalviewJS-based MSA page. The detailed implementation-level document library lives under [`msaexpor/`](./msaexpor/README.md).

The rewrite completely replaces the current Jalview image-export path exposed from the MSA `File` menu. The replacement module is named `msaexpor`.

`msaexpor` becomes the sole PHgo-owned image-export surface for the MSA page.

The rewrite is renderer-driven through Jalview's native SwingJS drawing pipeline. It must not export by taking a screenshot of the current MSA page, current SwingJS canvas, current child window, or current Jalview `AlignmentPanel`, and it must not keep a PHgo-owned MSA grid renderer as a substitute output path.

## Product Decision

- The current MSA `File` menu entry `Export image` is renamed to `Export image...`.
- Activating `Export image...` no longer opens Jalview's built-in simple format chooser/export path.
- Instead, it opens a PHgo-owned export settings window implemented by PHgo and backed by the `msaexpor` module.
- The export settings window opens inside the existing `/sessions/<session>/msa` page.
- The export settings window must use the existing Jalview/SwingJS internal-window registration and window-manager path so it behaves like other Jalview child windows.
- The content rendered inside that child window is PHgo-owned Fluent-style browser UI content, not a Jalview-native Swing form.
- `msaexpor` fully takes over MSA image export settings UI, preview logic, and final file writing for this feature area.
- `msaexpor` asks the PHgo Jalview bridge to generate a clean offscreen scene through Jalview-native rendering, saves that scene as the canonical SVG container, then derives PNG and PDF from that same scene.
- Export output must not include temporary UI elements such as the current selection rectangle, PHgo row checkboxes, menus, scrollbars, child-window chrome, or annotations.

## Why This Shape

PHgo needs a richer export workflow than Jalview's current image-export surface. The target workflow needs detailed control over row scope, residue ranges, line wrapping, layout, styling, and publication-oriented output.

Using the existing Jalview child-window framework is the right outer shell because it already provides:

- in-page window layering
- drag behavior
- resize behavior
- focus ordering
- coexistence with the live alignment page underneath
- consistency with the rest of the MSA page's utility windows

Using a PHgo-owned Fluent-style browser UI inside the child window is the right inner shell because PHgo needs a substantially richer, easier-to-extend form and preview surface than SwingJS-era controls provide. The current implementation does not require React to run inside the MSA page; a future React + Fluent rewrite must keep the same child-window, settings, Jalview-native renderer, and export contracts.

## Ownership Boundary

### Jalview/SwingJS owns

- menu entry placement inside the MSA `File` menu
- child-window registration with the existing Jalview desktop/window-manager system
- draggable/resizable/window-layer behavior
- keeping the alignment page active behind the export window
- normal Jalview utility-window coexistence

### `msaexpor` owns

- all export-image UI shown after `Export image...` is activated
- all export settings state for this workflow
- PHgo-specific preview orchestration for the Jalview-native export scene
- all final SVG/PNG/PDF export file generation for this workflow
- validation of export parameters
- mapping current MSA state into export-ready input

### PHgo bridge owns

- launching `msaexpor` from the Jalview `File` menu
- passing session identity and current MSA payload/state into `msaexpor`
- wiring save/apply-neutral export actions into the page without mutating the alignment

## User Experience Contract

### Entry

- The visible MSA menu text is `Export image...`.
- The trailing ellipsis is required because activation opens a configuration window instead of immediately writing a file.
- PHgo removes or bypasses the former Jalview path that asked only for an image/export format.

### Window Behavior

- The export window is a child window inside the current MSA page, not a browser popup, not a new tab, and not a PHgo page replacement.
- The export window must participate in the same internal-window stack as other Jalview child windows.
- The export window must be draggable.
- The export window must be resizable.
- The export window must allow interaction with the underlying MSA content when the Jalview window model allows that class of child window to coexist non-modally.
- The export window must look consistent with the existing MSA child-window environment.

### Content Behavior

- Window content is fully PHgo-owned Fluent-style UI.
- Window content is hosted inside a same-origin iframe within the SwingJS child window so browser-native form controls remain interactable.
- The control set is richer than Jalview's historic image-export UI.
- The visual language follows Fluent UI conventions and is theme-adjusted to blend into the current MSA environment.

## Visual Design Contract

`msaexpor` uses PHgo-owned Fluent-style controls, and the PHgo implementation must restyle them so the result feels native to the current MSA page instead of looking like a detached generic web dialog.

Required style direction:

- color tokens are adapted toward the current Jalview/MSA child-window palette
- corner radius is reduced or overridden to match the existing MSA window style more closely
- spacing and typography remain readable and do not visually fight the surrounding Jalview UI
- the export window looks like one coherent MSA utility window, not like a foreign application embedded inside Jalview

This is a PHgo-owned theme adaptation layer. If the UI is later migrated to actual React + Fluent components, those components must inherit the same MSA palette, compact corner radius, and export behavior.

## Functional Scope

The first rewritten `msaexpor` implementation includes exactly these setting groups:

- format selection: SVG, PNG, PDF
- resolution scale controls: 1x, 2x, 5x, 10x, default 2x
- cell width and cell height controls, default `9` by `13` logical px
- optional PHgo coordinate display, default off
- optional length ratio display, default off
- optional length percent display, default off
- alignment column numbering, default on, interval default 20
- optional right-side per-row residue end numbers, default off
- group rendering, default on
- feature rendering, default on
- advanced layout script for PHgo row selection, MSA boundary ranges, and per-block allocation
- `>~\range[/allocation]` in the advanced layout script means all current MSA rows
- automatic layout when the advanced layout script is off: all exportable rows, full aligned width, one full-width major block
- in-window preview of the same Jalview-native SVG container scene used for final export, refreshed manually through `Refresh preview`
- action buttons `Generate` and `Cancel`; `Generate` opens the save-location picker when available
- default export filename base is the numbered prefix from the payload/page title, such as `1` or `1.1`

New controls must be added by extending the `msaexpor` settings contract and tests first; the first implementation is not allowed to add undocumented export behavior.

## Integration Contract

### Trigger path

1. User opens `/sessions/<session>/msa`.
2. User activates `File -> Export image...`.
3. PHgo intercepts the PHgo-owned export path.
4. PHgo asks the Jalview desktop/window-manager layer to open or focus the `msaexpor` child window.
5. The child window surface appears inside the existing MSA page.
6. The child window content mounts PHgo Fluent-style controls and preview components.

### Data sources

`msaexpor` must read from the existing PHgo MSA/session data surfaces instead of scraping rendered pixels.

Primary sources include:

- current session id
- current aligned FASTA or aligned sequence model
- current row state and selected-row state
- display names and display prefixes
- sequence descriptions when needed
- PHgo/Jalview state already captured by the bridge
- current page-level metadata needed for styling or export naming
- current live Jalview/MSA renderer state needed to match residue, group, feature, label, and font styling

Implementation-level details for settings, DSL parsing, rendering, and tests are defined in [`docs/phylogenetic-tree/msaexpor/`](./msaexpor/README.md).

### Mutation rules

- Opening or using `msaexpor` must not modify sequence content, row state, group state, or tree artifacts unless the user explicitly performs an export action that writes only export files.
- Export configuration state is viewer/UI state, not biological alignment state.
- Exporting an image must not behave like `Apply`.
- Exporting an image must not trigger tree recomputation.

## Replacement Rule

PHgo treats the MSA image-export rewrite as a full replacement, not as a second parallel export path.

That means:

- users must not be routed through Jalview's old simple image-export chooser from the PHgo MSA page
- `msaexpor` is the only supported image-export entry under PHgo's MSA workflow
- if Jalview internals still contain legacy image-export code, PHgo must bypass it for this menu item instead of trying to keep both workflows visible

## Implementation Guidance

### Required outer-shell strategy

- reuse the vendored Jalview internal-frame/window registration path
- let Jalview keep window movement, resize affordances, z-order, and child-window lifecycle
- mount a PHgo-owned DOM host inside the child window content area
- render PHgo-owned Fluent-style content into that host

### Required inner-shell strategy

- use PHgo-owned Fluent-style browser controls for the settings form and live preview content
- keep PHgo-specific state management outside Jalview's historical form models
- keep export orchestration and file generation PHgo-owned while the actual MSA scene rendering stays Jalview-native

### Avoid

- do not keep Jalview's built-in export-image chooser as the visible first step
- do not build the detailed export settings form out of SwingJS-native controls
- do not replace the whole `/sessions/<session>/msa` page with a PHgo-only MSA renderer
- do not open the export workflow in a browser-native popup or separate page
- do not keep or call a PHgo-owned self-rendered MSA SVG grid as a substitute for the Jalview-native export bridge

## Non-goals

- replacing Jalview as the main MSA renderer
- moving export image to the tree viewer
- making the export window a top-level browser window
- introducing PHgo page chrome around the MSA page
- forcing the export settings UI to look like stock Fluent web app chrome

## Naming

- module internal name: `msaexpor`
- visible menu label: `Export image...`
- visible user-facing feature concept: PHgo MSA image export

## Test Expectations

Implementation must be validated against these behaviors:

- `File -> Export image...` opens the PHgo export window instead of the legacy Jalview export path
- the export window is registered as a Jalview child window inside the MSA page
- the window is draggable
- the window is resizable
- the window stacks correctly with other MSA child windows
- the content inside the window is PHgo-owned Fluent-style UI
- PHgo styling overrides make the window visually consistent with the current MSA child-window style
- export actions do not mutate alignment content or trigger `Apply`
- export actions do not trigger tree recomputation
