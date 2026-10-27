//! Make the official server's signing key: `official_keygen <file>` writes the PKCS#8 key
//! there (never into the repository) and prints the public key for `official::PUBLIC_KEY`.
fn main() {
    let path = std::env::args().nth(1).expect("official_keygen <key file>");
    if std::path::Path::new(&path).exists() {
        eprintln!("{path} exists; not overwriting it");
        std::process::exit(1);
    }
    let (doc, public) = omsi_net::official::generate_key().expect("key");
    std::fs::write(&path, doc).expect("write");
    println!("{public}");
}
