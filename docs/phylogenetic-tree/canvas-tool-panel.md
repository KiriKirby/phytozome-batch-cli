# Canvas Tree Tool Panel

## Purpose

The Canvas tree tool panel is a fixed-width right-side panel that appears inside the existing Canvas page. It contains the data-preparation controls that belong to tree analysis: runtime alignment method, runtime alignment/tree parameters, and display-name source selection.

The panel is collapsed by default.

## Toggle Button

Canvas adds a right-side button:

- collapsed state: `Show tree tools`
- expanded state: `Hide tree tools`
- suggested shortcut: `Ctrl+T`

The button belongs with the right-side action buttons. It should sit to the left of the current rightmost primary action buttons so the normal Canvas flow remains visible.

Opening the panel must preserve any existing Canvas state. Closing and reopening the panel must preserve all tree settings from the previous expanded state.

Each time the panel is opened, Canvas first verifies the strict application-local `mega-phgo-runtime` folder. In the supported Windows `amd64` release, that folder must already be bundled at `<application-dir>/mega-phgo-runtime`. If the custom runtime executable or runtime-owned MUSCLE binary is missing, Canvas reports that the bundle is incomplete and does not try to download runtime pieces. Canvas does not search installed MEGA folders or `PATH`.

After the runtime check succeeds, Canvas starts or reuses the tree viewer service and opens the browser page. The viewer page remains empty until the first explicit refresh.

## Panel Layout

The right panel has a fixed width. The initial target width should be large enough for parameter labels and dropdowns without truncation, then refined after the first implementation pass.

Implemented starting width:

```text
50 terminal columns
```

Canvas keeps at least 72 columns for the left-side list/table area. If the terminal is narrower than 122 columns, `Ctrl+T`/`Show tree tools` refuses to open the panel and shows a clear modal asking the user to widen the terminal. This prevents the fixed right panel from crushing the editable Canvas table.

The Canvas list/table content layout must be rebuilt on every normal page refresh, not only when the tree panel is toggled. This keeps the left list and right table visible after creating the first Canvas item from an empty Canvas, after deleting the last row, and after switching between empty/non-empty Canvas tables.

Panel pages are ordered so the data-mode decision comes first:

- page 1: `Target mode`
- page 2: `Align settings`
- page 3: `Tree settings`

The default focused page is page 1. Target defaults to Protein mode. When MEGA/runtime reports skipped or failed rows, the recovery dialog's Skip action always unchecks the reported rows and immediately retries the refresh with the remaining selection.

Each module is drawn as a normal boxed TUI region with its own title, matching the visual language used by the existing complex parameter dialogs such as filter and family settings. Yellow is used only for focus and active accents; the panel must not become a custom dashboard or a full yellow block.

Inside the module, group divider rows separate method selection, runtime parameter groups, inference settings, and display-name-source settings. Parameter section rows from the PHgo runtime registry, such as ClustalW pairwise/multiple/global options, must be rendered instead of silently filtered out.

Controls are rendered like standard TUI form fields:

- dropdown-backed finite choices display as `Label <value>`
- binary choices display as checkbox-style `[x]` or `[ ]`
- numeric and text-backed runtime fields display as `Label [value]`
- read-only or section registry rows display as group dividers rather than editable fields

If content is too long for the fixed right panel, the panel is split into numbered pages like the complex filter parameter modal. Page numbers are shown at the bottom of the right panel, and selecting a page moves focus to the first module on that page. The panel should not force Canvas rows or the sidebar into unusable widths; below a minimum terminal width, the tree panel should refuse to open with a clear modal.

## Focus Model

The tree panel is a separate focus region.

Left-side Canvas focus regions remain:

- Canvas list/sidebar
- Canvas table
- Canvas table header

Inside the left-side regions, `Tab` keeps its current behavior and cycles among the Canvas regions.

The tree panel uses a separate shortcut to switch between the left Canvas regions and the right tree panel.

Suggested focus shortcut:

```text
Ctrl+Y
```

This shortcut has no visible button. It appears only in bottom hint text while the tree panel is expanded.

When focus is in the tree panel, `Tab` cycles between tree panel modules. `Up` and `Down` move within the active module. `Enter` advances to the next module; from the final module it triggers `Refresh tree`. `PageUp` and `PageDown` switch tree panel pages when the right panel content is paginated. When focus returns to the left Canvas region, `Tab` resumes the normal Canvas cycle.

Mouse focus follows the clicked region. Clicking inside the tree panel activates the tree panel focus mode. Clicking the Canvas list, table, or table header returns focus to the left-side mode.

## Color Model

The tree panel uses a yellow accent family instead of the normal focused blue family.

Every control that would normally use the focused blue accent inside the panel must use the tree yellow accent:

- focused borders
- focused titles
- selected dropdown rows
- active checkbox highlights
- active input highlights
- primary action accent while panel focus is active

The panel must remain readable in the existing conservative TUI color constraints. Yellow is an accent, not a full background.

## Button Row Behavior

When the tree panel is not focused, Canvas keeps its normal button row.

When the tree panel is focused, the left-side workflow buttons are hidden except global/system buttons such as Back and Home. The panel-focused button row adds:

- `Refresh tree`

Suggested shortcut:

```text
Ctrl+R
```

`Refresh tree` is available only while the tree panel is expanded. It should be visible only when tree-panel focus is active.

The refresh action has a compute phase and a render phase as described in [PHgo Runtime Pipeline](./phgo-runtime-pipeline.md), then pushes the latest tree payload to the viewer service.

## Target And Alignment Controls

The first page chooses the target data mode before alignment:

- Protein mode: draw the tree from amino-acid sequences. This is the default lab workflow.
- DNA mode: draw the tree from nucleotide sequences. Protein rows are used as DNA only when a real nucleotide/CDS sequence is embedded in the row or resolved from source metadata; PHgo never reverse-translates protein into DNA.

MEGA 12.1 GUI gates DNA, protein, and codon actions by the active data type. PHgo mirrors that behavior through the target-mode control and MEGA-backed runtime request. PHgo does not locally infer, convert, repair, or skip biological sequence content before runtime execution.

The alignment method control is a dropdown at the top of the panel.

Supported alignment methods mirror the target mode:

- Protein mode: `ClustalW` and `MUSCLE`, both amino-acid alignments.
- DNA mode: `ClustalW (DNA)` and `MUSCLE (DNA)`, base-level nucleotide alignments.
- DNA mode: `ClustalW (Codons)` and `MUSCLE (Codons)`, CDS/codon alignments for coding nucleotide input.

After an alignment method is selected, the parameter controls below it must change to match that method.

Parameter names, default values, allowed values, applicability, and help text come from the runtime parameter registry in the program. The registry is derived from the MEGA 12.1 source tree and normalized into PHgo-owned parameter IDs.

The panel edits the parameter values directly. When the user clicks `Refresh tree`, the current values are written into `runtime-request.json`. If a parameter is not exposed in the panel, the runtime request uses the registry default rather than a hidden hand-written value.

The panel exposes alignment methods compatible with the current target mode, not merely the raw selected row mix. The refresh pipeline repeats the compatibility check before launching `mega-phgo-runtime`, so restored snapshots or stale panel states cannot force an incompatible runtime request.

Recommended TUI controls:

- dropdowns for finite option sets
- checkboxes for binary settings
- numeric input fields for penalties, iterations, and cutoffs
- read-only text rows for generated or fixed values
- section dividers for long parameter groups

Current implemented controls use the shared TUI field/editing primitives: dropdown-like controls open with Space, move with Up/Down, and confirm with Space or Enter; numeric/text parameters open the standard text edit modal until dedicated inline numeric widgets are added.

## Display-Name Source

Canvas removes the old source-row column from the table and from import logic. The tree system must not read, preserve, or offer that source-row value.

Canvas adds a final editable column:

```text
display_name
```

User-facing label:

```text
display name
```

The display-name source dropdown lists Canvas table columns except:

- checkbox
- row number
- removed source-row column
- display_name itself

Default display-name source:

```text
label_name
```

When the dropdown changes, Canvas immediately applies it:

1. Read the selected source column for every Canvas row.
2. If the source column value is not empty, copy it into `display_name`.
3. If the source column value is empty, copy the original FASTA head/header into `display_name`.
4. Preserve the user's ability to manually edit individual `display_name` cells afterward.

The tree viewer must use `display_name` as the base leaf label. It must not choose labels independently from Newick leaf names, and it must not append hidden duplicate-disambiguation suffixes such as `[PHGOT000123]`.

## PHgo Row Coordinates

The Tree settings page includes a checkbox directly under the `Show column` dropdown:

```text
Show PHgo row coordinates
```

This option is on by default. When enabled, the tree display label is:

```text
[canvas item number,row number within item] display_name
```

Example:

```text
[1,13] Aco018382.1
```

The coordinate prefix is display-only. For the tree viewer it is applied to the rendered display label. For the MSA viewer it is sent as separate `display_prefix` metadata and drawn only in Jalview's left sequence list; Jalview's real sequence name, right-click menu text, rename dialog value, and Apply name edit remain the raw base `display_name`. The PHgo MSA left sequence list always draws this text left-aligned immediately after the PHgo checkbox column, regardless of Jalview's right-align-ID setting, and it uses the raw sequence name instead of Jalview's sequence-limit display ID so generated suffixes such as `/1-564` are not shown. Turning the option off clears the prefix metadata and does not modify Canvas cells.

The Canvas left sidebar always prefixes each item title with `[item number]` independently of this option, so Canvas items remain identifiable even when the tree/MSA coordinate prefix is disabled.

## Manual Display-Name Edits

The existing Canvas rename/alias editing pattern should be reused for the `display_name` column.

Manual edits are authoritative until the user changes the display-name source dropdown again. Changing the dropdown reapplies the bulk copy rule and may overwrite manual display-name edits.

## Tree Method

Tree inference method selection belongs in the TUI because it changes computation, not only display. It sits in the same panel as the alignment controls and remains a computation control rather than a Reactree control.

The current implementation exposes the MEGA-runtime-backed tree methods that match the selected target mode and registry definitions: Neighbor-Joining, Minimum Evolution, UPGMA, Maximum Likelihood, and Maximum Parsimony.

## Initial Empty Preview

Opening the tree panel starts the viewer and opens the browser, but does not compute or render a tree.

The viewer should show an empty state for the current Canvas session until the first `Refresh tree`.

## Refresh And Relabeling

Changing the display-name source dropdown, manually editing the `display_name` column, or toggling `Show PHgo row coordinates` is a preview-only change. It updates the Canvas table and/or viewer payload metadata but must not rerun `mega-phgo-runtime` when the selected rows, sequence content, target mode, alignment method/parameters, and tree method/parameters are unchanged.

Changing the selected rows, sequence content, target mode, skipped-row unselect behavior, alignment method/parameters, or tree method/parameters is a computation change and requires a runtime refresh.

The first refresh in a new live Canvas session is always compute plus render. When a Canvas `.pgo` snapshot has just been opened, the restored payload may be displayed immediately, but the first user-triggered `Refresh tree` must still run a full `mega-phgo-runtime` compute pass before rendering.
