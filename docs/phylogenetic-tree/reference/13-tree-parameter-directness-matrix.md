# Tree Parameter Directness Matrix

This matrix records whether each PHgo tree parameter is consumed by the
MEGA-backed runtime, and whether any non-consumption is intentional according to
MEGA 12.1 source.

Automated guard:

- `internal/phylo/registry_test.go::TestTreeParameterDirectnessMatrixCoversEveryTreeParameter`
  must be updated whenever a new tree UI parameter is registered.
- `internal/phylo/run_test.go::TestBuildMegaPHGORuntimeRequestCarriesAllEditableMegaDefaults`
  verifies that every editable registered default reaches
  `runtime-request.json`.
- `internal/phylo/run_test.go::TestBuildMegaPHGORuntimeRequestPreservesTreeParameterOverrides`
  verifies that user overrides survive normalization and are passed through to
  the runtime request.

Canonical sources:

- `internal/phylo/registry.go`
- `_mega_source/MEGA12.1-source/PHgoRuntime/mega-phgo-runtime.lpr`
- `_mega_source/MEGA12.1-source/Process/processtreecmds.pas`
- `_mega_source/MEGA12.1-source/Process/processmltreecmds.pas`
- `_mega_source/MEGA12.1-source/Process/processparsimtreecmds.pas`
- `_mega_source/MEGA12.1-source/Common/mtreepack.pas`
- `_mega_source/MEGA12.1-source/TamuraFuncs/mtreesearchthread.pas`
- `_mega_source/MEGA12.1-source/TamuraFuncs/mlsearchthread.pas`
- `_mega_source/MEGA12.1-source/TamuraFuncs/parsimsearchthreads.pas`

## Shared / Display Rows

| Parameter ID | Runtime status | Source ruling |
| --- | --- | --- |
| `scope` | display-only | MEGA dialog row is descriptive; no algorithm field |
| `statistical_method` | display-only | selected PHgo `tree_method` dispatches MEGA path |
| `substitutions_type` | display-only | PHgo `conversion_target` / runtime sequence kind selects nucleotide vs amino path |

## Shared Test / Deletion / Thread Rows

| Parameter ID | Runtime status | Runtime path |
| --- | --- | --- |
| `phylogeny_test` | consumed | `RuntimeTreeBootstrapSelected` |
| `bootstrap_replicates` | consumed | `RuntimeTreeBootstrapReps` |
| `gaps_missing_data` | consumed | `RuntimeTreeGapsMissingData` -> MEGA subset options |
| `site_coverage_cutoff` | consumed | `RuntimeTreeSiteCoverage` -> MEGA data prep |
| `number_of_threads` | consumed | runtime thread count for bootstrap/ML/MP where MEGA thread exposes it |

## Distance Tree Rows

| Parameter ID | Runtime status | Source ruling |
| --- | --- | --- |
| `model_method` | consumed | `RuntimeTreeDistanceModel` / `BuildMegaDistancePack` |
| `rates_among_sites` | consumed | adds `gdGamma` / ML gamma flags |
| `gamma_parameter` | consumed | `DistPack.GammaParameter`, ML gamma |
| `pattern_among_lineages` | consumed | adds `gdHetero` for distance pack |
| `me_heuristic_method` | read-only fixed | MEGA exposes fixed `Close-Neighbor-Interchange (CNI)`; PHgo does not invent another path |
| `initial_tree_for_me` | read-only fixed | MEGA `ConstructMETree` uses initial NJ tree (`UseInitialNJTree := true`) |
| `me_search_level` | MEGA-recorded, no downstream algorithm read | `TTreePack.ConstructTreePack` stores `SearchFactor`, but `ConstructMETree` passes only `MaxTrees=1` to `TMeTreeSearchThread`; `SearchFactor` is not read in `mtreesearchthread.pas` |
| `distance_model` | compatibility fallback only | runtime accepts legacy/older requests; current UI uses `model_method` |

## Maximum Likelihood Rows

| Parameter ID | Runtime status | Runtime path |
| --- | --- | --- |
| `model_method` | consumed | `RuntimeMLModel` / `RuntimeMLDistType` |
| `rates_among_sites` | consumed | `RuntimeMLGamma`, `RuntimeMLUseInvar`, partition types |
| `discrete_gamma_categories` | consumed | `RuntimeMLGammaCategories` |
| `gamma_parameter` | consumed | `RuntimeMLGamma` |
| `ml_heuristic_method` | consumed | `RuntimeMLSearchLevel` (`NNI`, SPR level 3, SPR level 5) |
| `initial_tree_for_ml` | consumed | `RuntimeMLInitialTreeMethod`; multiple MP start uses MEGA `TMLTreeSearchThread.ExecuteMultiStartTreeSearch` |
| `number_of_initial_trees` | consumed | `RuntimeMLNumberOfInitialTrees`; also written to `TProcessPack.TextualSettingsList['No. of Initial Trees']` for MEGA non-visual getter |
| `initial_tree_file` | consumed when selected | resolved through runtime guide-tree resolver and imported by MEGA `TTreeList.ImportFromNewickFile` |
| `branch_swap_filter` | consumed | `RuntimeMLSearchFilter` |

## Maximum Parsimony Rows

| Parameter ID | Runtime status | Runtime path |
| --- | --- | --- |
| `mp_search_method` | consumed | `RuntimeMPSearchMethodText` / `RuntimeMPSearchMethod` dispatches SPR/TBR, Min-Mini, Max-mini |
| `initial_trees_random_addition` | consumed | `RuntimeMPIntParam(Request, 'initial_trees_random_addition', 10)` |
| `mp_search_level` | consumed | `RuntimeMPIntParam(Request, 'mp_search_level', 1)` for SPR/TBR and Min-Mini search factor |
| `max_trees_to_retain` | consumed | `RuntimeMPIntParam(Request, 'max_trees_to_retain', 100)` |

MP data gate follows MEGA `processparsimtreecmds.pas`: no common sites, no
parsimony-informative sites, and fewer than four taxa are runtime errors before
any MP search thread is allowed to emit a tree.
