// Compile the actual capture reader and lifecycle without rebuilding a second game binary.
#[path = "../crates/omsi-app/src/bcs_events/lifecycle.rs"]
mod lifecycle;
#[path = "../crates/omsi-app/src/bcs_events/feed.rs"]
mod feed;
