# MEGA 12 HTML Control Index

This is the code-level index for MEGA 12.1 HTML controls that affect PHgo
system-tree work. Source rule: ClustalW parameter UI/defaults/options/dynamic
visibility come from MEGA 12.1 HTML. MUSCLE and tree-type/tree-inference rows do
not have registered HTML dialogs in this source tree, so their authority is
recorded in `02-alignment-surface.md` and `03-tree-inference-surface.md`.
MEGA option JSON is ignored.

`MEGA12_HTML_DIR` is defined in `README.md`. The concrete files are registered
by `_mega_source/MEGA12.1-source/Common/megaprivatefiles.pas` lines 48-67.

## Registered Dialogs

| Constant | HTML | Area | Parameter-UI Authority |
| --- | --- | --- | --- |
| `wofClustalParametersCodonsFile` | `clustalw_parameters_codons.html` | ClustalW codon alignment | yes |
| `wofClustalParametersDnaFile` | `clustalw_parameters_DNA.html` | ClustalW DNA alignment | yes |
| `wofClustalParametersAAFile` | `clustalw_parameters_protein.html` | ClustalW protein alignment | yes |
| `wofSelectGeneticCodeDlgFile` | `select_genetic_code_dlg.html` | Codon genetic-code table | yes |
| `wofTreeOptionsTreeStyleFile` | `tree_options_tree.html` | Tree Explorer layout/rendering | rendering only |
| `wofTreeOptionsBranchFile` | `tree_options_branch.html` | Branch labels/lengths/stats | rendering only |
| `wofTreeOptionsLabelsFile` | `tree_options_labels.html` | Taxon labels/markers | rendering only |
| `wofTreeOptionsScaleFile` | `tree_options_scale.html` | Distance/time scale | rendering only |
| `wofTreeOptionsCutoffFile` | `tree_options_cutoff.html` | Condensed/consensus display | rendering only |
| `wofSubtreeDrawingOptionsFile` | `subtree_drawing_options.html` | Subtree drawing | rendering only |
| `wofNewickExportOptions` | `newick_export.html` | Newick export controls | export/rendering |
| `wofSeqNameOptions` | `seqname_option.html` | Sequence-name construction | naming/export |
| `wofAlignmentBuildMode` | `edit_build_alignment.html` | Alignment editor setup | editor workflow |
| `wofInputDataDlg` | `input_data_options.html` | Input data options | input workflow |
| `wofWebSeqDataExportFile` | `seqdata_export.html` | Sequence export | export |
| `wofDistDataExportFile` | `distdata_export.html` | Distance export | export |
| `wofSelectCDSsToImport` | `select_cds_to_import.html` | CDS import | import |
| `wofJSMessageDialog` | `js_message_dialog.html` | message dialog | utility |

No registered HTML dialog for NJ, ME, UPGMA, ML, or MP inference-parameter
preferences was found. The `tree_options_*.html` files are Tree Explorer
display/export dialogs, not tree-inference parameter dialogs. Current tree-type
parameter rows are sourced from `manalysisprefdlg.pas`,
`megaanalysisprefstrings.pas`, and real MEGA screenshots.

## Global HTML Mechanics

- Most dialogs use `jquery.validate`; submit handlers set hidden
  `is_validated` to `true`.
- Checkboxes store both browser checked state and an `ischecked` attribute.
  PHgo must read the HTML default from `checked` plus `ischecked`; unchecked
  with `ischecked=false` is false.
- Select defaults come from `<option selected>`. When no option is selected,
  clean browser load chooses the first option unless dialog JavaScript changes
  it.
- Select widgets use `select2` with search disabled for fixed option lists.
- Numeric validation uses jQuery validation `range`, and several Tree Explorer
  dialogs also initialize jQuery UI spinners. Where a spinner range and an HTML
  validation range differ, record both; do not silently normalize them.
- Font and color controls are hidden inputs backed by MEGA HTML JavaScript
  widgets. PHgo may adapt the control shape, but must preserve the same stored
  setting categories.

## ClustalW DNA

Source: `clustalw_parameters_DNA.html`.

Validation: all numeric fields below are required numbers with range `0..100`.
The dialog hidden field `is_validated` starts `false` and becomes `true` only on
validated submit.

| HTML ID | Label | Control | Default | Options / Range |
| --- | --- | --- | --- | --- |
| `clustalw_dna_pairwise_gap_opening_penalty` | Pairwise Gap Opening Penalty | text | `15` | `0..100` |
| `clustalw_dna_pairwise_gap_extension_penalty` | Pairwise Gap Extension Penalty | text | `6.66` | `0..100` |
| `clustalw_dna_multiple_gap_opening_penalty` | Multiple Gap Opening Penalty | text | `15` | `0..100` |
| `clustalw_dna_multiple_gap_extension_penalty` | Multiple Gap Extension Penalty | text | `6.66` | `0..100` |
| `select_dna_weight_matrix` | DNA Weight Matrix | select | `IUB` | `IUB`; `ClustalW (1.6)` with HTML value `ClustalW (1,6)` |
| `clustalw_dna_transition_weight` | Transition Weight | text | `0.5` | `0..100` |
| `clustalw_dna_use_negative_matrix` | Use Negative Matrix | select | `OFF` | `OFF`, `ON` |
| `clustalw_dna_divergent_cutoff` | Delay Divergent Cutoff (%) | text | `30` | `0..100` |
| `clustalw_dna_predefined_gap` | Keep Predefined Gap | checkbox | false | `ischecked=false` |
| `clustalw_dna_upload_guide_tree_file` | Specify Guide Tree | file | empty | file input |

Dynamic actions: checkbox changes update `ischecked`; select controls trigger
change on load. There is no HTML dynamic hide/show among the DNA rows.

## ClustalW Protein

Source: `clustalw_parameters_protein.html`.

Validation: numeric fields are required numbers with range `0..100`.

| HTML ID | Label | Control | Default | Options / Range |
| --- | --- | --- | --- | --- |
| `clustalw_protein_pairwise_gap_opening_penalty` | Pairwise Gap Opening Penalty | text | `10` | `0..100` |
| `clustalw_protein_pairwise_gap_extension_penalty` | Pairwise Gap Extension Penalty | text | `0.1` | `0..100` |
| `clustalw_protein_multiple_gap_opening_penalty` | Multiple Gap Opening Penalty | text | `10` | `0..100` |
| `clustalw_protein_multiple_gap_extension_penalty` | Multiple Gap Extension Penalty | text | `0.2` | `0..100` |
| `clustalw_protein_protein_weight_matrix` | Protein Weight Matrix | select | `Gonnet` | `BLOSUM`/`Blosum`, `PAM`/`Pam`, `Gonnet`, `Identity` |
| `clustalw_protein_residue_specific_pentalties` | Residue-specific Penalties | select | `ON` | `ON`, `OFF` |
| `clustalw_protein_hydrophilic_penalties` | Hydrophilic Penalties | select | `ON` | `ON`, `OFF` |
| `clustalw_protein_gap_separation` | Gap Separation Matrix | text | `4` | `0..100` |
| `clustalw_protein_end_gap_separation` | End Gap Separation | select | `OFF` | `ON`, `OFF` selected |
| `clustalw_protein_use_negative_matrix` | Use Negative Matrix | select | `OFF` | `ON`, `OFF` selected |
| `clustalw_protein_divergent_cutoff` | Delay Divergent Cutoff (%) | text | `30` | `0..100` |
| `clustalw_protein_predefined_gap` | Keep Predefined Gap | checkbox | false | `ischecked=false` |
| `clustalw_protein_upload_guide_tree_file` | Specify Guide Tree | file | empty | file input |

Dynamic actions: checkbox changes update `ischecked`; no HTML hide/show among
protein rows.

## ClustalW Codons

Source: `clustalw_parameters_codons.html`; genetic-code dialog source:
`select_genetic_code_dlg.html`.

Validation: numeric fields are required numbers with range `0..100`.

| HTML ID | Label | Control | Default | Options / Range |
| --- | --- | --- | --- | --- |
| `clustalw_codons_pairwise_gap_opening_penalty` | Pairwise Gap Opening Penalty | text | `10` | `0..100` |
| `clustalw_codons_pairwise_gap_extension_penalty` | Pairwise Gap Extension Penalty | text | `0.1` | `0..100` |
| `clustalw_codons_multiple_gap_opening_penalty` | Multiple Gap Opening Penalty | text | `10` | `0..100` |
| `clustalw_codons_multiple_gap_extension_penalty` | Multiple Gap Extension Penalty | text | `0.2` | `0..100` |
| `clustalw_codons_protein_weight_matrix` | Protein Weight Matrix | select | `BLOSUM` | `BLOSUM`/`Blosum`, `PAM`/`Pam`, `Gonnet`, `Identity` |
| `clustalw_codons_residue_specific_pentalties` | Residue-specific Penalties | select | `ON` | `ON`, `OFF` |
| `clustalw_codons_hydrophilic_penalties` | Hydrophilic Penalties | select | `ON` | `ON`, `OFF` |
| `clustalw_codons_gap_separation` | Gap Separation Matrix | text | `4` | `0..100` |
| `clustalw_codons_end_gap_separation` | End Gap Separation | select | `ON` | `ON`, `OFF`; no selected attribute, first option wins |
| `select_genetic_code` | Genetic Code Table | button/input | `Standard` | opens genetic-code dialog |
| `clustalw_codons_use_negative_matrix` | Use Negative Matrix | select | `ON` | `ON`, `OFF`; no selected attribute, first option wins |
| `clustalw_codons_divergent_cutoff` | Delay Divergent Cutoff (%) | text | `30` | `0..100` |
| `clustalw_codons_predefined_gap` | Keep Predefined Gap | checkbox | false | `ischecked=false` |
| `clustalw_codons_upload_guide_tree_file` | Specify Guide Tree | file | empty | click sets `UploadGuideTree=true` |

Dynamic actions:

- `select_genetic_code` hides `#mainform` and shows `#genetic_code_table_dlg`.
- If localStorage has no code selection, the button is assigned `codeName`
  `Standard` and code table
  `FFLLSSSSYY**CC*WLLLLPPPPHHQQRRRRIIIMTTTTNNKKSSRRVVVVAAAADDEEGGGG`.
- Genetic-code export-table and export-statistics check states are stored in
  localStorage and mirrored to hidden/attribute state.
- View/statistics export type defaults to `EXexcelDisp` when localStorage has no
  stored type.

## Genetic-Code Dialog

Source: `select_genetic_code_dlg.html`.

Actions:

- `Add` appends a new checkbox item with the standard code table as the initial
  table.
- `Edit` opens the 64-codon editor for the selected code.
- `Delete` removes the selected code item.
- `View` opens the code-table view page and stores
  `genetics_code_view_table_filetype`.
- `Statistics` opens statistics and stores
  `genetics_code_statistics_filetype`.
- Code checkbox change stores `genetic_code_value` and `genetic_code_table` in
  localStorage, updates `#codeData`, and unchecks all other code choices.

Export type options for both View and Statistics:

| Value | Meaning |
| --- | --- |
| `EXexcelDisp` | display in Excel |
| `EXexcelSave` | save Excel |
| `EXcsvSave` | save CSV |
| `EXtext` | text |

Built-in code choices and code-table strings:

| Code value | Code table |
| --- | --- |
| `standard` | `FFLLSSSSYY**CC*WLLLLPPPPHHQQRRRRIIIMTTTTNNKKSSRRVVVVAAAADDEEGGGG` |
| `vertebrate_mitochondrial` | `FFLLSSSSYY**CCWWLLLLPPPPHHQQRRRRIIMMTTTTNNKKSS**VVVVAAAADDEEGGGG` |
| `invertebrate_mitochondrial` | `FFLLSSSSYY**CCWWLLLLPPPPHHQQRRRRIIMMTTTTNNKKSSSSVVVVAAAADDEEGGGG` |
| `yeast_mitochondrial` | `FFLLSSSSYY**CCWWTTTTPPPPHHQQRRRRIIMMTTTTNNKKSSRRVVVVAAAADDEEGGGG` |
| `mold_mitochondrial` | `FFLLSSSSYY**CCWWLLLLPPPPHHQQRRRRIIIMTTTTNNKKSSRRVVVVAAAADDEEGGGG` |
| `protozoan_mitochondrial` | `FFLLSSSSYY**CCWWLLLLPPPPHHQQRRRRIIIMTTTTNNKKSSRRVVVVAAAADDEEGGGG` |
| `coelenterate_mitochondrial` | `FFLLSSSSYY**CCWWLLLLPPPPHHQQRRRRIIIMTTTTNNKKSSRRVVVVAAAADDEEGGGG` |
| `mycoplasma` | `FFLLSSSSYY**CCWWLLLLPPPPHHQQRRRRIIIMTTTTNNKKSSRRVVVVAAAADDEEGGGG` |
| `spiroplasma` | `FFLLSSSSYY**CCWWLLLLPPPPHHQQRRRRIIIMTTTTNNKKSSRRVVVVAAAADDEEGGGG` |
| `ciliate_nuclear` | `FFLLSSSSYYQQCC*WLLLLPPPPHHQQRRRRIIIMTTTTNNKKSSRRVVVVAAAADDEEGGGG` |
| `dasycladacean_nuclear` | `FFLLSSSSYYQQCC*WLLLLPPPPHHQQRRRRIIIMTTTTNNKKSSRRVVVVAAAADDEEGGGG` |
| `hexamita_nuclear` | `FFLLSSSSYYQQCC*WLLLLPPPPHHQQRRRRIIIMTTTTNNKKSSRRVVVVAAAADDEEGGGG` |
| `echinoderm_mitochondrial` | `FFLLSSSSYY**CCWWLLLLPPPPHHQQRRRRIIIMTTTTNNNKSSSSVVVVAAAADDEEGGGG` |
| `euplotid_nuclear` | `FFLLSSSSYY**CCCWLLLLPPPPHHQQRRRRIIIMTTTTNNKKSSRRVVVVAAAADDEEGGGG` |
| `bacterial_plastid` | `FFLLSSSSYY**CC*WLLLLPPPPHHQQRRRRIIIMTTTTNNKKSSRRVVVVAAAADDEEGGGG` |
| `plant_plastid` | `FFLLSSSSYY**CC*WLLLLPPPPHHQQRRRRIIIMTTTTNNKKSSRRVVVVAAAADDEEGGGG` |
| `alternative_yeast_nuclear` | `FFLLSSSSYY**CC*WLLLSPPPPHHQQRRRRIIIMTTTTNNKKSSRRVVVVAAAADDEEGGGG` |
| `ascidian_mitochondrial` | `FFLLSSSSYY**CCWWLLLLPPPPHHQQRRRRIIMMTTTTNNKKSSGGVVVVAAAADDEEGGGG` |
| `flatworm_mitochondrial` | `FFLLSSSSYYY*CCWWLLLLPPPPHHQQRRRRIIIMTTTTNNNKSSSSVVVVAAAADDEEGGGG` |
| `blepharisma_macronuclear` | `FFLLSSSSYY*QCC*WLLLLPPPPHHQQRRRRIIIMTTTTNNKKSSRRVVVVAAAADDEEGGGG` |
| `chlorophycean_mitochondrial` | `FFLLSSSSYY*LCC*WLLLLPPPPHHQQRRRRIIIMTTTTNNKKSSRRVVVVAAAADDEEGGGG` |
| `trematode_mitochondrial` | `FFLLSSSSYY**CCWWLLLLPPPPHHQQRRRRIIMMTTTTNNNKSSSSVVVVAAAADDEEGGGG` |
| `scenedesmus_obliquus_mitochondrial` | `FFLLSS*SYY*LCC*WLLLLPPPPHHQQRRRRIIIMTTTTNNKKSSRRVVVVAAAADDEEGGGG` |
| `thraustochytrium_mitochondrial` | `FF*LSSSSYY**CC*WLLLLPPPPHHQQRRRRIIIMTTTTNNKKSSRRVVVVAAAADDEEGGGG` |

The codon editor uses 64 `<select class="select_codon">` controls. Every codon
select exposes the same amino-acid option set:
`***`, `Ala`, `Arg`, `Asn`, `Asp`, `Cys`, `Gln`, `Glu`, `Gly`, `His`, `Ile`,
`Leu`, `Lys`, `Met`, `Phe`, `Pro`, `Ser`, `Thr`, `Trp`, `Tyr`, `Val`.
Selected defaults encode the standard table unless the selected code table or
user-edited localStorage state overrides it.

## Tree Explorer Layout HTML

Source: `tree_options_tree.html`. This is rendering/layout UI, not tree
inference-parameter UI.

| HTML ID | Area | Control | Default | Validation / Spinner |
| --- | --- | --- | --- | --- |
| `tree_options_rect_taxon_separation` | Rectangular | text/spinner | input `0` | validation `1..144`; spinner min `1`, max `144`, step `1` |
| `tree_options_rect_width` | Rectangular | text/spinner | input `0` | validation `0..8192`; spinner max `8129` |
| `tree_options_tree_taxon_name_circle` | Circle | checkbox | false | display taxon name horizontally |
| `tree_options_circle_start_angle` | Circle | text/spinner | `0` | `0..11` |
| `tree_options_circle_radius` | Circle | text/spinner | `0` | `0..8129` |
| `tree_options_circle_center_hole` | Circle | text/spinner | input `0` | validation/spinner `20..80` |
| `tree_options_tree_taxon_name_rad` | Radiation | checkbox | false | display taxon name horizontally |
| `tree_options_rad_branch_length` | Radiation | text/spinner | `0` | `0..8190` |
| `tree_options_rad_start_angle` | Radiation | text/spinner | `0` | `0..11` |

## Tree Explorer Branch HTML

Source: `tree_options_branch.html`.

| HTML ID | Label | Control | Default | Options / Range |
| --- | --- | --- | --- | --- |
| `tree_options_branch_lines` | Line Width | select | `1 pt` | `1..5 pt` |
| `tree_options_branch_display_stats` | Display Stats/Frequency | checkbox | false | `ischecked=false` |
| `tree_options_branch_stat_placement` | Stat Placement | select | `Automatic` | `Automatic`, `Above Branch`, `Below Branch` |
| `tree_options_branch_statistics_font` | Statistics Font | hidden/fontpicker | font widget default | fontpicker |
| `tree_options_branch_horizontal` | Stats Horizontal | spinner | `56` | `0..8190` |
| `tree_options_branch_vertical` | Stats Vertical | spinner | `56` | `0..8190` |
| `tree_options_branch_hide_lower` | Hide Lower Values | checkbox | false | threshold enabled by setting semantics |
| `tree_options_branch_hide_values` | Hide Values Lower Than | spinner | `56` | `0..100` |
| `tree_options_branch_display_branch` | Display Branch Length | checkbox | false | `ischecked=false` |
| `tree_options_branch_branch_placement` | Branch Length Placement | select | `Automatic` | `Automatic`, `Above Branch`, `Below Branch` |
| `tree_options_branch_length_font` | Branch Length Font | hidden/fontpicker | font widget default | fontpicker |
| `tree_options_branch_precision` | Branch Precision | spinner | `2` | `0..8` |
| `tree_options_branch_hide_shorter` | Hide Shorter Branches | checkbox | false | `ischecked=false` |
| `tree_options_hide_shorter` | Hide Shorter Than | spinner | `0.0` | `0..Infinity` |
| `tree_options_branch_display_divergence` | Display Divergence Times | checkbox | false | `ischecked=false` |
| `tree_options_branch_time_placement` | Time Placement | select | `Automatic` | `Automatic`, `Above Branch`, `Below Branch` |
| `tree_options_branch_divergence_font` | Divergence Font | hidden/fontpicker | font widget default | fontpicker |
| `tree_options_branch_divergence_precision` | Divergence Precision | spinner | `2` | `0..100` |
| `tree_options_branch_distance_horizontal` | Distance Horizontal | spinner | `56` | `0..8190` |
| `tree_options_branch_distance_vertical` | Distance Vertical | spinner | `56` | `0..8190` |

## Tree Explorer Label HTML

Source: `tree_options_labels.html`.

- `is_validated` starts `true`.
- `tree_options_labels_display_taxon_names`: checked, `ischecked=true`.
- `tree_options_branch_labels_font`: hidden fontpicker.
- `tree_options_branch_labels_color`: hidden default `#000000`.
- `tree_options_labels_display_taxon_markers`: checked, `ischecked=true`.
- `tree_options_labels_shape_options`: select default `msNone`; values
  `msNone`, `msOpenCircle`, `msFilledCircle`, `msOpenSquare`, `msFilledSquare`,
  `msOpenUpTriangle`, `msFilledUpTriangle`, `msOpenDownTriangle`,
  `msFilledDownTriangle`, `msOpenDiamond`, `msFilledDiamond`.
- `marker_color_hidden`: hidden default `#000000`.
- Selecting marker rows updates row attributes for marker shape and marker
  color.

## Tree Explorer Scale HTML

Source: `tree_options_scale.html`.

| HTML ID | Label | Control | Default | Options / Range |
| --- | --- | --- | --- | --- |
| `tree_options_scale_lines` | Scale Line Width | select | `1 pt` | `1..5 pt` |
| `tree_options_scale_font` | Scale Font | hidden/fontpicker | font widget default | fontpicker |
| `tree_options_scale_distance_scale` | Distance Scale | checkbox | false | `ischecked=false` |
| `tree_options_scale_name_caption_distance_scale` | Distance Caption | text | empty | text |
| `tree_options_distance_scale_length` | Distance Scale Length | spinner | input `2` | `0..8190` |
| `tree_options_distance_tick_interval` | Distance Tick Interval | spinner | input `0` | `0..8190` |
| `tree_options_scale_time_scale` | Time Scale | checkbox | false | `ischecked=false` |
| `tree_options_scale_name_caption_time_scale` | Time Caption | text | empty | text |
| `tree_options_scale_major_tick` | Major Tick | spinner | input `0` | `0..8190` |
| `tree_options_scale_minor_tick` | Minor Tick | spinner | input `0` | `0..8190` |
| `tree_options_scale_node_height_err` | Node Height Error Bar | checkbox | false | `ischecked=false` |

## Cutoff, Newick, Sequence Names, Subtree

Cutoff source: `tree_options_cutoff.html`.

- `tree_options_cutoff_condensed`: spinner default `50`, range `0..100`.
- `tree_options_cutoff_consensus`: spinner default `50`, range `0..100`.

Newick export source: `newick_export.html`.

- `newick_branch_lengths`, `newick_bootstrap_values`, `newick_node_labels`,
  `newick_gene_duplications`, `newick_speciations`: checkboxes, all false.
- `newick_relative_times`: radio checked, `ischecked=true`.
- `newick_divergence_times`: radio false.
- Checkbox changes update `ischecked`; radio changes clear all radio
  `ischecked` states and set only the selected radio true.

Sequence-name source: `seqname_option.html`.

- Hidden source fields: `abbreviated_name`, `full_name`, `subspecies`, `strain`,
  `host`, `senotype`, `gene`, `allele`, `uids`.
- Select2 controls: `seqname_first`, `seqname_second`, `seqname_third`,
  `seqname_fourth`; hidden selected-value inputs default to `species`.
- `seqname_use_initial` has `checked="false"` in HTML but `ischecked=false`;
  use `ischecked=false` as the stored MEGA state.

Subtree source: `subtree_drawing_options.html`.

- Hidden defaults: `group_name=unknown`, `node_id=0`.
- Caption text plus caption font hidden input.
- Node shape options match the label marker-shape list; default `msNone`.
- Color hidden inputs default `#000000`.
- `subtree_options_apply_to_taxon_markers`: checkbox false.
- Branch lines: `boFullBranch`, `boHalfBranch`, `boNoBranch`, `boBranchOnly`;
  `boFullBranch` selected.
- Branch width: select `1..10 pt`, default first option `1 pt`.
- Branch style: `psSolid`, `psDash`, `psDot`; default first option `psSolid`.
- `subtree_options_display_caption`: checked true.
- `subtree_options_align_vertically`: false.
- `subtree_options_display_bracket`: false.
- Bracket style: `brsSquare`, `brsLine`, `brsNone`; `brsNone` selected.
- `subtree_options_display_line_width`: spinner default `2`, range `0..10`.
- Display taxon names/node/taxon marker checkboxes: all false.
- `subtree_options_compress`: false. When checked, it enables
  `subtree_drawing_options_vertical_unit` and `subtree_options_fill_pattern`;
  when unchecked, both are disabled. Vertical unit default `2`, range `0..10`.
- Fill pattern options: `bsSolid`, `bsClear`, `bsFDiagonal`, `bsBDiagonal`,
  `bsCross`, `bsDiagCross`, `bsHorizontal`, `bsVertical`; initially disabled.
- `subtree_options_display_image`: checked true.
- Image options: `gaRight` After Text, `gaLeft` Before Text, `gaTop` Above Text,
  `gaBottom` Below Text.
- Upload image accepts `.bmp`, `.jpg`, `.jpeg`; FileReader preview updates image
  path and export download attributes. Clear and Export buttons set
  `ischecked=true` when clicked. `subtree_options_overwrite` false;
  `subtree_options_defaults` false and disabled.
