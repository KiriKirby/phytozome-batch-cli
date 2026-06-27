# Alignment Surface

This document records the MEGA-backed alignment methods exposed by PHgo and the
exact parameter UI surface that must be preserved.

Parameter UI rule: for alignment settings, use only MEGA 12.1 HTML option
dialogs. Ignore MEGA option JSON files for UI items, defaults, selected values,
checkbox states, option lists, dynamic visibility, and ranges.

Primary MEGA 12.1 HTML sources, all under `MEGA12_HTML_DIR` as defined in
`README.md`:

- `clustalw_parameters_DNA.html`
- `clustalw_parameters_protein.html`
- `clustalw_parameters_codons.html`
- `select_genetic_code_dlg.html`
- `_mega_source/MEGA12.1-source/Common/megaprivatefiles.pas`

## Mode Gating

| Canvas Target | Exposed Alignment Methods | Runtime Method |
| --- | --- | --- |
| Protein | ClustalW | `clustalw`, `sequence_kind=protein` |
| Protein | MUSCLE | `muscle`, `sequence_kind=protein` |
| DNA | ClustalW (DNA) | `clustalw`, `sequence_kind=nucleotide` |
| DNA | MUSCLE (DNA) | `muscle`, `sequence_kind=nucleotide` |
| DNA | ClustalW (Codons) | `clustalw_codons`, `sequence_kind=nucleotide` |
| DNA | MUSCLE (Codons) | `muscle_codons`, `sequence_kind=nucleotide` |

Stale snapshot methods must be normalized to the current target mode before the
runtime request is written.

## ClustalW DNA HTML Surface

Source: `clustalw_parameters_DNA.html`.

| Group | HTML ID | MEGA Label | Control | HTML Default |
| --- | --- | --- | --- | --- |
| Alignment / Pairwise Alignment | `clustalw_dna_pairwise_gap_opening_penalty` | Gap Opening Penalty | text input | `15` |
| Alignment / Pairwise Alignment | `clustalw_dna_pairwise_gap_extension_penalty` | Gap Extension Penalty | text input | `6.66` |
| Alignment / Multiple Alignment | `clustalw_dna_multiple_gap_opening_penalty` | Gap Opening Penalty | text input | `15` |
| Alignment / Multiple Alignment | `clustalw_dna_multiple_gap_extension_penalty` | Gap Extension Penalty | text input | `6.66` |
| Matrix | `select_dna_weight_matrix` | DNA Weight Matrix | select | `IUB` |
| Matrix | `clustalw_dna_transition_weight` | Transition Weight | text input | `0.5` |
| Matrix | `clustalw_dna_use_negative_matrix` | Use Negative Matrix | select | `OFF` |
| Matrix | `clustalw_dna_divergent_cutoff` | Delay Divergent Cutoff (%) | text input | `30` |
| Matrix | `clustalw_dna_predefined_gap` | Keep Predefined Gap | checkbox | unchecked / `ischecked=false` |
| Matrix | `clustalw_dna_upload_guide_tree_file` | Specify Guide Tree | file input | empty |

Select values:

- `select_dna_weight_matrix`: `IUB`, `ClustalW (1.6)`; the HTML value for the
  second option is `ClustalW (1,6)`.
- `clustalw_dna_use_negative_matrix`: `OFF`, `ON`.

## ClustalW Protein HTML Surface

Source: `clustalw_parameters_protein.html`.

| Group | HTML ID | MEGA Label | Control | HTML Default |
| --- | --- | --- | --- | --- |
| Alignment / Pairwise Alignment | `clustalw_protein_pairwise_gap_opening_penalty` | Gap Opening Penalty | text input | `10` |
| Alignment / Pairwise Alignment | `clustalw_protein_pairwise_gap_extension_penalty` | Gap Extension Penalty | text input | `0.1` |
| Alignment / Multiple Alignment | `clustalw_protein_multiple_gap_opening_penalty` | Gap Opening Penalty | text input | `10` |
| Alignment / Multiple Alignment | `clustalw_protein_multiple_gap_extension_penalty` | Gap Extension Penalty | text input | `0.2` |
| Weight | `clustalw_protein_protein_weight_matrix` | Protein Weight Matrix | select | `Gonnet` |
| Weight | `clustalw_protein_residue_specific_pentalties` | Residue-specific Penalties | select | `ON` |
| Weight | `clustalw_protein_hydrophilic_penalties` | Hydrophilic Penalties | select | `ON` |
| Weight | `clustalw_protein_gap_separation` | Gap Separation Matrix | text input | `4` |
| Weight | `clustalw_protein_end_gap_separation` | End Gap Separation | select | `OFF` |
| Global | `clustalw_protein_use_negative_matrix` | Use Negative Matrix | select | `OFF` |
| Global | `clustalw_protein_divergent_cutoff` | Delay Divergent Cutoff (%) | text input | `30` |
| Global | `clustalw_protein_predefined_gap` | Keep Predefined Gap | checkbox | unchecked / `ischecked=false` |
| Global | `clustalw_protein_upload_guide_tree_file` | Specify Guide Tree | file input | empty |

Select values:

- `clustalw_protein_protein_weight_matrix`: `BLOSUM`, `PAM`, `Gonnet`,
  `Identity`; `Gonnet` is selected in the HTML.
- `clustalw_protein_residue_specific_pentalties`: `ON`, `OFF`.
- `clustalw_protein_hydrophilic_penalties`: `ON`, `OFF`.
- `clustalw_protein_end_gap_separation`: `ON`, `OFF`; `OFF` is selected.
- `clustalw_protein_use_negative_matrix`: `ON`, `OFF`; `OFF` is selected.

## ClustalW Codons HTML Surface

Source: `clustalw_parameters_codons.html`.

| Group | HTML ID | MEGA Label | Control | HTML Default |
| --- | --- | --- | --- | --- |
| Alignment / Pairwise Alignment | `clustalw_codons_pairwise_gap_opening_penalty` | Gap Opening Penalty | text input | `10` |
| Alignment / Pairwise Alignment | `clustalw_codons_pairwise_gap_extension_penalty` | Gap Extension Penalty | text input | `0.1` |
| Alignment / Multiple Alignment | `clustalw_codons_multiple_gap_opening_penalty` | Gap Opening Penalty | text input | `10` |
| Alignment / Multiple Alignment | `clustalw_codons_multiple_gap_extension_penalty` | Gap Extension Penalty | text input | `0.2` |
| Weight | `clustalw_codons_protein_weight_matrix` | Protein Weight Matrix | select | `BLOSUM` |
| Weight | `clustalw_codons_residue_specific_pentalties` | Residue-specific Penalties | select | `ON` |
| Weight | `clustalw_codons_hydrophilic_penalties` | Hydrophilic Penalties | select | `ON` |
| Weight | `clustalw_codons_gap_separation` | Gap Separation Matrix | text input | `4` |
| Weight | `clustalw_codons_end_gap_separation` | End Gap Separation | select | `ON` |
| Weight | `select_genetic_code` | Genetic Code Table | button/dialog | `Standard` |
| Global | `clustalw_codons_use_negative_matrix` | Use Negative Matrix | select | `ON` |
| Global | `clustalw_codons_divergent_cutoff` | Delay Divergent Cutoff (%) | text input | `30` |
| Global | `clustalw_codons_predefined_gap` | Keep Predefined Gap | checkbox | unchecked / `ischecked=false` |
| Global | `clustalw_codons_upload_guide_tree_file` | Specify Guide Tree | file input | empty |

Select values:

- `clustalw_codons_protein_weight_matrix`: `BLOSUM`, `PAM`, `Gonnet`,
  `Identity`; no HTML `selected` attribute, so clean browser default is the
  first option, `BLOSUM`.
- `clustalw_codons_residue_specific_pentalties`: `ON`, `OFF`.
- `clustalw_codons_hydrophilic_penalties`: `ON`, `OFF`.
- `clustalw_codons_end_gap_separation`: `ON`, `OFF`; no HTML `selected`
  attribute, so clean browser default is `ON`.
- `clustalw_codons_use_negative_matrix`: `ON`, `OFF`; no HTML `selected`
  attribute, so clean browser default is `ON`.

## Genetic-Code Dialog

Codon ClustalW opens the MEGA HTML genetic-code dialog through
`select_genetic_code`. The button stores `codeName="Standard"` and the standard
code table on first load. The full codon grid in `clustalw_parameters_codons.html`
and `select_genetic_code_dlg.html` is the source for editable code behavior.

PHgo should expose the MEGA genetic-code table names and pass the selected name
and code table to the runtime. PHgo must not translate, repair, or infer codon
input before the runtime.

## MUSCLE Parameter Surface Status

No MUSCLE parameter HTML dialog is present in `MEGA12_HTML_DIR`, and
`Common/megaprivatefiles.pas` registers ClustalW HTML dialogs but not MUSCLE
HTML dialogs. The current MUSCLE parameter surface is therefore anchored by the
real MEGA screenshots in `C:/Users/wangsychn/Desktop/align`, the MEGA 12.1
MUSCLE text resources, and the runtime-owned MUSCLE invocation keys in
`PHgoRuntime/mega-phgo-runtime.lpr`. Do not use MEGA option JSON as authority.

## Current PHgo Drift From HTML

These were the documentation findings for the code-fix pass; the checked rows
are now fixed in `internal/phylo/registry.go`.

| PHgo Registry Area | Current PHgo Value | HTML Value | Required Direction |
| --- | --- | --- | --- |
| DNA ClustalW `use_negative_matrix` | stale `ON` | `OFF` | fixed |
| DNA ClustalW `keep_predefined_gaps` | stale `True` | unchecked / false | fixed |
| Protein ClustalW pairwise gap extension | stale `0.20` | `0.1` | fixed |
| Protein ClustalW protein weight matrix | stale `BLOSUM` | `Gonnet` | fixed |
| Protein ClustalW `keep_predefined_gaps` | stale `True` | unchecked / false | fixed |
| Codon ClustalW pairwise gap extension | stale `0.20` | `0.1` | fixed |
| Codon ClustalW end gap separation | stale `OFF` | `ON` | fixed |
| Codon ClustalW use negative matrix | stale `OFF` | `ON` | fixed |
| Codon ClustalW `keep_predefined_gaps` | stale `True` | unchecked / false | fixed |
| MUSCLE DNA/protein/codon parameter rows | screenshot/source/runtime-derived | no HTML dialog registered | registered; JSON ignored |

## Audit Requirements

- Every ClustalW parameter row must appear in the Canvas tree panel for its
  applicable method.
- UI structure must be checked against the MEGA 12.1 HTML option dialogs only.
- Defaults must come from HTML input values, HTML selected options, browser
  first-option defaults when no option is selected, and HTML checkbox states.
- MEGA option JSON files must not be used to resolve parameter UI behavior.
- Runtime requests must map PHgo protein aliases (`clustalw_protein`,
  `muscle_protein`) back to runtime `clustalw`/`muscle` with
  `sequence_kind=protein`.
- Tests must fail if the registry loses a MEGA HTML parameter, changes an HTML
  default without source justification, or exposes a method outside its target
  mode.
