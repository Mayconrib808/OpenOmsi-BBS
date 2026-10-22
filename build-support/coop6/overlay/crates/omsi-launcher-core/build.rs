//! On Windows, the application icon goes into `openomsi-launcher.exe`.

fn main() {
    // When building only the cooperative game, this crate is a library dependency.
    // Its GNU whole-archive resources would duplicate the game's icon/version.
    // The portable package keeps the official standalone launcher binaries.
    println!("cargo:rerun-if-env-changed=OPENOMSI_COOP_GAME_ONLY");
    if std::env::var("OPENOMSI_COOP_GAME_ONLY").as_deref() == Ok("1") {
        return;
    }
    if std::env::var("CARGO_CFG_TARGET_OS").as_deref() != Ok("windows") {
        return;
    }
    println!("cargo:rerun-if-changed=../../assets/icons/app/openomsi.ico");
    let mut res = winresource::WindowsResource::new();
    res.set_icon("../../assets/icons/app/openomsi.ico").set("ProductName", "openOMSI").set("FileDescription", "openOMSI launcher tools");
    if let Err(e) = res.compile() {
        println!("cargo:warning=no icon in the executable: {e}");
    }
}
