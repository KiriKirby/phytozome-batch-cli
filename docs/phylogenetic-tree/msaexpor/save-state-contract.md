# MSA Save State Contract

This document defines the PHgo MSA page `File -> Save` model. Save is manual, durable, and equivalent to the MSA module inside a `.pgo` session snapshot.

## Boundary

`File -> Save` persists every non-UI MSA state owned by Jalview/PHgo:

- row state: PHgo green/yellow/red state, display name, display prefix, row index, Canvas item index, and Canvas row
- sequence state: current sequence text, sequence name, description, and PHgo taxon mapping
- viewport/render settings: annotation visibility, sequence-feature visibility, boxes/text/colour-text/gap rendering, wrap mode, font, char geometry, text colour, colour scheme, and residue-shading identifiers when available
- groups: name, description, residue range, display flags, outline colour, text colour, colour scheme, and member sequences
- annotations: label, description, sequence mapping, visibility, below-alignment flag, graph settings, graph range/height, and all annotation cells including display character, description, secondary-structure marker, numeric value, and colour
- features: feature type, description, begin/end, score, feature group, status, strand, phase, attributes, ENA location, links, otherDetails, and optional feature colour/style fields exposed by JalviewJS
- markers: durable marker/bookmark/highlight-like objects when Jalview exposes them through the bridge

UI state is not durable. Do not save open menus, popup focus, scroll offsets, cursor/selection rectangle, child-window positions, browser viewport size, temporary export-window fields, or toast/error text.

## Actions

- `File -> Save` calls `PUT /sessions/<id>/msa/state` with a full durable state snapshot.
- Checkbox toggles call the same endpoint with rows only. Server-side merge must preserve the latest full durable state.
- `File -> Apply` may call save first, but Apply is not Save. Apply is the only action that writes MSA row/sequence changes back into Canvas and refreshes shared tree/MSA artifacts.
- Save must not call `/msa/apply`, must not change Canvas table state, and must not trigger tree recomputation or shared-payload refresh.

## Restore

When the MSA page opens, PHgo loads `GET /sessions/<id>/msa/state` and applies the saved durable state after Jalview is ready. Reopening the page restores the saved state unless PHgo has refreshed the tree/MSA shared payload. A PHgo refresh replaces the payload/alignment generation and normalizes row state against the new payload; remaining compatible durable state can still be merged by taxon/name/index.

Restore must be idempotent. PHgo-created restored groups, annotations, and features are removed or de-duplicated before applying the saved state again so repeated page opens do not stack duplicate objects.

## Snapshot Module

`.pgo` writes this state as `modules/canvas-msa-state-v1.xml`. This module is the authoritative serialized form for MSA Save inside session snapshots. Snapshot save reads the latest server-side MSA state, including the result of the most recent manual Save, and snapshot open seeds the viewer server with that state before the MSA page launches.
