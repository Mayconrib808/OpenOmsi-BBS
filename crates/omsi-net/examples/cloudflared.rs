//! Fetch cloudflared when it is missing (`OMSI_CLOUDFLARED_OWN=1` to ignore an installed one).
fn main() {
    let _ = env_logger_init();
    println!("{:?}", omsi_net::tunnel::ensure_cloudflared());
}
fn env_logger_init() {}
