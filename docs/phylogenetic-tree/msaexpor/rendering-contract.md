# `msaexpor` Rendering Contract

## Rendering Principle

`msaexpor` renders a clean MSA export scene through Jalview's native SwingJS drawing code, not screenshots and not a PHgo reimplementation of the MSA renderer.

The renderer input is:

- parsed aligned sequences
- PHgo row metadata
- resolved layout blocks
- explicit export settings
- current Jalview alignment viewport, ID canvas, sequence canvas, groups, features, fonts, colours, and render settings accessed by the bridge

The renderer output is:

- one transparent SVG document made from vector `<rect>` and `<text>` elements
- optional PNG derived by rasterizing that SVG
- optional transparent PDF derived from that SVG through `svg2pdf`

`window.__PHGOJalviewBridgeAPI.renderMSAExportScene(settings, layout)` is the only supported renderer. If it is missing or cannot render, preview/export fails explicitly.

## Coordinate Systems

### MSA Columns And Boundaries

Displayed alignment column labels are 1-based.

Advanced layout ranges use MSA boundary coordinates:

- boundary `0` is before displayed column `1`
- boundary `alignmentWidth` is after the last displayed column
- displayed column `N` occupies boundary interval `[N-1,N)`
- DSL range `10,100` exports boundary interval `[10,100)` and has length `90`
- DSL range `~,10` exports boundary interval `[0,10)`
- DSL range `10,~` exports boundary interval `[10,alignmentWidth)`

The renderer indexes aligned sequence strings with 0-based JavaScript substring/slice indexes. A resolved boundary range `[start,end)` maps directly to substring/slice indexes `[start,end)`.

### Residue Positions

Right-side residue numbers are ungapped residue positions for each sequence at the end of the visible block.

For a sequence row and a major block:

1. Count characters from boundary `0` up to the block end boundary.
2. Count only biological residue characters.
3. Ignore gap characters `-`.
4. Ignore ASCII whitespace.
5. Ignore terminal stop marker `*` only when it is the final non-gap, non-whitespace character of the aligned sequence.
6. Count internal `*` characters because PHgo keeps internal stop markers runtime-visible.
7. Display that count at the right of the row.

Different rows can show different right-side values for the same MSA block because gaps differ by sequence.

### Alignment Column Numbers

Top numbers are alignment-column labels.

They are not residue numbers and are not row-specific.

Default:

- shown
- interval `20`

Example labels above a block are `20`, `40`, `60`, `80` when interval is `20` and the block crosses those boundary coordinates.

## Layout Areas

Each major block has these horizontal areas:

```text
left label area | residue grid | right residue number area
```

The right residue number area is present only when `Show right residue numbers` is enabled.

Top alignment column labels sit above the residue grid and align to the grid cells.

## Labels

The left label text is built from:

- optional PHgo coordinate prefix
- sequence display name
- optional length ratio
- optional length percent

The PHgo row checkbox must never be exported.

If `Show PHgo coordinates` is disabled, the coordinate prefix is omitted even though the live MSA page may show it.

If `Show length ratio` and `Show length percent` are both disabled, no residue ratio suffix is exported.

Left labels are always left-aligned. The export renderer must measure every exported left-label string with Jalview's active ID-list font, take the widest measured label, and add a safety gap of at least `24` logical pixels and at least three residue-cell widths before the residue grid starts. The left-label area must not use a fixed maximum width such as `420px`; long labels expand the exported logical width instead of being clipped or overlapped by the MSA grid.

## Cell Rendering

Each MSA character renders into one grid cell.

Required behavior:

- residues and gaps align on a fixed grid
- cell width and height come from settings
- text position is stable across export scales
- gap characters render as visible text only if the chosen style says so
- empty padding cells created by unequal allocations render as blank background, not fake gaps

## Color and Style

Color and style are not independently computed by `msaexpor`.

The bridge temporarily applies export settings to Jalview's current viewport and then reads Jalview/SwingJS renderer objects to build vector SVG:

- the left-label area is built from the same PHgo/Jalview label helper used by `IdCanvas`
- residue cell fills come from Jalview's active `SequenceRenderer`/residue shading path
- residue, label, column-number, and right-number text is emitted as SVG `<text>`
- group outlines are emitted as SVG `<rect fill="none">` from real Jalview alignment groups
- bridge context sets `window.__PHGO_MSAEXPOR_RENDER_ACTIVE__` during vector scene construction
- `IdCanvas` must return checkbox column width `0` and must no-op `drawPHgoCheckbox` while that export flag is active
- `IdCanvas` must suppress left-label search highlights and any selection-list highlight while that export flag is active, even if a caller accidentally passes a live search/selection list
- `SeqCanvas` must suppress search highlights, cursor drawing, selection-group outlines, and all current mouse/selection transient UI while that export flag is active
- bridge context gates group and feature drawing through `showGroups` and `showFeatures`
- group boundaries in export must come only from real Jalview alignment groups; the current selection group is not a group for export purposes

No fallback color table, hash color, DOM pixel sampling, browser screenshot, or Jalview visible-panel capture is allowed. If Jalview cannot provide the required renderer objects, export fails.

For performance, the model-loading path must not precompute per-cell style arrays. Row order and sequence identity are collected for layout, but residue colors and group drawing are resolved from Jalview's live renderer/model during vector scene construction.

## Annotation Exclusion

The initial export must not include Jalview annotation panels.

This includes consensus annotation panels, histograms, logos, and all below-alignment tracks.

The native renderer may call Jalview canvas primitives, but it must call only the MSA board primitives needed for:

- left sequence labels
- residue grid
- optional real group outlines
- optional feature overlays
- optional top alignment-column numbers
- optional right-side residue numbers

It must not call annotation-panel drawing, wrapped annotation drawing, visible SwingJS window capture, or any current DOM screenshot path.

## Block Flow

Major blocks render top to bottom.

Uniform vertical spacing applies between all major blocks. Blocks generated from different DSL source lines do not get special additional spacing.

Within one major block:

- rows render in selected DSL order or automatic export order
- all rows in the block share the same residue column span
- labels and right numbers are vertically aligned to row cells

## Right Number Alignment

For DSL allocations with uneven block widths, right-side residue numbers align to the longest allocated block width for that DSL line.

Example:

```text
10,100/30,20,40
```

The first and second major blocks draw only 30 and 20 residue cells, but their right-number column is positioned as if the residue grid were 40 cells wide.

Top alignment column labels follow the same width alignment rule.

## Output Size

Logical SVG container width and height are computed from content:

```text
width = leftLabelWidth + alignedBlockWidth + rightNumberWidth + margins
height = topNumberHeight + blockRowsAndGaps + margins
```

Scale choices (`1x`, `2x`, `5x`, `10x`) multiply PNG output resolution only. They must not change SVG/PDF logical size, cell counts, row counts, wrapping, or label content.

For SVG:

- write the canonical vector SVG directly
- do not include a page/background rectangle
- do not embed a raster `<image>` or `data:image/png`

For PNG:

- rasterize the canonical vector SVG at the selected scale
- respect a maximum pixel budget to avoid browser memory failure
- report an actionable error if the requested scale/content is too large

For PDF:

- convert the canonical vector SVG into a PDF
- preserve transparent page background where the PDF renderer allows it
- keep residue/label text as vector text where `svg2pdf` supports the emitted SVG

## Determinism

For the same input data and settings, `msaexpor` must produce stable geometry and stable output content.

Do not use:

- current scroll position
- current mouse position
- current selection rectangle
- current zoom of the browser page
- current child-window size except for preview viewport layout

The final exported file is based on explicit settings and data only.
