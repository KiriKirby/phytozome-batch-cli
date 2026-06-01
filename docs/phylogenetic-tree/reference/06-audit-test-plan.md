# Audit And Test Plan

This is the checklist for freezing the system-tree feature. A box is complete
only when source-backed behavior, PHgo wiring, artifacts, snapshots, and tests
all agree.

## Phase 1: Source Inventory

- [x] Confirm `_mega_source/MEGA12.1-source` is the only MEGA source folder used
  for every parameter and behavior decision.
- [x] Re-check alignment HTML files against `internal/phylo/registry.go`.
- [x] Locate any MEGA 12.1 HTML analysis-options dialog for NJ, ME, UPGMA, ML,
  and MP; if none exists, record the absence and do not freeze tree parameter
  UI/defaults as MEGA parity.
- [x] Re-check tree method runtime behavior against MEGA command processors and
  thread/source code, while keeping parameter UI/defaults HTML-only.
- [x] Re-check Tree Explorer option HTML files against viewer features.
- [x] Mark every MEGA Tree Explorer feature as implemented, intentionally out
  of scope, or pending.

## Phase 2: Boundary Enforcement

- [x] Search for Go-side biological cleanup in tree input paths.
- [x] Search for Go-side pairwise distance, bootstrap, tree search, and Newick
  recursion.
- [x] Confirm PHgo never reverse-translates protein to DNA.
- [x] Confirm DNA target mode uses only embedded/source-resolved real
  nucleotide/CDS payloads.
- [x] Confirm runtime `error_text` is surfaced before missing artifact errors.
- [x] Confirm selected tree rows are not filtered by export-readiness helpers
  before `mega-phgo-runtime`.
- [x] Confirm normal Canvas FASTA export keeps its export-readiness filtering
  separate from the tree/converted-FASTA runtime path.
- [x] Confirm empty selected records are not silently removed from
  `input.fasta`.
- [x] Confirm sequence-source resolution errors are not swallowed into empty
  runtime records.
- [x] Confirm aligned/Newick artifacts are accepted only from runtime-declared
  or canonical output paths, never arbitrary stale files in the run directory.
- [x] Confirm Linux/macOS return unsupported instead of attempting runtime
  discovery.
- [x] Confirm Windows runtime discovery accepts only bundled application-local
  runtime files.

## Phase 3: Alignment Matrix

For each method, test parameter defaults, dynamic UI visibility, request JSON,
and real-runtime success/failure:

- [x] Protein ClustalW
- [x] Protein MUSCLE
- [x] DNA ClustalW
- [x] DNA MUSCLE
- [x] Codon ClustalW
- [x] Codon MUSCLE

Required tests:

- [x] registry default snapshot test
- [x] target-mode method gating test
- [x] stale snapshot method normalization test
- [x] runtime request method mapping test
- [x] runtime request editable-default propagation test
- [x] real-runtime alignment smoke probe for each method

## Phase 4: Tree Method Matrix

For each tree method, test protein and DNA runtime method mapping, request JSON,
runtime output, and bootstrap where supported. Parameter defaults/options for
tree methods are not final until a MEGA 12.1 HTML analysis-options source is
found.

- [x] Neighbor-Joining
- [x] Minimum Evolution
- [x] UPGMA
- [x] Maximum Likelihood
- [x] Maximum Parsimony

Required tests:

- [x] no-bootstrap real-runtime probe
- [x] bootstrap real-runtime probe
- [x] site-deletion option propagation
- [x] model/method propagation
- [x] gamma/rate option propagation
- [x] thread-count propagation
- [x] tree parameter directness matrix guard
- [x] runtime Newick nonempty assertion
- [x] runtime aligned FASTA nonempty assertion

## Phase 5: Parameter UI Completeness

- [x] Every source-backed alignment/tree parameter row appears in Canvas tree
  panel.
- [x] Any current registry row without HTML backing is marked with its
  alternate MEGA source authority.
- [x] Read-only rows are rendered read-only.
- [x] Picklists preserve MEGA strings exactly.
- [x] Numeric ranges and increments match MEGA.
- [x] Conditional rows appear only when MEGA applicability rules say they do.
- [x] No hidden scientific constants are used outside visible parameters.
- [x] TUI settings round-trip through `TreeSettings`.
- [x] TUI settings round-trip through `.pgo` snapshots.

## Phase 6: Rendering And Viewer

- [x] Stable `PHGOT...` leaves map to display names in Newick.
- [x] Stable `PHGOT...` aligned FASTA headers map to display names.
- [x] Quoted Newick labels round-trip through Reactree parser/exporter.
- [x] Underscores, punctuation, spaces, and single quotes display correctly.
- [x] Alignment panel remains synchronized with tree labels.
- [x] SVG export works.
- [x] PNG export works.
- [x] PDF export works.
- [x] `.pgv` export/import restores payload and viewer state.
- [x] Viewer service lifecycle is per Canvas page/session.
- [x] Standalone `.nwk` browser sessions do not share Canvas viewer state.

## Phase 7: Snapshots And Reuse

- [x] `.pgo` saves full tree artifacts.
- [x] `.pgo` opens into a restorable Canvas tree state.
- [x] Snapshot open restores payload without recomputing.
- [x] First explicit refresh after snapshot open forces runtime recompute.
- [x] Later display-name-only refresh is render-only.
- [x] Selection/order/sequence/target/method/parameter changes force compute.
- [x] Artifact reuse requires matching manifest and existing runtime artifacts.
- [x] Preview-only snapshot metadata sync does not alter compute fingerprints.

## Phase 8: Failure And Recovery

- [x] Missing runtime reports strict bundled-runtime error.
- [x] Missing MUSCLE binary reports strict runtime-owned MUSCLE error.
- [x] Runtime nonzero exit keeps stdout/stderr/log artifacts.
- [x] Runtime `error_text` reaches the user unchanged.
- [x] Missing `aligned.fasta` after an otherwise clean runtime response is
  reported as a missing artifact and never falls back to `input.fasta`.
- [x] Missing `tree.nwk` after an otherwise clean runtime response is reported
  as a missing artifact and never falls back to arbitrary stale `.nwk` files.
- [x] Cancellation works before request writing.
- [x] Cancellation works while runtime is executing.
- [x] Recovery dialog behavior preserves MEGA authority and does not silently
  skip rows before runtime execution.

## Phase 9: Release Verification

- [x] `go test ./...`
- [x] `go vet ./...`
- [x] `npm test` in `tree-viewer`
- [x] `npm run build` in `tree-viewer`
- [x] `scripts/build-tree-viewer.ps1` synced embedded viewer assets
- [x] Windows bundled runtime probe
- [x] Windows release package contains `mega-phgo-runtime.bin`
- [x] Windows release package contains `muscleWin64.bin`
- [x] Linux/macOS unsupported behavior verified
- [x] Tree docs updated after any behavior change

## Completion Standard

The feature is frozen only when every exposed mode is:

- source-backed by MEGA 12.1,
- reachable from the PHgo UI,
- executed by `mega-phgo-runtime`,
- preserved through artifacts and snapshots,
- rendered by the browser viewer without biological recomputation,
- covered by automated tests or real-runtime probes.
