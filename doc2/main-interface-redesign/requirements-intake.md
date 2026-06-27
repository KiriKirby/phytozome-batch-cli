# Main Interface Redesign Requirements Intake

This file records only requirements explicitly provided by the user or decisions confirmed during implementation.

## Current Confirmed Requirements

- The current application has many separate screens.
- The new main-interface direction is to gather the settings in one interface first, then execute the selected operation.
- Existing screens must be preserved during the redesign.
- The new interface is the hard-coded normal startup path in the current build; it no longer depends on `PHGO_NEW_MAIN_INTERFACE`.
- The user's startup sketch is conceptual only. Do not copy its temporary text, control sizes, typography, spacing, or exact visual layout; follow the current application's design language.
- The new startup/home interface should present the current three modes as three top-level tabs.
- The main content/panel title follows the currently selected tab.
- The top startup explanation should preserve the original content but be rewritten as one compact paragraph instead of multiple separate lines.
- The three tabs are switched with `PgUp` and `PgDn`.
- The plain `Tab` key moves between controls inside the active tab, not between the top-level tabs.
- In the Keyword tab, the content is split into two vertical modules: Search Options and Search Content.
- Keyword Search Options uses a database dropdown over all supported databases.
- Keyword search type and species-selection controls are shown or hidden based on the selected database and search type.
- Hidden Keyword option controls do not leave layout holes; later visible controls shift left.
- Current non-NCBI keyword databases show species selection only.
- NCBI keyword search must be modeled with search types now, even though only Protein is currently available and currently does not require species selection.
- Keyword species selection is a focusable button activated with `Space` or `Enter`; it opens a near full-screen modal inside the new main page and returns the selected species directly to Search Options.
- The Keyword species button shows a "set species..." style label before configuration and a shortest-form species name after configuration, truncated with ellipsis when needed.
- Keyword Search Content is a synchronized spreadsheet-like multi-column text editor, not independent text boxes.
- Keyword Search Content rows stay aligned across all visible columns; each row is one complete search request.
- Literal spaces are not allowed inside Keyword Search Content; `Space` switches between visible columns.
- In Keyword Search Content, Left/Right move inside the current cell and then cross to the neighboring visible column at cell boundaries.
- Keyword Search Content uses result-table-like column sizing from the longest header/value, starts at the header width with no wider minimum, keeps a fixed Row column on the left, and makes the focused column fully visible when possible.
- Empty input and `~` both mean an empty value in Keyword Search Content.
- In the active tab, `Tab` switches modules, arrow keys move within the focused module, and `Space`/`Enter` activate focused dropdowns or module buttons. Table editors keep their own cell/caret navigation.
- Keyword button bar has Clear Content on the left, Paste immediately to the right of Clear Content, Search on the right, and Wide Search on the right only when the selected database/search type supports it.
- Keyword Clear Content clears only the Search Content table/editor, not the whole Keyword tab and not Search Options.
- Keyword Paste pastes clipboard text only into the currently selected Search Content cell's current visible column, filling rows downward for multi-line paste and not distributing across other columns.
- Keyword Search and Wide Search require valid options/content; invalid invocation opens a modal explaining remaining problems.
- Button shortcuts should reuse the current application's existing button shortcuts wherever possible.
- The BLAST tab follows the same two-module pattern as Keyword: BLAST Options and BLAST Content.
- BLAST Options uses a database dropdown, a species selector when required, and a BLAST program dropdown only for databases/workflows that need explicit program selection.
- BLAST option controls are capability-driven and hidden controls do not leave layout holes.
- BLAST Content is the same synchronized multi-column grid-editor pattern as Keyword, with FASTA, Symbol name, and future Gene locus-style columns as capability-driven visible columns.
- Current BLAST modes show FASTA and Symbol name only; Gene locus remains hidden until a BLAST execution path actually consumes it.
- BLAST FASTA is a wrapping column fixed to half of the current grid width; pasted multi-line FASTA records stay in one logical row, manual in-cell newlines use plain `Enter` in the FASTA column, other columns remain single-line longest-content width starting at their header width, shorter cells are vertically centered in rows whose FASTA wraps, and Up/Down move inside wrapped cell lines before crossing logical rows.
- BLAST Clear Content clears only the BLAST Content table/editor, not BLAST Options.
- BLAST Paste pastes clipboard text only into the currently selected BLAST Content cell's current visible column, filling rows downward for multi-line paste and not distributing across other columns.
- Run BLAST requires hard-required options/content to be valid; invalid invocation opens a modal explaining remaining problems.
- If Symbol name and/or Gene locus are missing when Search, Wide Search, or Run BLAST starts, preserve the current prompt behavior for automatic identification.
- If both Symbol name and Gene locus need automatic identification, run Symbol name first, then Gene locus.
- The new interface should reuse the existing Symbol name and Gene locus auto-identification modules and update the tab grid state from their results.
- The Explore tab should keep the current menu-style interaction and should not be redesigned into the Keyword/BLAST two-module input layout unless a later requirement changes that.
- The bottom shortcut help stays in the same general position as the current application, but should be short and list only shortcuts that do not have visible buttons.
- The button bar is contextual and changes based on the active tab and the state/content inside that tab.
- Current implementation includes Keyword, BLAST, and Explore entry actions, in-page species selection, and execution into existing result review/export/Canvas/Run BLAST flows.

## Open Decisions

- Whether a future rollback/config switch is needed after the new interface stabilizes.
- Exact final labels after user review of the first implementation.
- Exact Keyword invalid-state modal wording.
- Whether Keyword Clear Content asks for confirmation.
- Whether BLAST Clear Content asks for confirmation.
- Whether any Explore menu actions need renamed contextual buttons or shortcut changes inside the new tabbed home interface.
- Whether unfinished new-main tab state needs snapshot persistence.

## Incoming Design Notes

Record future user-provided design content here before implementation.
