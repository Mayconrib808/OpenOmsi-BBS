# Trolleybuses

OMSI 2 itself has no overhead wire. Its trolleybus add-ons carry a plugin DLL that reads the
bus's place out of Omsi.exe's memory and moves fake poles over wires that are only meshes on a
map; none of that can run in openOMSI. openOMSI therefore has its own trolleybus simulation,
built into the game and described by two small text formats, so that a map can carry a real
overhead wire and a vehicle real poles and a real drive:

* the **wire** of a map: `trolley.txt` beside its `global.cfg` (section "The wire" below);
* the **gear** of a vehicle: `[trolley_pole]` and a drive in its `.bus` (section "The gear").

Everything is worked out from mechanics and electrics, with the numbers a real trolleybus has;
the constants are in the vehicle's file, not in the game.

> Real trolleybus add-ons for OMSI 2 carry no wire logic of their own; how they are read and run
> here is in [TROLLEY_MODS.md](TROLLEY_MODS.md). A map's wire splines are read as the wire when no
> `trolley.txt` is given, and a bus with `cp_SHTANGALEV`/`cp_SHTANGAPRAV` pole meshes gets poles
> without a `[trolley_pole]` block.

## What it does

* **Two wires** a fixed distance apart, one of each polarity. A bus is powered only while one
  pole is on the positive and the other on the negative wire, and both lie on wire a
  substation feeds. A section can be dead; between sections there is an insulator (a dead
  stretch around the joint, 0.8 m unless the file says otherwise).
* **Poles.** Each is a boom hinged on the roof with an elevation and a swing, a spring pressing
  the shoe up against the wire (120 N at the working angle by default), its own mass and
  inertia, and a shoe whose groove holds the wire. On the wire the shoe lies on it a boom's
  length from the hinge; the spring's force has to accelerate the boom along every rise of the
  wire. Where the wire rises faster than the boom can follow (a hanger, a kink, the wire's
  unevenness at speed) the shoe leaves it for a moment: the contact is open and an arc strikes
  if current flows. If the wire lifts out of the groove the pole has **come off** (bounce); it
  also comes off when
  * the boom stands too far off the wire's direction seen from above (40 degrees by default:
    about 4 m off the line for a 6.5 m boom), which is what a bus leaving the wire's line, or a
    wrong switch position, does;
  * a bend of the wire is taken too fast: the shoe cannot turn quicker than its sideways speed
    allows (`kink_speed`, 3 m/s: a 25 degree bend at 11 m/s is too much, at 3 m/s it is not);
  * the wire is out of the boom's reach or ends under the shoe.
  A pole that has come off springs up to its stop (80 degrees) and swings about under the bus's
  own motion until a rope holds it. A pole with a **rope retriever** pulls itself down to its
  hook half a second later.
* **By hand.** Out of the cab, near the back of the bus (walking, the outside camera or the free
  camera) hold **R**: the rope of the pole nearest to you is in your hand and the boom rises and
  follows where you look (the cross-hairs of the view, the wire it meets within 1.4 m); let go
  of R and the spring lifts the shoe into the wire, if you aimed it true (the shoe's horns catch
  a wire within 7 cm). A hand is not steady: a boom aimed at a wire from the ground wanders a
  few centimetres. **T** pulls the pole down on to its hook (on the wire it pulls the shoe off,
  with an arc if current flows). **Shift** with either takes both poles. In the cab only poles
  with a motor (`motor 1`) answer R and T, and they find the wire by themselves.
* **Power.** With no circuit the bus does not move: a vehicle with a modelled drive loses its
  force at once (the bus rolls on, slowing under its rolling resistance, and stops), and the
  torque of a scripted drive is switched off (a torque that brakes still works). The voltage a
  bus finds is the substation's less what the feeder drops with the section's current,
  including other trolleybuses on the same section.
* **Rheostat drive** (`[trolley_rheostat]`): a series-wound DC motor solved against a stack of
  resistors; a controller steps the resistors out (14 steps) and then the field in (two
  weakening steps) while the current stays under what the pedal asks for, the resistors turn
  what they take into heat, and the contactors click; dynamic braking excites the motors from
  the line and drives a resistor (without the line only the mechanical brakes work).
* **Asynchronous drive** (`[trolley_async]`): an inverter with a force limit up to its corner
  speed and a power limit above it, faded out as the voltage sags and tripped in a deep sag;
  regenerative braking gives power back to the section as far as its feeder takes it
  (`regen_kw` of the section plus what other buses draw), a brake resistor on the roof burns the
  rest; an optional **battery** (`[trolley_battery]`) moves the bus slowly (15 km/h, 30 kW by
  default) when it is not on the wire and charges from the line.
* **Switches.** A frog in the wire (`[trolley_switch]`) is set by a contact plate a few metres
  before it, when a shoe passes: with `mode power` the current the bus draws then says which
  way (power on takes the branch, coasting goes straight; `invert 1` the other way), with
  `mode indicator` the bus's indicator on the branch's side takes the branch. After a frog has
  been set it keeps its position for 2.5 s. Taking it the wrong way needs no rule: the shoe
  follows the branch while the bus goes straight on, and the pole comes off when the boom
  stands too far off the wire. A shoe entering a frog from either branch always goes on along
  the single wire.
* **Arcs and sound.** Where current is broken - the shoe leaving the wire, a pole coming off,
  an insulator or a frog crossed under power - an arc burns `L I^2 / 2` of the motor circuit's
  inductance (12 mH): the game draws a flash with sparks, lights the wire and the roof and
  cracks. Coasting through an insulator strikes none; power on through it a loud one. The
  shoes hiss on the wire, louder with the current and the speed (a loop with the rectifier's
  300 Hz ripple); the contactors click, a shoe landing on the wire and a pole on its hook
  thud, a frog clatters. The sounds are made in the game, not read from files.

## The wire: `trolley.txt`

A map's `trolley.txt` sits in the folder of its `global.cfg`; another content root (a mod) may
hold `maps/<name>/trolley.txt` for a map that has none, and `OMSI_TROLLEY_FILE` names a file for
a test. Lines are `key value`, grouped in `[block]`s, `//` ends a line's meaning, coordinates
are the map's (x east, y north, z up, metres).

```
[trolley_net]
voltage 600            nominal line voltage (V)
spacing 0.6            distance between the two wires (m)
roughness 0.0012       unevenness of the contact wire (m); worn or icy wire: 0.003 and more

[trolley_section]
id 1
name Substation A
live 1                 0: no substation feeds it
regen_kw 0             power its feeder takes back from a braking bus (kW)
feeder_ohm 0.05

[trolley_span]         the centre line of the pair of wires, as many points as the wire has
section 1              bends and hangers, in the direction the positive wire is on the right of
spacing 0.6            (optional)
p 395.0 -185.4 6.10
p 392.0 -185.4 6.05
...

[trolley_switch]       at a point where three spans meet
name Seeburger junction
pos 262 -185.4 6.1
mode power             power | indicator | fixed
trigger 9              the contact plate lies this far before the frog (m)
sense 12               the current that counts as power on (A)
invert 0
initial straight

[trolley_insulator]    at a joint of two spans
pos 300 -185.4 6.1
length 1.0             dead stretch (m)
```

Spans that end within 25 cm of each other are joined. Where the sections of two joined spans
differ there is an insulator (0.8 m) unless a `[trolley_insulator]` gives its length. A switch
needs a joint of exactly three spans; which one is the toe (the single side) and which leaves
straight or to the side is worked out from their directions. A pole can follow the wire through
a joint, a switch and across spans, and a crossing of two wires that do not share a joint is not
a connection.

The wire's masts and hangers are the map's own objects: the game draws the two wires (8.5 mm
thick) and nothing else. The sag between hangers is in the points.

`OMSI_TROLLEY_DEAD=2,3` takes the sections of those ids off the supply for a run.

## The gear: blocks of a vehicle's `.bus`

Inside a block every line is `key value ...`; keys left out keep the default.

```
[trolley_pole]              one per pole
side left                   left | right (or -1 | 1)
pos -0.3 1.0 4.16           the hinge on the roof (x right, y forward, z up, m)
length 6.7                  boom length (m)
rest_angle -1.5             elevation lying on the hook (degrees)
work_angle 16               elevation in work (degrees): where the spring's force is rated
tip_force 120               what the shoe presses the wire with at work_angle (N)
mass 16                     boom with rope (kg)
head_mass 3                 head and shoe (kg)
groove_depth 0.06           how far the wire sits in the shoe (m)
funnel 0.07                 half the width the horns catch a wire in (m)
max_wire_angle 40           how far the boom may stand off the wire (degrees)
kink_speed 3.0              sideways speed of the shoe a bend of the wire may ask (m/s)
retriever 35                a rope retriever pulls it down at this rate after dewiring (deg/s)
motor 1                     raised and lowered from the cab
visible 1                   the game draws the boom and its rope (0: the model draws its own)
reach 9                     how far from the rope's handle it can be handled (m)
```

```
[trolley_rheostat]          a series DC drive with a resistor controller (all keys optional)
gear_ratio 9.6              motor to wheels
gear_efficiency 0.93
motors 1
armature_ohm 0.14           armature and series field
flux_sat 3.3                flux linkage at saturation (V s/rad)
flux_half_amps 25           the current at which half of it is built up
steps 14                    resistor steps
start_ohm 2.6               the whole external resistance
field_steps 0.72 0.5        flux kept in the field-weakening steps
step_up_s 0.22              seconds a step takes when the controller climbs / falls
step_down_s 0.08
current_min 120             the pedal's current at its ends (A, per motor)
current_max 280
brake_current 220           dynamic braking: the largest generator current (A)
brake_excitation 0.75
brake_force 56000           mechanical brakes with the pedal down (N)
overspeed 380               motor speed limit (rad/s)

[trolley_async]             an inverter drive
max_force 30000             force at the wheels from standstill (N)
max_power_kw 170            power at the wheels above the corner speed
efficiency 0.9
regen_power_kw 150
regen_force 26000
chopper 1                   a brake resistor takes what the line will not
v_min 480                   voltage below which the force fades
v_trip 380                  voltage below which the inverter trips
v_regen_max 720
brake_force 56000

[trolley_battery]           optional, for the asynchronous drive
kwh 4
kw 30
max_kmh 15

[trolley_aux]
6.0                         lights, compressor and heater taken from the line (kW)

[trolley_vars]              names of the script variables the gear reads, if the model's differ
direction Trolley_Direction     > 0.5 forward, < -0.5 reverse, else neutral (default: forward)
indicator_left Blinker_Left     for switches in `indicator` mode
indicator_right Blinker_Right
raise_request Trolley_Poles_Raise   +1 raise, -1 lower the poles that have a motor
```

With a modelled drive the vehicle's own `M_Wheel` is replaced by the drive's force (times the
wheel radius of the driven axle) and the drive's mechanical brake force is added to
`Brakeforce` (the vehicle's variable list must declare `Brakeforce`, `M_Wheel`, `Throttle`
and `Brake`, as every vehicle's does). Without a drive block the script's driving torque is
kept but scaled by the line voltage over the nominal voltage, and cut when the poles bring no
current: **a mod's scripted drive works on a wire and stops without one**.

### Variables for the model and the sound

The game writes these every physics step (declared in the model or not; a model may animate or
make sound by them):

| Variable | Meaning |
|---|---|
| `Trolley_Pole<k>_Elev`, `_Azim` | pole k (1, 2...): elevation over the horizontal and swing from straight back (degrees, + to the right) |
| `Trolley_Pole<k>_State` | 0 down on the hook, 1 being pulled down, 2 in the hand, 3 loose, 4 on the wire with the contact open, 5 on the wire |
| `Trolley_Pole<k>_Contact` | 1 while the shoe touches the wire |
| `Trolley_Pole<k>_Force` | what the shoe presses with (N) |
| `Trolley_Volts`, `Trolley_Amps` | the voltage at the bus and the current from the line (negative: given back) |
| `Trolley_Powered` | 1 while both poles are on live wire |
| `Trolley_Notch` | the rheostat controller's position |
| `Trolley_Arc` | 0..1 how hard an arc burns now |
| `Trolley_Hiss` | 0..1 the shoes' hiss |
| `Trolley_Battery` | the battery's charge, 0..1 |

A vehicle whose model carries its own poles puts `visible 0` in its `[trolley_pole]` and rotates
its pole meshes by `Trolley_Pole<k>_Elev` and `_Azim`.

## Keys

| Key | Where | Does |
|---|---|---|
| R (hold) | outside the bus near its back (on foot, outside or free camera) | take the rope of the nearest pole: it rises and follows your look; let go to put it on the wire |
| T | the same | pull the nearest pole down (the shoe off the wire, the boom on its hook) |
| Shift + R / T | the same | both poles |
| R / T | in the cab | raise / lower the poles that have a motor |

On foot you get out with Ctrl+Shift+G (the `get_up` setting); the outside camera is F3.

## Checking it offscreen

```
OMSI_TROLLEY_FILE=trolley.txt OMSI_TROLLEY_UP=1 OMSI_DEBUG_TROLLEY=1 OMSI_AUTOPILOT=40 \
  openomsi --map maps/<map>/global.cfg --bus Vehicles/<bus>.bus --offscreen out.png \
  --drive 30 --spawn x,y,heading --view outside --snapshots 14.5
```

`OMSI_TROLLEY_UP` puts the poles on the wire by themselves (8 s), `OMSI_TROLLEY_HANDS` scripts
hands ("2-4:raise:01;12-14:pull:0": seconds, raise or pull, poles), `OMSI_TROLLEY_DEAD` kills
sections, `OMSI_DEBUG_TROLLEY=1` logs the gear every second (set `=0.25` for four times as
often) with every event of the poles (a frog passed, an insulator, a dewiring and why) and every arc.
Unit tests of the wire network, the poles' contact and dewiring, the switches, the drives and
the whole gear are in `crates/omsi-trolley`.

## Limits

* Only the player's bus has gear simulated; AI vehicles do not run on the wire.
* Crossings of two wires (an overhead crossing with insulated sections) are not modelled apart
  from the geometry; a shoe never jumps from one wire to the other.
* The poles are drawn as plain booms with a rope, not as the model of any bus; a model that
  has its own sets `visible 0` and follows the variables.
* Wire and gear are not sent over LAN yet; another player sees a trolleybus as an ordinary bus.


## The mouse's hand on a pole

A click on the cab's pole switch (a plain OMSI 2 trolleybus: `cp_SHTANGALEV`, `cp_SHTANGAPRAV`
or what its model calls it), or - from outside the cab, with the outside, free or foot view -
a click on a pole itself, takes that pole in hand. The tip of the boom follows the mouse: it goes
to the wire the cursor points at (as the R key's gaze does), or to where the cursor points when
no wire is near. The next click lets go, and the spring lifts the shoe into the wire it was aimed
at; a wire missed, the pole flies loose and the motor's retriever (if it has one) pulls it down.
A pole on the wire is taken off it by the same click (an arc, as for any dewiring under
current). One hand: one pole at a time. T still pulls a pole down by its rope.

From the driver's seat the poles are behind the head and the wire is out of the cursor's reach,
so a click on the switch has the motor raise the pole to the wire nearest its tip (the hand lets
go when it is up). A plain trolleybus draws no boom of its own: the model's raised pole mesh is
turned about its hinge by `Trolley_Pole<k>_Elev` and `_Azim` (two `[newanim]`-style animations
made when the model is loaded), and the cab's switch reads 0 with the pole on its hook (the
folded mesh) and 1 otherwise.
