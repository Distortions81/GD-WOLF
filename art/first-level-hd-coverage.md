# First-level HD art coverage

Checked against the embedded WL1 first map, current runtime asset lookup and local HD override files on 2026-09-30.

All **136 gameplay sprite shapes** are covered: guard and dog animations, scenery and pickups, player weapons, and the map player marker. Their inventory is in `art/e1f1-hd/requirements.json`.

The map uses **13 wall designs**. Eight have HD override files, and five still need artwork:

| Tile | Missing design |
| --- | --- |
| 04 | Framed portrait on gray stone |
| 06 | Stone arch and golden eagle decoration |
| 10 | Framed emblem artwork on wood |
| 11 | Framed portrait on wood |
| 21 | Elevator interior wall/rail, plus its separate switch face |

The elevator's activated switch variant also needs coverage. The approved rough stone direction is installed for tile02. Tile01 and tile03's background still use the earlier smooth-block finish and need matching revisions.

Tile09 currently uses the identical PNG as tile08, despite the originals having different block arrangements. It needs its own layout.

The map contains 20 ordinary doors (raw tiles90/91) and two elevator doors (tile100). The runtime selects ordinary door pages98/99, frame pages100/101, and elevator pages102/103. There are no HD page overrides for these. Existing artwork at horizontal tiles50/51 does not replace the `AllPages` entries selected by the door renderer; reuse suitable door artwork after mapping it correctly.

The in-game HUD has no HD replacements: status-bar background, health faces, number/blank glyphs, weapon icons and key icons. The current renderer also composes the HUD into its original 320px-wide buffer, so simply dropping larger picture PNGs into place will require an accompanying composition change.

Shared menu artwork and bitmap text can be covered in a separate UI pass. Floors and ceilings are rendered from palette colors/gradients, so there are no missing floor/ceiling texture files.

Recommended order: match the base stone and banner to approved tile02, finish missing wall decorations and the elevator, restore tile09's separate layout, connect doors and frames, then redraw and scale the HUD.
