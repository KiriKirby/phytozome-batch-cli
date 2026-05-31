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
<application-dir>/mega-phgo-runtime/mega-phgo-runtime(.exe)
```

PHgo does not search `PATH`, does not use `C:\Program Files\MEGA12`, does not use `C:\Program Files\MEGA12cc`, and does not reuse any other installed MEGA runtime. The folder itself is the installation contract. When it is missing, expanding the Canvas tree tool panel or refreshing the tree prompts to download the exact PHgo runtime asset configured in `internal/megaphgo/runtime-release.json`, then extracts it directly into the application-local `mega-phgo-runtime` folder.

Managed PHgo runtime packages are currently published only for Windows amd64. Linux and macOS builds are not published yet, so those platforms should report the managed runtime package as unsupported instead of attempting to download a non-existent asset.

Runtime freshness is not tracked by a marker file inside `mega-phgo-runtime`. The source-controlled manifest's release tag, exact asset filename, and resulting download URL are the version contract. A local folder is accepted only after the PHgo probe and runtime-owned MUSCLE checks pass; when the runtime changes, the manifest tag/filename changes and the installer downloads that new asset. Do not add a separate release marker or version sentinel.

The runtime folder must also contain the runtime-owned MUSCLE binary used by `mega-phgo-runtime`:

```text
Windows: mega-phgo-runtime/muscleWin64.exe
Linux:   mega-phgo-runtime/muscleUnix64.exe
macOS:   mega-phgo-runtime/muscledarwin64
```

Current packaging status:

- Windows runtime is built and verified from the MEGA 12.1 PHgo runtime project.
- Linux and macOS runtime packages are intentionally not built or published for current releases.
- On Linux and macOS, managed runtime install must directly report unsupported instead of searching for placeholders or attempting a package download.
- Do not fill Linux/macOS runtime folders with MUSCLE-only files, MEGA-CC binaries, installed MEGA files, renamed placeholders, or partial archives.
- `scripts/package-mega-phgo-runtime.ps1` packages the Windows runtime when present. Linux/macOS runtime packaging is reserved for a future explicit support decision and must remain out of the normal release path.
- If future Linux/macOS runtime support is intentionally re-enabled, those runtime release folders must contain real `mega-phgo-runtime` executables built on matching platforms or with a configured Lazarus/FPC cross toolchain, plus the runtime-owned MUSCLE binary, and they must pass the same probe checks before packaging.
- For current local verification and release packaging, pass `-Platform windows-amd64`.

These files are bundled as part of the PHgo runtime package. The runtime never searches for a system `muscle` executable.

Expanding the Canvas tree panel and refreshing a tree both run the same strict availability check: the application-local executable must exist, must respond to `--phgo-runtime-probe` with the PHgo runtime token, and the runtime-owned MUSCLE binary must exist in the same folder. A same-named installed MEGA/MEGA-CC binary, renamed placeholder, nested executable, or `PATH` executable is not accepted.

Runtime release downloads deliberately use no HTTP timeout, matching the BLAST+ installer behavior, because users may be on very slow networks. The local runtime probe is not a download; it only verifies that the executable responds to `--phgo-runtime-probe`, and it allows up to 60 seconds to avoid false failures from slow disks or antivirus scanning.

The runtime reads the request, performs alignment and tree inference, and writes:

```text
runtime-response.json
aligned.fasta
tree.nwk
runtime-summary.txt
runtime.log
```

The request carries stable IDs, sequence kind, conversion settings, selected alignment/tree methods, parameter maps, input FASTA text, and artifact paths. This is the direct PHgo-to-runtime API. `.mao` files are not part of the PHgo runtime contract.

After the runtime finishes, PHgo validates the aligned FASTA before accepting the result. If the runtime returns the wrong biological type for the selected conversion target, or returns a sequence count that does not match the request, PHgo raises an explicit error instead of silently accepting a bad tree.

## Conversion Settings

Canvas tree conversion is configured before alignment:

- Protein mode is the default target.
- Convert is the default mismatch action.
- Skipped rows and failed conversions are unchecked by default.

The right-panel target is the source of truth for both sequence preparation and method availability. Protein mode exposes only the protein ClustalW/MUSCLE choices. DNA mode exposes only ClustalW (DNA), MUSCLE (DNA), ClustalW (Codons), and MUSCLE (Codons). If a restored snapshot or stale UI state contains an incompatible method, refresh normalizes or rejects it before `mega-phgo-runtime` is launched.

In Protein mode, nucleotide rows can be sent to `mega-phgo-runtime` for MEGA-owned DNA-to-protein translation before amino-acid alignment. PHgo may classify and group rows, but it does not translate DNA locally and must not carry a Go codon table.

In DNA mode, nucleotide rows are aligned as DNA. Protein rows are only accepted when the source row or source resolver can provide a real nucleotide sequence for that row. PHgo must not invent reverse translations from protein FASTA. Protein-only rows are reported through the same skip/error path instead of being silently included in a DNA tree.

FASTA rows are not treated as biologically anonymous if PHgo metadata can identify their source. DNA-mode refresh may resolve a protein FASTA row to real CDS/nucleotide data when the row carries a source database, proteome/genome ID, Phytozome genome text, report/header URL, or PHGO FASTA header metadata that uniquely maps to a supported resolver. This is still real source resolution, not reverse translation.

The maintained real-runtime probe for `C:\Users\wangsychn\Desktop\output\123.pgo` exercises the production path: Protein/Convert must show `conversion.applied` and `converted_dna_to_protein` in `runtime.log`, Protein/Skip must report the mismatched DNA rows, DNA mode must resolve row-source metadata such as Lemna BLAST rows before alignment, and DNA mode must run ClustalW (DNA), MUSCLE (DNA), ClustalW (Codons), and MUSCLE (Codons) on DNA-capable rows from that snapshot.

`input.fasta` preserves the PHgo-selected sequences for auditability. Before handing protein or unknown non-codon inputs to MEGA-derived ClustalW or the runtime-owned MUSCLE binary, `mega-phgo-runtime` performs a narrow runtime-only cleanup so common exported protein FASTA remains computable:

- terminal protein stop codons (`*`) are removed
- internal protein stop codons are converted to `X`
- unsupported protein gap-like characters such as `.` and `~` are converted to `-`
- other unsupported protein characters are converted to `X`

The cleanup is not applied to nucleotide or codon methods. When cleanup changes anything, `runtime.log` records a `protein.sanitized` line with the number of terminal stops trimmed, internal stops replaced, and invalid characters replaced.

## Metadata

Every input record has metadata:

- stable tree taxon ID
- current display name
- source type
- original FASTA head/header
- Canvas table values available for display-name source selection
- sequence kind
- row identity for snapshot restoration

The viewer uses this metadata to map `PHGOT...` Newick leaves to `display_name`.

## Alignment Methods

Supported alignment methods are selected by the conversion target:

- Protein mode: `ClustalW` and `MUSCLE` for amino-acid alignment.
- DNA mode: `ClustalW (DNA)` and `MUSCLE (DNA)` for base alignment.
- DNA mode: `ClustalW (Codons)` and `MUSCLE (Codons)` for coding-sequence codon alignment.

ClustalW uses MEGA 12.1 source code linked into the PHgo runtime. MUSCLE uses the MEGA-distributed MUSCLE binary from the same application-local `mega-phgo-runtime` folder, launched by the PHgo runtime with PHgo-controlled input/output paths and parameters.

Each method is backed by a parameter definition registry. The registry stores PHgo-owned parameter IDs, labels, defaults, allowed values, applicability, and runtime method names. Protein `ClustalW` and `MUSCLE` are UI-specific method IDs that map to runtime `clustalw` and `muscle` with `sequence_kind=protein`. DNA base methods map to runtime `clustalw` and `muscle` with `sequence_kind=nucleotide`. Codon methods map to runtime `clustalw_codons` and `muscle_codons`.

The UI exposes methods compatible with the current conversion target. Protein mode shows two align choices; DNA mode shows four because it includes both base-level and codon-level DNA alignment. The pipeline validates the selected method again while building the run plan, so stale snapshots or old panel state cannot launch an incompatible runtime request.

## Tree Methods

Tree inference is runtime-only. The current verified runtime tree implementation is:

- Neighbor-Joining

Maximum Likelihood and Maximum Parsimony are not exposed until the PHgo runtime owns verified implementations, defaults, artifact parsing, and tests for those methods.

## Refresh Semantics

`Refresh tree` has two explicit phases:

- compute: prepare selected sequences, run `mega-phgo-runtime` for conversion/skip handling, alignment, and tree inference
- render: push the current payload/metadata to Reactree

The first refresh in a live Canvas session is always a full compute refresh. After a `.pgo` Canvas snapshot is opened, the restored payload may be shown, but the first user-triggered `Refresh tree` is also always a full compute refresh; it must not reuse snapshot-restored alignment/tree artifacts. Later refreshes may run render-only only when the sole change since the last successful compute is a display label change.

The progress UI must make the active phase obvious: loading selected rows, preparing the runtime request, converting or skipping mismatched rows inside `mega-phgo-runtime`, explicitly reporting render-only artifact reuse when applicable, and finally refreshing Reactree.

Recompute is required when any of these values change:

- selected row set
- selected row order
- sequence content
- conversion target
- conversion action
- skip-and-unselect behavior for skipped/failed conversion rows
- alignment method
- alignment parameter values
- tree method
- tree parameter values

Recompute is not required when only these values change:

- manual `display_name` edits
- display-name bulk source changes that only relabel existing selected rows
- Reactree layout
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

The alignment fingerprint includes selected row identity, row order, sequence content, conversion target/action, skip-and-unselect behavior, alignment method, and alignment parameters.

The tree fingerprint includes the alignment output fingerprint, tree method, and tree parameters.

The preview fingerprint includes display-name source, display names, and viewer options.

The compute fingerprints deliberately exclude final `display_name` values and the display-name source selector. This keeps the refresh rule aligned with the Canvas workflow: relabeling leaves should update the viewer payload and browser rendering without re-running the runtime.

Artifact reuse compares only computation settings and compute fingerprints. A run with the same selected rows, sequence content, conversion settings, alignment settings, tree settings, aligned FASTA, and Newick may be reused even when the metadata display names changed; the reused artifact set is rewritten with fresh metadata and viewer payload for the browser.

Reuse is not allowed merely because fingerprints match. Before a reused run is published to Reactree, PHgo validates the reused `aligned.fasta` against the current requested target. If a historical snapshot contains nucleotide-only rows in Protein mode, protein rows in DNA mode, ambiguous aligned sequences, or the wrong sequence count, PHgo suppresses the stale payload. The first refresh after opening a snapshot bypasses reuse entirely so the current `mega-phgo-runtime` recomputes and replaces any stale historical artifacts.

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
- 4/6 either running `mega-phgo-runtime` for conversion/skip handling plus alignment/tree inference, forcing snapshot-open recomputation, or explicitly reporting render-only artifact reuse after validation
- 5/6 preparing/updating Reactree metadata and payload
- 6/6 Reactree viewer updated

The same progress context is passed into the runtime installation/download path and runtime execution path, so cancellation can stop before or during long-running work.

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
