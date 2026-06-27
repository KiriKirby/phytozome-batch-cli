# Keyword Tab Design Notes

This document records confirmed requirements and implementation-level design for the new Keyword tab.

## Source Sketch

The user's sketch is a conceptual reminder only. Do not copy its temporary labels, typography, sizes, spacing, border weights, or exact proportions. The real implementation must follow the current application's established TUI design language and shared tview patterns.

Reference sketch:

![Keyword tab conceptual sketch](./assets/keyword-tab-sketch.png)

## Page Structure

The Keyword tab contains two vertical modules:

- Search Options
- Search Content

The contextual bottom button bar belongs to the whole active tab, not to either module.

## Search Options Module

Search Options contains a left-to-right row of controls whose visibility depends on the selected database.

Controls:

- database selector
- search-type selector
- species selector button

The database selector is a dropdown over all supported keyword databases.

The search-type selector and species selector depend on the selected database and selected search type:

- Most current non-NCBI databases show species selection only.
- NCBI currently exposes only Protein keyword search.
- NCBI must be modeled as a database with search types now, even though only Protein is available today.
- NCBI Protein currently does not require species selection.
- Future NCBI search types may require species selection, so the visibility framework must already support database + search-type-dependent species requirements.

Layout compaction rule:

- Hidden controls do not leave holes.
- If search type is hidden but species selection is visible, species selection shifts left into the next available position.
- The options row is generated from visible controls in order rather than from fixed columns with blank placeholders.
- Visible Search Options controls are laid out horizontally and share the available width equally, whether there are two controls or three.
- The Search Options and Search Content module titles are right-aligned inside their borders.
- Every Search Options control uses the same vertical field form: title on the upper line and the actual dropdown/button below it.

## Species Selector Button

Species selection is a focusable button inside the Search Options module.

It does not receive its own global shortcut. It is reached by module/internal navigation inside the active tab and activated with `Space` or `Enter`.

Button display:

- Before configuration, show a concise "set species..." style label.
- After configuration, show the selected species using the shortest available display form.
- If the species label does not fit in the button width, truncate with an ellipsis.
- The button must not expand or force layout changes because of a long species name.

Activation behavior:

- Pressing `Space` or `Enter` opens a large modal inside the new main page.
- The modal is near full-screen with similar top/bottom and left/right margins.
- The modal loads candidates through a new-main callback and chooses between the legacy interaction styles: small candidate sets are shown as a direct numbered list, while larger candidate sets show the searchable/index-style filter.
- Direct-list species selection supports numeric direct selection for the visible first nine candidates.
- The legacy species/search-index pages are preserved for old workflow paths; the new path has its own modal interaction so it can evolve independently.
- Search is not launched by species selection.

## Search Content Module

Search Content is a spreadsheet-like multi-column text editor. It combines table behavior with large text-input behavior.

It should visually match the application's other table views, not merely approximate them: use the same two-space cell padding, bold header text, muted table separators, header divider line, active-cell blue highlight, fixed Row column, and no separate placeholder/hint row inside the editor.

Initial conceptual columns:

- search term
- symbol name
- gene locus

Column visibility depends on database/search type:

- Search term is always required.
- Current implementation generates columns from database/search-type capability.
- Current non-NCBI keyword modes show Search term and Symbol name.
- NCBI Protein shows Search term, Symbol name, and Gene locus.
- Execution uses Gene locus where the current workflow supports it, currently NCBI keyword search.
- Hidden columns do not create blank spacing.

Rows are synchronized across all visible columns:

- A row represents one complete search request.
- The visible column cells in the same row always move together as one row unit.
- There must not be independent scrolling where one column's row 10 aligns with another column's row 11.
- Adding, deleting, pasting, wrapping, or scrolling must preserve row alignment across columns.

Whitespace rule:

- Literal spaces are not allowed inside this editor.
- `Space` is an interaction key used to move between the visible columns in the current row.
- Empty input and `~` both mean an empty value.

Horizontal navigation:

- Left/Right move inside the current cell text when possible.
- When the caret reaches the far left of the current cell and the user presses Left again, focus moves to the previous visible column in the same row.
- When the caret reaches the far right of the current cell and the user presses Right again, focus moves to the next visible column in the same row.
- Space also moves to the next visible column in the current row.
- Space must not insert a character.

Vertical navigation:

- Up/Down move to the same visible column in the previous/next logical row inside the Search Content editor.
- Top-level application navigation must not use Up/Down to leave this editor or move between modules.
- `Enter` inside the Keyword Search Content editor moves to the next logical row in the same column and creates that row when needed.
- Outside the Search Content editor, `Enter` activates the focused dropdown or module button.

Column width and visibility:

- Use the same sizing principle as result tables: each column width is derived from the longest visible value/header for that column.
- Initial column width is the header width itself. Do not impose a wider minimum before the user enters content.
- The grid has a fixed left Row column. Row numbers are shown only for created/populated rows, and the Row column stays visible when data columns scroll horizontally.
- Visible column separators are drawn between Row and data columns and between data columns.
- When a column is focused, horizontal viewport handling must make the whole focused column visible when possible.
- Avoid half-visible trailing columns when the viewport is wide enough to show whole columns.
- Long cell content may require an internal horizontal viewport/caret offset for that cell, but it must not resize the column while editing.

Validation:

- Search term must be present for every non-empty logical row.
- Optional columns may be empty or `~` according to the selected database/search type.
- Rows that are completely empty after normalizing `~` are ignored or cleaned according to the final implementation decision.
- Invalid partial rows block Search and Wide Search and should produce a modal listing the unresolved problems.

## Focus Model

Global tab switching:

- `PgUp` switches to the previous top-level tab.
- `PgDn` switches to the next top-level tab.

Focus traversal inside the active tab:

- Plain `Tab` moves between modules inside the active tab, primarily Search Options and Search Content.
- Up/Down/Left/Right move between controls inside the currently focused module.
- Focusable controls include dropdowns, module buttons, and the Search Content editor.
- Plain `Tab` does not switch top-level tabs.
- Within the Search Content editor, arrow keys belong to the editor's row/cell navigation.

Dropdowns:

- A focused dropdown opens with `Space` or `Enter`.
- Merely focusing a dropdown must not open it.
- While a dropdown is open, it captures Up/Down, Space, Enter, and Esc. Up/Down navigate dropdown items; Space/Enter accepts the highlighted item; Esc closes the dropdown without changing the selected value.
- Dropdown selection changes update dependent control visibility immediately.

Buttons:

- Focused module buttons activate with `Space`.
- Contextual button-bar actions follow the existing app button behavior.

## Button Bar

The Keyword tab contextual button bar contains:

- Clear Content on the left.
- Paste immediately to the right of Clear Content.
- Aliases on the left, shown only when the Search Content grid focus is on a Symbol name cell.
- Wide Search on the right side, shown only when the selected database/search type supports wide search.
- Search on the right side.

Clear Content:

- Clears only the Search Content grid/editor in the current Keyword tab.
- It does not reset Search Options.
- It does not reset other top-level tabs.

Paste:

- Pastes clipboard text into the currently selected cell's visible column in the Search Content editor.
- Paste does not distribute input across other columns.
- Multi-line pasted text fills rows downward in the current column from the active row.
- Pasted `~` values normalize to empty values.
- Pasted literal spaces are rejected or normalized according to the final paste-format decision, but spaces must not become in-cell content.

Search and Wide Search:

- These actions are usable only when all Keyword tab content is valid.
- If invoked while invalid, show a modal explaining what is still wrong.
- Wide Search is hidden entirely when the database/search type does not support it.
- Search and Wide Search must not silently start with incomplete species/search-type/content settings.
- Missing auto-fillable Symbol name or Gene locus values use the shared pre-execution auto-identification prompt flow instead of a generic invalid-state modal.
- Only truly blank Symbol name/Gene locus cells are auto-fillable; `~` and `~~` are preserved.
- If both Symbol name and Gene locus need auto-identification, run Symbol name first, then Gene locus.
- Auto-identification writes results back into the Search Content grid and returns to the Keyword tab so the user can edit before starting the action again.
- When auto-identification returns alias candidates, the focused Symbol name cell exposes `Aliases Ctrl+L` in the visible button bar.

Button shortcuts:

- Button shortcuts should reuse the current application's existing button shortcuts wherever possible.
- Do not invent new shortcuts when an equivalent legacy action already has a stable shortcut.
- Shortcut help remains reserved for shortcuts without visible button equivalents.

## Suggested Code-Level Model

Define the Keyword tab state independently from legacy screens:

```go
type KeywordTabState struct {
    DatabaseID      string
    SearchTypeID    string
    Species         *SpeciesSelection
    Rows            []KeywordInputRow
    Focus           KeywordFocus
    LastValidation  KeywordValidationResult
}

type KeywordInputRow struct {
    SearchTerm string
    SymbolName string
    GeneLocus  string
}
```

Define database capabilities as data, not scattered conditionals:

```go
type KeywordDatabaseCapability struct {
    DatabaseID       string
    SearchTypes      []KeywordSearchTypeCapability
    DefaultSearchID  string
    SupportsWide     bool
}

type KeywordSearchTypeCapability struct {
    SearchTypeID     string
    RequiresSpecies  bool
    ShowsSpecies     bool
    ShowsSymbolName  bool
    ShowsGeneLocus   bool
    SupportsWide     bool
}
```

Visible option controls should be generated from current capabilities:

```go
func (s KeywordTabState) VisibleOptionControls(c KeywordDatabaseCapability) []OptionControl
```

Visible content columns should also be generated from capabilities:

```go
func (s KeywordTabState) VisibleInputColumns(c KeywordSearchTypeCapability) []KeywordInputColumn
```

The Search Content editor should be a dedicated widget rather than three unrelated `InputField` widgets. It needs one shared model, one row cursor, one column cursor, one vertical viewport, and per-cell caret positions:

```go
type KeywordGridEditor struct {
    rows          []KeywordInputRow
    columns       []KeywordInputColumn
    activeRow     int
    activeCol     int
    caretByCell   map[CellCoord]int
    rowOffset     int
    colOffset     int
}
```

Implementation direction:

- Render the editor as a custom tview primitive or a thin wrapper around the same table-rendering utilities used by result tables.
- Keep editing semantics in the widget, not in the page router.
- Normalize values through a single helper where `""` and `"~"` become empty.
- Reject literal spaces at input time and/or normalize pasted input before committing it.
- Implement paste as an editor operation scoped to the active visible column, not as a page-level multi-column parser.
- Use shared table column-width logic where possible so Keyword input and result tables behave consistently.
- Keep the widget independent from legacy keyword screens, then translate `KeywordTabState` into the existing keyword workflow request only when Search or Wide Search starts.
- Insert the shared Symbol name / Gene locus auto-identification flow before final search execution.

## Open Decisions

- Exact final visible labels for database, search type, species, content columns, and buttons.
- Whether completely empty rows are removed immediately, preserved visually until search, or ignored only at validation.
- Exact paste format rules for multi-line/multi-column input beyond the current active-column paste behavior.
- Whether Clear Content asks for confirmation.
