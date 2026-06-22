# Runtime Action Flow

This document describes behavior that has no MEGA 12.1 HTML parameter-setting
surface. Evidence source: `_mega_source/MEGA12.1-source/PHgoRuntime/mega-phgo-runtime.lpr`
plus the MEGA 12.1 units it calls. Confidence for these runtime facts is high
because they are direct source reads. This document is not a substitute for
HTML when defining align/tree-type parameter UI/defaults.

## Hard Boundary

PHgo may create labels, FASTA text, metadata, runtime requests, snapshots, and
viewer payloads. The runtime owns alignment, codon translation/untranslation for
codon alignment, distance preparation, tree search, bootstrap, aligned FASTA,
Newick, summary, runtime log, and runtime error text.

No PHgo-local biological fallback is allowed for alignment, distance matrices,
bootstrap, tree recursion, Newick construction, residue repair, stop-codon
cleanup, sequence-kind guessing, reverse translation, or tree-method emulation.

## CLI And Probe

Runtime executable: `mega-phgo-runtime`.

Windows packaging note: the release bundle stores the runtime process image as
`mega-phgo-runtime.bin` beside `muscleWin64.bin`, but PHgo does not execute that
asset path directly. `megaphgo.PrepareExecution` creates a temporary
`mega-phgo-runtime.exe` and copies the runtime-owned MUSCLE binary into the same
temporary directory before probe or request execution. Source-level debugging may
run the built `_mega_source/.../mega-phgo-runtime.exe` directly.

Entry behavior:

1. With exactly one argument equal to `--phgo-runtime-probe`, write
   `mega-phgo-runtime probe ok` and exit `0`.
2. With any argument count other than one, fail with
   `usage: mega-phgo-runtime <runtime-request.json>`.
3. With one request path, call `RunRequest`.
4. On exception, write `mega-phgo-runtime: <message>` to stderr and exit `1`.

## Request Parsing

`ParseRequest` reads the JSON request file as a root object.

Top-level fields:

- `schema_version`
- `session_id`
- `run_id`
- `sequence_kind`
- `input_fasta`
- `records[]`

`settings` fields:

- `conversion_target`, defaulting to `sequence_kind`
- `alignment_method`, default `clustalw`
- `alignment_params`, stored as raw JSON object text
- `tree_method`, default `neighbor_joining`
- `tree_params`, stored as raw JSON object text

`artifacts` fields:

- `base_dir`, default request directory
- `input_fasta`
- `metadata_json`
- `aligned_fasta`
- `newick`
- `summary`
- `runtime_log`

Artifact paths are normalized relative to `base_dir` unless already absolute.
Records are copied into runtime structs with `taxon_id`, `display_name`,
`sequence_kind`, `canvas_item`, and `canvas_row`. Current runtime code does not
use those records for biological filtering.

Canvas row coordinate display is a PHgo metadata/viewer concern, not a runtime
choice. When enabled, PHgo prefixes metadata display labels with
`[canvas item,row]` before writing the viewer payload; it does not alter the
runtime taxon IDs, runtime FASTA headers, or MEGA computation inputs.

## FASTA Handling

`ParseFasta`:

- Splits input by lines.
- Ignores blank lines.
- Starts a record on `>`.
- Header is the trimmed text after `>`.
- Sequence lines are trimmed, uppercased, and concatenated.
- On header flush, the record is added even if the sequence is empty.

This parser does not repair residues, trim stops, remove invalid letters, or
skip biologically incompatible rows. Empty sequence acceptance is a current
runtime behavior that PHgo should surface via runtime failure rather than mask
with local filtering.

`FastaFromLists` writes each record as:

```text
>name
wrapped sequence at 80 chars
```

## Target Kind

`TargetSequenceKind` normalizes `conversion_target`:

- empty, `protein`, `aa`, `amino_acid` -> `protein`
- `dna`, `nucleotide` -> `nucleotide`
- otherwise falls back to `sequence_kind`; `dna` becomes `nucleotide`, anything
  not nucleotide becomes protein.

`RuntimeTreeUsesNucleotide` is true only when normalized target kind is
`nucleotide`.

## Alignment Dispatch

`RunAlignment`:

- empty, `clustalw`, `clustalw_codons` -> `RunMegaClustalWAlignment`
- `muscle`, `muscle_codons` -> `RunRuntimeOwnedMuscleAlignment`
- anything else -> `unsupported alignment method`

At least two sequences are required before either aligner runs.

## ClustalW Runtime Flow

MEGA component: `TClustalWThread` from MEGA 12.1 source.

Steps:

1. Validate method is `clustalw` or `clustalw_codons`.
2. For `clustalw_codons`, call `ApplyMegaCodonGapReset` unless
   `keep_predefined_gaps=true`, then call `PrepareCodonAlignment`.
3. Log method, target kind, taxon count, min sequence length, max sequence
   length.
4. Create `TClustalWThread`.
5. Set `IsDNA` only for nucleotide non-codon ClustalW.
6. Assign `SeqList` and `SeqNames` directly to the MEGA thread.
7. Map runtime parameter JSON to MEGA thread fields.
8. If `guide_tree` is non-empty, resolve it as either an existing path or raw
   Newick text saved to `guide-tree.nwk`; import via `TTreeList`, copy first
   tree to `TTreeData`, and call `Thread.SetTreeData`.
9. Start thread, wait for completion, check `Thread.Canceled`.
10. For codons, call `FinishCodonAlignment` to untranslate aligned amino-acid
    sequences back to codon alignment.

Runtime parameter keys consumed by ClustalW:

| Runtime key | MEGA field | Runtime fallback |
| --- | --- | --- |
| `pairwise_gap_opening_penalty` | DNA/protein pairwise gap opening | DNA `15.0`, protein `10.0` |
| `pairwise_gap_extension_penalty` | DNA/protein pairwise gap extension | DNA `6.66`, protein `0.1` |
| `multiple_gap_opening_penalty` | DNA/protein multiple gap opening | DNA `15.0`, protein `10.0` |
| `multiple_gap_extension_penalty` | DNA/protein multiple gap extension | DNA `6.66`, protein `0.2` |
| `dna_weight_matrix` | DNA matrix | `IUB` |
| `transition_weight` | transition weight | `0.5` |
| `protein_weight_matrix` | protein matrix | runtime fallback `BLOSUM` |
| `residue_specific_penalties` | residue-specific penalty | `true` |
| `hydrophilic_penalties` | hydrophilic penalty | `true` |
| `gap_separation_distance` | gap separation | `4` |
| `end_gap_separation` | end gap separation | `false` |
| `use_negative_matrix` | negative matrix | `false` |
| `delay_divergent_cutoff` | divergent cutoff | `30` |
| `guide_tree` | guide tree file/text | empty |
| `genetic_code` | codon code lookup | `Standard` |
| `keep_predefined_gaps` | codon pre-gap reset | `false` |

Important drift: runtime fallback values are not automatically MEGA HTML
defaults. For final PHgo UI, ClustalW defaults must come from
`11-mega12-html-control-index.md`. PHgo should pass explicit HTML-derived
values so runtime fallbacks do not silently override the MEGA 12.1 HTML surface.

## Codon Alignment Runtime Flow

`PrepareCodonAlignment`:

1. Reads `genetic_code`, default `Standard`.
2. Looks up the table by MEGA default code table names; unknown names fall back
   to `GetStandardGeneticCode`.
3. Creates `TSequenceList`, marks it DNA and protein-coding.
4. Copies each original nucleotide sequence into a MEGA `TSequence`.
5. Calls `SeqList.Translate`.
6. Replaces runtime sequence strings with translated amino-acid strings for
   alignment.

`FinishCodonAlignment`:

1. Copies aligned amino-acid strings back into the MEGA sequence list.
2. Calls `SeqList.UnTranslate`.
3. Replaces runtime sequence strings with aligned codon strings.

`ApplyMegaCodonGapReset`:

- If `keep_predefined_gaps=true`, do nothing.
- Otherwise strip gaps from every input sequence using MEGA `StripGaps`.
- If a sequence becomes empty, fail with a MEGA ClustalW gap-reset error.

## MUSCLE Runtime Flow

MUSCLE is runtime-owned, not launched by PHgo.

Executable resolution:

- Windows: runtime directory + `muscleWin64.bin`
- macOS: runtime directory + `muscledarwin64`
- other: runtime directory + `muscleUnix64.exe`

Steps:

1. Validate method is `muscle` or `muscle_codons`.
2. Require the runtime-owned MUSCLE executable to exist.
3. For codons, call `PrepareCodonAlignment`.
4. Write `muscle-input.fasta` in artifact base dir.
5. Execute MUSCLE with `poWaitOnExit`.
6. Require zero exit status.
7. Require `muscle-output.fasta` to exist.
8. Parse output FASTA and require the same sequence count.
9. For codons, call `FinishCodonAlignment`.

Runtime parameter keys passed to MUSCLE:

| Runtime key | MUSCLE flag | Fallback |
| --- | --- | --- |
| `gap_open` | `-gapopen` | `-2.9` |
| `gap_extend` | `-gapextend` | `0.0` |
| `hydrophobicity_multiplier` | `-hydrofactor` | `1.2` |
| `max_memory_mb` | `-maxmb` | `2048` |
| `max_iterations` | `-maxiters` | `16` |
| `cluster_method_iterations_1_2` | `-cluster1` | `UPGMA` -> `upgma` |
| `cluster_method_other_iterations` | `-cluster2` | `UPGMA` -> `upgma` |
| `min_diag_length_lambda` | `-diaglength` | `24` |

`-quiet`, `-in`, `-out`, and `-log` are always supplied. No registered MEGA
12.1 MUSCLE parameter HTML dialog was found, so these flags are runtime behavior
only, not final MEGA UI parity.

## Data Preparation

Runtime creates MEGA `TD_InputSeqData` without GUI:

- `IsNuc` / `IsAmino` from target kind.
- `IsCoding=false` for tree analysis data preparation.
- Gap symbol `-`, missing symbol `?`, identity symbol `.`.
- `NoOfTaxa`, `NoOfSites`, `NoOfNucSites`, `NoOfSitesUsed` from aligned data.
- One `TOtuInfo` per sequence with `Name`, `RsvName`, `IsUsed=true`, and raw
  sequence buffer.
- `D_InputSeqData` is assigned while preparing, then cleared.

Data subset behavior:

- Distance prep calls `PrepareDataForDistAnalysis`.
- Parsimony prep calls `PrepareDataForParsimAnalysis`.
- ML prep uses the same MEGA site/taxon subset machinery through
  `PrepareDataForDistAnalysis` and then passes those prepared names/sequences to
  MEGA ML classes. The GUI-only `PrepareDataForMLAnalysis(TAnalysisInfo)` wrapper
  is not used in PHgoRuntime because it dereferenced GUI/non-visual analysis
  context and caused runtime access violations.
- Nucleotide prep adds `dsoUseNuc` and `dsoUseNonCod`; protein prep adds
  `dsoUseAmino`.
- Distance adds `dsoDistMap`; parsimony adds `dsoParsimMap`; ML adds `dsoNoMap`.
- `gaps_missing_data` maps by text:
  - contains `complete` -> complete deletion
  - contains `partial` -> partial deletion
  - contains `pairwise` -> pairwise deletion
- `site_coverage_cutoff` fallback is `95`, clamped to `0..100`.

Runtime fallback for `gaps_missing_data`:

- ML and MP: `Use all sites`
- NJ, ME, UPGMA: `Pairwise deletion`

Again, these are runtime adapter fallbacks, not MEGA HTML tree-parameter UI
defaults.

## Distance Model And Distance Pack

Methods using the distance path: Neighbor-Joining, Minimum Evolution, UPGMA.

`RuntimeTreeDistanceModel` maps text in `model_method` or `distance_model`:

- shared: empty or `p-distance` -> `gdPropDist`; `No.` or
  `number of differences` -> `gdNoOfDiff`
- nucleotide: `jukes` -> `gdJukesCantor`; `kimura` -> `gdKimura2para`;
  `tajima-nei` -> `gdTajimaNei`; `tamura 3` -> `gdTamura`;
  `tamura-nei` -> `gdTamuraNei`; `maximum composite` -> `gdMCL`;
  `logdet` -> `gdLogDet`
- protein: `poisson` -> `gdPoisson`; `equal input` -> `gdEqualInput`;
  `dayhoff` -> `gdDayhoff`; `jones` or `jtt` -> `gdJones`

`BuildMegaDistancePack` always adds `gdPairwise`, then `gdOneNuc` or `gdAmino`,
then the model. If `rates_among_sites` contains `gamma distributed`, it adds
`gdGamma` and sets `GammaParameter` from `gamma_parameter`, fallback `1.0`. If
`pattern_among_lineages` contains `heterogeneous`, it adds `gdHetero`.

## Distance Tree Flow

MEGA GUI source anchors:

- NJ: `Process/processtreecmds.pas::ConstructNJTree` creates
  `TNJTreeSearchThread`.
- UPGMA: `ConstructUPGMATree` creates `TUPGMATreeSearchThread`.
- ME: `ConstructMETree` creates `TMeTreeSearchThread`, sets
  `UseInitialNJTree := true`, and sets `MaxNoOfTrees := MAI.MyTreePack.MaxTrees`.

`BuildMegaDistanceTreeNewick`:

1. Require at least two sequences.
2. Build distance computer with `TNucDist` or `TAminoDist`.
3. Compute distances through MEGA distance classes.
4. Create `TTreeList` and copy OTU names.
5. If bootstrap selected, call `RunMegaDistanceBootstrap`.
6. Else:
   - `neighbor_joining`: `TTreeData` + `TNJTreeComputer.MakeTree`
   - `upgma`: `TTreeData` + `TUPGMATreeComputer.MakeTree`
   - `minimum_evolution`: create initial tree list, then
      `TMETreeComputer.Create(TreeList, Distances)`, set
      `UseInitialNJTree=true`, `MaxNoOfTrees=1`, `Threshold=1e-10`, call
      `MakeTree`
7. Export Newick via `TreeList.OutputNewickTree(0, true, bootstrapSelected, 0.0)`.

`me_search_level` source finding: MEGA UI row `opsMESearchLevelPanel2` is stored
by `TTreePack.ConstructTreePack` as `SearchFactor`, while `MaxTrees` is set to
`1`. MEGA 12.1 `ConstructMETree` passes only `MaxTrees` to
`TMeTreeSearchThread`; `SearchFactor` has no downstream read in
`mtreesearchthread.pas`. PHgo must preserve this source fact and must not invent
a non-MEGA algorithmic effect for the visible row.

Distance bootstrap:

- Requires at least four taxa.
- ME uses `TBootstrapMEThread`.
- NJ uses `TBootstrapNJThread`.
- UPGMA uses `TBootstrapUPGMAThread`.
- `NoOfThreads` from `number_of_threads`, fallback `1`, minimum `1`.
- `NoOfBootstraps` from `bootstrap_replicates`, fallback `500`, minimum `1`.
- Fails if canceled or if no bootstrap partition frequency is produced.

## Maximum Likelihood Flow

MEGA components: `TMLTreeAnalyzer`, ML model classes, `TMLTreeSearchThread`, and
`TBootstrapMLThread`.

Model mapping:

- nucleotide: `jukes` -> `TJCModel`; `kimura` -> `TK2Model`; `tamura 3` ->
  `TT3Model`; `hasegawa` -> `THKYModel`; empty or `tamura-nei` -> `TTN93Model`;
  `general time reversible` -> `TGTRModel`
- protein: empty, `jones`, or `jtt` -> `TJTTModel`; `dayhoff` ->
  `TDayhoffModel`; `wag` -> `TWAGModel`; `lg` -> `TLGModel`; `mtrev` ->
  `TmtREV24Model`; `cprev` -> `TcpREVModel`; `rtrev` -> `TrtREVModel`;
  `poisson` -> `TPoissonModel` without empirical frequencies; `equal input`
  -> `TPoissonModel` with equal input
- Protein `+F` is detected by `+f` in model text for the model classes that
  support empirical frequencies.

Rates:

- `rates_among_sites` containing `gamma distributed` sets gamma from
  `gamma_parameter`, fallback `1.0`.
- `rates_among_sites` containing `invariant` sets invariant-site use.
- `discrete_gamma_categories` fallback `5`, minimum `1`.

Search:

- `ml_heuristic_method` containing `extensive` -> search level `5`.
- containing `spr` or `level 3` -> search level `3`.
- otherwise -> search level `1`.
- `branch_swap_filter`: `None` -> `1.0 + FP_CUTOFF`; `Weak` -> `0.9`;
  `Moderate` -> `0.7`; `Strong` -> `0.5`.
- `initial_tree_for_ml`: use tree from file, multiple MP trees, NJ, MP, or
  default. User tree path/text is resolved through the same guide-tree resolver.

Non-bootstrap ML:

1. Prepare ML sequence data through MEGA's `PrepareDataForDistAnalysis`
   subset/deletion path and keep the resulting prepared names/sequences as the
   only input to ML search. The runtime must not use PHgo-side biological
   cleanup.
2. Create model and call `Model.SetParamsFromSeqs`.
3. Create `TMLTreeAnalyzer`.
4. Apply progress, search level, filter, initial tree option, and threads.
5. For `Make multiple initial trees automatically (Maximum Parsimony)`, build
   a MEGA `TAnalysisInfo` plus `TProcessPack.TextualSettingsList['No. of
   Initial Trees']`, then run MEGA's `TMLTreeSearchThread` so
   `ExecuteMultiStartTreeSearch` owns MP-start-tree generation and ML searches.
6. Otherwise, `TPHgoMLTreeSearchThread.RunPHgoSearch` calls `Initialize`,
   `PrepareSearchMLTree`, `Search`, checks cancellation, and reports success.
7. Export the resulting MEGA tree through `TTreeList.OutputNewickTree`.

Bootstrap ML:

1. Require at least four taxa.
2. Build `TAnalysisInfo`, `TTreePack`, `TDistPack`, and partition list from
   the prepared MEGA ML sequence data.
3. Add `ttML`, `ttInferTree`, `ttBootstrap`, plus `ttSPRFast`,
   `ttSPRExtensive`, or `ttNNI` from search level.
4. Run `TBootstrapMLThread`.
5. Require success and an original tree in `Info.MyOriTreeList`.
6. Export first original tree with bootstrap support values.

## Maximum Parsimony Flow

MEGA components: `TMPTree`, `TBootstrapMPTreeSearchThread`,
`TBranchBoundSearchThread`, `TBootstrapBranchBoundSearchThread`,
`TMiniMini_CNISearchThread`, and `TBootstrapMiniMini_CNISearchThread`.

Runtime mapping:

- Target nucleotide uses `Bits=8`; protein uses `Bits=32`.
- Parsimony data preparation follows `processparsimtreecmds.pas`: after
  `PrepareDataForParsimAnalysis`, runtime rejects no common sites, then rejects
  `InfoSites < 1` with `There are no parsimony informative sites`, then
  rejects fewer than four taxa for parsimony analysis.
- `mp_search_method` containing `tbr` -> `TBR` through `TMPTree`.
- containing `min-mini` -> MEGA Min-Mini CNI thread path.
- containing `max-mini` or `branch-&-bound` -> MEGA branch-and-bound thread path.
- otherwise -> `SPR` through `TMPTree`.
- `initial_trees_random_addition` fallback `10`.
- `mp_search_level` fallback `1`.
- `max_trees_to_retain` fallback `100`.

Non-bootstrap MP:

1. Prepare parsimony mapped data through MEGA and apply MEGA's no-common-site,
   no-informative-site, and minimum-four-taxa checks before any search thread.
2. For SPR/TBR, create `TMPTree`, apply search method, initial-tree count,
   search level, and max trees, then call `SearchMPTree`.
3. For Min-Mini, create `TMiniMini_CNISearchThread`, pass the MEGA mapped data,
   MP search-level/search-factor value, and max-tree retention, then start/wait.
4. For Max-mini Branch-&-bound, create `TBranchBoundSearchThread`, pass the
   MEGA mapped data and max-tree retention, then start/wait.
5. Copy or retain results in `TTreeList`.
6. Export Newick.

Bootstrap MP:

1. Require at least four taxa.
2. Prepare parsimony mapped data and apply the same MEGA MP data checks.
3. Build `TAnalysisInfo`, partition list, and runtime progress.
4. For SPR/TBR, create `TBootstrapMPTreeSearchThread` with mapped data, site
   count, bit width, and thread count, then apply bootstrap reps and MP search
   fields.
5. For Min-Mini, create `TBootstrapMiniMini_CNISearchThread`, pass bootstrap
   reps, MP search-level/search-factor, thread count, max-tree retention, and
   the partition list.
6. For Max-mini Branch-&-bound, create `TBootstrapBranchBoundSearchThread`,
   pass bootstrap reps, max-tree retention, and the partition list.
7. Start/wait; require nonzero bootstrap frequency.
8. Export Newick with bootstrap values.

## Artifact And Error Flow

`RunRequest` sequence:

1. Parse request.
2. Ensure artifact base dir exists.
3. Initialize runtime log with `mega-phgo-runtime started`.
4. Parse FASTA; log `fasta.parsed taxa=N`.
5. Require names and sequence counts match; require at least two FASTA records.
6. Run alignment.
7. Write `aligned.fasta`.
8. Build tree Newick.
9. Write `tree.nwk`.
10. Write `summary`.
11. Write `runtime-response.json` with no `error_text`.

On any exception after request parsing:

1. Write `runtime-response.json` with `error_text=<exception message>`.
2. Re-raise so CLI exits nonzero and stderr contains the runtime error.

Unsupported internal residues are runtime errors. Before writing the runtime
FASTA, PHgo strips terminal `*` markers from every sequence because a final `*`
is a terminator marker with no alignment-site meaning for PHgo's tree
computation. Internal `*` characters remain in the runtime input and are left
for MEGA to accept or reject. PHgo does not skip rows locally or retry with a
different alignment method.

`runtime-response.json` fields:

- `schema_version=1`
- `runtime=mega-phgo-runtime`
- `completed_at`
- `artifacts` object with base/input/meta/aligned/newick/summary/log paths
- optional `skipped_records`
- optional `error_text`

PHgo failure/cancellation wrapper:

- If the Go context is canceled before request writing, PHgo stops before
  creating `runtime-request.json`.
- If the context is canceled while the runtime process is executing,
  `exec.CommandContext` terminates the process, PHgo keeps
  `runtime.stdout.txt` and `runtime.stderr.txt`, and the user-facing error is
  `context canceled`.
- If the runtime writes `runtime-response.json.error_text`, PHgo surfaces that
  exact MEGA/runtime text before any missing-artifact checks.

The `Skipped` array currently remains empty in runtime flow; PHgo should not
invent skipped biological rows before MEGA executes.

## Log Messages To Preserve In Tests

Representative runtime log anchors:

- `fasta.parsed taxa=...`
- `clustalw.start ...`
- `clustalw.complete canceled=false`
- `clustalw.codons.reset_gaps applied=true`
- `tree.data_preparation kind=distance|parsimony|maximum_likelihood ...`
- `distance.bootstrap.start ...`
- `distance.bootstrap.complete ...`
- `maximum_likelihood.model created=true ...`
- `maximum_likelihood.start ...`
- `maximum_likelihood.search complete=true`
- `maximum_likelihood.bootstrap.start ...`
- `maximum_likelihood.bootstrap complete=true`
- `maximum_parsimony.bootstrap.start ...`
- `maximum_parsimony.bootstrap complete=true`

These log anchors are useful for real-runtime smoke tests because they prove the
MEGA component path executed instead of a PHgo-side fallback.
