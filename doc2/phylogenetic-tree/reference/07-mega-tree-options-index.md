# MEGA 12.1 Tree Options Index

This file is the current source/runtime index for PHgo Canvas system-tree tree
methods. It is useful for runtime behavior and current registry review, but it
is not a final parameter UI/default authority until MEGA 12.1 HTML
analysis-options dialogs are found. It is derived from MEGA 12.1 source,
primarily:

- `_mega_source/MEGA12.1-source/MegaDlgs/manalysisprefdlg.pas`
- `_mega_source/MEGA12.1-source/Common/megaanalysisprefstrings.pas`
- `_mega_source/MEGA12.1-source/Common/manalysisinfo.pas`
- `_mega_source/MEGA12.1-source/Process/processtreecmds.pas`
- `_mega_source/MEGA12.1-source/Process/processmltreecmds.pas`
- `_mega_source/MEGA12.1-source/Process/processparsimtreecmds.pas`
- `_mega_source/MEGA12.1-source/TamuraFuncs/mlsearchthread.pas`
- `_mega_source/MEGA12.1-source/TamuraFuncs/mltreeanalyzer.pas`
- `_mega_source/MEGA12.1-source/TamuraFuncs/mtreesearchthread.pas`
- `_mega_source/MEGA12.1-source/TamuraFuncs/mptree.pas`

PHgo may render a two-choice MEGA picklist as a checkbox, but the stored value
must remain the MEGA string. For final align/tree-type parameter UI parity,
the checkbox/picklist decision, option order, and default state must come from
MEGA 12.1 HTML.

HTML status: this index is a runtime/source matrix only. A registered MEGA 12.1
HTML analysis-options dialog for NJ, ME, UPGMA, ML, or MP has not yet been
located. If that HTML source is found later, it becomes the UI/default
authority. Until then, the defaults and dynamic rules below must not be treated
as final PHgo UI/default parity; they describe current adapter behavior for
debugging and correction. MEGA option JSON files are ignored.

## Shared Dynamic Rules

- `Test of Phylogeny = None / Bootstrap method` is a checkbox in PHgo. Checked means `Bootstrap method`; unchecked means `None`.
- `Bootstrap Replicates` is shown only when `Test of Phylogeny` is bootstrap. Default `500`, min `1`, max `10000`, increment `100`.
- Runtime status: Neighbor-Joining, Minimum-Evolution, and UPGMA distance bootstrap, maximum-likelihood bootstrap, and maximum-parsimony SPR/TBR bootstrap are wired to MEGA runtime bootstrap thread classes. Minimum Evolution uses the MEGA `TBootstrapMEThread` path and must be treated as a real runtime computation, not a PHgo-local fallback.
- Distance-tree `Gaps/Missing Data` values are `Complete deletion`, `Pairwise deletion`, `Partial deletion`; default `Pairwise deletion`.
- ML/MP `Gaps/Missing Data` values are `Complete deletion`, `Use all sites`, `Partial deletion`; default `Use all sites`.
- `Site Coverage Cutoff (%)` is shown only for `Partial deletion`. Default `95`, min `5`, max `100`, increment `5`.
- Distance-tree `Rates among Sites` values are `Uniform Rates`, `Gamma Distributed (G)`.
- ML `Rates among Sites` values are `Uniform Rates`, `Gamma Distributed (G)`, `Has Invariant Sites (I)`, `Gamma Distributed With Invariant Sites (G+I)`.
- `Gamma Parameter` is shown only for gamma distance-tree rates. Default `1.0`, min `0.05`, max `999`, increment `0.1`.
- ML `No of Discrete Gamma Categories` is shown for gamma-bearing ML rates. Default `5`, min `2`, max `16`.
- Distance-tree `Number of Threads` is applicable only for bootstrap. ML `Number of Threads` is always applicable. MEGA default is `GetDefaultNoOfProcessors`, effectively CPU count minus one when more than one CPU is available.
- `Initial Tree File` is shown only when ML `Initial Tree for ML = Use tree from file`.
- `No. of Initial Trees` is shown only when ML `Initial Tree for ML = Make multiple initial trees automatically (Maximum Parsimony)`.

## Distance Trees

Applies to Neighbor-Joining, Minimum Evolution, and UPGMA.

Shared rows:

- `Statistical Method`: method label, read-only.
- `Test of Phylogeny`: `None`, `Bootstrap method`; default `None`.
- `Substitutions Type`: `Amino acid` for protein, `Nucleotide` for DNA; read-only.
- `Rates among Sites`: `Uniform Rates`, `Gamma Distributed (G)`; default `Uniform Rates`.
- `Pattern among Lineages`: `Same (Homogeneous)`, `Different (Heterogeneous)`; default `Same (Homogeneous)`.
- `Gaps/Missing Data`: `Complete deletion`, `Pairwise deletion`, `Partial deletion`; default `Pairwise deletion`.

Protein distance model values:

- `No. of differences`
- `p-distance`
- `Poisson model` default
- `Equal input model`
- `Dayhoff model`
- `Jones-Taylor-Thornton (JTT) model`

DNA distance model values:

- `No. of differences`
- `p-distance`
- `Jukes-Cantor model`
- `Kimura 2-parameter model`
- `Tajima-Nei model`
- `Tamura 3-parameter model`
- `Tamura-Nei model`
- `Maximum Composite Likelihood` default
- `LogDet (Tamura-Kumar)`

Minimum Evolution extra rows:

- `ME Heuristic Method`: `Close-Neighbor-Interchange (CNI)`, read-only.
- `Initial Tree for ME`: `Obtain initial tree by Neighbor-Joining`, read-only.
- `ME Search Level`: default `1`, min `1`, max `2`, increment `1`.
- Source note: MEGA stores `ME Search Level` as `TTreePack.SearchFactor`, but
  `ConstructMETree` passes only `MaxTrees=1` to `TMeTreeSearchThread`; no
  downstream `SearchFactor` read exists in `mtreesearchthread.pas`.

## Maximum Likelihood

Shared rows:

- `Statistical Method`: `Maximum Likelihood`, read-only.
- `Test of Phylogeny`: `None`, `Bootstrap method`; default `None`.
- `Substitutions Type`: `Amino acid` for protein, `Nucleotide` for DNA; read-only.
- `Rates among Sites`: `Uniform Rates`, `Gamma Distributed (G)`, `Has Invariant Sites (I)`, `Gamma Distributed With Invariant Sites (G+I)`; default `Uniform Rates`.
- `Gaps/Missing Data`: `Complete deletion`, `Use all sites`, `Partial deletion`; default `Use all sites`.
- `ML Heuristic Method`: `Nearest-Neighbor-Interchange (NNI)`, `Subtree-Pruning-Regrafting - Fast (SPR level 3)`, `Subtree-Pruning-Regrafting - Extensive (SPR level 5)`; default `NNI`.
- `Initial Tree for ML`: default `Make initial tree automatically (Default - NJ/MP)`.
- `Branch Swap Filter`: `None`, `Weak`, `Moderate`, `Strong`; default `None`.
- `Number of Threads`: MEGA processor default.

ML initial-tree values:

- `Make initial tree automatically (Default - NJ/MP)`
- `Make initial tree automatically (Maximum Parsimony)`
- `Make multiple initial trees automatically (Maximum Parsimony)`
- `Make initial tree automatically (Neighbor Joining)`
- `Use tree from file`
- `Use Topology Editor` exists in MEGA GUI through the topology editor path, but PHgo must not expose it unless an equivalent non-GUI runtime input path is implemented.

ML branch-swap runtime mapping:

- `None` -> `1.0 + FP_CUTOFF`
- `Weak` -> `0.9`
- `Moderate` -> `0.7`
- `Strong` -> `0.5`

Protein ML model values:

- `Poisson model`
- `Equal input model`
- `Dayhoff model`
- `Dayhoff model with Freqs. (+F)`
- `Jones-Taylor-Thornton (JTT) model` default
- `JTT with Freqs. (+F) model`
- `WAG model`
- `WAG with Freqs. (+F) model`
- `LG model`
- `LG with Freqs. (+F) model`
- `General Reversible Mitochondrial (mtREV)`
- `mtREV with Freqs. (+F) model`
- `General Reversible Chloroplast (cpREV)`
- `cpREV with Freqs. (+F) model`
- `General Reverse Transcriptase model (rtREV)`
- `rtREV with Freqs. (+F) model`

DNA ML model values:

- `Jukes-Cantor model`
- `Kimura 2-parameter model`
- `Tamura 3-parameter model`
- `Hasegawa-Kishino-Yano model`
- `Tamura-Nei model` default
- `General Time Reversible model`

## Maximum Parsimony

Shared rows:

- `Statistical Method`: `Maximum Parsimony`, read-only.
- `Test of Phylogeny`: `None`, `Bootstrap method`; default `None`.
- `Substitutions Type`: `Amino acid` for protein, `Nucleotide` for DNA; read-only.
- `Gaps/Missing Data`: `Complete deletion`, `Use all sites`, `Partial deletion`; default `Use all sites`.
- `MP Search Method`: MEGA picklist is `Subtree-Pruning-Regrafting (SPR)`, `Tree-Bisection-Reconnection (TBR)`, `Min-Mini Heuristic`, `Max-mini Branch-&-bound`; PHgo exposes all four values.
- `No. of Initial Trees (random addition)`: default `10`, min `1`, max `100000`, increment `1`.
- `MP Search Level`: default `1`, min `1`, max `5` for SPR/TBR, increment `1`.
- `Max No. of Trees to Retain`: default `100`, min `1`, max `10000`, increment `1`.
- Runtime status: SPR/TBR inference and bootstrap call MEGA's `TMPTree` / `TBootstrapMPTreeSearchThread` path. Min-Mini and Max-mini are registered in the UI and connected in `PHgoRuntime/mega-phgo-runtime.lpr` to MEGA's `TMiniMini_CNISearchThread` / `TBootstrapMiniMini_CNISearchThread` and `TBranchBoundSearchThread` / `TBootstrapBranchBoundSearchThread` source paths. The rebuilt Windows runtime was smoke-tested with informative MP data: SPR, Min-Mini, and Max-mini all produced Newick, and bootstrap probes with 5 reps produced Newick with support values.
- MEGA data gate: MP runtime must reject alignments with no parsimony-informative sites before tree search, matching `processparsimtreecmds.pas` (`There are no parsimony informative sites`). PHgo must surface that runtime error instead of accepting an arbitrary tree.

## Runtime Smoke Status

- ML default initial-tree search: rebuilt runtime produced Newick through
  `TMLTreeAnalyzer` / `TPHgoMLTreeSearchThread`.
- ML multiple initial trees: rebuilt runtime produced Newick through MEGA's
  `TMLTreeSearchThread.ExecuteMultiStartTreeSearch`; `number_of_initial_trees`
  is written into `TProcessPack.TextualSettingsList['No. of Initial Trees']`,
  matching the non-visual MEGA getter path.
- ML bootstrap: rebuilt runtime produced Newick with a 5-rep smoke probe.
- Distance bootstrap: real-runtime probes passed for NJ, UPGMA, and ME.
- MP bootstrap: real-runtime probes passed for ML and MP test coverage, and
  manual MP search-method probes passed for SPR, Min-Mini, and Max-mini.
