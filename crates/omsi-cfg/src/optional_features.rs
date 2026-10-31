//! Optional native additions share setting keys in the game and launcher. Missing
//! keys stay off, including legacy files. Enabling one never enables another.

#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub struct OptionalFeatures {
    pub bcs_event_scenes: bool,
    pub bcs_inspectors: bool,
    pub passenger_door_comments: bool,
    pub passenger_exit_thanks: bool,
    pub extra_traffic_density: bool,
    pub ai_bus_greetings: bool,
    pub passenger_connections_enabled: bool,
}

pub struct Feature {
    pub key: &'static str,
    pub label: &'static str,
}

pub const HELP: &str = "Takes effect when the game starts the next time";
pub const HOST_HELP: &str = "In multiplayer, shared AI vehicles are controlled by the host. Restart the game after changing this option.";
pub fn help(key: &str) -> &'static str {
    if matches!(
        key,
        "bcs_event_scenes" | "extra_traffic_density" | "ai_bus_greetings"
    ) {
        HOST_HELP
    } else {
        HELP
    }
}
pub const FEATURES: &[Feature] = &[
    Feature {
        key: "bcs_event_scenes",
        label: "BCS event scenes",
    },
    Feature {
        key: "bcs_inspectors",
        label: "BCS ticket inspectors",
    },
    Feature {
        key: "passenger_door_comments",
        label: "Passenger door and missed-stop comments",
    },
    Feature {
        key: "passenger_exit_thanks",
        label: "Passengers thank the driver when alighting",
    },
    Feature {
        key: "extra_traffic_density",
        label: "Extra traffic on wide roads",
    },
    Feature {
        key: "ai_bus_greetings",
        label: "AI bus greetings",
    },
    Feature {
        key: "passenger_connections_enabled",
        label: "Passenger connections between lines",
    },
];

pub fn is_key(key: &str) -> bool {
    FEATURES.iter().any(|feature| feature.key == key)
}

impl OptionalFeatures {
    pub fn needs_bcs_capture(self) -> bool {
        self.bcs_event_scenes || self.bcs_inspectors
    }

    pub fn get(self, key: &str) -> Option<bool> {
        Some(match key {
            "bcs_event_scenes" => self.bcs_event_scenes,
            "bcs_inspectors" => self.bcs_inspectors,
            "passenger_door_comments" => self.passenger_door_comments,
            "passenger_exit_thanks" => self.passenger_exit_thanks,
            "extra_traffic_density" => self.extra_traffic_density,
            "ai_bus_greetings" => self.ai_bus_greetings,
            "passenger_connections_enabled" => self.passenger_connections_enabled,
            _ => return None,
        })
    }

    pub fn set(&mut self, key: &str, enabled: bool) -> Option<&'static str> {
        let feature = FEATURES.iter().find(|feature| feature.key == key)?;
        match key {
            "bcs_event_scenes" => self.bcs_event_scenes = enabled,
            "bcs_inspectors" => self.bcs_inspectors = enabled,
            "passenger_door_comments" => self.passenger_door_comments = enabled,
            "passenger_exit_thanks" => self.passenger_exit_thanks = enabled,
            "extra_traffic_density" => self.extra_traffic_density = enabled,
            "ai_bus_greetings" => self.ai_bus_greetings = enabled,
            "passenger_connections_enabled" => self.passenger_connections_enabled = enabled,
            _ => return None,
        }
        Some(feature.key)
    }

    pub fn write_text(self, text: &mut String) {
        use std::fmt::Write;
        for feature in FEATURES {
            let _ = writeln!(
                text,
                "{}={}",
                feature.key,
                u8::from(self.get(feature.key).unwrap_or(false))
            );
        }
    }

    /// Returning before any random draw is the simulation's existing 0% path.
    pub fn connection_percent(self, requested: f32) -> f32 {
        if !self.passenger_connections_enabled {
            return 0.0;
        }
        if requested.is_finite() {
            requested.clamp(0.0, 100.0)
        } else {
            20.0
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn options_start_off_are_independent_and_keep_explicit_false() {
        let mut value = OptionalFeatures::default();
        for feature in FEATURES {
            assert_eq!(value.get(feature.key), Some(false));
            value.set(feature.key, true).unwrap();
            for other in FEATURES {
                assert_eq!(value.get(other.key), Some(other.key == feature.key));
            }
            value.set(feature.key, false).unwrap();
            assert_eq!(value, OptionalFeatures::default());
        }
        assert_eq!(value.set("unknown_feature", true), None);
        let mut saved = String::new();
        value.write_text(&mut saved);
        for feature in FEATURES {
            assert!(saved.contains(&format!("{}=0\n", feature.key)));
        }
    }

    #[test]
    fn disabled_connections_are_zero_even_if_the_saved_percentage_is_one_hundred() {
        let mut value = OptionalFeatures::default();
        assert_eq!(value.connection_percent(100.0), 0.0);
        value.passenger_connections_enabled = true;
        assert_eq!(value.connection_percent(100.0), 100.0);
        assert_eq!(value.connection_percent(f32::NAN), 20.0);
        value.passenger_connections_enabled = false;
        assert_eq!(value.connection_percent(20.0), 0.0);
    }

    #[test]
    fn comments_and_traffic_do_not_start_bcs_capture_and_inspectors_do_not_enable_scenes() {
        let mut value = OptionalFeatures::default();
        for feature in FEATURES {
            value.set(feature.key, true);
            assert_eq!(
                value.needs_bcs_capture(),
                matches!(feature.key, "bcs_event_scenes" | "bcs_inspectors")
            );
            if feature.key == "bcs_inspectors" {
                assert!(!value.bcs_event_scenes);
            }
            value.set(feature.key, false);
        }
        assert!(!value.needs_bcs_capture());
    }
}
