# Main Interface Redesign Implementation Boundary

This document defines the current engineering boundary for the new main-interface work.

## Separation Rule

The new interface must be implemented as a separate path. Existing interactive screens, page routing, workflow screens, result review pages, and recovery flows must remain available while the new path is still being validated.

## Legacy Compatibility

- Do not remove existing screens during the redesign phase.
- Do not silently change legacy workflow behavior as a side effect of building the new path.
- Shared components may be reused only when the behavior remains compatible with the existing screens.
- Any shared-state or routing change must preserve the current workflow entry points unless a later confirmed requirement says otherwise.

## Startup Selection Rule

The current build opens the new interface from normal startup without relying on environment variables, WezTerm config, or linker-injected defaults.

- Normal root startup: use the new main interface.
- Handoff, transfer, result review, recovery, and old-screen code paths remain available where they are explicitly routed.
- Do not reintroduce `PHGO_NEW_MAIN_INTERFACE` or `PHGO_BUNDLE_NEW_MAIN_INTERFACE_DEFAULT` as packaging-time requirements.

## Documentation Rule

Before adding implementation details that affect user-visible behavior, record the confirmed design requirement or decision in this documentation library.

## Current New-Path Boundary

- The new TUI page collects tab settings and returns a structured action to workflow.
- Workflow translates the returned state into existing Keyword, BLAST, Explore, result review, export, Canvas, and Run BLAST flows.
- Species selection is implemented as an in-page large modal in the new TUI path. It uses a new-main species option callback and does not modify or replace the legacy selector screens. The modal mirrors the legacy split between direct small-list selection and searchable/index-style selection for larger candidate sets.
- Startup helper preflight owns cache cleanup, release update check, and symbol-name database check/update before the normal main page opens. It must not spawn the main page before the full preflight completes. Normal startup must not show helper and main tabs at the same time; only the explicit symbol-name database "use while downloading" flow may do that. While preflight is not complete, manually opened main tabs must keep using the startup-state wait modal instead of entering the main page silently. After helper success, tab `[0]` should close on clean exit; failures should remain visible.
