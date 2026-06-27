# Code Audit Correction Map

This is the ordered code-reading ledger for turning PHgo system-tree into a
strict MEGA 12.1 front end. It records what the current Go code does, why that
matters, and the correction needed before freeze.

Scope rule:

- ClustalW alignment parameter settings are verified from MEGA 12.1 HTML.
- MUSCLE parameter settings are verified from real MEGA screenshots plus MEGA
  12.1 runtime/source anchors because no MUSCLE HTML parameter dialog is
  registered in the current source tree.
- Tree-type parameter settings are verified from MEGA 12.1
  `manalysisprefdlg.pas` / `megaanalysisprefstrings.pas` plus real MEGA
  screenshots because these analysis-preference rows are not registered as the
  ClustalW-style HTML web dialogs in this source tree.
- Non-parameter behavior with no HTML setting UI may be verified from MEGA
  12.1 source code and `mega-phgo-runtime`.
- PHgo may orchestrate, serialize, snapshot, and render. It must not align,
  infer, repair, reverse-translate, bootstrap, or silently skip biological
  input before MEGA.

## 1. Registry And Parameter Defaults

Files read:

- `internal/phylo/types.go`
- `internal/phylo/registry.go`
- `internal/phylo/registry_test.go`
- `MEGA12_HTML_DIR/clustalw_parameters_DNA.html`
- `MEGA12_HTML_DIR/clustalw_parameters_protein.html`
- `MEGA12_HTML_DIR/clustalw_parameters_codons.html`
- `MEGA12_HTML_DIR/select_genetic_code_dlg.html`

Current behavior:

- `DefaultAlignmentMethod` is base DNA `clustalw`; protein normalization maps
  it to `clustalw_protein`.
- `TreeDefinitions()` is ordered as ML, NJ, ME, UPGMA, MP for protein and DNA.
  `NormalizeTreeSettingsForKind` falls back to the first compatible tree
  definition when a stale/invalid tree method is present.
- [x] ClustalW registry defaults have been corrected to the MEGA 12.1 HTML
  defaults.
- [x] MUSCLE DNA/protein/codon rows have been registered from MEGA screenshots,
  MEGA source resources, and runtime keys.
- [x] Tree inference registry rows have been checked against MEGA 12.1
  analysis-preference source and real default screenshots.

Required corrections:

| File | Current code | Required behavior |
| --- | --- | --- |
| `internal/phylo/registry.go` | DNA ClustalW `use_negative_matrix` default `ON` | [x] HTML default `OFF`; options preserve `OFF`, `ON` |
| `internal/phylo/registry.go` | DNA ClustalW `keep_predefined_gaps` default `True` | [x] HTML checkbox unchecked / false |
| `internal/phylo/registry.go` | Protein pairwise gap extension `0.20` | [x] HTML default `0.1` |
| `internal/phylo/registry.go` | Protein matrix default `BLOSUM` | [x] HTML selected default `Gonnet` |
| `internal/phylo/registry.go` | Protein keep predefined gaps `True` | [x] HTML checkbox false |
| `internal/phylo/registry.go` | Codon pairwise gap extension `0.20` | [x] HTML default `0.1` |
| `internal/phylo/registry.go` | Codon end gap separation `OFF` | [x] HTML first/default value `ON` |
| `internal/phylo/registry.go` | Codon use negative matrix `OFF` | [x] HTML first/default value `ON` |
| `internal/phylo/registry.go` | Codon keep predefined gaps `True` | [x] HTML checkbox false |
| `internal/phylo/registry.go` | MUSCLE params exposed without HTML source | [x] Registered from MEGA screenshots/source/runtime keys; no JSON authority used |
| `internal/phylo/registry.go` | Tree params exposed without registered HTML dialog | [x] Registered from MEGA analysis-preference source and screenshots |
| `internal/phylo/registry.go` | Invalid/stale tree method falls back to ML because ML is first | [x] Fallback uses explicit `DefaultTreeMethod` |
| `internal/phylo/registry_test.go` | ClustalW tests expected stale defaults | [x] Updated to corrected defaults |
| `internal/phylo/registry_test.go` | Tree defaults/options require source-backed assertions | [x] Added source-backed assertions, including MP Min-Mini/Max-mini options |

Freeze standard:

- ClustalW tests must continue to guard the HTML defaults.
- MUSCLE and tree-type tests must continue to name their non-HTML authority:
  screenshots/source/runtime for MUSCLE; analysis-preference source/screenshots
  for tree types.

## 2. FASTA Handoff

Files read:

- `internal/phylo/fasta.go`
- `internal/phylo/build.go`

Current behavior:

- `InputFASTA` serializes records as stable `PHGOT...` FASTA headers and wraps
  sequence text.
- `BuildInput` assigns stable taxon IDs and display names. This is acceptable
  orchestration.
- `InputFASTA` skips records with empty sequences.
- `BuildRunPlan` uses `InputFASTA`, so selected rows that resolve to empty
  sequence text can disappear before MEGA sees the run.

Required corrections:

| File | Current code | Required behavior |
| --- | --- | --- |
| `internal/phylo/fasta.go:7` | Empty sequences are skipped | Preserve selected records or fail explicitly before runtime; do not silently drop them |
| `internal/phylo/build.go:138` | Plan inherits the skipped FASTA | Build plan must keep row count/FASTA/metadata consistent |

Allowed serialization:

- Stable `PHGOT...` IDs.
- FASTA line wrapping.
- Whitespace-only folding for FASTA text transport.

Forbidden biological cleanup:

- Stop-codon trimming.
- Residue/base replacement.
- Padding.
- Protein-to-DNA reverse translation.
- Row removal based on local biological compatibility.

## 3. Runtime Artifact Acceptance

Files read:

- `internal/phylo/run.go`
- `internal/phylo/source_runtime.go`
- `internal/phylo/run_test.go`

Current behavior:

- Runtime request/response flow is correct in shape.
- `runtime-response.json.error_text` is surfaced before artifact checks.
- `findAlignedFASTA` first checks expected aligned output names, then scans
  for any `.fasta`/`.fas` in the run directory.
- `input.fasta` is always in the run directory, so a clean-but-incomplete
  runtime can be misread as having produced an alignment.
- `findNewick` also falls back to arbitrary `.nwk`/`.newick`/`.tree` files.

Required corrections:

| File | Current code | Required behavior |
| --- | --- | --- |
| `internal/phylo/run.go:56` | Fallback accepts any FASTA in run dir | Accept only runtime-declared aligned FASTA or exact canonical names such as `aligned.fasta` |
| `internal/phylo/run.go:72` | Fallback accepts any tree-ish file | Accept only runtime-declared Newick or exact canonical names such as `tree.nwk` |
| `internal/phylo/run_test.go` | No regression covering `input.fasta` misread | Add a test where runtime omits `aligned.fasta`; expected error must mention missing aligned artifact |

Artifact rule:

- `input.fasta` is PHgo-owned input.
- `aligned.fasta` and `tree.nwk` are MEGA runtime-owned output.
- PHgo must never promote input or stale files into runtime output.

## 4. Canvas Row Selection And Sequence Resolution

Files read:

- `internal/workflow/canvas.go`
- `internal/workflow/blast.go`
- `internal/workflow/canvas_tree_snapshot_test.go`
- `internal/prompt/prompt.go`

Initial audited behavior:

- Tree refresh starts from selected Canvas rows, but helper paths filter rows
  through `canvasRowHasSequenceForExport`.
- `canvasTreeRowSourcesWithSkippedForSettings` swallows sequence-choice errors
  and continues with an empty choice.
- DNA mode uses real nucleotide/CDS data only when embedded or resolved through
  row metadata/cache. No reverse translation was found in the tree path.
- Unknown sequence kind can be relabeled to the current target kind in
  normalization metadata.
- General FASTA import uses `sanitizeSequence`, which removes `*`, whitespace,
  FASTA headers, and uppercases sequence text before rows reach later Canvas
  workflows.

Required corrections:

| File | Current code | Required behavior |
| --- | --- | --- |
| `internal/workflow/canvas.go:2216` | Tree selection uses export-readiness filtering | Tree refresh must use selected rows in visible order; runtime decides failures |
| `internal/workflow/canvas.go:633` | Sequence-choice error becomes empty choice | Surface the error or pass a clearly represented selected record; do not silently drop |
| `internal/workflow/canvas.go:1206` | Unknown kind can be rewritten to target kind | Keep metadata honest; target mode belongs in global settings, not inferred row biology |
| `internal/workflow/blast.go:10998` | General sequence sanitizer removes `*` | Decide whether strict tree input must bypass imported-FASTA cleanup or document this as pre-tree import normalization |
| `internal/workflow/canvas_tree_snapshot_test.go:686` | Test expects two selected rows reach refresh stub | Extend with real `InputFASTA`/runtime-plan checks so missing sequences cannot disappear later |

Closure:

- [x] `selectedCanvasRowsInOrder` and `selectedCanvasRowsInVisibleOrder` now
  preserve all checked rows for the tree path instead of calling
  `canvasRowHasSequenceForExport`.
- [x] Normal Canvas FASTA export uses
  `selectedCanvasRowsInCurrentOrderForExport`, keeping export-readiness
  filtering out of tree refresh runtime input.
- [x] Empty selected records are preserved through `InputFASTA`; the empty
  record header remains visible to MEGA runtime instead of disappearing.
- [x] Missing non-resolvable sequence payloads become explicit empty selected
  records; attempted source-resolution failures are returned as errors and are
  no longer swallowed.
- [x] Source construction failures from row metadata, such as unsupported
  snapshot/source database names, are returned directly instead of being
  treated as "no resolver".
- [x] Unknown row `SequenceKind` is kept as metadata. The global Protein/DNA
  target still controls the runtime request, but PHgo no longer rewrites row
  biology to match that target.
- [x] Tests added/updated:
  `TestSelectedCanvasRowsForExportFiltersRowsWithoutExportSequence`,
  `TestExportCanvasSelectionsPlainFastaSkipsRowsWithoutExportSequence`,
  `TestCanvasTreeRowSourcesReturnsNucleotideResolverErrors`, and
  `TestCanvasTreeDNAModeReturnsSourceConstructionErrors`.
  `TestSelectedCanvasRowsInOrderKeepsSelectedRowsWithoutExportReadySequence`,
  `TestNormalizeCanvasTreeRowSourcesPreservesUnknownRowKind`,
  `TestCanvasTreeDNAModeDoesNotInventNucleotideWhenResolverMisses`, and the
  existing `InputFASTA` empty-record regression.

Allowed PHgo responsibilities:

- Select checked rows.
- Preserve current visible order.
- Resolve already real nucleotide/CDS payloads by metadata/cache.
- Pass stable labels and metadata.

Forbidden PHgo responsibilities:

- Letter-based biological inference.
- Reverse translation.
- Local repair or validation of sequence content.
- Silent pre-runtime skipping.

## 5. TUI Parameter Rendering

Files read:

- `internal/tui/templates.go`
- `internal/prompt/prompt.go`

Current behavior:

- The Canvas system-tree panel is registry-driven.
- `canvasTreeMethodFromDefinition` copies registry parameter definitions into
  TUI controls.
- Picklists with two options are rendered as checkboxes by heuristic.
- Dynamic visibility is hard-coded for several parameter IDs:
  `rates_among_sites`, `initial_tree_for_ml`, `phylogeny_test`,
  `gaps_missing_data`, `ml_heuristic_method`.

Required corrections:

| File | Current code | Required behavior |
| --- | --- | --- |
| `internal/tui/templates.go:1315` | Align page renders all registry params | Only HTML-backed alignment params can be final |
| `internal/tui/templates.go:1461` | Tree page renders all registry tree params | Tree parameter rows must be HTML-backed or explicitly pending/hidden |
| `internal/tui/templates.go:1531` | Visibility rules are Go hard-codes | For align/tree parameters, dynamic visibility must be copied from MEGA 12.1 HTML behavior |
| `internal/tui/templates.go:1652` | Two-option picklists become checkboxes | This is acceptable only when it preserves the HTML control semantics and default state |
| `internal/prompt/prompt.go:5809` | Panel method lists come from registry | Registry must be HTML-correct before the panel can be called final |

UI freeze standard:

- Controls may be adapted to the TUI.
- Parameter item count, option strings, defaults, checkbox states, ranges, and
  dynamic visibility must be identical to MEGA 12.1 HTML for align/tree-type
  settings.

## 6. Snapshots, Reuse, And Viewer Boundary

Files read:

- `internal/workflow/session_snapshot.go`
- `internal/workflow/canvas_tree_snapshot_test.go`
- `internal/phylo/build.go`
- `internal/phylo/artifact.go`

Current behavior:

- Snapshot restoration preserves tree artifacts and sets the next explicit
  refresh to full compute.
- Display-name-only refresh can reuse runtime artifacts when compute
  fingerprints match.
- Snapshot restore does not biologically validate aligned FASTA payloads.
- The old Canvas "converted FASTA" export was removed from the UI/runtime path
  because it was export-only post-processing over aligned FASTA, not a MEGA
  GUI-equivalent conversion workflow.
- Canvas tree panel open, preview open, refresh preflight, and missing-tool
  retry preflight now use task/progress modals before runtime checks or viewer
  startup.
- Canvas/Explore tree viewer browser launch is detached from short-lived task
  modal contexts, so the progress modal closing cannot kill the OS browser
  launcher before the preview opens.
- Exact kind-specific alignment variants preserve edited MEGA parameters during
  normalization, so protein ClustalW/MUSCLE parameter changes remain compute
  fingerprint inputs and force full runtime refresh.

Required corrections:

| Area | Required behavior |
| --- | --- |
| Snapshot open | Restore artifacts/payload without recomputing; first explicit refresh recomputes |
| Reuse | Require manifest plus canonical runtime artifacts; never use artifact scanning that can pick `input.fasta` |
| Converted FASTA export | Removed from Canvas export UI/runtime path because PHgo must not insert a non-MEGA conversion step |
| Loading dialogs | Tree panel, preview, refresh preflight, and missing-tool retry preflight run through task/progress UI |
| Browser launch | System browser launch honors pre-start cancellation but is not owned by the task-modal context after launch starts |
| Parameter reuse | Alignment/tree parameter changes must alter compute fingerprints; only display-name changes may render-only reuse |
| Display names | Preview-only changes must not alter alignment/tree fingerprints |

## 7. Immediate Fix Order

1. [x] Fix ClustalW registry defaults/options to HTML.
2. [x] Update ClustalW registry tests to HTML.
3. [x] Stop `findAlignedFASTA` and `findNewick` from scanning arbitrary files.
4. [x] Stop selected-row pre-filtering and empty-sequence silent drops before
   runtime.
5. [x] Add regression tests for missing aligned/newick artifacts and selected rows
   with missing/empty sequence payloads.
6. [x] Register MUSCLE parameter UI from screenshots/source/runtime keys because
   no registered HTML dialog exists; ignore JSON.
7. [x] Register tree-type parameter UI from MEGA analysis-preference
   source/screenshots; include MP Min-Mini and Max-mini options and dynamic
   visibility.
8. [x] Rebuild `mega-phgo-runtime.bin` with Lazarus/FPC so the Windows runtime
   binary contains the updated MP Min-Mini/Max-mini runtime source path.
9. [x] Re-smoke Maximum Parsimony runtime branches with informative data:
   SPR/TBR path, Min-Mini, and Max-mini produce Newick; 5-rep bootstrap probes
   for all three paths produce Newick with support values.
10. [x] Restore MEGA's MP data-preparation gate: no parsimony-informative sites
    now returns the runtime error `There are no parsimony informative sites`
    before any MP search path can emit a tree.
11. [x] Fix ML runtime data preparation: prepared ML names/sequences now come
    from MEGA's subset/deletion path without the GUI-only wrapper that caused
    `Access violation` in the PHgo runtime.
12. [x] Wire ML `Make multiple initial trees automatically (Maximum Parsimony)`
    to MEGA's `TMLTreeSearchThread.ExecuteMultiStartTreeSearch`, including the
    non-visual `TProcessPack.TextualSettingsList['No. of Initial Trees']`
    value required by `TAnalysisInfo.NumStartingMpTrees`.
13. [x] Real-runtime probes passed after rebuild: `go test ./...`; targeted
    runtime probes for ML branch-swap, NJ/UPGMA/ME bootstrap, ML bootstrap, and
    MP bootstrap; manual smoke probes for ML default, ML multiple-start, ML
    bootstrap, and MP SPR/Min-Mini/Max-mini.
14. [x] Renderer/PGV baseline passed after runtime changes:
    `npm test` and `npm run build` in `tree-viewer`.
15. [x] Distance-tree source audit recorded: ME `ME Search Level` is stored as
    `TTreePack.SearchFactor`, but MEGA 12.1 `ConstructMETree` only passes
    `MaxTrees=1` to `TMeTreeSearchThread`; no downstream `SearchFactor` read
    exists in `mtreesearchthread.pas`.
16. [x] Add tree-parameter directness matrix covering every PHgo tree UI row:
    consumed runtime keys, display-only rows, MEGA-recorded/no-downstream rows,
    and compatibility-only runtime fallbacks.
17. [x] Add automated directness guards:
    `internal/phylo/registry_test.go` now fails if any tree UI parameter lacks
    a runtime/display/source classification, and `internal/phylo/run_test.go`
    now fails if editable MEGA defaults do not reach `runtime-request.json`.
18. [x] Complete viewer `.pgv` save/read loop:
    `tree-viewer/src/pgv.js` validates imported snapshots, `main.jsx` restores
    payload and Reactree state in local snapshot mode, and the synced embedded
    viewer assets include the new reader.
19. [x] Close Canvas selected-row/source-resolution boundary:
    selected tree rows are no longer filtered by export-readiness helpers,
    normal FASTA export keeps a separate export-only filter, unknown row
    sequence kind is preserved, empty selected records still reach
    `input.fasta`, and real source-resolution/source-construction errors are
    surfaced.
20. [x] Close runtime cancellation/failure handling:
    cancellation before request writing returns `context canceled` without
    writing `runtime-request.json`; cancellation during runtime execution keeps
    stdout/stderr artifacts; runtime `error_text` still wins over secondary
    artifact checks.
21. [x] Close viewer/session isolation:
    Canvas viewer close stops the Canvas-scoped server, browser `.pgv` import
    enters local snapshot mode, and viewer payload/state are isolated per
    session ID for Canvas and standalone `.nwk/.pgv` browser sessions.
22. [x] Re-run real-runtime freeze probes:
    Protein ClustalW/MUSCLE, DNA ClustalW/MUSCLE, codon ClustalW/MUSCLE,
    Protein/DNA NJ/ME/UPGMA/ML/MP, ML branch-swap, distance bootstrap,
    Minimum Evolution bootstrap, and MP bootstrap all produced runtime output
    through `mega-phgo-runtime`.

## 8. Confidence Ledger

| Area | Current confidence | Reason |
| --- | --- | --- |
| ClustalW DNA/protein/codon parameter UI | High | HTML files are registered and inspected |
| MUSCLE runtime execution | Medium-high | Runtime-owned MUSCLE call path and flags are in `mega-phgo-runtime`; external MUSCLE source is secondary |
| MUSCLE parameter UI/defaults | High for current registry | Real MEGA screenshots match source/runtime keys; no JSON authority used |
| NJ/ME/UPGMA/ML/MP runtime ownership | High | MEGA source/runtime thread anchors exist and Windows runtime binary rebuild/probe passed |
| NJ/ME/UPGMA/ML/MP parameter UI/defaults | High for current registry | MEGA 12.1 analysis-preference source plus screenshots |
| Artifact/snapshot boundary | High | Canonical artifact acceptance, `.pgo` tree artifacts, `.pgv` read/write, viewer session isolation, and cancellation artifact retention are covered |
| Selected-row handoff | High | Checked rows are not export-filtered; empty rows reach runtime; source-resolution errors surface |
| Runtime parameter directness | High | Registry/request tests guard every editable tree/alignment default and every tree UI parameter classification |
| Viewer `.pgv` snapshot loop | High | Save/read parser tests pass and embedded viewer assets have been rebuilt |
