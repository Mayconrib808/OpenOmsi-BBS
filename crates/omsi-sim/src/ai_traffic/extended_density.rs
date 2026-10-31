//! Opt-in, map-local day-of-week traffic curves (#2206). When no
//! extended_trafficdens.txt exists, original OMSI curves remain authoritative.
//! The file also parses passenger rules for a future map-wide demand system;
//! passenger demand is deliberately NOT modified by this road-only integration.
#[derive(Clone, Debug, Default)]
pub struct ExtendedDensity {
    pub road: Vec<Group>,
    pub passenger: Vec<Group>,
}

#[derive(Clone, Debug)]
pub struct Group {
    pub name: String,
    pub factor: f32,
    pub days: Vec<DayCurve>,
}

#[derive(Clone, Debug)]
pub struct DayCurve {
    /// 1 = Monday .. 7 = Sunday, 8 = Public Holiday.
    pub day: i32,
    pub holiday: bool,
    pub points: Vec<(f32, f32)>,
}

impl ExtendedDensity {
    /// Use the active Chrono's optional extension, otherwise the map's base
    /// extension. Resolve only once when a date changes, never per frame.
    /// An absent file intentionally preserves all original OMSI density rules.
    pub fn load_for_date(map_dir: &std::path::Path, date: i32) -> Option<Self> {
        let folders = omsi_map::active_chrono_dirs(map_dir, date);
        folders.iter().rev().map(|d| d.as_path())
            .chain(std::iter::once(map_dir))
            .find_map(|dir| {
                let path = omsi_cfg::resolve_path(dir, "extended_trafficdens.txt");
                omsi_cfg::CfgFile::read(&path).ok()
                    .map(|f| Self::parse(&f.lines.join("\n")))
            })
    }

    /// Parse an opt-in extension, rejecting malformed sections without modifying
    /// the game's stock unsched_trafficdens.txt parser.
    pub fn parse(source: &str) -> Self {
        let mut result = Self::default();
        let mut lines = source
            .lines()
            .map(str::trim)
            .filter(|s| !s.is_empty() && !s.starts_with(';') && !s.starts_with("//"));
        enum Area { None, Road, Passenger }
        let mut area = Area::None;
        let mut current_group: Option<usize> = None;
        let mut current_day: Option<usize> = None;
        while let Some(line) = lines.next() {
            match line.to_ascii_lowercase().as_str() {
                "[group_road]" | "[group_passenger]" => {
                    area = if line.eq_ignore_ascii_case("[group_road]") { Area::Road } else { Area::Passenger };
                    current_group = None;
                    current_day = None;
                    let (Some(name), Some(factor)) = (lines.next(), lines.next()) else { break };
                    let Ok(factor) = factor.parse::<f32>() else { continue };
                    if name.is_empty() || !factor.is_finite() || factor < 0.0 || factor > 100.0 { continue }
                    let groups = match area {
                        Area::Road => &mut result.road,
                        Area::Passenger => &mut result.passenger,
                        Area::None => continue,
                    };
                    if groups.len() >= 256 { continue }
                    groups.push(Group { name: name.to_owned(), factor, days: Vec::new() });
                    current_group = Some(groups.len() - 1);
                }
                "[set_day_of_week]" => {
                    current_day = None;
                    let (Some(day), Some(holiday)) = (lines.next(), lines.next()) else { break };
                    let (Ok(day), Ok(holiday)) = (day.parse::<i32>(), holiday.parse::<i32>()) else { continue };
                    if !(1..=8).contains(&day) || !(0..=1).contains(&holiday) { continue }
                    let groups = match area {
                        Area::Road => &mut result.road,
                        Area::Passenger => &mut result.passenger,
                        Area::None => continue,
                    };
                    if let Some(group) = current_group.and_then(|i| groups.get_mut(i)) {
                        if group.days.len() < 32 {
                            group.days.push(DayCurve { day, holiday: holiday == 1, points: Vec::new() });
                            current_day = Some(group.days.len() - 1);
                        }
                    }
                }
                "[trafficdensity]" => {
                    let (Some(hour), Some(density)) = (lines.next(), lines.next()) else { break };
                    let (Ok(hour), Ok(density)) = (hour.parse::<f32>(), density.parse::<f32>()) else { continue };
                    if !hour.is_finite() || !density.is_finite() || !(0.0..=24.0).contains(&hour) || !(0.0..=10.0).contains(&density) { continue }
                    let groups = match area {
                        Area::Road => &mut result.road,
                        Area::Passenger => &mut result.passenger,
                        Area::None => continue,
                    };
                    if let Some(days) = current_group.and_then(|g| groups.get_mut(g)).map(|g| &mut g.days) {
                        if let Some(curve) = current_day.and_then(|d| days.get_mut(d)) {
                            if curve.points.len() < 256 { curve.points.push((hour, density)); }
                        }
                    }
                }
                _ => {}
            }
        }
        result
    }

    /// An exact day/holiday entry overrides this group only. Missing
    /// combinations fall back to the original OMSI map density curves.
    fn density_at(groups: &[Group], name: &str, weekday: i32, holiday: bool, hour: f32) -> Option<f32> {
        if !(0..=6).contains(&weekday) || !hour.is_finite() { return None }
        let group = groups.iter().rev().find(|g| g.name.trim().eq_ignore_ascii_case(name.trim()))?;
        let rules = &group.days;
        let selection = if holiday {
            rules.iter().rev().find(|d| d.day == 8 && d.holiday && !d.points.is_empty())
        } else {
            None
        };
        let choice = selection.or_else(|| rules.iter().rev().find(|d| d.day == weekday + 1 && d.holiday == holiday && !d.points.is_empty()))?;
        Some((group.factor * omsi_map::global::curve_at(&choice.points, hour).max(0.0)).clamp(0.0, 3.0))
    }

    pub fn road_at(&self, name: &str, weekday: i32, holiday: bool, hour: f32) -> Option<f32> {
        Self::density_at(&self.road, name, weekday, holiday, hour)
    }

    /// Standard is the OMSI global passenger group. The caller multiplies the
    /// selected map value by AIPassFactor and keeps stock global.cfg when absent.
    pub fn passenger_at(&self, weekday: i32, holiday: bool, hour: f32) -> Option<f32> {
        Self::density_at(&self.passenger, "Standard", weekday, holiday, hour)
    }
}

#[cfg(test)]
mod tests {
    use super::ExtendedDensity;
    #[test]
    fn different_weekdays_and_public_holidays_keep_missing_days_untouched() {
        let t = ExtendedDensity::parse("[group_road]\nNormalCars\n1.5\n[set_day_of_week]\n1\n0\n[trafficdensity]\n0\n0.4\n[set_day_of_week]\n2\n0\n[trafficdensity]\n0\n1.0\n[set_day_of_week]\n8\n1\n[trafficdensity]\n0\n0.1\n");
        assert!((t.road_at("normalcars",0,false,0.0).unwrap() - 0.6).abs() < 0.0001);
        assert!((t.road_at("NormalCars",1,false,0.0).unwrap() - 1.5).abs() < 0.0001);
        assert!((t.road_at("NormalCars",0,true,0.0).unwrap() - 0.15).abs() < 0.0001);
        assert_eq!(t.road_at("NormalCars",2,false,0.0),None);
        assert_eq!(t.road_at("Unknown",0,false,0.0),None);
    }
    #[test]
    fn invalid_rules_cannot_inject_nan_or_negative_traffic() {
        let t = ExtendedDensity::parse("[group_road]\nCars\n1\n[set_day_of_week]\n9\n0\n[trafficdensity]\n0\n9\n[set_day_of_week]\n1\n0\n[trafficdensity]\n0\nNaN\n[trafficdensity]\n0\n0.75\n");
        assert_eq!(t.road_at("Cars",0,false,0.0),Some(0.75));
    }
    #[test]
    fn passenger_density_changes_by_day_without_changing_road_rules() {
        let t = ExtendedDensity::parse("[group_passenger]\nStandard\n1.0\n[set_day_of_week]\n1\n0\n[trafficdensity]\n0\n0.8\n[set_day_of_week]\n7\n0\n[trafficdensity]\n0\n0.2\n[set_day_of_week]\n8\n1\n[trafficdensity]\n0\n0.1\n");
        assert_eq!(t.passenger_at(0, false, 0.0), Some(0.8));
        assert_eq!(t.passenger_at(6, false, 0.0), Some(0.2));
        assert_eq!(t.passenger_at(0, true, 0.0), Some(0.1));
        assert_eq!(t.passenger_at(4, false, 0.0), None);
        assert_eq!(t.road_at("Standard", 0, false, 0.0), None);
    }
    #[test]
    fn passenger_rules_are_parsed_but_do_not_change_road_traffic() {
        let t = ExtendedDensity::parse("[group_passenger]\nStandard\n1\n[set_day_of_week]\n1\n0\n[trafficdensity]\n0\n0.5\n");
        assert_eq!(t.passenger.len(),1);
        assert_eq!(t.road_at("Standard",0,false,0.0),None);
    }
}
