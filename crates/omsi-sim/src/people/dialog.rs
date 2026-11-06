//! Stable identifiers for the additional passenger captions. The application owns
//! their text and translations; the simulation only reports which reaction occurred.

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum PassengerDialog {
    OpenDoor,
    MissedStop,
    LetMeOff,
    ExitThanks,
}

impl PassengerDialog {
    pub const ALL: [Self; 4] = [
        Self::OpenDoor,
        Self::MissedStop,
        Self::LetMeOff,
        Self::ExitThanks,
    ];

    pub const fn id(self) -> &'static str {
        match self {
            Self::OpenDoor => "BridgeOpenDoor",
            Self::MissedStop => "BridgeMissedStop",
            Self::LetMeOff => "BridgeLetMeOff",
            Self::ExitThanks => "BridgeExitThanks",
        }
    }

    pub fn from_id(id: &str) -> Option<Self> {
        match id {
            "BridgeOpenDoor" => Some(Self::OpenDoor),
            "BridgeMissedStop" => Some(Self::MissedStop),
            "BridgeLetMeOff" => Some(Self::LetMeOff),
            "BridgeExitThanks" => Some(Self::ExitThanks),
            _ => None,
        }
    }

    /// A missed stop takes precedence over a door closed on a departing rider.
    pub const fn door_reaction(missed_stop: bool, closed_on_rider: bool) -> Self {
        if missed_stop {
            Self::MissedStop
        } else if closed_on_rider {
            Self::LetMeOff
        } else {
            Self::OpenDoor
        }
    }
}

#[cfg(test)]
mod tests {
    use super::PassengerDialog as Dialog;

    #[test]
    fn the_existing_door_identifier_remains_compatible_and_stock_lines_stay_separate() {
        assert_eq!(Dialog::from_id("BridgeOpenDoor"), Some(Dialog::OpenDoor));
        for line in ["Thanks_1", "TooBad_A", "Pardon_1", "Unknown", ""] {
            assert_eq!(Dialog::from_id(line), None);
        }
        for dialog in Dialog::ALL {
            assert_eq!(Dialog::from_id(dialog.id()), Some(dialog));
        }
    }

    #[test]
    fn missed_stop_and_closed_door_have_their_own_caption_with_stable_priority() {
        assert_eq!(Dialog::door_reaction(true, true), Dialog::MissedStop);
        assert_eq!(Dialog::door_reaction(true, false), Dialog::MissedStop);
        assert_eq!(Dialog::door_reaction(false, true), Dialog::LetMeOff);
        assert_eq!(Dialog::door_reaction(false, false), Dialog::OpenDoor);
    }
}
