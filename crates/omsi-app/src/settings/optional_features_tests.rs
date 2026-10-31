use super::Settings;
use omsi_cfg::optional_features::{FEATURES, HELP, HOST_HELP};

#[test]
fn both_settings_writers_preserve_every_independent_switch_and_explicit_false() {
    for bits in 0..(1usize << FEATURES.len()) {
        let mut text = String::from("version=4\npassenger_effects_221=1\nboarding=walk\n");
        for (index, feature) in FEATURES.iter().enumerate() {
            text.push_str(&format!(
                "{}={}\n",
                feature.key,
                usize::from(bits & (1 << index) != 0)
            ));
        }
        let native = Settings::from_text(&text);
        let launcher = omsi_launcher_lib::settings_from_text_on(Some(&text), false);
        let native_saved = native.to_text();
        let launcher_saved = omsi_launcher_lib::settings_to_text(&launcher, Some(&text));
        let back_native = Settings::from_text(&launcher_saved);
        let back_launcher = omsi_launcher_lib::settings_from_text_on(Some(&native_saved), false);
        assert_eq!(back_native.boarding, "walk");
        for (index, feature) in FEATURES.iter().enumerate() {
            let enabled = bits & (1 << index) != 0;
            assert_eq!(
                native.optional_features.get(feature.key),
                Some(enabled),
                "native {}",
                feature.key
            );
            assert_eq!(
                launcher[feature.key].as_bool(),
                Some(enabled),
                "launcher {}",
                feature.key
            );
            assert_eq!(
                back_native.optional_features.get(feature.key),
                Some(enabled),
                "native restart {}",
                feature.key
            );
            assert_eq!(
                back_launcher[feature.key].as_bool(),
                Some(enabled),
                "launcher restart {}",
                feature.key
            );
            for saved in [&native_saved, &launcher_saved] {
                assert_eq!(
                    saved
                        .lines()
                        .filter(|line| line.starts_with(&format!("{}=", feature.key)))
                        .count(),
                    1
                );
                assert!(saved.contains(&format!("{}={}\n", feature.key, u8::from(enabled))));
            }
        }
    }
}

#[test]
fn fresh_and_legacy_settings_do_not_silently_enable_optional_additions() {
    for text in [
        "",
        "version=2\n",
        "version=4\npassenger_umbrellas=0\npassenger_hailing=0\n",
    ] {
        let native = Settings::from_text(text);
        let launcher = omsi_launcher_lib::settings_from_text_on(Some(text), false);
        for feature in FEATURES {
            assert_eq!(native.optional_features.get(feature.key), Some(false));
            assert_eq!(launcher[feature.key], false);
        }
        assert!(!native.passenger_umbrellas && !native.passenger_hailing);
        assert_eq!(launcher["passenger_umbrellas"], false);
        assert_eq!(launcher["passenger_hailing"], false);
    }
    assert_eq!(Settings::default().passenger_animation, "original");
    let empty_saved = omsi_launcher_lib::settings_to_text(&serde_json::json!({}), None);
    let empty_native = Settings::from_text(&empty_saved);
    assert!(!empty_native.passenger_hailing && !empty_native.passenger_umbrellas);
    assert_eq!(empty_native.passenger_animation, "original");
    let legacy =
        "version=4\npassenger_animation=original\npassenger_umbrellas=1\npassenger_hailing=1\n";
    let native = Settings::from_text(legacy);
    assert_eq!(native.passenger_animation, "original");
    assert!(native.passenger_umbrellas && native.passenger_hailing);
}

#[test]
fn every_optional_control_and_help_has_catalog_text_in_all_supported_languages() {
    let keys = FEATURES.iter().map(|feature| feature.label).chain([
        HELP,
        HOST_HELP,
        "Passenger umbrellas",
        "Signal buses on this map",
        "Passengers signal approaching buses",
        "Connection chance",
        "Probability for eligible transfers; default 20%",
        "Override ticket purchase chance",
        "Ticket purchase chance",
        "Default",
        "How passengers get their tickets",
        "No maps found.",
    ]);
    for key in keys {
        for &(_, _, locale, _, _) in omsi_cfg::lang::LANGUAGES {
            if locale == "en" {
                continue;
            }
            let native = crate::_rust_i18n_try_translate(locale, key)
                .unwrap_or_else(|| panic!("missing {locale}: {key}"));
            assert!(!native.trim().is_empty());
            assert_ne!(
                native.as_ref(),
                key,
                "English placeholder for {locale}: {key}"
            );
            let table = omsi_launcher_lib::i18n::table(crate::_LOCALES, locale);
            assert_eq!(
                table.get(key).map(String::as_str),
                Some(native.as_ref()),
                "different settings interfaces {locale}: {key}"
            );
        }
    }
}
