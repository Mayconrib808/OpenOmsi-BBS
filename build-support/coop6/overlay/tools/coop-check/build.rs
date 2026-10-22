fn read(path: &str) -> String {
    println!("cargo:rerun-if-changed={path}");
    std::fs::read_to_string(path).unwrap()
}
fn between<'a>(text: &'a str, from: &str, to: &str) -> &'a str {
    let start = text.find(from).unwrap();
    &text[start..start + text[start..].find(to).unwrap()]
}
fn main() {
    let out = std::path::PathBuf::from(std::env::var_os("OUT_DIR").unwrap());
    let util = read("../../crates/omsi-scene/src/camera_util.rs");
    let start = util.find("pub fn ray_may_hit(").unwrap();
    let tail = &util[start..];
    let end = tail.find("\n}\n").unwrap() + 3;
    std::fs::write(out.join("broad_phase.rs"), &tail[..end]).unwrap();

    // Compile the production projection and unprojection without opening a GPU
    // or compiling the renderer's platform integrations for this native harness.
    let view = read("../../crates/omsi-render/src/view.rs");
    std::fs::write(
        out.join("camera.rs"),
        between(
            &view,
            "#[derive(Debug, Clone, Copy)]\npub struct Camera",
            "#[derive(Clone, Debug)]\npub struct Lighting",
        ),
    )
    .unwrap();
    std::fs::write(
        out.join("cursor.rs"),
        between(
            &util,
            "pub fn pixel_angle(",
            "/// Whether a mirror is in the picture",
        ),
    )
    .unwrap();

    // Exercise the actual App routing method against an intermediate avatar
    // camera; no duplicate implementation of the camera selection in the test.
    let coop = read("../../crates/omsi-app/src/coop.rs");
    let method = between(&coop, "    fn coop_cursor_ray(", "\n    fn coop_hit(");
    let capture = between(
        &coop,
        "    pub(crate) fn coop_drawn_view(",
        "\n    fn coop_drop_ticket_blocks(",
    );
    std::fs::write(
        out.join("coop_cursor.rs"),
        format!("impl App {{\n{method}\n{capture}\n}}\n"),
    )
    .unwrap();

    // Native paper-ticket clicks use the same production geometry as drawing.
    let money = read("../../crates/omsi-scene/src/money.rs");
    let fields = between(&money, "pub struct TicketBlocks {", "\nimpl TicketBlocks {");
    let start = money.find("    pub fn hit(").unwrap();
    let tail = &money[start..];
    let end = tail.find("\n    }\n").unwrap() + 7;
    std::fs::write(
        out.join("ticket_hit.rs"),
        format!("{fields}\nimpl TicketBlocks {{\n{}\n}}", &tail[..end]),
    )
    .unwrap();
}
