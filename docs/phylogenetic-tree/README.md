# Phylogenetic Tree System Design Index

This directory is the source of truth for the standalone phylogenetic tree system.

The tree system is intentionally independent from keyword search, BLAST review, normal Canvas FASTA export, and session export. Canvas is the launch surface and sequence selection surface, but tree analysis, tree preview, and tree export are owned by this subsystem.

## Core Decisions

- Canvas exposes a toggleable phylogenetic tree tool panel.
- `mega-phgo-runtime`, built from the MEGA 12.1 source tree and adapted for PHgo, is the only computation backend for alignment and tree inference.
- Runtime installation is folder-based: PHgo only uses the application-local `mega-phgo-runtime` folder and never searches `PATH` or installed MEGA folders.
- Reactree.js is the selected tree viewer and renderer core.
- The browser hosts the Reactree page, which provides the interactive tree UI, preview controls, and export controls.
- Rendering and graphical export belong to the external tree viewer service, not to the Go TUI.
- The initial tree preview is empty after the tool panel opens; computation starts only when the user explicitly refreshes the tree.
- Generated tree state must be preserved as artifacts so reopening a session can restore the same review state without recomputing.

## Document Map

- [System Tree Reference Library](./reference/README.md)
  Source-backed documentation library for the MEGA 12.1 anchors, boundary
  contract, alignment and tree parameter surfaces, rendering/export inventory,
  artifacts, snapshots, and final audit test plan.
- [Architecture](./architecture.md)
  Defines process boundaries, ownership, lifecycle, and non-goals.
- [Canvas Tool Panel](./canvas-tool-panel.md)
  Defines the TUI panel, focus model, shortcuts, button behavior, yellow highlight style, and display-name workflow.
- [PHgo Runtime Pipeline](./phgo-runtime-pipeline.md)
  Defines the alignment and tree-computation pipeline and cache/recompute rules.
- [Reactree Viewer](./reactree-viewer.md)
  Defines the external viewer service, Reactree.js responsibilities, browser interaction, and live update model.
- [MSA Image Export Rewrite](./msaexport-image-export.md)
  Defines the `msaexpor` module that completely replaces the current PHgo MSA image-export workflow with a PHgo-owned export window inside the JalviewJS MSA page.
- [MSA Export Implementation Library](./msaexpor/README.md)
  Implementation-level documentation for the data-driven `msaexpor` renderer, settings, advanced group DSL, SVG/PNG/PDF export contract, and tests.
- [Artifact Contract](./artifact-contract.md)
  Defines the files and JSON contracts exchanged between Canvas, `mega-phgo-runtime`, and the viewer.
- [Implementation Plan](./implementation-plan.md)
  Defines the staged implementation path and test responsibilities.

## Boundary Statement

The Go TUI owns user workflow state, selected rows, runtime execution, and file contracts.

The tree viewer service owns tree rendering, tree interaction, graphical export, and browser-facing preview state.

The two systems communicate by stable artifacts and a small local service API.
