# FASTA Export Header Contract

## Scope

This document defines the FASTA header modes shared by Keyword, BLAST, Family BLAST, and Canvas export. The system-tree runtime has its own internal FASTA contract and does not inherit an export header selection.

## Modes

| Mode | Exported identity |
| --- | --- |
| PHgo | Full structured `phgo://` provenance header. Canvas adds its Canvas provenance fields. |
| PHgo Lite | Compact `species|ID2(symbol)` identity header. |
| Original | The source FASTA header. |
| Minimal | The best available primary ID only. |
| Display name | Canvas only; the Canvas display name only. |

PHgo Lite must be offered everywhere that full PHgo headers are offered. It is an export/import convenience format, not an input format for the MEGA PHgo system-tree runtime.

## PHgo Lite

The normal form is:

```text
>species|ID2(symbol)
```

For example:

```text
>Bd21-3|Bradi3g18960(Bd4CL1)
```

`species` uses the source's short species label when one is available, otherwise its normal species label. `ID2` uses the same identifier preference as the full PHgo header for the specific export row. `symbol` uses the resolved label/symbol name.

When the symbol is empty, `~`, or `~~`, the header has no parenthetical suffix:

```text
>Bd21-3|Bradi3g18960
```

The parser accepts either generated form. A no-symbol Lite header is recognized when its species field is the generated single FASTA token (normally an abbreviation or an underscore-normalized full name), which avoids treating a generic source header containing visible species-name spaces as PHgo metadata. It also accepts legacy Lite headers containing `(~)` or `(~~)`, but restores no label from those placeholder suffixes. New exporters never write those suffixes.

## File Boundary

The in-memory header may include visible spaces in a species label. At the final FASTA writing boundary every whitespace character becomes `_`, as required for all PHgo-generated FASTA headers. FASTA import remains backward compatible with headers containing spaces.

## Verification

Tests must cover full PHgo Lite output, the absent-symbol form, placeholder-symbol omission, legacy placeholder parsing, keyword/BLAST query and result exports, Canvas export, and session export-setting restoration.
