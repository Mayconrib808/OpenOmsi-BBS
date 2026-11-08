//! NVIDIA DLSS 5 Neural Rendering readiness, deliberately fail-closed.
//!
//! The public NVIDIA Streamline 2.14.16 SDK does not expose a documented DLSS 5
//! integration header/API, the model distribution contract, or signed NR runtime
//! through its normal public release. We cannot enable the feature just because
//! an RTX GPU and Super Resolution DLLs are present. Do not load community-patched
//! neural DLLs: they do not prove compatibility or redistribution rights.
//!
//! This launcher endpoint reports hardware readiness for a future official
//! integration. No UI control is allowed to claim rendering is active.

use serde_json::{json, Value};


#[cfg(windows)]
fn controller_config() -> std::path::PathBuf {
    omsi_launcher_lib::data_dir().join("dlss5-controller-path.txt")
}

/// An optional, independent third-party controller selected by the player.
/// It is never bundled, downloaded or injected by the Bridge.
#[cfg(windows)]
fn selected_controller() -> Option<std::path::PathBuf> {
    let path = std::fs::read_to_string(controller_config()).ok()?;
    let path = std::path::PathBuf::from(path.trim());
    valid_controller_path(&path).then_some(path)
}

#[cfg(windows)]
fn valid_controller_path(path: &std::path::Path) -> bool {
    path.is_absolute()
        && path.is_file()
        && path.file_name()
            .is_some_and(|name| name.to_string_lossy().eq_ignore_ascii_case("UniversalDLSS5.exe"))
}

/// Explicit choice of a *local* external controller binary; no networking or injection.
pub fn select_controller(path: String) -> Result<Value, String> {
    #[cfg(not(windows))]
    {
        let _ = path;
        Err("An external DLSS 5 controller is supported only on Windows".into())
    }
    #[cfg(windows)]
    {
        if omsi_cfg::upscaler_files::dlss5_adapter().is_none() {
            return Err("GeForce RTX 50 is required for DLSS 5 Neural Rendering".into());
        }
        let selected = std::path::PathBuf::from(path);
        if !valid_controller_path(&selected) {
            return Err("Select a local file named UniversalDLSS5.exe, from a source you trust".into());
        }
        let config = controller_config();
        if let Some(parent) = config.parent() {
            std::fs::create_dir_all(parent).map_err(|e| format!("Create DLSS 5 configuration folder: {e}"))?;
        }
        std::fs::write(&config, selected.to_string_lossy().as_bytes())
            .map_err(|e| format!("Save DLSS 5 controller location: {e}"))?;
        Ok(status())
    }
}

/// Open the third-party controller as itself. The player must explicitly select
/// Attach inside it: we neither launch the game twice nor automatically inject
/// into a running game or any multiplayer client.
pub fn open_controller() -> Result<Value, String> {
    #[cfg(not(windows))]
    {
        Err("An external DLSS 5 controller is supported only on Windows".into())
    }
    #[cfg(windows)]
    {
        if omsi_cfg::upscaler_files::dlss5_adapter().is_none() {
            return Err("GeForce RTX 50 is required for DLSS 5 Neural Rendering".into());
        }
        let exe = selected_controller().ok_or_else(|| {
            "Select an installed UniversalDLSS5.exe before opening its controller".to_string()
        })?;
        let folder = exe.parent().ok_or("DLSS 5 controller path has no parent directory")?;
        std::process::Command::new(&exe)
            .current_dir(folder)
            .spawn()
            .map_err(|e| format!("Start external DLSS 5 controller: {e}"))?;
        Ok(json!({"controller_opened": true, "neural_rendering_active": false}))
    }
}

pub fn status() -> Value {
    let gpu = omsi_cfg::upscaler_files::dlss5_adapter();
    let game_installed = cfg!(windows)
        && omsi_cfg::upscaler_files::program_dir().join("openomsi.exe").is_file();
    #[cfg(windows)]
    let controller = selected_controller();
    #[cfg(not(windows))]
    let controller: Option<std::path::PathBuf> = None;
    json!({
        "external_controller_found": controller.is_some(),
        "external_controller_path": controller.as_ref().map(|p| p.to_string_lossy().to_string()),
        "platform_supported": cfg!(all(windows, target_arch = "x86_64")),
        "game_installed": game_installed,
        "gpu_supported": gpu.is_some(),
        "gpu_name": gpu,
        "sdk_ready": false,
        "renderer_integrated": false,
        "active": false,
        "reason": "Official NVIDIA DLSS 5 Neural Rendering SDK integration and renderer motion-vector inputs are not available yet",
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn never_claim_neural_rendering_is_active_without_official_integration() {
        let state = status();
        assert_eq!(state["sdk_ready"], false);
        assert_eq!(state["renderer_integrated"], false);
        assert_eq!(state["active"], false);
    }
}
