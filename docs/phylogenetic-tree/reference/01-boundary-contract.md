# Boundary Contract

The system-tree feature is a MEGA front end. PHgo exists to select data, expose
MEGA parameters, launch the custom MEGA runtime, preserve artifacts, and display
results. It must not become a second phylogenetics implementation.

## Ownership Table

| Area | Owner | PHgo May Do | PHgo Must Not Do |
| --- | --- | --- | --- |
| Sequence choice | PHgo workflow | Select checked Canvas rows, preserve visible order, resolve real nucleotide/CDS payloads when row metadata supports it | Infer biology from letters, reverse-translate protein, trim stops, repair residues, skip biologically awkward rows before MEGA |
| Alignment | MEGA runtime | Pass FASTA, method ID, and parameter values | Implement ClustalW/MUSCLE behavior in Go, post-process alignment to make it acceptable |
| Distance calculation | MEGA runtime | Pass tree method/model/site parameters | Compute pairwise distances in Go |
| Tree search | MEGA runtime | Pass tree method and search parameters | Keep local NJ/ME/UPGMA/ML/MP tree builders |
| Bootstrap | MEGA runtime | Pass test choice, replicate count, and thread count | Resample sites or compute bootstrap support in Go |
| Newick export | MEGA runtime | Read and preserve runtime Newick | Recursively generate Newick from PHgo-local tree structures |
| Error text | MEGA runtime | Surface `runtime-response.json.error_text` first | Mask runtime errors behind secondary missing-file messages |
| Rendering | Browser viewer | Send Newick, aligned FASTA, metadata, preview state | Compute alignment/tree biology in the viewer |
| Snapshots | PHgo | Preserve panel state, artifacts, payload, manifest, viewer state | Recompute on snapshot open unless the user explicitly refreshes |

## Data Handoff

PHgo writes a run directory and launches:

```text
mega-phgo-runtime <absolute path to runtime-request.json>
```

The request carries:

- schema version
- session ID and run ID
- target sequence kind
- normalized MEGA-backed alignment and tree settings
- stable PHgo taxon IDs
- input FASTA text
- absolute output artifact paths

The runtime writes:

- `runtime-response.json`
- `aligned.fasta`
- `tree.nwk`
- `runtime-summary.txt`
- `runtime.log`
- optional stdout/stderr files captured by PHgo

PHgo accepts a run only after reading the runtime response and required
artifacts. If the response contains `error_text`, that text is the primary user
error.

## Runtime Discovery

Runtime discovery is deliberately strict:

- Windows amd64 uses only the bundled application-local files:
  `mega-phgo-runtime.bin` and `muscleWin64.bin`.
- PHgo must not search `PATH`.
- PHgo must not use installed MEGA or MEGA-CC folders.
- PHgo must not accept renamed placeholder binaries.
- Linux and macOS currently report unsupported for system-tree computation.

This prevents accidental drift from the audited MEGA 12.1 PHgo runtime.

## Target Mode

The Canvas tree target is authoritative:

- Protein mode exposes ClustalW and MUSCLE amino-acid alignment.
- DNA mode exposes ClustalW (DNA), MUSCLE (DNA), ClustalW (Codons), and MUSCLE
  (Codons).

Protein mode sends selected protein payloads as-is. DNA mode may use a real
nucleotide/CDS payload only when already embedded or source-resolved through row
metadata. PHgo does not invent nucleotide input from protein input.

## Parameter Surface Rule

Every alignment or tree-type/tree-inference parameter used by PHgo must be
visible in the parameter UI, stored in `TreeSettings`, serialized into
snapshots, and passed to the runtime. There must be no hidden scientific
constants in Go. UI layout may adapt MEGA's HTML controls to the TUI, but
option names, choices, defaults, numeric ranges, checkbox states, and dynamic
applicability must come from MEGA 12.1 HTML only.

The HTML-only rule is not a blanket rule for non-UI behavior. Runtime launch,
artifact ownership, MEGA thread/algorithm selection, error precedence,
snapshot restoration, and browser handoff may be audited from MEGA 12.1 source
code and `mega-phgo-runtime` behavior because those areas do not have a
parameter-setting HTML UI.

## Closed Boundary Violations

The code audit found PHgo-side behavior that had to be corrected before the
feature could be frozen. These items are closed in the current implementation:

- Tree refresh uses checked Canvas rows in visible order and no longer filters
  candidates through FASTA-export readiness checks before runtime launch.
  Normal Canvas FASTA export has its own export-readiness helper, so that
  export behavior does not leak back into MEGA runtime input.
- `canvasTreeRowSourcesWithSkippedForSettings` returns sequence-choice,
  nucleotide resolver, and source-construction errors directly. Missing
  non-resolvable payloads may still become explicit empty selected records, but
  attempted source-resolution failures are not silently converted into empty
  records.
- `internal/phylo/fasta.go` preserves empty-sequence records when writing
  `input.fasta`. Empty selected records are an input/error condition; PHgo
  must not silently remove them before MEGA sees the run.
- `internal/phylo/run.go` no longer falls back from missing `aligned.fasta` to
  arbitrary `.fasta`/`.fas` files in the run directory. Because `input.fasta`
  always exists, this fallback could misclassify raw input as MEGA alignment
  output.
- `internal/phylo/run.go` similarly no longer scans arbitrary tree-ish files for
  Newick output. Runtime-declared or canonical artifact names should be
  required to prevent stale-artifact acceptance.
- `internal/workflow/blast.go:10998` removes `*` during general Canvas FASTA
  parsing. That broader import behavior must be documented or bypassed for
  strict tree input if raw FASTA residue preservation is required.

## Reuse Rule

Artifact reuse is a workflow optimization, not a scientific shortcut. Reuse is
allowed only when:

- the run manifest exists,
- `aligned.fasta` and `tree.nwk` exist,
- computation fingerprints match,
- computation settings match,
- the session is not in the first refresh after opening a snapshot.

Display names and viewer layout may change without recomputing. Row identity,
row order, sequence content, target mode, alignment settings, and tree settings
require recomputation.
