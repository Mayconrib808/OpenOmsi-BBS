//! #906 / #1670: select the best installed passenger outfit for host weather.
//! Model authors tag CTI item names winter, cold, rain, summer, etc.
//! No hats, textures or umbrellas are invented. Untagged CTI remains random.

#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Climate {
    pub temp_c: f32,
    pub rain: bool,
    pub snow: bool,
}
impl Default for Climate {
    fn default() -> Self {
        Self { temp_c: 22.0, rain: false, snow: false }
    }
}
impl Climate {
    pub fn new(temp_c: f32, rain: bool, snow: bool) -> Self {
        Self { temp_c: if temp_c.is_finite() { temp_c.clamp(-55.0, 55.0) } else { 22.0 }, rain, snow }
    }
}

fn word(name: &str, tags: &[&str]) -> bool {
    let lower = name.to_lowercase();
    lower.split(|c:char| !c.is_alphanumeric()).any(|s| tags.contains(&s))
}

/// Rank only explicit seasonal variants; a name with no season tag is neutral.
pub fn clothing_score(name: &str, climate: Climate) -> Option<i32> {
    let summer = word(name, &["summer", "hot", "light", "shorts", "verao", "verão", "sommer"]);
    let cold = word(name, &["winter", "cold", "warm", "coat", "jacket", "inverno", "winterjacke", "casaco"]);
    let frost = word(name, &["snow", "frost", "freezing", "hat", "boots", "neve", "muetze", "mütze"]);
    let rain = word(name, &["rain", "wet", "raincoat", "umbrella", "chuva", "regen"]);
    if !summer && !cold && !frost && !rain { return None; }
    let mut score = 0;
    if climate.temp_c <= 5.0 {
        score += if frost { 10 } else { 0 };
        score += if cold { 8 } else { -10 };
        if summer { score -= 25; }
    } else if climate.temp_c <= 15.0 {
        score += if cold { 13 } else { 0 };
        if summer { score -= 15; }
    } else if climate.temp_c >= 23.0 {
        score += if summer { 12 } else { 0 };
        if cold || frost { score -= 15; }
    } else if cold || summer || frost {
        score -= 1;
    }
    if climate.rain || climate.snow {
        if rain { score += 11; }
        if frost && climate.snow { score += 5; }
    } else if rain { score -= 14; }
    Some(score)
}

/// Return a one-based variant index when a suitable tagged outfit exists.
pub fn choose_variant(names: &[String], climate: Climate, random_start: usize) -> Option<usize> {
    if names.is_empty() { return None; }
    let mut best: Option<(usize,i32)> = None;
    for off in 0..names.len() {
        let index = (random_start + off) % names.len();
        if let Some(score) = clothing_score(&names[index],climate) {
            if score > 0 && best.is_none_or(|(_,v)| score > v) {
                best = Some((index+1,score));
            }
        }
    }
    best.map(|(idx,_)|idx)
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn cold_and_rain_prefer_relevant_available_clothes() {
        let names = vec!["Summer shorts".into(),"Winter coat".into(),"Winter raincoat".into()];
        assert_eq!(choose_variant(&names,Climate::new(-3.0,true,true),0),Some(3));
        assert_eq!(choose_variant(&names,Climate::new(30.0,false,false),1),Some(1));
        assert_eq!(choose_variant(&names,Climate::new(10.0,false,false),0),Some(2));
    }
    #[test]
    fn untagged_models_keep_their_original_outfits() {
        assert_eq!(choose_variant(&["Outfit1".into(),"Shirt A".into()],Climate::new(0.0,false,false),0),None);
        assert_eq!(clothing_score("winter_coat",Climate::new(30.0,false,false)),Some(-15));
        assert_eq!(Climate::new(f32::NAN,false,false).temp_c,22.0);
    }
}
