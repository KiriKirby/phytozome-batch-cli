# HTML Parameter UI Audit

This is the HTML-only audit ledger for system-tree parameter UI. It exists to
prevent MEGA option JSON files or older MEGA source folders from silently
overriding the current MEGA 12.1 HTML dialogs.

## HTML-Only Rule

- Canonical source tree: `_mega_source/MEGA12.1-source`.
- Parameter UI source for ClustalW alignment modules: MEGA 12.1 HTML option
  dialogs only.
- Parameter UI source for MUSCLE and tree-type/tree-inference modules: first
  confirm whether a registered MEGA 12.1 HTML dialog exists; when none exists,
  use the newest MEGA 12.1 source/runtime plus real screenshots as the
  documented non-JSON authority.
- Ignore MEGA option JSON files for UI items, defaults, selected values,
  checkbox states, option lists, dynamic visibility, and ranges.
- Do not inspect older MEGA source folders for this audit.
- Treat `MEGA12_HTML_DIR` as the MEGA 12.1 HTML resource directory. Its concrete
  source path is recorded once in `README.md`; the only allowed source version
  remains MEGA 12.1.
- Non-parameter behavior with no HTML parameter-setting surface may be audited
  from MEGA 12.1 source/runtime behavior; record that evidence separately from
  UI/default parity.

## Registered HTML Dialogs

`Common/megaprivatefiles.pas` registers the current web option dialog paths:

| Dialog Constant | HTML File | PHgo Area |
| --- | --- | --- |
| `wofClustalParametersCodonsFile` | `clustalw_parameters_codons.html` | ClustalW codon alignment |
| `wofClustalParametersDnaFile` | `clustalw_parameters_DNA.html` | ClustalW DNA alignment |
| `wofClustalParametersAAFile` | `clustalw_parameters_protein.html` | ClustalW protein alignment |
| `wofTreeOptionsBranchFile` | `tree_options_branch.html` | Tree Explorer branch display |
| `wofTreeOptionsCutoffFile` | `tree_options_cutoff.html` | Condensed/consensus cutoffs |
| `wofTreeOptionsLabelsFile` | `tree_options_labels.html` | Taxon names and markers |
| `wofTreeOptionsScaleFile` | `tree_options_scale.html` | Distance/time scale display |
| `wofTreeOptionsTreeStyleFile` | `tree_options_tree.html` | Rectangular/circle/radiation layouts |
| `wofNewickExportOptions` | `newick_export.html` | Newick export variants |
| `wofSelectGeneticCodeDlgFile` | `select_genetic_code_dlg.html` | Genetic-code editor |

## ClustalW HTML Defaults

| Mode | Parameter | HTML Default |
| --- | --- | --- |
| DNA | Pairwise gap opening | `15` |
| DNA | Pairwise gap extension | `6.66` |
| DNA | Multiple gap opening | `15` |
| DNA | Multiple gap extension | `6.66` |
| DNA | DNA weight matrix | `IUB` |
| DNA | Transition weight | `0.5` |
| DNA | Use negative matrix | `OFF` |
| DNA | Delay divergent cutoff | `30` |
| DNA | Keep predefined gap | unchecked / false |
| Protein | Pairwise gap opening | `10` |
| Protein | Pairwise gap extension | `0.1` |
| Protein | Multiple gap opening | `10` |
| Protein | Multiple gap extension | `0.2` |
| Protein | Protein weight matrix | `Gonnet` |
| Protein | Residue-specific penalties | `ON` |
| Protein | Hydrophilic penalties | `ON` |
| Protein | Gap separation matrix | `4` |
| Protein | End gap separation | `OFF` |
| Protein | Use negative matrix | `OFF` |
| Protein | Delay divergent cutoff | `30` |
| Protein | Keep predefined gap | unchecked / false |
| Codons | Pairwise gap opening | `10` |
| Codons | Pairwise gap extension | `0.1` |
| Codons | Multiple gap opening | `10` |
| Codons | Multiple gap extension | `0.2` |
| Codons | Protein weight matrix | `BLOSUM` |
| Codons | Residue-specific penalties | `ON` |
| Codons | Hydrophilic penalties | `ON` |
| Codons | Gap separation matrix | `4` |
| Codons | End gap separation | `ON` |
| Codons | Genetic code table | `Standard` |
| Codons | Use negative matrix | `ON` |
| Codons | Delay divergent cutoff | `30` |
| Codons | Keep predefined gap | unchecked / false |

Default interpretation rules:

- For `<input value="...">`, use the HTML `value`.
- For `<option selected>`, use the selected option.
- For a `<select>` with no selected option, use the first option in clean
  browser load unless dialog JavaScript explicitly changes it.
- For checkboxes, use `checked`/`ischecked`; unchecked with `ischecked=false`
  is false.

## MUSCLE HTML Status

No MUSCLE parameter HTML dialog is present in the registered MEGA 12.1 web
dialog constants or in `MEGA12_HTML_DIR`.

The current MUSCLE parameter registry is anchored by real MEGA screenshots in
`C:/Users/wangsychn/Desktop/align`, MEGA 12.1 MUSCLE text resources, and
`PHgoRuntime/mega-phgo-runtime.lpr` runtime keys. This is a deliberate
non-JSON source path because no MUSCLE HTML dialog is registered.

## Tree-Type Parameter HTML Status

No NJ, ME, UPGMA, ML, or MP analysis-parameter HTML dialog is registered in
`Common/megaprivatefiles.pas`, and none was found in `MEGA12_HTML_DIR`.

The tree-type parameter registry is anchored by MEGA 12.1
`MegaDlgs/manalysisprefdlg.pas`, `Common/megaanalysisprefstrings.pas`, and real
MEGA screenshots in `C:/Users/wangsychn/Desktop/tree`. No MEGA option JSON is
used for defaults, options, or dynamic visibility.

Current re-check result:

- `_mega_source` contains `MEGA12.1-source` as the only MEGA source tree used
  for MEGA decisions. `_mega_source/muscle-rcedgar` is MUSCLE-only secondary
  source evidence and is not used as a MEGA source.
- `Common/megaprivatefiles.pas` registers ClustalW parameter HTML plus Tree
  Explorer rendering/export HTML. It does not register analysis-preference HTML
  for NJ, ME, UPGMA, ML, or MP.
- The `MEGA7_Install/OptionsDialogs` path segment is a directory name inside
  the MEGA 12.1 source tree. It is treated as the MEGA 12.1 HTML resource
  directory only, not as MEGA7 authority.

## Current PHgo Drift

| Area | PHgo Registry | MEGA 12.1 Authority | Status |
| --- | --- | --- | --- |
| DNA ClustalW use negative matrix | `OFF` | HTML `OFF` | fixed |
| DNA ClustalW keep predefined gaps | `False` | HTML unchecked / false | fixed |
| Protein ClustalW pairwise gap extension | `0.1` | HTML `0.1` | fixed |
| Protein ClustalW protein weight matrix | `Gonnet` | HTML `Gonnet` | fixed |
| Protein ClustalW keep predefined gaps | `False` | HTML false | fixed |
| Codon ClustalW pairwise gap extension | `0.1` | HTML `0.1` | fixed |
| Codon ClustalW end gap separation | `ON` | HTML first/default `ON` | fixed |
| Codon ClustalW use negative matrix | `ON` | HTML first/default `ON` | fixed |
| Codon ClustalW keep predefined gaps | `False` | HTML false | fixed |
| MUSCLE parameter UI | registry has screenshot/source/runtime rows | screenshots + source/runtime | fixed for current surface |
| NJ/ME/UPGMA/ML/MP tree parameter UI | registry has source-backed rows | analysis prefs source + screenshots | fixed for current surface |

## Tree Explorer HTML Inventory

Tree display/export HTML is covered in detail by
`04-rendering-viewer-surface.md`. The registered files are:

- `tree_options_tree.html`
- `tree_options_branch.html`
- `tree_options_labels.html`
- `tree_options_scale.html`
- `tree_options_cutoff.html`
- `subtree_drawing_options.html`
- `newick_export.html`

These files govern rendering and export controls only. They do not authorize
PHgo-side alignment, distance calculation, bootstrap, tree search, or Newick
generation.
