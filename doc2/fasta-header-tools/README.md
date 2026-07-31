# FASTA Header Tools

This document library owns the static GitHub Pages tool at `docs/wt.html`.
Read this document before maintaining or extending that page.

## Product boundary

- The tool runs entirely in the browser. It has no server endpoint, build step, framework, analytics, or external runtime dependency. Its local `docs/wt.js` implementation is ES5 so IE11 can run the core workflow. Browsers without JavaScript or the standard local File APIs are unsupported and receive a native browser alert.
- Preserve the inherited FrontPage table layout, backgrounds, fieldsets, native controls, and page-level visual style. The document and page-header structure intentionally match `docs/index.html`, including its legacy non-doctype compatibility mode. Modernization is limited to internal HTML semantics, JavaScript behavior, accessibility labels, and safety fixes.
- The header is a locked compatibility boundary, not a tool-design surface. `docs/wt.html` must not contain a `<!doctype>` and must render with `document.compatMode === "BackCompat"`, exactly as `docs/index.html` does. Its outer page table remains `760` pixels wide; the banner row remains `160` pixels high with its nested `150`-pixel table; the navigation row remains `50` pixels high. The header's table/body markup and styling must stay aligned with `docs/index.html`; only the selected navigation item is different (`TOOLS`).
- The tool page must not retain `webbot`, ActiveX, FrontPage FileUpload/SaveResults, or `_derived` form-component markup. Those legacy component markers can trigger IE security prompts even when no tool behavior uses them.
- `docs/` is the website root, not a design-document location. Keep maintenance documentation for this tool in this directory.

## Current behavior

- One or more local text/FASTA files are required before tool controls and preview become interactive. The visible file control remains native. Browsers with the File System Access API obtain file handles through its native picker so exports retain the source-folder context. IE11 and other legacy browsers use the normal file control and `FileReader`. The file control is the only file-name display.
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
- In Chrome and Edge, one-file save dialogs start in the source file's folder. With multiple files, the native target-folder picker starts in the first selected file's folder, then writes every output there concurrently, using ` (2)`, ` (3)`, and so on for duplicate filenames.
- Firefox and Safari do not expose file-handle and folder-writing APIs, so they retain the standard single or multiple-download fallback.
- IE11 uses its `msSaveOrOpenBlob` native save dialog for each selected output file. IE11 cannot receive source-folder handles from a file input, so it cannot default the dialog to the source directory.

## Maintenance checklist

1. Read this document and the FASTA tool entry in `AGENT.md` before changing `docs/wt.html`.
2. Keep the site static and dependency-free.
3. Test both a CRLF and an LF FASTA with blank lines and semicolon comments. Confirm only `>` header lines gain the suffix.
4. Test task visibility, custom-format visibility, both reset controls, explicit preview refresh, multi-file first-file preview, duplicate export names, and both export paths.
5. Update this document and the `AGENT.md` entry when behavior, browser support, or the tool boundary changes.
6. Before publishing a `wt.html` edit, run `powershell -NoProfile -ExecutionPolicy Bypass -File scripts\check-wt-header.ps1`. Also open `index.html` and `wt.html` from the same local server and confirm equal `compatMode`, outer page width, banner height, and navigation height. Keep new behavior in `docs/wt.js` whenever possible so the locked header is not touched.
