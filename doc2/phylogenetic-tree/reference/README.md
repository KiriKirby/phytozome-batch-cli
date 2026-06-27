# System Tree Reference Library

This directory is the MEGA-backed reference library for finalizing the PHgo
system-tree feature. It is intentionally separate from the general workflow
design notes so audits can cite a stable source-of-truth document for every
parameter, boundary, artifact, and test obligation.

## Canonical Source Version

Use `_mega_source/MEGA12.1-source` as the canonical and only MEGA source tree
for this library. The version anchor is `Common/megaverconsts.pas`
(`MAJOR_VERSION = 12`, `MINOR_VERSION = 1`) plus the MEGA 12.1 project/runtime
tree.

For shortness, this library calls the MEGA 12.1 HTML resource directory
`MEGA12_HTML_DIR`. Its concrete path in this checkout is
`_mega_source/MEGA12.1-source/MEGA7_Install/OptionsDialogs`; that path segment is
only a resource-directory name inside the MEGA 12.1 source tree. The only
allowed source version remains MEGA 12.1.

For alignment and tree-type/tree-inference parameter-setting UI, use only the
MEGA 12.1 HTML option dialogs registered by `Common/megaprivatefiles.pas` or
otherwise present in `_mega_source/MEGA12.1-source` as current HTML dialogs.
Ignore MEGA option JSON files for UI items, defaults, selected values,
checkbox states, option lists, numeric ranges, and dynamic visibility.

This HTML-only rule is specific to parameter settings. For MEGA behavior with
no parameter-setting UI, use MEGA 12.1 source/runtime evidence and record the
confidence level.

## Library Map

- [MEGA Source Map](./00-mega-source-map.md)
  Lists the exact MEGA 12.1 folders and files used for computation, alignment,
  tree inference, Tree Explorer rendering, Newick export, help, and parameter
  dialogs.
- [Boundary Contract](./01-boundary-contract.md)
  Defines the hard line between PHgo orchestration, MEGA computation, and the
  browser viewer.
- [Alignment Surface](./02-alignment-surface.md)
  Documents all runtime-backed alignment modes, parameter rows, defaults, and
  source files.
- [Tree Inference Surface](./03-tree-inference-surface.md)
  Documents runtime-backed tree methods and adapter keys; it explicitly marks
  NJ/ME/UPGMA/ML/MP parameter UI/default parity as unresolved until MEGA 12.1
  HTML source is found.
- [Rendering And Export Surface](./04-rendering-viewer-surface.md)
  Inventories MEGA Tree Explorer display/export functionality and maps it to
  the PHgo browser viewer contract.
- [Artifacts And Snapshots](./05-artifact-and-snapshot-contract.md)
  Defines handoff files, request/response JSON, manifest fingerprints, viewer
  payloads, and snapshot preservation.
- [Audit And Test Plan](./06-audit-test-plan.md)
  Gives the end-to-end checklist for turning the system-tree feature into a
  frozen, paper-grade MEGA front end.
- [MEGA Tree Options Index](./07-mega-tree-options-index.md)
  Tracks the tree-computation parameter matrix and runtime anchors.
- [MEGA-PHgo Deep Audit](./08-mega-phgo-deep-audit.md)
  Live audit log for source checks, fixes, probes, and remaining risks.
- [HTML Parameter UI Audit](./09-html-parameter-ui-audit.md)
  HTML-only source manifest and current PHgo drift table.
- [Code Audit Correction Map](./10-code-audit-correction-map.md)
  Ordered code-reading ledger with concrete PHgo fixes needed before freezing
  the system-tree feature.
- [MEGA 12 HTML Control Index](./11-mega12-html-control-index.md)
  Code-level index of registered MEGA 12.1 HTML controls, defaults, validation,
  options, and dynamic UI actions.
- [Runtime Action Flow](./12-mega-runtime-action-flow.md)
  Code-level runtime orchestration map for requests, alignment, tree inference,
  bootstrap, Newick/alignment artifacts, errors, and logs.
- [Tree Parameter Directness Matrix](./13-tree-parameter-directness-matrix.md)
  Per-parameter ledger showing whether each PHgo tree UI row is consumed by the
  MEGA-backed runtime, display-only, MEGA-recorded without downstream algorithm
  use, or compatibility-only.
- [Jalview MSA State Contract](./14-jalview-msa-state-contract.md)
  Defines manual Jalview File > Save persistence for MSA state, including
  built-in/user-defined colour scheme serialization and PHgo popup-menu bounds.

## Non-Negotiable Rules

- MEGA owns sequence alignment, distance calculation, tree inference,
  bootstrap, and Newick/alignment output.
- PHgo may write input FASTA, stable labels, metadata, and runtime request
  parameters. It must not repair biological sequence content or implement
  fallback biological algorithms.
- PHgo align and tree-type parameter items must match the MEGA 12.1 HTML
  parameter-setting surface exactly for the modes it exposes: no missing MEGA
  items, no hidden constants, and no extra PHgo-only scientific options.
- The browser viewer owns rendering and graphical export only. It may adapt
  MEGA Tree Explorer concepts to Reactree controls, but it must not recompute
  alignment or tree data.
- A feature is considered ported only when it is reachable from PHgo, backed by
  MEGA/runtime behavior, preserved through artifacts/snapshots where relevant,
  and covered by tests or a real-runtime probe.
