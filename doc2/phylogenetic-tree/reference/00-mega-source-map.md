# MEGA Source Map

This map records the MEGA 12.1 source files that must be consulted before
changing system-tree behavior. The goal is to keep future work anchored to the
current bundled source.

## Source Version Priority

1. Only source: `_mega_source/MEGA12.1-source`

Do not inspect older MEGA source folders for this audit. `README.md` defines
`MEGA12_HTML_DIR` for the MEGA 12.1 HTML resource directory; use that alias in
this library instead of treating directory names as version evidence. The MEGA
12.1 version discriminator is:

- `_mega_source/MEGA12.1-source/Common/megaverconsts.pas`
  (`MAJOR_VERSION = 12`, `MINOR_VERSION = 1`)
- `_mega_source/MEGA12.1-source/Common/megaprivatefiles.pas`, which registers
  the current HTML option dialogs under `Private/OptionsDialogs/mega_web_dialogs`
- the MEGA 12.1 resource manifest under the source tree,
  which packages those same resources for the MEGA 12.1 build

## PHgo Runtime Project

- `_mega_source/MEGA12.1-source/PHgoRuntime/mega-phgo-runtime.lpr`
  Custom PHgo runtime entry point. It parses `runtime-request.json`, runs
  MEGA 12.1 alignment/tree paths, writes `runtime-response.json`,
  `aligned.fasta`, `tree.nwk`, logs, and summary text.
- `_mega_source/MEGA12.1-source/PHgoRuntime/mega-phgo-runtime.lpi`
  Lazarus project file for the runtime build.
- `_mega_source/MEGA12.1-source/PHgoRuntime/lib/x86_64-win64/`
  Build output/cache. Useful for confirming linked units, not a source of
  behavioral truth.

## Alignment Sources

- `MEGA12_HTML_DIR/clustalw_parameters_DNA.html`
- `MEGA12_HTML_DIR/clustalw_parameters_protein.html`
- `MEGA12_HTML_DIR/clustalw_parameters_codons.html`
  Current MEGA 12.1 ClustalW parameter UI dialogs. These are the first source
  for UI structure, labels, and controls.
- `_mega_source/MEGA12.1-source/MegaVcl/mclustalw.pas`
  MEGA ClustalW implementation used by the runtime-linked path.
- `_mega_source/MEGA12.1-source/Alignment/`
  Alignment loading and data structures.
- `_mega_source/MEGA12.1-source/AlignmentEditor/`
  GUI alignment workflow context. Use for behavior comparison, not PHgo UI
  embedding.
- MEGA 12.1 source-tree MUSCLE text resources
  Human-oriented MUSCLE option templates. Do not let these override current
  MEGA 12.1 HTML-backed settings when they differ.
- Non-HTML option resources may exist in the source tree, but they are ignored
  for align and tree-type parameter UI/default decisions. The audit rule is
  HTML-only for those parameter-setting UI surfaces.

## Tree Command Processing

- `_mega_source/MEGA12.1-source/Process/processtreecmds.pas`
  Distance-tree command flow for Neighbor-Joining, Minimum Evolution, UPGMA,
  bootstrap, captions, and Newick preparation.
- `_mega_source/MEGA12.1-source/Process/processmltreecmds.pas`
  Maximum Likelihood command flow.
- `_mega_source/MEGA12.1-source/Process/processparsimtreecmds.pas`
  Maximum Parsimony command flow.
- `_mega_source/MEGA12.1-source/Process/processdistcmds.pas`
  Distance-analysis option handling used by distance tree methods.

## Tree Algorithms

- `_mega_source/MEGA12.1-source/TamuraFuncs/mtreesearchthread.pas`
  UPGMA, NJ, ME, distance bootstrap thread classes and tree search helpers.
- `_mega_source/MEGA12.1-source/TamuraFuncs/mltreeanalyzer.pas`
  ML analyzer setup and search machinery.
- `_mega_source/MEGA12.1-source/TamuraFuncs/mltree.pas`
  ML tree data structures.
- `_mega_source/MEGA12.1-source/TamuraFuncs/mptree.pas`
  Maximum Parsimony tree search implementation.
- `_mega_source/MEGA12.1-source/TamuraFuncs/parsimsearchthreads.pas`
  MP bootstrap and search thread families, including branch-bound/min-mini
  paths that should remain hidden until PHgo runtime wiring is complete.
- `_mega_source/MEGA12.1-source/TamuraFuncs/mtreedata.pas`
- `_mega_source/MEGA12.1-source/TamuraFuncs/mtreelist.pas`
- `_mega_source/MEGA12.1-source/Common/mtreepack.pas`
  Tree containers, packed tree data, and Newick export support.

## Preference Strings And Analysis Metadata

- `_mega_source/MEGA12.1-source/Common/megaanalysisprefstrings.pas`
  Canonical display strings and option labels.
- `_mega_source/MEGA12.1-source/Common/manalysisinfo.pas`
  Analysis metadata fields used by command processors and captions.
- `_mega_source/MEGA12.1-source/MegaDlgs/manalysisprefdlg.pas`
  MEGA preference dialog construction and applicability rules. This is useful
  for runtime/source behavior, but it is not a substitute for HTML when
  freezing align/tree-type parameter UI defaults.

## Tree Explorer Rendering And UI

- `_mega_source/MEGA12.1-source/TreeEditor/mtreeviewform.pas`
- `_mega_source/MEGA12.1-source/TreeEditor/mtree_display_setup.pas`
- `_mega_source/MEGA12.1-source/TreeEditor/mtree_display_settings_cache.pas`
- `_mega_source/MEGA12.1-source/MegaVcl/mtreebox.pas`
  Tree Explorer behavior, display settings, and tree canvas mechanics.
- `_mega_source/MEGA12.1-source/TreeEditor/frames/`
  Tree options frames and subtree-specific panels.
- `_mega_source/MEGA12.1-source/TreeEditor/msvgtreerenderer.pas`
- `_mega_source/MEGA12.1-source/TreeEditor/mpdftreerenderer.pas`
- `_mega_source/MEGA12.1-source/TreeEditor/mbitmaptreerenderer.pas`
- `_mega_source/MEGA12.1-source/TreeEditor/memftreerenderer.pas`
  MEGA graphical export/rendering backends. PHgo does not embed these units,
  but the viewer must reproduce their user-facing export categories where
  applicable.

## Tree Explorer HTML Option Dialogs

The HTML dialogs are the primary source for Tree Explorer parameter UI. They
are also registered in `Common/megaprivatefiles.pas` as the current web options
dialog files.

- `MEGA12_HTML_DIR/tree_options_tree.html`
  Rectangular, circle, and radiation tree layout controls.
- `MEGA12_HTML_DIR/tree_options_branch.html`
  Branch line width, statistics/frequency, branch length, divergence time, and
  precision/placement controls.
- `MEGA12_HTML_DIR/tree_options_labels.html`
  Taxon names, taxon markers, label font/color, and marker shape/color controls.
- `MEGA12_HTML_DIR/tree_options_scale.html`
  Distance scale and time scale controls.
- `MEGA12_HTML_DIR/tree_options_cutoff.html`
  Condensed-tree and consensus-tree cutoff values.
- `MEGA12_HTML_DIR/subtree_drawing_options.html`
  Subtree drawing customization.
- `MEGA12_HTML_DIR/newick_export.html`
  Newick export options.

## Help And Behavior Docs

- `_mega_source/MEGA12.1-source/Private/Help/mega_web_help/TreeExplorer_HC_files/`
- `_mega_source/MEGA12.1-source/Private/Help/mega_web_help/Tree_tab_in_Options_dialog_box.htm`
- `_mega_source/MEGA12.1-source/Private/Help/mega_web_help/Compute_Menu_in_Tree_Explorer.htm`
- `_mega_source/MEGA12.1-source/Private/Help/mega_web_help/Display_Newick_Trees_from_File.htm`
- `_mega_source/MEGA12.1-source/Private/Help/mega_web_help/Analyze_User_Tree_by_Maximum_Likelihood.htm`
- `_mega_source/MEGA12.1-source/Private/Help/mega_web_help/Analyze_User_Tree_by_Parsimony.htm`
  User-facing MEGA semantics. Use these to resolve ambiguous source behavior,
  but keep source code authoritative for implemented defaults.

## PHgo Integration Anchors

- `internal/phylo/types.go`
  Public tree settings, method IDs, parameter definitions, input records,
  payloads, manifests, and run results.
- `internal/phylo/registry.go`
  PHgo method registry. Every row here must be traceable to MEGA 12.1 source or
  intentionally marked runtime-hidden.
- `internal/phylo/source_runtime.go`
  Runtime request/response writer, strict executable validation, and runtime
  launch.
- `internal/phylo/build.go`
  Input metadata, stable taxon IDs, payload creation, and fingerprinting.
- `internal/phylo/run.go`
  Artifact reuse rules.
- `internal/workflow/canvas.go`
  Canvas row selection, target mode handling, runtime execution, viewer update,
  compute-versus-render refresh gating, tree runtime/viewer progress modals,
  and Canvas FASTA export. The old "converted FASTA" Canvas export is removed
  from the UI because PHgo no longer performs pre-alignment biological
  conversion outside the MEGA runtime.
- `internal/workflow/session_snapshot.go`
- `internal/sessionsnapshot/`
  `.pgo` snapshot save/open contract.
- `internal/viewersnapshot/`
- `tree-viewer/src/pgv.js`
  `.pgv` viewer snapshot contract.
- `tree-viewer/src/`
  Reactree viewer adapter, label mapping, alignment display, graphical export,
  and browser-side state persistence.
