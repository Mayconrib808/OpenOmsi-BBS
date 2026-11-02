//! Optional native additions share setting keys in the game and launcher. Missing
//! keys use the Bridge's enabled defaults; an explicit 0 always remains off.

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct OptionalFeatures {
    pub bcs_event_scenes: bool,
    pub bcs_inspectors: bool,
    pub passenger_door_comments: bool,
    pub passenger_exit_thanks: bool,
    pub extra_traffic_density: bool,
    pub ai_bus_greetings: bool,
    pub passenger_connections_enabled: bool,
    pub ai_driver_wave: bool,
    pub passenger_ambient_talk: bool,
    pub ai_bus_overtaking: bool,
    /// Additional lane courtesy for buses leaving a bay (opt-in, host controlled).
    pub cars_yield_to_buses: bool,
    /// Physically stopped, hazard-flashing AI vehicles (host authority, opt-in).
    pub illegal_parking: bool,
    /// Demand accumulates per stop rather than refilling when a bus departs.
    pub independent_passenger_demand: bool,
    /// Prefer tagged clothing variants suited to local weather at spawn time.
    pub weather_clothing: bool,
}

impl Default for OptionalFeatures {
    fn default() -> Self {
        // The twelve user-visible features of the modified OpenOMSI ship
        // enabled for fresh installs AND old configs without these keys.
        // Never override a saved 0 (the parser applies explicit values).
        // weather_clothing was removed due to broken cold-weather outfits;
        // passenger_connections_enabled is an obsolete separate switch,
        // superseded by the upstream 0.3.47 passenger_connections setting.
        Self {
            bcs_event_scenes: true,
            bcs_inspectors: true,
            passenger_door_comments: true,
            passenger_exit_thanks: true,
            extra_traffic_density: true,
            ai_bus_greetings: true,
            passenger_connections_enabled: false,
            ai_driver_wave: true,
            passenger_ambient_talk: true,
            ai_bus_overtaking: true,
            cars_yield_to_buses: true,
            illegal_parking: true,
            independent_passenger_demand: true,
            weather_clothing: false,
        }
    }
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
        "bcs_event_scenes" | "extra_traffic_density" | "ai_bus_greetings" | "ai_bus_overtaking" | "ai_driver_wave" | "cars_yield_to_buses" | "illegal_parking" | "independent_passenger_demand"
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
        key: "ai_driver_wave",
        label: "AI bus drivers wave when greeting",
    },
    Feature {
        key: "passenger_ambient_talk",
        label: "Optional passenger conversations inside buses",
    },
    Feature {
        key: "ai_bus_overtaking",
        label: "AI buses safely pass other stopped buses",
    },
    Feature {
        key: "cars_yield_to_buses",
        label: "Cars change lanes for buses leaving stops",
    },
    Feature {
        key: "illegal_parking",
        label: "Temporary illegally parked AI cars",
    },
    Feature {
        key: "independent_passenger_demand",
        label: "Independent passenger demand at each stop",
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
            "ai_driver_wave" => self.ai_driver_wave,
            "passenger_ambient_talk" => self.passenger_ambient_talk,
            "ai_bus_overtaking" => self.ai_bus_overtaking,
            "cars_yield_to_buses" => self.cars_yield_to_buses,
            "illegal_parking" => self.illegal_parking,
            "independent_passenger_demand" => self.independent_passenger_demand,
            "weather_clothing" => self.weather_clothing,
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
            "ai_driver_wave" => self.ai_driver_wave = enabled,
            "passenger_ambient_talk" => self.passenger_ambient_talk = enabled,
            "ai_bus_overtaking" => self.ai_bus_overtaking = enabled,
            "cars_yield_to_buses" => self.cars_yield_to_buses = enabled,
            "illegal_parking" => self.illegal_parking = enabled,
            "independent_passenger_demand" => self.independent_passenger_demand = enabled,
            "weather_clothing" => self.weather_clothing = enabled,
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
    fn options_start_on_are_independent_and_keep_explicit_false() {
        let mut value = OptionalFeatures::default();
        for feature in FEATURES {
            assert_eq!(value.get(feature.key), Some(true));
            value.set(feature.key, false).unwrap();
            for other in FEATURES {
                assert_eq!(value.get(other.key), Some(other.key != feature.key));
            }
            value.set(feature.key, true).unwrap();
            assert_eq!(value, OptionalFeatures::default());
        }
        assert_eq!(value.set("unknown_feature", true), None);
        let mut saved = String::new();
        value.write_text(&mut saved);
        for feature in FEATURES {
            assert!(saved.contains(&format!("{}=1\n", feature.key)));
        }
        assert!(!value.weather_clothing);
        assert!(!value.passenger_connections_enabled);
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
            value.set(feature.key, false);
        }
        assert!(!value.needs_bcs_capture());
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
