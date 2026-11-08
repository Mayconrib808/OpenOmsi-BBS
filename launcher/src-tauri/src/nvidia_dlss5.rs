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
static INSTALLING_CONTROLLER: std::sync::atomic::AtomicBool = std::sync::atomic::AtomicBool::new(false);

#[cfg(windows)]
struct ControllerInstallGuard;

#[cfg(windows)]
impl ControllerInstallGuard {
    fn new() -> Result<Self, String> {
        INSTALLING_CONTROLLER.compare_exchange(false, true,
            std::sync::atomic::Ordering::AcqRel, std::sync::atomic::Ordering::Relaxed)
            .map_err(|_| "A DLSS 5 controller installation is already running".to_string())?;
        Ok(Self)
    }
}

#[cfg(windows)]
impl Drop for ControllerInstallGuard {
    fn drop(&mut self) {
        INSTALLING_CONTROLLER.store(false, std::sync::atomic::Ordering::Release);
    }
}

#[cfg(windows)]
struct TemporaryDirectory(std::path::PathBuf);

#[cfg(windows)]
impl Drop for TemporaryDirectory {
    fn drop(&mut self) { let _ = std::fs::remove_dir_all(&self.0); }
}

#[cfg(windows)]
fn controller_config() -> std::path::PathBuf {
    omsi_launcher_lib::data_dir().join("dlss5-controller-path.txt")
}

/// An independent third-party controller selected by the player or found
/// after an explicit user-initiated, checksum-verified installer download.
#[cfg(windows)]
fn selected_controller() -> Option<std::path::PathBuf> {
    if let Ok(saved) = std::fs::read_to_string(controller_config()) {
        let saved = std::path::PathBuf::from(saved.trim());
        if valid_controller_path(&saved) { return Some(saved); }
    }
    // NSIS's common install locations. No scanning arbitrary drives or DLL loading.
    let mut candidates = Vec::new();
    for key in ["ProgramFiles", "ProgramW6432", "ProgramFiles(x86)"] {
        if let Some(root) = std::env::var_os(key) {
            candidates.push(std::path::PathBuf::from(root).join("UniversalDLSS5").join("UniversalDLSS5.exe"));
        }
    }
    if let Some(root) = std::env::var_os("LOCALAPPDATA") {
        candidates.push(std::path::PathBuf::from(&root).join("Programs").join("UniversalDLSS5").join("UniversalDLSS5.exe"));
        candidates.push(std::path::PathBuf::from(root).join("UniversalDLSS5").join("UniversalDLSS5.exe"));
    }
    candidates.into_iter().find(|p| valid_controller_path(p))
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


/// The player explicitly requests installation: fetch the controller from its
/// published v0.5.1 GitHub release, validate the pinned SHA256, then open NSIS.
/// Does NOT download or install NVIDIA proprietary DLSS-NR runtime files.
/// An open installer is not evidence that Neural Rendering is active.
pub fn install_controller(accepted_terms: bool) -> Result<Value, String> {
    if !accepted_terms {
        return Err("Review and accept the third-party controller installation notice first".into());
    }
    #[cfg(not(all(windows, target_arch = "x86_64")))]
    {
        Err("The experimental DLSS 5 controller installer requires Windows x64".into())
    }
    #[cfg(all(windows, target_arch = "x86_64"))]
    {
        let _guard = ControllerInstallGuard::new()?;
        if omsi_cfg::upscaler_files::dlss5_adapter().is_none() {
            return Err("The DLSS 5 controller is offered only for GeForce RTX 50".into());
        }
        let cache = omsi_launcher_lib::data_dir().join("dlss5-controller");
        std::fs::create_dir_all(&cache)
            .map_err(|e| format!("Create DLSS 5 installer cache: {e}"))?;
        let installer = cache.join("UniversalDLSS5-0.5.1-Setup-x64.exe");
        let workspace = std::env::temp_dir().join(format!(
            "openomsi-dlss5-controller-{}-{}",
            std::process::id(),
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH).map_err(|e| e.to_string())?.as_nanos()
        ));
        std::fs::create_dir(&workspace)
            .map_err(|e| format!("Create DLSS 5 download workspace: {e}"))?;
        let _cleanup = TemporaryDirectory(workspace.clone());
        let script = workspace.join("download-dlss5-controller.ps1");
        std::fs::write(&script, include_str!("../../../scripts/windows/download-dlss5-controller.ps1"))
            .map_err(|e| format!("Prepare verified DLSS 5 downloader: {e}"))?;
        let powershell = std::path::PathBuf::from(
            std::env::var_os("SystemRoot").unwrap_or_else(|| "C:\\Windows".into())
        ).join(r"System32\WindowsPowerShell\v1.0\powershell.exe");
        use std::os::windows::process::CommandExt;
        let output = std::process::Command::new(&powershell)
            .creation_flags(0x0800_0000)
            .args(["-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File"])
            .arg(&script).arg("-Destination").arg(&installer)
            .output()
            .map_err(|e| format!("Launch verified DLSS 5 downloader: {e}"))?;
        if !output.status.success() {
            let out = String::from_utf8_lossy(&output.stderr);
            let detail = if out.trim().is_empty() {
                String::from_utf8_lossy(&output.stdout).to_string()
            } else { out.to_string() };
            return Err(format!("DLSS 5 controller download or SHA-256 verification failed: {}",
                detail.trim().chars().take(1200).collect::<String>()));
        }
        if !installer.is_file() {
            return Err("The verified DLSS 5 installer is missing after download".into());
        }
        // The NSIS installer remains interactive: the player reviews and
        // approves the separate third-party app's installation themselves.
        // A restart/re-check of launcher status will locate its common path.
        std::process::Command::new(&installer)
            .current_dir(&cache)
            .spawn()
            .map_err(|e| format!("Open verified third-party installer: {e}"))?;
        let mut state = status();
        state["controller_installer_started"] = json!(true);
        state["neural_rendering_active"] = json!(false);
        Ok(state)
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
        "controller_installer_available": cfg!(all(windows, target_arch = "x86_64")),
        "controller_setup_release": "UniversalDLSS5 v0.5.1 (experimental, independent third party)",
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
