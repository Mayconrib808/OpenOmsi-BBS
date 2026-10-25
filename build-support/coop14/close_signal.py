"""Keep the driver's and conductor's signal identical in the 0.3.41 port.

Sound asset: assets/sounds/coop_turnstile_coin.wav in the *client* runtime.
This changes only the optional metallic cue; physical doors remain driver-controlled.
No runtime fetch, duplicate peer sound or protocol change is introduced.
"""
from pathlib import Path

def replace_exact(path, before, after):
    raw=path.read_text(encoding="utf-8")
    count=raw.count(before)
    if count!=1:
        raise RuntimeError(f"Closing-signal code differs in {path}: {count} matches")
    path.write_text(raw.replace(before,after),encoding="utf-8",newline="\n")

def apply(game: Path):
    p=game/"crates/omsi-app/src/coop.rs"
    replace_exact(p,
'''                    if self.foot_bus().and_then(|b| self.coop_vehicle(b))
                        .is_some_and(|v| !native_close_cue(v)) {
                        self.coop_play_close_cue();
                    }''',
'''                    // This button is the conductor's distinct "Fechou" signal,
                    // not a physical door command. Play the chosen cue on ACK
                    // regardless of the bus's own pneumatic door sound set.
                    self.coop_play_close_cue();''')
    replace_exact(p,
'''            if self.player.as_ref().is_some_and(|p| !native_close_cue(&p.vehicle)) {
                self.coop_play_close_cue();
            }''',
'''            // The owner hears exactly the same custom cue the conductor hears
            // on first ACK; later retransmissions do not replay either side.
            self.coop_play_close_cue();''')
    # The signed bundle installs the approved recording at this WAV path.
    assert p.read_text(encoding="utf-8").count("assets/sounds/coop_turnstile_coin.wav") == 2
    print("Custom close signal: playback on owner acceptance and authorized conductor ACK.")
if __name__ == "__main__":
    apply(Path("game"))
