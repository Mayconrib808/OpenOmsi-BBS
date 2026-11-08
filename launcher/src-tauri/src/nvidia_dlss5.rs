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

pub fn status() -> Value {
    let gpu = omsi_cfg::upscaler_files::dlss5_adapter();
    let game_installed = cfg!(windows)
        && omsi_cfg::upscaler_files::program_dir().join("openomsi.exe").is_file();
    json!({
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
