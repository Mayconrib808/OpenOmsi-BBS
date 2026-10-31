//! Only explicit/curated vehicle definitions assign a service role. No filename heuristics.
use omsi_sim::{
    VehicleType,
    ai_traffic::response::{Service, Signals},
};
use serde::Deserialize;
use std::{
    collections::BTreeMap,
    path::{Component, Path, PathBuf},
    sync::{Arc, Mutex},
};

const BMW: &str = "Vehicles/MIPA/MIPA_AI_BMW_F31/MIPA_AI_BMW_F31_pol_active.ovh";
const ROMAN: &str = "Vehicles/RO_Roman_Diesel/Roman_apca.bus";
const AMBULANCE: &str = "Vehicles/MB_T1/ai_mb_t1_rtw.ovh";

#[derive(Clone)]
pub(super) struct Model {
    pub ty: Arc<VehicleType>,
    pub signals: Arc<Signals>,
}
#[derive(Default)]
pub(super) struct Catalog {
    pub medical: Option<Model>,
    pub police: Option<Model>,
    pub fire: Option<Model>,
}
impl Catalog {
    pub fn model(&self, service: Service) -> Option<Model> {
        match service {
            Service::Medical => &self.medical,
            Service::Police => &self.police,
            Service::Fire => &self.fire,
        }
        .clone()
    }
}

#[derive(Default, Deserialize)]
struct Binding {
    model: String,
    #[serde(default)]
    en_route: BTreeMap<String, f32>,
    #[serde(default)]
    on_scene: BTreeMap<String, f32>,
    #[serde(default)]
    departing: BTreeMap<String, f32>,
}
#[derive(Default, Deserialize)]
struct Config {
    #[serde(default)]
    ambulance: Option<Binding>,
    #[serde(default)]
    police: Option<Binding>,
    #[serde(default)]
    fire: Option<Binding>,
}

#[derive(Default)]
pub(super) struct Loader {
    value: Option<Arc<Mutex<Option<Catalog>>>>,
}
impl Loader {
    pub fn start(&mut self, root: PathBuf) {
        if self.value.is_some() {
            return;
        }
        if let Some(root)=std::env::current_exe().ok().and_then(|p|p.parent().map(|p|p.join("bcs-assets"))).filter(|p|p.is_dir()) {
            // A shared relative Vehicles/OpenOMSI_BCS path resolves on every
            // client/server; installed content retains its existing priority.
            omsi_cfg::add_content_root(root);
        }
        let value = Arc::new(Mutex::new(None));
        self.value = Some(value.clone());
        let launched = std::thread::Builder::new()
            .name("BCS emergency model loader".into())
            .spawn(move || {
                let path = root.join("bcs-emergency-vehicles.json");
                let config = if path.exists() {
                    match std::fs::metadata(&path)
                        .ok()
                        .filter(|m| m.len() <= 65536)
                        .and_then(|_| std::fs::read(&path).ok())
                        .and_then(|b| serde_json::from_slice::<Config>(&b).ok())
                    {
                        Some(c) => c,
                        None => {
                            log::warn!(
                                "BCS emergency model configuration is invalid; no models loaded"
                            );
                            return;
                        }
                    }
                } else {
                    Config::default()
                };
                let fallback = std::env::current_exe().ok().and_then(|p|p.parent().map(|p|p.join("bcs-assets")));
                let police_custom=config.police.is_some();let fire_custom=config.fire.is_some();
                let catalog = Catalog {
                    medical: load(&root, config.ambulance, AMBULANCE, Service::Medical),
                    police: load(&root, config.police, BMW, Service::Police).or_else(||
                        if police_custom {None} else {fallback.as_ref().and_then(|p|load(p,None,"Vehicles/OpenOMSI_BCS/Police/Police.ovh",Service::Police))}),
                    fire: load(&root, config.fire, ROMAN, Service::Fire).or_else(||
                        if fire_custom {None} else {fallback.as_ref().and_then(|p|load(p,None,"Vehicles/OpenOMSI_BCS/Fire/Fire.ovh",Service::Fire))}),
                };
                if let Ok(mut slot) = value.lock() {
                    *slot = Some(catalog);
                }
            });
        if launched.is_err() {
            self.value = None;
        }
    }
    pub fn model(&self, service: Service) -> Option<Model> {
        self.value.as_ref()?.lock().ok()?.as_ref()?.model(service)
    }
}

fn relative(text: &str) -> Option<PathBuf> {
    let text = text.replace('\\', "/");
    let path = PathBuf::from(&text);
    if text.contains(':')
        || path.is_absolute()
        || path
            .components()
            .any(|c| !matches!(c, Component::Normal(_)))
    {
        return None;
    }
    match path.extension()?.to_str()?.to_ascii_lowercase().as_str() {
        "bus" | "ovh" => Some(path),
        _ => None,
    }
}

fn load(root: &Path, binding: Option<Binding>, default: &str, service: Service) -> Option<Model> {
    let binding = binding.unwrap_or_else(|| Binding {
        model: default.into(),
        ..Default::default()
    });
    let signals = signals(&binding, service)?;
    let relative = relative(&binding.model)?;
    let path = omsi_cfg::resolve_path(root, relative.to_str()?);
    if !path.is_file() {
        log::info!(
            "BCS {:?} visual model not installed: {}",
            service,
            relative.display()
        );
        return None;
    }
    let canonical = root.canonicalize().ok()?;
    if !path.canonicalize().ok()?.starts_with(&canonical) {
        return None;
    }
    let ty = match VehicleType::load_ai(root, &path) {
        Ok(t) => Arc::new(t),
        Err(e) => {
            log::warn!("BCS {:?} model could not load: {e}", service);
            return None;
        }
    };
    Some(Model {
        ty,
        signals: Arc::new(signals),
    })
}

fn signals(binding: &Binding, service: Service) -> Option<Signals> {
    let valid = |vars: &BTreeMap<String, f32>| {
        vars.len() <= 32
            && vars
                .iter()
                .all(|(n, v)| !n.is_empty() && n.len() <= 80 && v.is_finite() && v.abs() <= 1e6)
    };
    if !valid(&binding.en_route) || !valid(&binding.on_scene) || !valid(&binding.departing) {
        return None;
    }
    let model = binding.model.replace('\\', "/");
    let mut signals = {
        let variables = |names: &[&str], v: f32| names.iter().map(|n| (n.to_string(), v)).collect();
        match (service, model.as_str()) {
            (Service::Police, path) if path.eq_ignore_ascii_case(BMW) => Signals {
                en_route: variables(&["rtk_stadthorn"], 1.0),
                on_scene: variables(&["rtk_stadthorn"], 0.0),
                departing: variables(
                    &["rtk_stadthorn", "bl_frontblitzer", "bl_rtk1", "bl_rtk2"],
                    0.0,
                ),
            },
            (Service::Fire, path) if path.eq_ignore_ascii_case(ROMAN) => Signals {
                en_route: variables(&["Folgetonhorn"], 1.0),
                on_scene: variables(&["Folgetonhorn"], 0.0),
                departing: variables(&["Folgetonhorn", "blaulicht_1", "blaulicht_2"], 0.0),
            },
            (_, path) if path.starts_with("Vehicles/OpenOMSI_BCS/") => Signals {
                en_route: variables(&["bcs_siren","bcs_beacon"],1.0),
                on_scene: vec![("bcs_siren".into(),0.0),("bcs_beacon".into(),1.0)],
                departing: variables(&["bcs_siren","bcs_beacon","bcs_blue"],0.0),
            },
            _ => Signals::default(),
        }
    };
    for (target, overrides) in [
        (&mut signals.en_route, &binding.en_route),
        (&mut signals.on_scene, &binding.on_scene),
        (&mut signals.departing, &binding.departing),
    ] {
        let mut merged: BTreeMap<_, _> = std::mem::take(target).into_iter().collect();
        merged.extend(overrides.iter().map(|(name, value)| (name.clone(), *value)));
        *target = merged.into_iter().collect();
    }
    Some(signals)
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn model_must_be_a_relative_vehicle_definition() {
        assert!(relative(BMW).is_some());
        assert!(relative(ROMAN).is_some());
        for s in [
            "C:\\outside.ovh",
            "../outside.bus",
            "/tmp/other.ovh",
            "Vehicles/car.exe",
            "",
        ] {
            assert!(relative(s).is_none(), "{s}");
        }
    }
    #[test]
    fn explicit_default_keeps_curated_signals_and_allows_overrides() {
        let mut binding = Binding {
            model: BMW.replace('/', "\\"),
            ..Default::default()
        };
        binding.on_scene.insert("rtk_stadthorn".into(), 0.5);
        let signals = signals(&binding, Service::Police).unwrap();
        assert!(signals.en_route.contains(&("rtk_stadthorn".into(), 1.0)));
        assert!(signals.on_scene.contains(&("rtk_stadthorn".into(), 0.5)));
        assert!(signals.departing.contains(&("bl_frontblitzer".into(), 0.0)));
    }
    #[test]
    fn an_unrelated_model_does_not_receive_bmw_script_variables() {
        let binding = Binding {
            model: "Vehicles/MyPolice/custom.ovh".into(),
            ..Default::default()
        };
        let signals = signals(&binding, Service::Police).unwrap();
        assert!(signals.en_route.is_empty());
        assert!(signals.departing.is_empty());
    }
}
