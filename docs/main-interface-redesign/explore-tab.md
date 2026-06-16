# Explore Tab Design Notes

This document records confirmed requirements for the new Explore tab.

## Confirmed Direction

The Explore tab should keep the current menu-style interaction.

Do not redesign Explore into the Keyword/BLAST two-module input layout unless a later confirmed requirement changes this.

## Behavior

- Explore appears as the third top-level tab.
- It is reached with `PgDn` from BLAST or `PgUp` from Keyword.
- Plain `Tab` and `Enter` navigate between focusable controls inside the Explore tab, following the shared main-interface focus rules.
- The internal Explore content should preserve the current menu structure and current available actions.
- The Explore tab stays inside the shared top-level tab content frame used by all tabs, but it must not add a second inner Explore-specific frame or an inner `Explore` title.
- The contextual bottom button bar may adapt to the current Explore menu state, but should not invent new workflow steps before they are specified.

## Open Decisions

- Whether any current Explore menu actions need renamed buttons in the new contextual button bar.
- Whether any existing Explore shortcuts need changes when embedded inside the new tabbed home interface.
