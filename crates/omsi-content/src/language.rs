//! `.olf` language files: first line is the language code, then `KEY<TAB>text`.

use omsi_cfg::CfgFile;
use std::collections::HashMap;
use std::path::Path;

#[derive(Debug, Clone, PartialEq, Default)]
pub struct Language {
    pub code: String,
    pub strings: HashMap<String, String>,
}

impl Language {
    pub fn load(path: &Path) -> Result<Language, omsi_cfg::CfgError> {
        let f = CfgFile::read(path)?;
        let mut l = Language::default();
        let mut it = f.lines.iter();
        if let Some(first) = it.next() {
            l.code = first.split('\t').next().unwrap_or("").trim().to_string();
        }
        for line in it {
            if let Some((k, v)) = line.split_once('\t') {
                l.strings.insert(k.trim().to_string(), v.to_string());
            }
        }
        Ok(l)
    }

    /// Merge another file into this one (later files override).
    pub fn merge(&mut self, other: Language) {
        self.strings.extend(other.strings);
    }

    pub fn get<'a>(&'a self, key: &'a str) -> &'a str {
        self.strings.get(key).map(|s| s.as_str()).unwrap_or(key)
    }
}
