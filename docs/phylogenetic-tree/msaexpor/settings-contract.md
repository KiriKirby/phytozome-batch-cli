# `msaexpor` Settings Contract

## Formats

`msaexpor` supports the same output family as the tree viewer:

- `SVG`
- `PNG`
- `PDF`

SVG is the canonical container format returned by the Jalview-native render bridge. PNG and PDF are derived from the same generated scene so all formats share identical layout and content.

## Scale Choices

The size selector is a resolution scale, not a fixed long-edge size.

Allowed scale choices:

- `1x`
- `2x`
- `5x`
- `10x`

Default: `2x`.

`1x` is not a tiny UI pixel export. It is the baseline publication layout scale chosen by PHgo so residue text is readable. Higher scales multiply output resolution while keeping the same logical layout proportions.

Two exports with the same data, same cell dimensions, same font settings, and same scale must produce the same element sizes. Changing the amount of content changes the canvas width/height by adding or removing cells/rows, not by shrinking existing elements.

## Cell Geometry

`msaexpor` has separate numeric inputs for residue cell width and residue cell height.

Default geometry:

- `Cell width`: `9` logical px
- `Cell height`: `13` logical px
- residue font: `10` logical px `Consolas, Courier New, monospace`

These defaults define the `1x` logical layout. The default export scale is still `2x`, so PNG raster output uses twice the logical pixel resolution while preserving the same geometry.

The geometry contract is:

- every residue/gap occupies exactly one cell
- cell width controls horizontal advance
- cell height controls row advance
- labels, top column numbers, right residue numbers, groups, and features align to this grid
- changing cell size changes the whole figure layout predictably

Field names:

- `Cell width`
- `Cell height`

## Basic Boolean Options

Initial settings:

| Setting | Default | Notes |
| --- | --- | --- |
| `Show PHgo coordinates` | off | Shows the row's PHgo display prefix such as `[1,13]` in the left label area. |
| `Show length ratio` | off | Shows values such as `60/64`. |
| `Show length percent` | off | Shows values such as `93.8%`. |
| `Show alignment column numbers` | on | Shows top alignment-column numbers such as `20`, `40`, `60`, `80`. |
| `Show right residue numbers` | off | Shows each row's ungapped residue position at the end of the visible block, aligned at the right edge. |
| `Show groups` | on | Renders group-level highlighting or labels when group data is available. |
| `Show features` | on | Renders sequence features when feature data is available. |

The setting identifiers and first implementation labels above are fixed for the first implementation.

## Alignment Column Number Frequency

Field name: `Column number interval`.

Default: `20`.

The input is enabled only when `Show alignment column numbers` is enabled.

Rules:

- value must be a positive integer
- numbers refer to MSA alignment columns, not ungapped sequence positions
- numbers are rendered over the residue grid at positions where the MSA boundary coordinate at the right edge of the cell is divisible by the interval
- a block that does not contain any interval hit renders no top number for that block

## Advanced Grouping Switch

Field name: `Use advanced layout script`.

Default: off.

When off:

- all rows and all alignment columns are exported as one full-width major block
- the advanced layout text box is disabled
- `Show right residue numbers` is not changed by the off state

When on:

- the advanced layout text box is enabled
- `Show right residue numbers` is automatically enabled
- the user can still see that the right-residue-number setting is on because the advanced layout relies on it
- advanced layout lines fully determine which rows and MSA column spans are exported and how spans are split into major blocks

## Advanced Layout Text

The first implementation uses a large multiline text box.

Each non-empty line defines one or more exported major blocks. Lines are processed top to bottom.

Example:

```text
>1,4/1,3/1,6/1,5/1,8/1,7\10,100/~,~,~
>1,4/1,3/1,6\10,200/30,~,~
```

Use `>~\...` when the exported item set should be all current MSA rows.

The DSL is defined in [Group DSL](./group-dsl.md).

## Actions and File Naming

The window action buttons are:

- `Refresh preview`
- `Generate`
- `Cancel`

`Refresh preview` manually regenerates the in-window SVG preview from the current settings. Editing fields must not automatically rerender the preview; edits instead mark the preview with `Preview needs refresh` until the user refreshes it. The preview always uses the Jalview-native SVG container scene at preview scale `1x` in a scrollable browser region, independent of the selected final format or export scale.

`Generate` validates settings, regenerates the canonical Jalview-native SVG container scene, and opens the PHgo save bridge or browser/operating-system save-location picker. If neither save path is available, export fails with an in-window error.

`Cancel` closes the `msaexpor` child window without exporting and without changing MSA state.

Default export filename base:

- first choice: the leading numbered item parsed from the current payload title
- second choice: the leading numbered item parsed from the current page title
- final default: sanitized session id

Examples:

- `Phgomsar: 1.1 AT1G01010` exports by default as `1.1.svg`, `1.1.png`, or `1.1.pdf`.
- `Phgomsar: 1 MSA` exports by default as `1.svg`, `1.png`, or `1.pdf`.

## Automatic Layout When Advanced Grouping Is Off

The automatic layout exports:

- all current MSA rows in current PHgo/Jalview order, regardless of PHgo green/yellow/red selection state
- the full aligned width
- exactly one major block from boundary `0` to `alignmentWidth`

The automatic path must use the same renderer as the DSL path. It must synthesize an internal layout plan rather than using a separate renderer.
