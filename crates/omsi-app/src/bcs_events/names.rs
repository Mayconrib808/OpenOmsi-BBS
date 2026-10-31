//! International name usage is an appearance hint, never a confirmed BCS sex field.
use std::{collections::HashMap, sync::OnceLock};
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub(super) enum Appearance { Male, Female, Unknown }
fn plain(name: &str) -> String {
    name.to_lowercase().chars().map(|c| match c {
        'á'|'à'|'â'|'ã'|'ä'|'å'=>'a', 'é'|'è'|'ê'|'ë'=>'e',
        'í'|'ì'|'î'|'ï'=>'i', 'ó'|'ò'|'ô'|'õ'|'ö'=>'o',
        'ú'|'ù'|'û'|'ü'=>'u', 'ç'=>'c', 'ñ'=>'n', _=>c,
    }).filter(|c| !('\u{0300}'..='\u{036f}').contains(c)).collect()
}
pub(super) fn infer(name: &str) -> Appearance {
    static NAMES: OnceLock<HashMap<&'static str, Appearance>> = OnceLock::new();
    let names = NAMES.get_or_init(|| include_str!("names-data/names.tsv").lines().filter_map(|line| {
        let (name, kind) = line.split_once('\t')?;
        Some((name, if kind == "F" {Appearance::Female} else {Appearance::Male}))
    }).collect());
    let key = plain(name.trim());
    names.get(key.as_str()).copied().or_else(|| key.split_whitespace().next().and_then(|first| names.get(first).copied())).unwrap_or(Appearance::Unknown)
}
#[cfg(test)] mod tests {
    use super::*;
    #[test] fn international_and_renamed_inspectors() {
        assert_eq!(infer("Wagner Rocha"),Appearance::Male);
        assert_eq!(infer("Mônica"),Appearance::Female);
        assert_eq!(infer("Monika"),Appearance::Female);
        assert_eq!(infer("Michael"),Appearance::Male);
        assert_eq!(infer("xyz-personagem-123"),Appearance::Unknown);
    }
}
