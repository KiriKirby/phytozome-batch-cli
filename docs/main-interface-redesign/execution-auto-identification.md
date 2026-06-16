# Execution Auto-Identification Flow

This document records the shared pre-execution behavior for missing Symbol name and Gene locus values in the new main interface.

## Scope

Applies to actions started from configured tab content:

- Keyword Search
- Keyword Wide Search
- BLAST Run BLAST

## Missing-Cell Rule

Only a truly blank editable cell is eligible for auto-identification.

- Empty string means missing and may be auto-filled.
- `~` means the user intentionally left the value empty and must not be changed.
- `~~` means a previous auto-identification found no result and must not be changed unless the user edits it.
- Auto-identification fills only eligible blank cells.
- If auto-identification has no result for an eligible blank cell, write `~~`.

## Prompt Order

When the user invokes Search, Wide Search, or Run BLAST:

1. Validate hard-required options and content first.
2. Check missing Symbol name cells.
3. If the current workflow has a visible/consumed Gene locus field, check missing Gene locus cells after Symbol name.
4. Each field is prompted independently. Skipping Symbol name does not skip Gene locus.
5. Every new action invocation repeats the same Symbol name then Gene locus checks.

## Prompt Buttons

The auto-identification prompt uses the normal modal button layout:

- Left side: `Close`.
- Left side after Close: `Skip`, only when skipping is allowed for that field/workflow.
- Right/default: `Auto identify`.

`Auto identify` is the default action. `Close` returns to the new main interface without starting the action. `Skip` leaves eligible blank cells blank and continues to the next pre-execution check or to execution.

BLAST multi-search mode, defined as more than one BLAST input row, requires Symbol name and does not offer `Skip` for Symbol name.

## Editable Review

Auto-identification is not a hidden one-way step.

- After auto-identification writes values into the new-main grid, the app returns to the same tab.
- The user can edit the filled Symbol name or Gene locus cells before invoking the action again.
- The next action invocation re-runs the pre-execution checks, so any still-blank eligible cells are handled again.
- Button availability and validation are recalculated after the grid state changes.

## Keyword Implementation

Keyword Symbol name identification reuses the existing keyword auto-identification logic.

- Keyword needs search results before it can identify Symbol name, so the implementation performs the keyword search needed for identification, fills the grid, then returns to the Keyword tab for user review.
- NCBI Protein Gene locus identification reuses the existing NCBI Gene locus lookup flow, fills the grid, then returns to the Keyword tab.
- Non-NCBI Keyword modes currently expose Search term and Symbol name only; NCBI Protein exposes Search term, Symbol name, and Gene locus.

## BLAST Implementation

BLAST Symbol name identification reuses the existing BLAST query-label flow.

- The BLAST input is parsed and resolved enough to run identification.
- Filled Symbol name values are written back to the BLAST grid.
- Current BLAST new-main capabilities show FASTA and Symbol name only. Gene locus remains hidden until a BLAST execution path visibly consumes it.
- Future BLAST Gene locus support must use the same independent prompt/review pattern after Symbol name.

## Alias Control

When the new-main grid focus is on a `Symbol name` cell, the left side of the button bar exposes `Aliases` with `Ctrl+L`.

- The action is hidden outside `Symbol name` cells.
- It uses the same alias chooser semantics as the existing result-table flow: list available aliases, copy the selected alias, rename/custom-enter a symbol name, or set the selected alias as the cell's Symbol name.
- The new-main alias chooser omits Rename because the grid cell itself is directly editable after closing the modal.
- Up/Down stop at the first/last alias instead of wrapping. Mouse click selects list items.
- Keyword auto-identification writes the ranked alias list back to the grid row, not only the first visible Symbol name.
- BLAST auto-identification writes source/query alias candidates back to the grid row, not only the first visible Symbol name.
- Choosing an alias updates only the visible Symbol name cell. It does not mutate the stored alias candidate list; aliases are reference/fill choices, not an edit target.
