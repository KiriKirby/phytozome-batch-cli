# Tree Inference Surface

This document records the MEGA-backed tree methods, the current PHgo runtime
parameter registry, dynamic rules, and runtime source anchors used by PHgo.

Primary sources:

- `_mega_source/MEGA12.1-source/Process/processtreecmds.pas`
- `_mega_source/MEGA12.1-source/Process/processmltreecmds.pas`
- `_mega_source/MEGA12.1-source/Process/processparsimtreecmds.pas`
- `_mega_source/MEGA12.1-source/Common/megaanalysisprefstrings.pas`
- `_mega_source/MEGA12.1-source/MegaDlgs/manalysisprefdlg.pas`
- `_mega_source/MEGA12.1-source/TamuraFuncs/mtreesearchthread.pas`
- `_mega_source/MEGA12.1-source/TamuraFuncs/mltreeanalyzer.pas`
- `_mega_source/MEGA12.1-source/TamuraFuncs/mptree.pas`
- `_mega_source/MEGA12.1-source/TamuraFuncs/parsimsearchthreads.pas`
- `_mega_source/MEGA12.1-source/PHgoRuntime/mega-phgo-runtime.lpr`
- `internal/phylo/registry.go` for the current PHgo registry only

Tree inference parameter source status: MEGA 12.1 does not register these
analysis-preference rows as the same HTML web dialogs used by ClustalW. The
canonical source for tree-type parameter rows is therefore the current MEGA
12.1 analysis-preference dialog code in `MegaDlgs/manalysisprefdlg.pas`, with
labels and picklists from `Common/megaanalysisprefstrings.pas`, checked against
the real MEGA screenshots in `C:/Users/wangsychn/Desktop/tree`. MEGA option
JSON files remain ignored for this UI/default decision.

For behavior such as which MEGA tree thread computes NJ, ME, UPGMA, ML, MP,
bootstrap, and Newick output, MEGA 12.1 source and `mega-phgo-runtime` are the
runtime authorities.

## Exposed Runtime-Backed Methods

| PHgo Method ID | Label | Runtime Method | Protein | DNA | Runtime Anchor |
| --- | --- | --- | --- | --- | --- |
| `neighbor_joining` | Neighbor-Joining | `neighbor_joining` | yes | yes | `TBootstrapNJThread`, distance tree path |
| `minimum_evolution` | Minimum Evolution | `minimum_evolution` | yes | yes | `TBootstrapMEThread`, CNI tree path |
| `upgma` | UPGMA | `upgma` | yes | yes | `TUPGMATreeComputer`, `TBootstrapUPGMAThread` |
| `maximum_likelihood` | Maximum Likelihood | `maximum_likelihood` | yes | yes | `TMLTreeAnalyzer`, `TBootstrapMLThread` |
| `maximum_parsimony` | Maximum Parsimony | `maximum_parsimony` | yes | yes | `TMPTree`, `TBootstrapMPTreeSearchThread`, `TBranchBoundSearchThread`, `TMiniMini_CNISearchThread` |

## Runtime Adapter Dynamic Rules

Source status: high confidence from `manalysisprefdlg.pas`,
`megaanalysisprefstrings.pas`, and real MEGA 12.1 screenshots.

- `Test of Phylogeny` values: `None`, `Bootstrap method`; default `None`.
- `Bootstrap Replicates` appears only for bootstrap; default `500`, range
  `1..10000`, step `100`.
- Distance-tree `Gaps/Missing Data`: `Complete deletion`, `Pairwise deletion`,
  `Partial deletion`; default `Pairwise deletion`.
- ML/MP `Gaps/Missing Data`: `Use all sites`, `Partial deletion`,
  `Complete deletion`; default `Use all sites`.
- `Site Coverage Cutoff (%)` appears only for `Partial deletion`; default `95`,
  range `5..100`, step `5`.
- Distance-tree `Rates among Sites`: `Uniform Rates`,
  `Gamma Distributed (G)`.
- Distance-tree `Gamma Parameter` appears only for gamma rates; default `1.0`,
  range `0.05..999`, step `0.1`.
- ML `Rates among Sites`: `Uniform Rates`, `Gamma Distributed (G)`,
  `Has Invariant Sites (I)`, `Gamma Distributed With Invariant Sites (G+I)`.
- ML `No of Discrete Gamma Categories` appears for gamma-bearing ML rates;
  default `5`, range `2..16`.
- Distance-tree `Number of Threads` is meaningful for bootstrap. ML and MP
  expose thread count for runtime-backed threaded search/bootstrap paths.
- MEGA thread default is processor count minus one when more than one processor
  is available, never below one.
- MP dynamic rows follow `UpdateTreeSearchRows`: SPR/TBR show random-addition
  initial trees and MP search level; Min-Mini hides random-addition initial
  trees but keeps the MP search level row; Max-mini Branch-&-bound hides both
  random-addition initial trees and MP search level. Max-tree retention remains
  visible.

## Distance Tree Methods

Applies to Neighbor-Joining, Minimum Evolution, and UPGMA.

| Group | Parameter ID | MEGA Label | Type | Protein Default | DNA Default |
| --- | --- | --- | --- | --- | --- |
| Analysis | `scope` | Scope | read-only string | `All Selected Taxa` | `All Selected Taxa` |
| Analysis | `statistical_method` | Statistical Method | read-only picklist | method label | method label |
| Phylogeny Test | `phylogeny_test` | Test of Phylogeny | picklist | `None` | `None` |
| Phylogeny Test | `bootstrap_replicates` | Bootstrap Replicates | integer | `500` | `500` |
| Substitution Model | `substitutions_type` | Substitutions Type | read-only picklist | `Amino acid` | `Nucleotide` |
| Substitution Model | `model_method` | Model/Method | picklist | `Poisson model` | `Maximum Composite Likelihood` |
| Rates and Patterns | `rates_among_sites` | Rates among Sites | picklist | `Uniform Rates` | `Uniform Rates` |
| Rates and Patterns | `gamma_parameter` | Gamma Parameter | float | `1.0` | `1.0` |
| Rates and Patterns | `pattern_among_lineages` | Pattern among Lineages | picklist | `Same (Homogeneous)` | `Same (Homogeneous)` |
| Data Subset To Use | `gaps_missing_data` | Gaps/Missing Data | picklist | `Pairwise deletion` | `Pairwise deletion` |
| Data Subset To Use | `site_coverage_cutoff` | Site Coverage Cutoff (%) | integer | `95` | `95` |
| System Resource Usage | `number_of_threads` | Number of Threads | integer | MEGA CPU default | MEGA CPU default |

### Protein Distance Models

`No. of differences`, `p-distance`, `Poisson model`, `Equal input model`,
`Dayhoff model`, `Jones-Taylor-Thornton (JTT) model`.

### DNA Distance Models

`No. of differences`, `p-distance`, `Jukes-Cantor model`,
`Kimura 2-parameter model`, `Tajima-Nei model`, `Tamura 3-parameter model`,
`Tamura-Nei model`, `Maximum Composite Likelihood`,
`LogDet (Tamura-Kumar)`.

### Minimum Evolution Extra Rows

| Parameter ID | MEGA Label | Type | Default / Value |
| --- | --- | --- | --- |
| `me_heuristic_method` | ME Heuristic Method | read-only picklist | `Close-Neighbor-Interchange (CNI)` |
| `initial_tree_for_me` | Initial Tree for ME | read-only picklist | `Obtain initial tree by Neighbor-joining` |
| `me_search_level` | ME Search Level | integer | `1`, range `1..2`, step `1` |

## Maximum Likelihood

| Group | Parameter ID | MEGA Label | Type | Protein Default | DNA Default |
| --- | --- | --- | --- | --- | --- |
| Analysis | `statistical_method` | Statistical Method | read-only picklist | `Maximum Likelihood` | `Maximum Likelihood` |
| Phylogeny Test | `phylogeny_test` | Test of Phylogeny | picklist | `None` | `None` |
| Phylogeny Test | `bootstrap_replicates` | Bootstrap Replicates | integer | `500` | `500` |
| Substitution Model | `substitutions_type` | Substitutions Type | read-only picklist | `Amino acid` | `Nucleotide` |
| Substitution Model | `model_method` | Model/Method | picklist | `Jones-Taylor-Thornton (JTT) model` | `Tamura-Nei model` |
| Rates and Patterns | `rates_among_sites` | Rates among Sites | picklist | `Uniform Rates` | `Uniform Rates` |
| Rates and Patterns | `discrete_gamma_categories` | No of Discrete Gamma Categories | integer | `5` | `5` |
| Data Subset To Use | `gaps_missing_data` | Gaps/Missing Data | picklist | `Use all sites` | `Use all sites` |
| Data Subset To Use | `site_coverage_cutoff` | Site Coverage Cutoff (%) | integer | `95` | `95` |
| Tree Inference Options | `ml_heuristic_method` | ML Heuristic Method | picklist | `Nearest-Neighbor-Interchange (NNI)` | same |
| Tree Inference Options | `initial_tree_for_ml` | Initial Tree for ML | picklist | `Make initial tree automatically (Default - NJ/MP)` | same |
| Tree Inference Options | `number_of_initial_trees` | No. of Initial Trees | integer | `10` | `10` |
| Tree Inference Options | `initial_tree_file` | Initial Tree File | string | empty | empty |
| Tree Inference Options | `branch_swap_filter` | Branch Swap Filter | picklist | `None` | `None` |
| System Resource Usage | `number_of_threads` | Number of Threads | integer | MEGA CPU default | MEGA CPU default |

### ML Protein Models

`Poisson model`, `Equal input model`, `Dayhoff model`,
`Dayhoff model with Freqs. (+F)`, `Jones-Taylor-Thornton (JTT) model`,
`JTT with Freqs. (+F) model`, `WAG model`, `WAG with Freqs. (+F) model`,
`LG model`, `LG with Freqs. (+F) model`,
`General Reversible Mitochondrial (mtREV)`,
`mtREV with Freqs. (+F) model`,
`General Reversible Chloroplast (cpREV)`,
`cpREV with Freqs. (+F) model`,
`General Reverse Transcriptase model (rtREV)`,
`rtREV with Freqs. (+F) model`.

### ML DNA Models

`Jukes-Cantor model`, `Kimura 2-parameter model`,
`Tamura 3-parameter model`, `Hasegawa-Kishino-Yano model`,
`Tamura-Nei model`, `General Time Reversible model`.

### ML Initial Tree Values

`Make initial tree automatically (Default - NJ/MP)`,
`Make initial tree automatically (Maximum Parsimony)`,
`Make multiple initial trees automatically (Maximum Parsimony)`,
`Make initial tree automatically (Neighbor Joining)`, `Use tree from file`.

Runtime wiring note: the multiple-initial-trees value is not equivalent to
calling `TMLTreeAnalyzer.PrepareSearchMLTree` with `MultipleMPTreesMethod`.
MEGA handles it through `TMLTreeSearchThread.ExecuteMultiStartTreeSearch`, which
generates MP starting trees and searches them. PHgoRuntime must also populate
`TProcessPack.TextualSettingsList['No. of Initial Trees']` so the non-visual
MEGA `NumStartingMpTrees` getter reads the visible UI value.

MEGA GUI also has topology-editor paths. PHgo must not expose them until an
equivalent non-GUI runtime input path is implemented and tested.

### Branch Swap Filter Runtime Mapping

| UI Value | Runtime Meaning |
| --- | --- |
| `None` | `1.0 + FP_CUTOFF` |
| `Weak` | `0.9` |
| `Moderate` | `0.7` |
| `Strong` | `0.5` |

## Maximum Parsimony

| Group | Parameter ID | MEGA Label | Type | Protein Default | DNA Default |
| --- | --- | --- | --- | --- | --- |
| Analysis | `statistical_method` | Statistical Method | read-only picklist | `Maximum Parsimony` | `Maximum Parsimony` |
| Phylogeny Test | `phylogeny_test` | Test of Phylogeny | picklist | `None` | `None` |
| Phylogeny Test | `bootstrap_replicates` | Bootstrap Replicates | integer | `500` | `500` |
| Substitution Model | `substitutions_type` | Substitutions Type | read-only picklist | `Amino acid` | `Nucleotide` |
| Data Subset To Use | `gaps_missing_data` | Gaps/Missing Data | picklist | `Use all sites` | `Use all sites` |
| Data Subset To Use | `site_coverage_cutoff` | Site Coverage Cutoff (%) | integer | `95` | `95` |
| Tree Inference Options | `mp_search_method` | MP Search Method | picklist | `Subtree-Pruning-Regrafting (SPR)` | same |
| Tree Inference Options | `initial_trees_random_addition` | No. of Initial Trees (random addition) | integer | `10` | `10` |
| Tree Inference Options | `mp_search_level` | MP Search Level | integer | `1` | `1` |
| Tree Inference Options | `max_trees_to_retain` | Max No. of Trees to Retain | integer | `100` | `100` |
| System Resource Usage | `number_of_threads` | Number of Threads | integer | MEGA CPU default | MEGA CPU default |

Exposed MP search methods:

- `Subtree-Pruning-Regrafting (SPR)`
- `Tree-Bisection-Reconnection (TBR)`
- `Min-Mini Heuristic`
- `Max-mini Branch-&-bound`

## Audit Requirements

- All five exposed methods must produce Newick through MEGA runtime components.
- Bootstrap probes must cover NJ, ME, UPGMA, ML, and MP.
- MP probes must use at least one dataset with parsimony-informative sites; a
  no-informative-site dataset is an expected MEGA error (`There are no
  parsimony informative sites`), not a valid Newick-producing smoke case.
- ML probes must cover the default initial-tree path, the multiple MP initial
  tree path, and bootstrap; all must produce Newick through MEGA runtime
  classes.
- Registry tests must verify protein and DNA method surfaces separately, with
  tree parameter rows/defaults grounded in the MEGA 12.1 analysis-preference
  source and screenshot anchors.
- UI tests may verify that PHgo renders its current registry consistently, but
  they must not introduce options that are absent from MEGA 12.1 source.
- Runtime request tests must verify display labels do not change compute
  fingerprints, while any tree parameter change does.
