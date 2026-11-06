//! Passenger effects requested as defaults in Bridge 2.2.1. No map is installed here.
pub const MAPS:&str="Brigadeiro V2;Fikcyjny Szczecin;Fikcyjny Szczecin by ReMi Mod;SP_Carrao City;Metropole Salvador;RMBH;SP Noroeste V2 (World Coordinates);Vale do Aço 2025;Vilarejo 5.5";
pub fn map_key(s:&str)->String{
 let path=s.replace('\\',"/").to_lowercase();
 let name=path.trim_end_matches('/').trim_end_matches("/global.cfg").rsplit('/').next().unwrap_or(&path);
 name.chars().filter_map(|c|match c{
  'á'|'à'|'â'|'ã'|'ä'=>Some('a'),'é'|'è'|'ê'|'ë'=>Some('e'),'í'|'ì'|'î'|'ï'=>Some('i'),
  'ó'|'ò'|'ô'|'õ'|'ö'=>Some('o'),'ú'|'ù'|'û'|'ü'=>Some('u'),'ç'=>Some('c'),
  c if c.is_alphanumeric()=>Some(c),_=>None
 }).collect()
}
pub fn migrate_maps(existing:&str)->String{
 let mut result:Vec<String>=existing.split(';').filter(|s|!s.trim().is_empty()).map(str::to_string).collect();
 for name in MAPS.split(';'){
  if !result.iter().any(|s|map_key(s)==map_key(name)){result.push(name.to_string());}
 }
 result.join(";")
}
#[cfg(test)]
mod tests{
 use super::*;
 #[test]fn labels_paths_and_accents_match(){
  assert_eq!(map_key("F:\\OMSI\\maps\\SP_Carrao City\\global.cfg"),map_key("SP Carrao City"));
  assert_eq!(map_key("Vale_do_Aço_2025"),map_key("Vale do Aco 2025"));
  assert_eq!(MAPS.split(';').count(),9);
  assert!(!MAPS.contains("Grundorf")&&!MAPS.contains("Berlin"));
  let maps=migrate_maps("custom;SP_Carrao City");assert_eq!(maps.split(';').filter(|s|map_key(s)=="spcarraocity").count(),1);
  assert!(maps.starts_with("custom;"));
 }
}
