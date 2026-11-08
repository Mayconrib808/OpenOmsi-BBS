//! Optional ReShade Vulkan layer, scoped to the openOMSI game process.

use ash::vk;
use ring::digest::{SHA256, digest};
use serde_json::{Value, json};
use std::ffi::{CStr, OsString};
use std::io::Read;
use std::path::{Path, PathBuf};
use std::sync::Mutex;
use std::sync::atomic::{AtomicBool, Ordering};
use std::time::Duration;

const DOWNLOAD: &str = "https://reshade.me/downloads/ReShade_Setup_6.8.0.exe";
const SETUP_SHA256: [u8; 32] = [
    0x20, 0x7a, 0xea, 0x16, 0x20, 0x5f, 0xbf, 0x95, 0x2b, 0xc8, 0xfe, 0x18, 0x79, 0x96, 0x66, 0x72,
    0x45, 0x4c, 0xf0, 0x40, 0x02, 0xe7, 0xad, 0x34, 0x23, 0x7c, 0x79, 0x90, 0xa5, 0xb3, 0xc0, 0xb4,
];
const DLL_SHA256: [u8; 32] = [
    0xb2, 0x94, 0x5c, 0x29, 0xe7, 0x09, 0x54, 0x91, 0xa9, 0x01, 0x74, 0x6b, 0x40, 0x0e, 0x58, 0xdb,
    0x9b, 0x15, 0x92, 0xab, 0x09, 0x2b, 0xac, 0xf2, 0xa8, 0x88, 0xce, 0x37, 0xf0, 0x2d, 0x08, 0xda,
];
const DLL: &str = "ReShade64.dll";
const MANIFEST: &str = "openomsi-reshade.json";
const LAYER: &str = "VK_LAYER_OPENOMSI_reshade";
const LAYER_JSON: &str = r#"{
  "file_format_version": "1.0.0",
  "layer": {
    "name": "VK_LAYER_OPENOMSI_reshade",
    "type": "GLOBAL",
    "library_path": "./ReShade64.dll",
    "api_version": "1.3.268",
    "implementation_version": "1",
    "description": "ReShade for openOMSI",
    "device_extensions": [{
      "name": "VK_EXT_tooling_info",
      "spec_version": "1",
      "entrypoints": ["vkGetPhysicalDeviceToolPropertiesEXT"]
    }]
  }
}
"#;
const MAX_DOWNLOAD: u64 = 24 << 20;
static INSTALL: Mutex<()> = Mutex::new(());
static ACTIVE: AtomicBool = AtomicBool::new(false);

fn dir() -> PathBuf {
    super::data_dir().join("reshade").join("6.8.0")
}

fn enabled_path() -> PathBuf {
    super::data_dir().join("reshade").join("enabled")
}

fn checked_bytes(path: &Path, expected: &[u8; 32]) -> bool {
    std::fs::metadata(path).is_ok_and(|m| m.len() <= MAX_DOWNLOAD)
        && std::fs::read(path).is_ok_and(|bytes| digest(&SHA256, &bytes).as_ref() == expected)
}

fn installed(dir: &Path) -> bool {
    let layer = dir.join("layer");
    checked_bytes(&layer.join(DLL), &DLL_SHA256)
        && std::fs::read_to_string(layer.join(MANIFEST)).is_ok_and(|text| text == LAYER_JSON)
        && std::fs::read_dir(&layer).is_ok_and(|mut entries| {
            entries.all(|e| {
                e.is_ok_and(|e| {
                    e.file_name() == MANIFEST
                        || !e
                            .path()
                            .extension()
                            .is_some_and(|ext| ext.eq_ignore_ascii_case("json"))
                })
            })
        })
        && dir.join("ReShade.ini").is_file()
        && dir.join("OpenOMSIRoadFilm.ini").is_file()
        && dir.join("OpenOMSIRoadFilm.fx").is_file()
}

fn loader() -> Option<ash::Entry> {
    let system = PathBuf::from(std::env::var_os("SystemRoot")?).join("System32/vulkan-1.dll");
    // Only the installed system Vulkan loader is allowed to run during detection.
    unsafe { ash::Entry::load_from(system) }.ok()
}

fn graphics_device(
    info: &vk::PhysicalDeviceProperties,
    queues: &[vk::QueueFamilyProperties],
) -> bool {
    matches!(
        info.device_type,
        vk::PhysicalDeviceType::INTEGRATED_GPU | vk::PhysicalDeviceType::DISCRETE_GPU
    ) && info.api_version >= vk::API_VERSION_1_1
        && queues
            .iter()
            .any(|q| q.queue_count > 0 && q.queue_flags.contains(vk::QueueFlags::GRAPHICS))
}

fn vulkan_gpu() -> Option<String> {
    let entry = loader()?;
    let app = vk::ApplicationInfo::default().api_version(vk::API_VERSION_1_1);
    // The probe creates no device and enables no layers or instance extensions.
    unsafe {
        let instance = entry
            .create_instance(
                &vk::InstanceCreateInfo::default().application_info(&app),
                None,
            )
            .ok()?;
        let mut adapters = instance
            .enumerate_physical_devices()
            .unwrap_or_default()
            .into_iter()
            .filter_map(|device| {
                let info = instance.get_physical_device_properties(device);
                let queues = instance.get_physical_device_queue_family_properties(device);
                graphics_device(&info, &queues).then(|| {
                    (
                        info.device_type == vk::PhysicalDeviceType::DISCRETE_GPU,
                        CStr::from_ptr(info.device_name.as_ptr())
                            .to_string_lossy()
                            .into_owned(),
                    )
                })
            })
            .collect::<Vec<_>>();
        instance.destroy_instance(None);
        adapters.sort_by_key(|(discrete, _)| !*discrete);
        adapters.into_iter().next().map(|(_, name)| name)
    }
}

fn compatible_api(api: &str) -> bool {
    matches!(
        api.trim().to_ascii_lowercase().as_str(),
        "" | "auto" | "vulkan"
    )
}

fn manual_reshade(game: &Path) -> bool {
    game.join("ReShade.ini").is_file() || game.join("dxgi.dll").is_file()
}

pub fn status() -> Value {
    let game = omsi_cfg::upscaler_files::program_dir();
    let gpu = vulkan_gpu();
    let conflict = manual_reshade(&game);
    let supported = game.join("openomsi.exe").is_file() && gpu.is_some() && !conflict;
    json!({
        "gpu": gpu,
        "supported": supported,
        "conflict": conflict,
        "installed": installed(&dir()),
        "enabled": enabled_path().is_file(),
    })
}

fn put(path: &Path, bytes: &[u8]) -> anyhow::Result<()> {
    let staged = path.with_extension(format!("{}.tmp", std::process::id()));
    let result = (|| -> std::io::Result<()> {
        std::fs::write(&staged, bytes)?;
        std::fs::rename(&staged, path)
    })();
    if result.is_err() {
        let _ = std::fs::remove_file(staged);
    }
    result.map_err(Into::into)
}

fn extract_dll(setup: &[u8]) -> anyhow::Result<Vec<u8>> {
    use anyhow::ensure;
    ensure!(
        setup.len() as u64 <= MAX_DOWNLOAD && digest(&SHA256, setup).as_ref() == SETUP_SHA256,
        "The official ReShade download does not match the pinned SHA-256"
    );
    let mut zip = zip::ZipArchive::new(std::io::Cursor::new(setup))?;
    let mut dll = Vec::new();
    zip.by_name(DLL)?
        .take(MAX_DOWNLOAD + 1)
        .read_to_end(&mut dll)?;
    ensure!(
        dll.len() as u64 <= MAX_DOWNLOAD && digest(&SHA256, &dll).as_ref() == DLL_SHA256,
        "The ReShade Vulkan DLL does not match the pinned SHA-256"
    );
    Ok(dll)
}

fn install(dir: &Path) -> anyhow::Result<()> {
    use anyhow::{Context, ensure};
    let agent = ureq::AgentBuilder::new()
        .timeout(Duration::from_secs(45))
        .redirects(0)
        .build();
    let mut setup = Vec::new();
    agent
        .get(DOWNLOAD)
        .call()
        .context("Download ReShade from reshade.me")?
        .into_reader()
        .take(MAX_DOWNLOAD + 1)
        .read_to_end(&mut setup)?;
    let dll = extract_dll(&setup)?;
    let layer = dir.join("layer");
    std::fs::create_dir_all(&layer)?;
    put(&layer.join(DLL), &dll)?;
    put(&layer.join(MANIFEST), LAYER_JSON.as_bytes())?;
    for (name, contents) in [
        ("ReShade.ini", include_bytes!("ReShade.ini").as_slice()),
        (
            "OpenOMSIRoadFilm.ini",
            include_bytes!("OpenOMSIRoadFilm.ini").as_slice(),
        ),
        (
            "OpenOMSIRoadFilm.fx",
            include_bytes!("OpenOMSIRoadFilm.fx").as_slice(),
        ),
    ] {
        let path = dir.join(name);
        if !path.exists() {
            put(&path, contents)?;
        }
    }
    ensure!(
        installed(dir),
        "The ReShade files could not be verified after installation"
    );
    Ok(())
}

/// The explicit launcher button downloads only when a verified runtime is absent.
pub fn set_enabled(enabled: bool) -> anyhow::Result<Value> {
    use anyhow::ensure;
    let _guard = INSTALL
        .try_lock()
        .map_err(|_| anyhow::anyhow!("Another ReShade operation is in progress"))?;
    if enabled {
        ensure!(
            status()["supported"].as_bool() == Some(true),
            "ReShade needs a Vulkan-capable GPU and an unmodified openOMSI graphics installation"
        );
        let dir = dir();
        if !installed(&dir) {
            install(&dir)?;
        }
        put(&enabled_path(), b"1\n")?;
    } else {
        match std::fs::remove_file(enabled_path()) {
            Ok(()) => {}
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => {}
            Err(e) => return Err(e.into()),
        }
    }
    Ok(status())
}

fn with_layer(layers: Option<OsString>) -> OsString {
    let mut names = layers.unwrap_or_default();
    if !names.is_empty() {
        names.push(";");
    }
    names.push(LAYER);
    names
}

/// Called only for a normal game before it creates any graphics instance.
pub fn prepare_game(api: &str) -> bool {
    let dir = dir();
    if !enabled_path().is_file()
        || !compatible_api(api)
        || manual_reshade(&omsi_cfg::upscaler_files::program_dir())
        || !installed(&dir)
        || vulkan_gpu().is_none()
    {
        return false;
    }

    let search = if std::env::var_os("VK_LAYER_PATH").is_some() {
        "VK_LAYER_PATH"
    } else {
        "VK_ADD_LAYER_PATH"
    };
    let inherited = std::env::var_os(search);
    let paths = inherited
        .as_ref()
        .into_iter()
        .flat_map(std::env::split_paths);
    let Ok(paths) = std::env::join_paths(std::iter::once(dir.join("layer")).chain(paths)) else {
        return false;
    };

    // This module is Windows x64 only: changing the process environment is thread-safe.
    unsafe { std::env::set_var(search, paths) };
    let visible = loader()
        .and_then(|e| unsafe { e.enumerate_instance_layer_properties() }.ok())
        .is_some_and(|layers| {
            layers.iter().any(|l| {
                l.layer_name_as_c_str()
                    .is_ok_and(|n| n.to_bytes() == LAYER.as_bytes())
            })
        });
    if !visible {
        // For example, the Vulkan loader ignores layer search overrides for elevated processes.
        unsafe {
            match inherited {
                Some(value) => std::env::set_var(search, value),
                None => std::env::remove_var(search),
            }
        }
        return false;
    }
    unsafe {
        std::env::set_var(
            "VK_INSTANCE_LAYERS",
            with_layer(std::env::var_os("VK_INSTANCE_LAYERS")),
        );
        std::env::set_var("RESHADE_BASE_PATH_OVERRIDE", dir);
    }
    ACTIVE.store(true, Ordering::Relaxed);
    true
}

pub fn active() -> bool {
    ACTIVE.load(Ordering::Relaxed)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn reshade_requires_a_real_vulkan_graphics_device() {
        let info = vk::PhysicalDeviceProperties {
            api_version: vk::API_VERSION_1_1,
            device_type: vk::PhysicalDeviceType::INTEGRATED_GPU,
            ..Default::default()
        };
        let queue = vk::QueueFamilyProperties {
            queue_count: 1,
            queue_flags: vk::QueueFlags::GRAPHICS,
            ..Default::default()
        };
        assert!(graphics_device(&info, &[queue]));
        assert!(!graphics_device(&info, &[]));
        assert!(!graphics_device(
            &vk::PhysicalDeviceProperties {
                device_type: vk::PhysicalDeviceType::CPU,
                ..info
            },
            &[queue]
        ));
        assert!(!graphics_device(
            &vk::PhysicalDeviceProperties {
                api_version: vk::API_VERSION_1_0,
                ..info
            },
            &[queue]
        ));
        assert!(compatible_api("auto") && compatible_api("vulkan") && !compatible_api("dx12"));
    }
}
