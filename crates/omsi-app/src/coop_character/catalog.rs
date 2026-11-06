//! Installed character choices and the persistent local choice, shared by all roles.
use serde::{Deserialize, Serialize};
use std::collections::HashSet;
use std::path::{Path, PathBuf};

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Choice {
    pub file: String,
    pub label: String,
}

#[derive(Default, Deserialize, Serialize)]
pub struct Preference {
    pub character: String,
}

/// The same relative .hum name that the existing protocol can carry.
pub fn normalize(text: &str) -> Option<String> {
    let name = text.trim().replace('\\', "/");
    if name.is_empty() || name.len() > 260 || name.starts_with('/')
        || name.chars().any(|c| c == ':' || c == '|' || c.is_control())
        || name.split('/').any(|s| s.trim_matches(|c| c == '.' || c == ' ').is_empty())
        || !name.split('/').next()?.eq_ignore_ascii_case("humans")
        || !name.to_ascii_lowercase().ends_with(".hum") {
        return None;
    }
    Some(name)
}

impl Preference {
    pub fn load(path: &Path) -> Self {
        std::fs::read(path).ok()
            .and_then(|bytes| serde_json::from_slice::<Self>(&bytes).ok())
            .map(|p| Self { character: normalize(&p.character).unwrap_or_default() })
            .unwrap_or_default()
    }

    pub fn save(&self, path: &Path) -> std::io::Result<()> {
        if !self.character.is_empty() && normalize(&self.character).is_none() {
            return Err(std::io::Error::new(std::io::ErrorKind::InvalidInput, "Invalid character"));
        }
        if let Some(parent) = path.parent() { std::fs::create_dir_all(parent)?; }
        std::fs::write(path, serde_json::to_vec_pretty(self)?)
    }
}

pub fn preference_path() -> Option<PathBuf> {
    let user = std::env::var_os("USERPROFILE").or_else(|| std::env::var_os("HOME"))?;
    Some(PathBuf::from(user).join(".openomsi").join("coop-character.json"))
}

pub fn saved_choice() -> Option<String> {
    let p = preference_path().map(|p| Preference::load(&p)).unwrap_or_default();
    (!p.character.is_empty()).then_some(p.character)
}

/// Roots are priority ordered, so a mod shadowing a stock file appears once.
pub fn scan(roots: &[PathBuf]) -> Vec<Choice> {
    let mut found = Vec::new();
    let mut seen = HashSet::new();
    for root in roots {
        let mut stack = vec![(root.clone(), String::from("Humans"), 0usize)];
        while let Some((dir, relative, depth)) = stack.pop() {
            if depth > 16 || found.len() >= 20_000 { continue; }
            let mut entries = omsi_cfg::vfs::list_dir(&dir).unwrap_or_default();
            entries.sort_by_key(|(n, _)| n.to_string_lossy().to_lowercase());
            for (name, is_dir) in entries.into_iter().rev() {
                let name = name.to_string_lossy();
                if name.contains('/') || name.contains('\\') { continue; }
                let rel = format!("{relative}/{name}");
                if is_dir {
                    stack.push((dir.join(name.as_ref()), rel, depth + 1));
                } else if let Some(file) = normalize(&rel) {
                    if seen.insert(file.to_lowercase()) {
                        let label = file.strip_prefix("Humans/").unwrap_or(&file)
                            .strip_suffix(".hum").unwrap_or(&file).replace('/', " / ");
                        found.push(Choice { file, label });
                    }
                }
            }
        }
    }
    found.sort_by_key(|c| c.label.to_lowercase());
    found
}

pub fn matches(choice: &Choice, query: &str) -> bool {
    choice.label.to_lowercase().contains(&query.trim().to_lowercase())
}

#[cfg(test)]
mod tests {
    use super::*;
    fn temporary() -> PathBuf {
        let dir = std::env::temp_dir().join(format!("coop-character-{}-{}", std::process::id(),
            std::time::SystemTime::now().duration_since(std::time::UNIX_EPOCH).unwrap().as_nanos()));
        std::fs::create_dir_all(&dir).unwrap();
        dir
    }
    #[test]
    fn only_installed_relative_human_names_are_accepted() {
        assert_eq!(normalize("Humans\\Pessoas BR\\Mulher.hum").unwrap(), "Humans/Pessoas BR/Mulher.hum");
        for bad in ["../x.hum", "Humans/../x.hum", "Humans/.../x.hum", "C:/Humans/x.hum",
            "/Humans/x.hum", "Humans//x.hum", "Vehicles/x.hum", "Humans/x.bus",
            "Humans/x.hum|INFO", "Humans/x\n.hum", "\\\\server\\Humans\\x.hum"] {
            assert!(normalize(bad).is_none(), "{bad:?}");
        }
    }
    #[test]
    fn catalog_has_passengers_and_drivers_in_nested_folders_without_duplicates() {
        let dir = temporary();
        let high = dir.join("mod");
        let low = dir.join("stock");
        for (root, name) in [(&high,"Pack/Mulher.hum"),(&low,"pack/mulher.hum"),
            (&low,"Pack/Driver.hum"),(&low,"Pack/Nested/Cobrador.HUM"),(&low,"Pack/ignore.txt")] {
            let file=root.join(name);
            std::fs::create_dir_all(file.parent().unwrap()).unwrap();
            std::fs::write(file, b"human").unwrap();
        }
        let choices=scan(&[high,low]);
        assert_eq!(choices.len(),3);
        assert!(choices.iter().any(|c|c.file=="Humans/Pack/Mulher.hum"));
        assert!(choices.iter().any(|c|matches(c,"cobrador")));
        std::fs::remove_dir_all(dir).unwrap();
    }
    #[test]
    fn choosing_and_resetting_a_character_survive_a_restart() {
        let dir=temporary();
        let path=dir.join("profile/coop-character.json");
        Preference {character:"Humans/Other/woman01.hum".into()}.save(&path).unwrap();
        assert_eq!(Preference::load(&path).character,"Humans/Other/woman01.hum");
        Preference::default().save(&path).unwrap();
        assert!(Preference::load(&path).character.is_empty());
        std::fs::write(&path,br#"{"character":"../private.hum"}"#).unwrap();
        assert!(Preference::load(&path).character.is_empty());
        std::fs::remove_dir_all(dir).unwrap();
    }
}

