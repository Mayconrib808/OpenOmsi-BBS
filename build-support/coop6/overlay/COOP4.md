# openOMSI 0.3.38-coop.4 — authorized conductor controls

Base: complete 0.3.38-coop.3, upstream 866c60de268f3fc8737287b8e2d13993c49b1f1f.
Protocol 14 and the COOP1 command format are preserved.

The cooperative receiver used to reject a mesh when its plain mouse trigger was
absent. The local player accepts controls implemented only by _drag or _off,
including animated doors/flaps. The receiver now keeps an independent script
press, drag and release state. A completed short click on a drag-only control
runs the same bounded AutoDrag stepping used by the local player; a real drag
keeps its authored intermediate position. Timeout/revocation cancels without
starting an automatic click. The driver's pointer is not borrowed or released.
The existing local-player AutoDrag implementation was extracted unchanged into
the shared mouse_control module. Scripts continue to own animation variables.

Ordinary authorized mesh controls no longer depend on the changing passenger ID
at the desk. Fare-related mesh events, HTML pages, native tickets and change
still validate the transaction ID, conductor, epoch and owner. Permissions are
revoked when the passenger leaves or the driver's vehicle changes.

Native paper ticket attachments are shown on the remote bus for its authorized
conductor. Their existing geometric hit test emits the native ticket action to
the driver's simulation. No second fare simulation is created. Original ticket
pack content is read from the installed map, not distributed in this package.

An unavailable/loading/disconnected target is reported as unavailable, rather
than interpreted as a model without a bell. Bell diagnostics count coupled rear
sections too. Logs identify the actual event, press/drag and refusal reason.
The reported int_haltewunsch and cashdesk_changer_1_00 remain ordinary model events.

Validation: 30 native regressions passed; one installed-content test is ignored
because the original game/bus assets are not available here. New tests execute
real generated vehicle scripts: a drag-only conductor flap opens and closes,
manual dragging, release-only controls, cancelled presses, single-consumption
ticket keys, stale passenger IDs, rear bells, and a picked control passing through
COOP1 serialization into the owner's script without changing the remote copy.
The Windows executable is cross-compiled separately. This is not a live Windows
or two-player gameplay validation and does not establish that the exact Caio
Millennium IV Scania portinha uses a drag-only script. Its assets were not supplied.

The Enhanced+ device-loss crash in the supplied logs and the network sound queue
warnings are separate outstanding issues; this revision does not claim to fix
them. Preserve the matching Steam runtime from coop.3.
