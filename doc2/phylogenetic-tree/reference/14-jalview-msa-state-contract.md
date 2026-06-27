# Jalview MSA State Contract

This note defines the PHgo contract for JalviewJS state saved by **File > Save**.
Save is manual and durable: it preserves alignment edits, row states, features,
groups, annotations, markers, render settings, and colours for the next Canvas
snapshot or page reopen. It must not persist transient UI state such as open
menus, popup coordinates, scroll offsets, window geometry, cursors, or browser
viewport dimensions.

## Colour State

PHgo stores the active global colour scheme in `msa_state.colours.scheme`.
`msa_state.settings.colour_scheme` may mirror the same value for diagnostics,
but restore must treat `colours.scheme` as authoritative.

The schema is:

```json
{
  "type": "builtin | user_defined | none",
  "name": "Jalview scheme name",
  "display_name": "human menu label",
  "applet_parameter": "A=ff0000;C=00ff00"
}
```

Built-in schemes use Jalview's own scheme names, for example `Clustal`,
`Blosum62`, `% Identity`, `Zappo`, `Taylor`, `Hydrophobic`,
`Helix Propensity`, `Strand Propensity`, `Turn Propensity`, `Buried Index`,
`Nucleotide`, `Purine/Pyrimidine`, `RNA Helices`, `T-Coffee Scores`, and
`Sequence ID`.

On restore, PHgo verifies the built-in scheme through
`jalview.schemes.ColourSchemes` and then calls
`AlignFrame.changeColour_actionPerformed(name)`. That uses Jalview's native
path for rebuilding the scheme object, repainting, and synchronizing the Colour
menu. PHgo must not infer a built-in scheme from the checked menu item alone,
because the `User Defined...` menu entry is an action, not a durable scheme.

User-defined schemes are serialized with Jalview's
`UserColourScheme.toAppletParameter()`. Restore constructs a
`UserColourScheme` from that parameter and applies the object directly through
`AlignFrame.changeColour(...)`; the saved display/name is reapplied only as
metadata. A user-defined restore without `applet_parameter` is considered
incomplete and must be skipped rather than replacing residues with black.

Legacy snapshots that only contain `colour_scheme_name` remain readable. They
are normalized into the schema above during restore, but every new save writes
the structured `scheme` object.

## Popup Menus

PHgo uses Jalview's sequence/group context menu for MSA editing only. The
Jalview RNA/VARNA 2D structure submenu is not part of the PHgo MSA workflow and
must not be added to the right-click popup. Removing the menu is a source-level
JalviewJS vendor patch, not a CSS hiding rule, so it cannot affect popup
positioning or leave invisible interactive items.

## Regression Requirements

- Selecting a built-in scheme such as Clustalx, saving, and refreshing must
  restore the same built-in scheme and must not check `User Defined...`.
- Selecting or creating a user-defined scheme, saving, and refreshing must
  restore the residue colour map from `applet_parameter`.
- Restore must never call generic `setGlobalColourScheme$O`, restore saved
  background colours, or apply null/unknown schemes except for explicit `None`.
- Right-click popups must not contain `VARNA 2D Structure`.
