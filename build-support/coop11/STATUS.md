# Coop 11 development candidate

Base: `7bf8d0932c7d71f25ecbaba4f8cf94f95a9a9feb`, preserving every greetings hotfix, Coop10, character, cooperative fare and dynamic-world overlay.

Implemented source candidate:

- Actual full Player construction on the visiting machine; native physics, scripts, inputs, picking, cameras and destination controls. No keyboard replay or camera-only driver grant.
- Original bus remains a passive canonical BCS/passenger proxy. Vehicle ID and trip stay at the original owner; local parked vehicles and their riders remain separate.
- Reliable authenticated snapshot channel; model loading before freeze; fuel, odometer, numeric/string state, VM registers/PRNG, clock, rigid/wheel/articulation state and display surfaces transferred without refuelling.
- Driver freezes before return. Owner never resumes a frozen bus on silence alone; a return/suspension acknowledgement is required. Lost-link driver cuts local authority.
- Native fare actions are tied to the original passenger transaction; conductor panel events follow the active driver, with owner-side authorization.
- F1 camera/hit-testing target alignment, active character refresh across content roots, live livery rendering without respawn.
- Dedicated streaming regardless of saved radius or `--all`; all bus/walker interest centers retained; no global timetable fleet texture warmup. Integrated synthetic 612-tile/two-region loading/unloading regression.
- Updater activates the managed P14 sidecar transactionally, preserving opaque P15/custom configuration and the original full HostAgent binaries.

Not published. This is development source, not an approved gameplay fix. Two-player Windows gameplay with real bus models, BCS trip continuity, conductor tickets/change, articulated bus handover, disconnect/reconnect and active skin changes still require validation. The existing greetings hotfix's Windows candidate passed in run 40021213496.
