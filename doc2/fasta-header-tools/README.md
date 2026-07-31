# FASTA Header Tools

This document library owns the static GitHub Pages tool at `docs/wt.html`.
Read this document before maintaining or extending that page.

## Product boundary

- The tool runs entirely in the browser. It has no server endpoint, build step, framework, analytics, or external runtime dependency.
- Preserve the inherited FrontPage table layout, backgrounds, fieldsets, native controls, and page-level visual style. Modernization is limited to internal HTML semantics, JavaScript behavior, accessibility labels, and safety fixes.
- `docs/` is the website root, not a design-document location. Keep maintenance documentation for this tool in this directory.

## Current behavior

- One or more local text/FASTA files are required before tool controls and preview become interactive. The native file control is the only file-name display.
- Task selection toggles between Batch Add Suffix and the reserved Convert Format UI.
- The format selectors currently expose only `PHGO Header` and `Custom Format`. Conversion is deliberately not implemented and reports that status instead of altering data.
- Reset beside Custom Suffix clears only the suffix. The final Reset clears the selected file, options, preview, and all form fields.
- Preview changes only when Refresh Preview is pressed. It is a 20-to-100-line, non-wrapping, non-resizable text area with both scroll directions available for overflow.

## FASTA suffix contract

- Batch Add Suffix appends the custom suffix directly to every header line that starts with `>` at the beginning of a line. It adds no delimiter or extra space.
- The implementation preserves the original line endings and all other content. Sequence lines, blank lines, and comment/note lines are not rewritten.
- A leading UTF-8 BOM is preserved when it appears before the first FASTA header.
- No biological parsing, sequence normalization, header validation, or conversion is performed. This prevents annotation/comment content from being mistaken for a header.

## Export contract and browser constraint

- Exports are always offered as `.fasta`, using each source file stem as the suggested filename. Preview always uses only the first selected file; export processes every selected file.
- With one file, browsers with the File System Access API show their native save dialog. With multiple files, those browsers show a native target-folder picker and write each output there concurrently, using ` (2)`, ` (3)`, and so on for duplicate filenames. Other browsers use the standard multiple-download fallback.
- Static browser pages can read a file chosen by the user but cannot learn its absolute source path or command the save dialog to open in that source directory. This is enforced by browser security; do not add fake path logic or an upload service to bypass it.

## Maintenance checklist

1. Read this document and the FASTA tool entry in `AGENT.md` before changing `docs/wt.html`.
2. Keep the site static and dependency-free.
3. Test both a CRLF and an LF FASTA with blank lines and semicolon comments. Confirm only `>` header lines gain the suffix.
4. Test task visibility, custom-format visibility, both reset controls, explicit preview refresh, multi-file first-file preview, duplicate export names, and both export paths.
5. Update this document and the `AGENT.md` entry when behavior, browser support, or the tool boundary changes.
