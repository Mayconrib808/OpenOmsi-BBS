//! Read-only game data queries. Names and zero-based argument order match the OMSI
//! M.V. getters for familiarity; implementations read game data without running scripts.

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum QueryType {
    Number,
    String,
    Rgba,
}

#[derive(Debug, Clone, PartialEq)]
pub enum QueryValue {
    Number(f32),
    String(String),
}

macro_rules! queries {
    ($($variant:ident($name:literal, [$($arg:literal : $ty:ident),*], $result:ident)),* $(,)?) => {
        #[derive(Debug, Clone, Copy, PartialEq, Eq)]
        pub enum DataQuery { $($variant,)* }
        impl DataQuery {
            pub const ALL: &'static [Self] = &[$(Self::$variant,)*];
            pub fn name(self) -> &'static str { match self { $(Self::$variant => $name,)* } }
            pub fn args(self) -> &'static [(&'static str, QueryType)] { match self { $(Self::$variant => &[$(($arg, QueryType::$ty)),*],)* } }
            pub fn result(self) -> QueryType { match self { $(Self::$variant => QueryType::$result,)* } }
        }
    }
}
queries! {
    TextLength("TextLength", ["text":String, "font":Number], Number),
    StReadPixel("STReadPixel", ["texture":Number, "x":Number, "y":Number], Rgba),
    StGetR("STGetR", ["texture":Number], Number),
    StGetG("STGetG", ["texture":Number], Number),
    StGetB("STGetB", ["texture":Number], Number),
    StGetA("STGetA", ["texture":Number], Number),
    GetRouteIndex("GetRouteIndex", ["code":Number], Number),
    GetRouteTerminusIndex("GetRouteTerminusIndex", ["route":Number], Number),
    GetTerminusCode("GetTerminusCode", ["terminus":Number], Number),
    GetTerminusIndex("GetTerminusIndex", ["code":Number], Number),
    GetTerminusString("GetTerminusString", ["terminus":Number, "field":Number], String),
    GetBusstopCount("GetBusstopCount", ["route":Number], Number),
    GetRouteBusstopIdent("GetRouteBusstopIdent", ["route":Number, "stop":Number], String),
    GetBusstopIndex("GetBusstopIndex", ["ident":String], Number),
    GetBusstopString("GetBusstopString", ["stop":Number, "field":Number], String),
    GetDepotStringGlobal("GetDepotStringGlobal", ["field":Number], String),
    GetTTLineString("GetTTLineString", [], String),
    GetTTDelay("GetTTDelay", [], Number),
    GetTTBusstopCount("GetTTBusstopCount", [], Number),
    GetTTBusstopIndex("GetTTBusstopIndex", [], Number),
    GetTTerminusIndex("GetTTerminusIndex", [], Number),
    GetTTTerminusIndex("GetTTTerminusIndex", [], Number),
    GetHeightAbovePoint("GetHeightAbovePoint", ["x":Number, "y":Number, "z":Number], Number),
    GetHumanCountOnPathLink("GetHumanCountOnPathLink", ["link":Number], Number),
    GetTTBusstopName("GetTTBusstopName", ["stop":Number], String),
    GetTTBusstopDep("GetTTBusstopDep", ["stop":Number], Number),
    GetTTBusstopArr("GetTTBusstopArr", ["stop":Number], Number),
    NrSpecRandom("NrSpecRandom", ["seed":Number], Number),
    GetTicketName("GetTicketName", ["ticket":Number], String),
    GetTTicketName("GetTTicketName", ["ticket":Number], String),
    GetTicketValue("GetTicketValue", ["ticket":Number], Number),
    GetHumanCountOnSeat("GetHumanCountOnSeat", ["seat":Number], Number),
    GetArrBusLine("GetArrBusLine", ["arrival":Number], String),
    GetArrBusTerminus("GetArrBusTerminus", ["arrival":Number], String),
    GetArrBusTimeDiff("GetArrBusTimeDiff", ["arrival":Number], Number),
    GetRadioName("GetRadioName", [], String),
    GetRadioSong("GetRadioSong", [], String),
}
impl DataQuery {
    pub fn from_name(name: &str) -> Option<Self> {
        Self::ALL
            .iter()
            .copied()
            .find(|q| q.name().eq_ignore_ascii_case(name))
    }
    pub fn validate(self, values: &[QueryValue]) -> Result<(), String> {
        if values.len() != self.args().len() {
            return Err(format!(
                "{} expects {} arguments",
                self.name(),
                self.args().len()
            ));
        }
        for (value, (name, ty)) in values.iter().zip(self.args()) {
            if !matches!((value, ty), (QueryValue::Number(n), QueryType::Number) if n.is_finite())
                && !matches!((value, ty), (QueryValue::String(_), QueryType::String))
            {
                return Err(format!(
                    "{} argument {name} must be a finite number or string of the declared type",
                    self.name()
                ));
            }
        }
        Ok(())
    }
}
