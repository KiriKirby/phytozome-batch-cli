# PHgo Runtime Pipeline

## Purpose

This document defines how the tree system uses `mega-phgo-runtime` for all biological computation.

`mega-phgo-runtime` is a PHgo-owned runtime built from the MEGA 12.1 source tree. It replaces direct MEGA-CC use and `.mao` file generation in the PHgo tree workflow.

The runtime is the only computation engine for:

- sequence alignment
- tree inference

The external viewer service never performs biological computation. It only renders and exports already computed tree artifacts.

## Inputs

The pipeline starts from selected, sequence-ready Canvas rows.

Rows are processed in visible Canvas order. Unselected rows and rows without sequence payloads are ignored.

PHgo writes `input.fasta` using stable internal IDs rather than user-facing labels:

```text
>PHGOT000001
MSEQUENCE...
>PHGOT000002
MSEQUENCE...
```

User-facing labels live in metadata, not in runtime leaf identifiers. This avoids escaping and duplicate-name problems in Newick.

## Runtime Protocol

PHgo writes `runtime-request.json` and launches:

```text
mega-phgo-runtime <path-to-runtime-request.json>
```

The executable must come from the application-local folder:

```text
<application-dir>/mega-phgo-runtime.bin
```

On Windows, `mega-phgo-runtime.bin` is a packaged asset name, not the final
process image PHgo launches. Runtime probing and execution call
`megaphgo.PrepareExecution`, which copies the bundled asset to a temporary
`mega-phgo-runtime.exe` beside a temporary copy of `muscleWin64.bin`, launches
that `.exe`, and cleans the temporary directory afterward. Development probes
against the MEGA source tree may use the source-built
`_mega_source/MEGA12.1-source/PHgoRuntime/lib/x86_64-win64/mega-phgo-runtime.exe`
directly. Do not open or execute the bundled `.bin` directly.

PHgo does not search `PATH`, does not use `C:\Program Files\MEGA12`, does not use `C:\Program Files\MEGA12cc`, and does not reuse any other installed MEGA runtime. The folder itself is the installation contract. Windows `amd64` release bundles must already contain the exact `mega-phgo-runtime` folder at the application root. If that bundled folder is missing or invalid, the package is incomplete or corrupted and the user should reinstall the full bundle instead of downloading runtime pieces separately.

Managed PHgo runtime support currently exists only in the bundled Windows `amd64` release. Linux and macOS builds do not ship the runtime yet, so those platforms should report system-tree unsupported instead of attempting any runtime download.

Runtime freshness is not tracked by a marker file inside `mega-phgo-runtime`. The bundled folder is accepted only after the PHgo probe and runtime-owned MUSCLE checks pass. Do not add a separate release marker or version sentinel.

The bundled runtime files must also include the runtime-owned MUSCLE binary used by `mega-phgo-runtime`:

```text
Windows: muscleWin64.bin
Linux:   mega-phgo-runtime/muscleUnix64.exe
macOS:   mega-phgo-runtime/muscledarwin64
```

Current packaging status:

- Windows runtime is built and verified from the MEGA 12.1 PHgo runtime project.
- Linux and macOS runtime packages are intentionally not bundled for current releases.
- On Linux and macOS, runtime checks must directly report unsupported instead of searching for placeholders or attempting a package download.
- Do not fill Linux/macOS runtime folders with MUSCLE-only files, MEGA-CC binaries, installed MEGA files, renamed placeholders, or partial archives.
- Windows release packaging copies the contents of `assets/mega-phgo-runtime/windows-amd64/runtime/` directly into the bundle root.
- If future Linux/macOS runtime support is intentionally re-enabled, those runtime release folders must contain real `mega-phgo-runtime` executables built on matching platforms or with a configured Lazarus/FPC cross toolchain, plus the runtime-owned MUSCLE binary, and they must pass the same probe checks before packaging.
- For current local verification and release packaging, pass `-Platform windows-amd64`.

These files are bundled as part of the PHgo runtime package. The runtime never searches for a system `muscle` executable.

Expanding the Canvas tree panel and refreshing a tree both run the same strict availability check: the application-local executable must exist, must respond to `--phgo-runtime-probe` with the PHgo runtime token, and the runtime-owned MUSCLE binary must exist in the same folder. A same-named installed MEGA/MEGA-CC binary, renamed placeholder, nested executable, or `PATH` executable is not accepted.

The local runtime probe is not a download; it only verifies that the executable responds to `--phgo-runtime-probe`, and it allows up to 60 seconds to avoid false failures from slow disks or antivirus scanning.

The runtime reads the request, performs alignment and tree inference, and writes:

```text
runtime-response.json
aligned.fasta
tree.nwk
runtime-summary.txt
runtime.log
```

The request carries stable IDs, target sequence kind, selected alignment/tree methods, parameter maps, input FASTA text, and artifact paths. This is the direct PHgo-to-runtime API. `.mao` files are not part of the PHgo runtime contract.

After the runtime finishes, PHgo accepts or rejects the result from the runtime response and required artifact presence. PHgo does not run an extra biological validator over MEGA output; `mega-phgo-runtime` is the authority for alignment/tree correctness and runtime failure text.

## Target Mode

Canvas tree target mode is configured before alignment:

- Protein mode is the default target.
- Runtime-reported skipped/failed rows are unchecked when the recovery dialog's Skip action continues; PHgo immediately retries the refresh with the remaining selection.

The right-panel target is the source of truth for both sequence preparation and method availability. Protein mode exposes only the protein ClustalW/MUSCLE choices. DNA mode exposes only ClustalW (DNA), MUSCLE (DNA), ClustalW (Codons), and MUSCLE (Codons). If a restored snapshot or stale UI state contains an incompatible method, refresh normalizes or rejects it before `mega-phgo-runtime` is launched.

MEGA 12.1 GUI behavior is data-type gated: DNA alignment actions are enabled for nucleotide data, protein alignment actions for protein data, and codon actions for coding nucleotide data. The GUI does not reverse-translate protein input into DNA. PHgo follows that behavior in the TUI form: it chooses a MEGA target mode and sends the chosen input to `mega-phgo-runtime`; MEGA either computes or emits the runtime error.

PHgo uses selected sequence payloads with only runtime-format cleanup: terminal `*` markers are stripped from the FASTA handed to MEGA because a final `*` is a terminator marker with no alignment-site meaning for PHgo's tree computation. PHgo does not translate nucleotide rows locally, infer protein content from letters, remove internal `*`, or repair residues before runtime execution.

In DNA mode, PHgo may use a real nucleotide/CDS sequence only when the selected row already embeds one or when row metadata points to a supported source resolver that can fetch the real nucleotide sequence. PHgo must not invent reverse translations from protein FASTA, must not classify protein-only rows by sequence letters, and must not skip them locally before runtime execution. Protein-only rows handed to DNA mode are left for MEGA runtime to accept or fail.

The maintained real-runtime probe for `C:\Users\wangsychn\Desktop\output\123.pgo` exercises the production path: Protein mode must not show PHgo conversion logs, DNA mode must resolve real row-source metadata such as Lemna BLAST rows before alignment when available, and DNA mode must run ClustalW (DNA), MUSCLE (DNA), ClustalW (Codons), and MUSCLE (Codons) on DNA-capable rows from that snapshot.

`input.fasta` is the runtime FASTA handed to MEGA. It may differ from the stored Canvas/source sequence only by removed terminal `*` markers; metadata keeps the row mapping needed to trace the source sequence. PHgo does not translate, reverse-translate, remove internal `*`, or repair protein/nucleotide content before runtime execution. The runtime request is the handoff boundary; MEGA-derived alignment/tree components accept the selected data or report the runtime failure.

The maintained desktop probe for `C:\Users\wangsychn\Desktop\4CLtree.pgo`
verifies this boundary with real user data. It previously failed at
`Unsupported protein residue "*" at sequence 85, site 213`; PHgo now strips
terminal `*` markers before launching MEGA while preserving internal `*`
validation. PHgo must still surface later MEGA/runtime errors directly and must
not switch from ClustalW to MUSCLE or change the tree method automatically.

## Metadata

Every input record has metadata:

- stable tree taxon ID
- current display name
- source type
- original FASTA head/header
- Canvas table values available for display-name source selection
- sequence kind
- row identity for snapshot restoration

The viewer uses this metadata to map `PHGOT...` Newick leaves to the current display name.

Canvas display names are built in two layers:

- the base name comes from the selected Show column/display-name source or the editable Canvas `display_name` cell
- PHgo always emits a separate `display_prefix` value with `[canvas item number,row number within item]`

The coordinate prefix is not written back to Canvas table cells. It exists only in the shared tree/MSA metadata payload. MSA always draws the prefix in Jalview's left ID list. Tree labels default to raw display names; Reactree's View > Labels > `Coord` toggle can show the prefix in the browser viewer without changing PHgo payloads or runtime artifacts. PHgo no longer appends duplicate-label suffixes such as `[PHGOT000123]`, and it respects raw display names even when two leaves have the same text.

## Alignment Methods

Supported alignment methods are selected by the target mode:

- Protein mode: `ClustalW` and `MUSCLE` for amino-acid alignment.
- DNA mode: `ClustalW (DNA)` and `MUSCLE (DNA)` for base alignment.
- DNA mode: `ClustalW (Codons)` and `MUSCLE (Codons)` for coding-sequence codon alignment.

ClustalW uses MEGA 12.1 source code linked into the PHgo runtime. MUSCLE uses the MEGA-distributed MUSCLE binary from the same application-local `mega-phgo-runtime` folder, launched by the PHgo runtime with PHgo-controlled input/output paths and parameters.

Each method is backed by a parameter definition registry. The registry stores PHgo-owned parameter IDs, labels, defaults, allowed values, applicability, and runtime method names. Protein `ClustalW` and `MUSCLE` are UI-specific method IDs that map to runtime `clustalw` and `muscle` with `sequence_kind=protein`. DNA base methods map to runtime `clustalw` and `muscle` with `sequence_kind=nucleotide`. Codon methods map to runtime `clustalw_codons` and `muscle_codons`.

The UI exposes methods compatible with the current target mode. Protein mode shows two align choices; DNA mode shows four because it includes both base-level and codon-level DNA alignment. The pipeline normalizes the selected method again while building the run plan, so stale snapshots or old panel state cannot launch an incompatible runtime request.

## Tree Methods

Tree inference is runtime-only. The verified runtime tree methods are:

- Neighbor-Joining
- Minimum Evolution
- UPGMA
- Maximum Likelihood
- Maximum Parsimony

Minimum Evolution bootstrap is wired through MEGA's `TBootstrapMEThread` path in `mega-phgo-runtime` and covered by a real-runtime probe.

## Refresh Semantics

`Refresh tree` has two explicit phases:

- compute: prepare selected sequences, run `mega-phgo-runtime` for MEGA alignment and tree inference
- render: push the current payload/metadata to Reactree

The first refresh in a live Canvas session is always a full compute refresh. After a `.pgo` Canvas snapshot is opened, the restored payload may be shown, but the first user-triggered `Refresh tree` is also always a full compute refresh; it must not reuse snapshot-restored alignment/tree artifacts. Later refreshes may run render-only only when the sole change since the last successful compute is a PHgo display label change, such as raw `display_name` edits or display-name source changes.

The progress UI must make the active phase obvious: loading selected rows, preparing the runtime request, running `mega-phgo-runtime`, explicitly reporting render-only artifact reuse when applicable, and finally refreshing Reactree.

Recompute is required when any of these values change:

- selected row set
- selected row order
- sequence content
- target mode
- alignment method
- alignment parameter values
- tree method
- tree parameter values

Recompute is not required when only these values change:

- manual `display_name` edits
- display-name bulk source changes that only relabel existing selected rows
- Reactree layout
- Reactree `Coord` coordinate-label visibility
- branch length visibility
- bootstrap visibility
- alignment panel visibility
- export-only settings

When recompute is not required, Canvas sends updated metadata and page-state config to the viewer service so it can rerender without rerunning `mega-phgo-runtime`.

## Cache Keys

The runner computes separate fingerprints:

- input fingerprint
- alignment parameter fingerprint
- tree parameter fingerprint
- preview fingerprint

The alignment fingerprint includes selected row identity, row order, sequence content, target mode, alignment method, and alignment parameters.

The tree fingerprint includes the alignment output fingerprint, tree method, and tree parameters.

The preview fingerprint includes display-name source, display names, display prefixes, and viewer options.

The compute fingerprints deliberately exclude final `display_name` values, the display-name source selector, display prefixes, and Reactree viewer-only options such as `Coord`. This keeps the refresh rule aligned with the Canvas workflow: relabeling leaves should update the viewer payload and browser rendering without re-running the runtime.

Artifact reuse compares only computation settings and compute fingerprints. A run with the same selected rows, sequence content, target mode, alignment settings, tree settings, aligned FASTA, and Newick may be reused even when the metadata display names changed; the reused artifact set is rewritten with fresh metadata and viewer payload for the browser.

The first refresh after opening a snapshot bypasses reuse entirely so the current `mega-phgo-runtime` recomputes and replaces any stale historical artifacts. Later reuse depends on runtime artifact presence and manifest/fingerprint consistency, not on PHgo-side biological reclassification.

## Progress

Refresh progress should be user-visible and cancellable.

Suggested stages:

- preparing selected Canvas sequences
- writing runtime input FASTA and metadata
- writing `runtime-request.json`
- running `mega-phgo-runtime`
- reading `runtime-response.json`
- preparing viewer payload
- updating tree viewer

If alignment/tree artifacts are reused, progress should say so explicitly.

Implemented Canvas refresh progress uses the existing cancellable TUI task modal:

- 0/6 preparing tree refresh
- 1/6 preparing selected Canvas rows
- 2/6 loading selected Canvas sequence payloads
- 3/6 writing tree input FASTA and runtime request
- 4/6 either running `mega-phgo-runtime` for MEGA alignment/tree inference, forcing snapshot-open recomputation, or explicitly reporting render-only artifact reuse
- 5/6 preparing/updating Reactree metadata and payload
- 6/6 Reactree viewer updated

The same progress context is passed into runtime request preparation and runtime execution, so cancellation can stop before or during long-running work.

Task/progress modals must not return immediately after cancellation while their
worker goroutine is still running. The Cancel button/Esc path cancels the task
context, updates the visible status without re-entering the TUI event queue, and
waits for the associated runtime, download, export, or table-preparation task to
exit before the workflow resumes.

## Error Handling

Runtime failures must return to the Canvas page with a clear modal that includes:

- failed stage
- runtime executable path used
- output directory containing logs/artifacts
- concise error text

Partial artifacts may remain in the run directory for diagnosis.

If `runtime-response.json` contains `error_text`, PHgo surfaces that runtime-specific text directly instead of masking it with a later missing-artifact error. `runtime.log`, `runtime.stdout.txt`, and `runtime.stderr.txt` are preserved so ClustalW/MUSCLE failures can be diagnosed from the exact run directory.

## Output Artifacts

Each refresh writes or reuses:

- input FASTA
- input metadata JSON
- runtime request JSON
- runtime response JSON
- aligned sequence output
- Newick tree output
- runtime summary/log output
- run manifest JSON
