# MEGA-PHgo Deep Audit

This is the live working index for the full system-tree audit. It tracks source
anchors, PHgo integration points, renderer and snapshot contracts, gaps found,
and fixes applied.

## Scope

- PHgo owns orchestration only: TUI state, request construction, runtime launch,
  artifact bookkeeping, snapshot restore, and viewer refresh.
- MEGA owns every biological computation: alignment, distance calculation,
  tree inference, bootstrap, Newick/alignment artifacts, and runtime error
  text.
- The renderer owns visual tree interaction and graphical export only. It must
  consume computed artifacts and metadata without recomputing biology.

## Source Anchors

- MEGA runtime project: `_mega_source/MEGA12.1-source/PHgoRuntime/`
- MEGA alignment UI/options:
  `MEGA12_HTML_DIR` as defined in `README.md`
- MEGA alignment source:
  `_mega_source/MEGA12.1-source/Alignment/`,
  `_mega_source/MEGA12.1-source/AlignmentEditor/`,
  `_mega_source/MEGA12.1-source/MegaVcl/mclustalw.pas`
- MEGA tree command processing:
  `_mega_source/MEGA12.1-source/Process/processtreecmds.pas`,
  `_mega_source/MEGA12.1-source/Process/processmltreecmds.pas`,
  `_mega_source/MEGA12.1-source/Process/processparsimtreecmds.pas`,
  `_mega_source/MEGA12.1-source/Process/processdistcmds.pas`
- MEGA tree algorithms:
  `_mega_source/MEGA12.1-source/TamuraFuncs/`,
  `_mega_source/MEGA12.1-source/Compute/`
- MEGA tree and export UI:
  `_mega_source/MEGA12.1-source/TreeEditor/`,
  `_mega_source/MEGA12.1-source/DataOutput/mnewick_export_thread.pas`
- PHgo Go integration: `internal/phylo/`, `internal/workflow/canvas.go`
- PHgo session snapshots: `internal/sessionsnapshot/`,
  `internal/workflow/session_snapshot.go`,
  `internal/workflow/canvas_tree_snapshot_test.go`
- PHgo viewer snapshots: `internal/viewersnapshot/`, `tree-viewer/src/pgv.js`
- React renderer adapter: `internal/phylo/viewer.go`, `tree-viewer/src/`

## Initial Findings

- Existing docs already define strict runtime-only computation and snapshot
  contracts. This audit extends them with a file-by-file check and concrete
  fixes.
- `docs/phylogenetic-tree/reference/07-mega-tree-options-index.md` already contains a MEGA
  12.1-backed tree option matrix. This audit revisits Minimum Evolution status
  against source and runtime probes.
- `docs/phylogenetic-tree/phgo-runtime-pipeline.md` had stale text saying only
  Neighbor-Joining was verified and describing PHgo-side conversion behavior. Current
  code exposes all five MEGA-backed tree methods and treats target mode as the
  only TUI data-mode choice.

## Work Queue

- [x] Map all `internal/phylo` public types and settings to MEGA/runtime fields.
- [x] Map all TUI Canvas tree controls to `phylo.TreeSettings`.
- [x] Map all runtime request fields to `PHgoRuntime` Pascal request parsing.
- [x] Verify alignment parameters against MEGA 12.1 HTML option dialogs only.
- [x] Locate MEGA 12.1 HTML parameter dialogs for distance-tree, ML, MP, and ME
  inference settings; none are registered in MEGA 12.1, so tree inference
  parameters are anchored to analysis-preference source plus real screenshots.
- [x] Remove or block any PHgo-local biological computation that belongs to MEGA.
- [x] Fix Minimum Evolution runtime path or explicitly make it unavailable with
  exact runtime failure text.
- [x] Verify computation-to-render payload preserves all runtime information the
  viewer can use.
- [x] Complete Canvas `.pgo` tree snapshot save/open artifact restoration.
- [x] Complete viewer `.pgv` snapshot save/open browser-state restoration.
- [x] Run final Go tests, viewer tests, and real runtime smoke probes.

## Change Log

- 2026-06-01: Created audit index after reading `AGENT.md` and locating the
  MEGA 12.1, PHgo, snapshot, and viewer code areas.
- 2026-06-01: Confirmed from `mega_main.pas` and input-option dialogs that
  MEGA gates DNA/protein/codon actions by active data type and does not
  reverse-translate protein to DNA. PHgo DNA mode now uses only embedded or
  source-resolved real nucleotide/CDS input; otherwise it leaves input for
  MEGA runtime to accept or fail.
- 2026-06-01: Switched the parameter UI audit rule to HTML-only. MEGA option
  JSON files are ignored for UI/default decisions. Current PHgo registry drift
  from HTML is tracked in `09-html-parameter-ui-audit.md`.
- 2026-06-02: Clarified the HTML-only rule: it applies specifically to
  alignment and tree-type/tree-inference parameter-setting UI/defaults. Non-UI
  runtime behavior with no parameter-setting surface remains auditable from
  MEGA 12.1 source and `mega-phgo-runtime`. Older MEGA source folders must not
  be used for this audit.
- 2026-06-02: Removed the earlier implication that tree-inference parameter
  defaults could be finalized from command processors/preference strings alone.
  Those sources remain valid for runtime behavior, not for align/tree-type
  parameter UI/default parity.
- 2026-06-01: Re-enabled Minimum Evolution bootstrap by removing the PHgo
  runtime guard before MEGA `TBootstrapMEThread`, rebuilt the Windows runtime,
  and updated the real-runtime probe to require ME bootstrap completion.
- 2026-06-01: Verified Canvas `.pgo` tree artifact/payload restoration and
  viewer `.pgv` payload/state restoration with focused Go tests; added a
  browser-side PGV contract test.
- 2026-06-01: Completed final verification with `go test ./...`, `go vet ./...`,
  `npm test`, `npm run build`, and real `mega-phgo-runtime` probes for alignment,
  all five tree methods, ML/MP/distance bootstrap, and Minimum Evolution
  bootstrap.
- 2026-06-02: Re-checked the registered MEGA 12.1 HTML resource list in
  `Common/megaprivatefiles.pas`: ClustalW parameter HTML and Tree Explorer
  rendering/export HTML are present; NJ/ME/UPGMA/ML/MP analysis-parameter HTML
  is absent.
- 2026-06-02: Closed Canvas handoff gaps: selected tree rows are no longer
  filtered by FASTA export readiness, unknown row sequence-kind metadata is
  preserved, empty selected records remain visible in `input.fasta`, and real
  source-resolution errors surface instead of becoming empty records.
- 2026-06-02: Final Canvas boundary pass split normal FASTA export from tree
  input selection: plain FASTA export still filters to export-ready rows, while
  tree refresh keeps the checked row set intact for `mega-phgo-runtime`.
  `canvasTreeRowSourcesWithSkippedForSettings` now returns nucleotide resolver
  and source-construction errors directly instead of replacing them with empty
  records.
- 2026-06-02: Removed the old Canvas "converted FASTA" export from the UI and
  runtime path. It was export-only post-processing over aligned FASTA, not a
  MEGA GUI-equivalent conversion workflow.
- 2026-06-02: Added task/progress wrappers for Canvas tree panel open, preview
  open, refresh runtime preflight, and missing-tool retry preflight so runtime
  checks and viewer startup no longer leave an unlabelled black wait screen.
- 2026-06-02: Fixed preview browser launch after adding progress wrappers:
  browser launcher processes are now detached from the short-lived task-modal
  context, while cancellation is still honored before launch starts. This keeps
  the preview dialog from flashing closed without opening the system browser.
- 2026-06-02: Fixed kind-normalized alignment settings so exact protein
  variants such as `clustalw_protein` preserve edited MEGA parameters. This
  keeps alignment/tree fingerprints sensitive to real parameter changes and
  prevents accidental render-only reuse after a compute input changes.
- 2026-06-02: Added and ran a desktop FASTA runtime matrix probe over
  `C:\Users\wangsychn\Desktop\4CL_other_yt2_0.fasta` and
  `C:\Users\wangsychn\Desktop\4CL_other.fasta`: Protein mode covers ClustalW
  and MUSCLE against all five tree methods; DNA mode covers ClustalW, MUSCLE,
  ClustalW Codons, and MUSCLE Codons against all five tree methods. PHgo does
  not modify the source files.
- 2026-06-02: Closed runtime/viewer final checks: cancellation before request
  writing and during runtime execution is tested, viewer sessions are isolated
  by session ID, Windows runtime packaging rules are source-checked, and
  real-runtime freeze probes passed for all exposed alignment and tree methods.
- 2026-06-20: Re-ran the production MEGA PHgo path against
  `C:\Users\wangsychn\Desktop\4CLtree.pgo` with the source-built Windows
  runtime executable. The first failure was traced to terminal `*` markers in
  the runtime FASTA, which PHgo now strips from all sequence ends before
  launching MEGA. PHgo must keep internal `*` characters runtime-visible and
  must not auto-switch ClustalW to MUSCLE or otherwise choose a replacement
  aligner/tree method for the user.
- 2026-06-20: Fixed Windows bundled-runtime probing so
  `mega-phgo-runtime.bin` is treated as a package asset and prepared as a
  temporary `.exe` with runtime-owned `muscleWin64.bin` before probing or
  execution. Packaging scripts now probe the same way instead of trying to open
  the `.bin` directly.
- 2026-06-20: Removed the old duplicate-label suffix mechanism that appended
  hidden taxon IDs such as `[PHGOT000123]` to Reactree labels. Canvas tree/MSA
  display now uses the default-on Show PHgo row coordinates option to prefix
  metadata labels with `[canvas item,row]` without writing that prefix into the
  Canvas `display_name` column. This option is preview-only and changes only
  the preview fingerprint, so runtime alignment/tree artifacts are reused when
  compute inputs are unchanged.
- 2026-06-20: Tightened PHgo task-modal cancellation: Cancel/Esc cancels the
  task context and waits for the task goroutine to exit; the legacy task modal
  no longer queues a synchronous redraw from inside the input-event cancel path.
  Added regression coverage for the wait-on-cancel behavior.
