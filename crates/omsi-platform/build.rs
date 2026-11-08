//! Stamps the build with its version (`MAJOR.MINOR.PATCH`, see docs/VERSIONING.md) and the
//! commit it came from, so a screenshot or a log line says which version is running
//! (`omsi_platform::VERSION`, `omsi_platform::BUILD`; the updater compares the version with
//! the latest release).

use std::process::Command;

fn main() {
    let git = |args: &[&str]| -> Option<String> {
        let out = Command::new("git").args(args).output().ok()?;
        out.status.success().then(|| String::from_utf8_lossy(&out.stdout).trim().to_string())
    };
    let hash = git(&["rev-parse", "--short", "HEAD"]).unwrap_or_else(|| "unknown".into());
    let date = git(&["log", "-1", "--format=%cd", "--date=format:%Y-%m-%d %H:%M"]).unwrap_or_default();
    let dirty = git(&["status", "--porcelain"]).map(|s| !s.trim().is_empty()).unwrap_or(false);
    println!("cargo:rustc-env=OMSI_BUILD={hash}{} {date}", if dirty { "+" } else { "" });
    println!("cargo:rustc-env=OPENOMSI_VERSION={}", version(&git));
    println!("cargo:rerun-if-env-changed=OPENOMSI_VERSION");
    println!("cargo:rerun-if-changed=../../VERSION");
    println!("cargo:rerun-if-changed=../../ENGINE_BASE.json");
    println!("cargo:rerun-if-changed=../../.git/HEAD");
    println!("cargo:rerun-if-changed=../../.git/index");
}

/// `MAJOR.MINOR` from the VERSION file, then the release tagged on this commit, or one more
/// than the last release of that `MAJOR.MINOR` (as scripts/version.sh; the CI passes its
/// result in `OPENOMSI_VERSION`).
fn version(git: &dyn Fn(&[&str]) -> Option<String>) -> String {
    if let Ok(v) = std::env::var("OPENOMSI_VERSION") {
        if !v.trim().is_empty() {
            // This candidate carries the full v0.3.52 engine and Coop protocol 17.
            // Fail closed if the source manifest and requested Windows label disagree.
            let base = std::fs::read_to_string("../../ENGINE_BASE.json")
                .expect("Missing ENGINE_BASE.json for the 0.3.52/P17 release");
            assert!(
                base.contains("\"version\": \"0.3.52\"")
                    && base.contains("\"upstream_tag\": \"v0.3.52\"")
                    && base.contains("\"protocol\": 17"),
                "Engine source metadata does not identify verified 0.3.52/P17"
            );
            assert!(
                v.trim() == "0.3.52-coop.17" || v.trim().starts_with("0.3.52-coop.17-"),
                "0.3.52/P17 engine build must use an exact matching release label"
            );
            return v.trim().to_string();
        }
    }
    if std::path::Path::new("../../ENGINE_BASE.json").is_file() { return "0.3.52-coop.17-dev".into(); }
    let base = std::fs::read_to_string("../../VERSION").map(|s| s.trim().to_string()).unwrap_or_else(|_| "0.0".into());
    let prefix = format!("v{base}.");
    let pattern = format!("{prefix}*");
    let last = |args: &[&str]| -> Option<u64> {
        git(args)?
            .lines()
            .filter_map(|t| t.strip_prefix(&prefix).filter(|n| n.bytes().all(|b| b.is_ascii_digit()))?.parse().ok())
            .max()
    };
    let n = last(&["tag", "-l", "--points-at", "HEAD", &pattern]).or_else(|| last(&["tag", "-l", &pattern]).map(|n| n + 1)).unwrap_or(0);
    format!("{base}.{n}")
}
