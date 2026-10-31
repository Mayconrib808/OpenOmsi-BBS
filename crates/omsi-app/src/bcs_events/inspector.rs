use super::{lifecycle::InspectorSnapshot, names::{self,Appearance}};
use std::{collections::BTreeMap,path::PathBuf};
#[derive(Default)]
pub(super) struct InspectorActor {
    person:Option<u32>, context:Option<String>, identity:Option<u32>,
    settings:Option<(PathBuf,BTreeMap<u32,String>)>,
}
impl InspectorActor {
    fn appearance(&mut self, snapshot:&InspectorSnapshot)->Option<bool> {
        let (path,choices)=self.settings.get_or_insert_with(|| {
            let root=std::env::var_os("LOCALAPPDATA").map(PathBuf::from).unwrap_or_else(std::env::temp_dir);
            let path=root.join("OpenOmsi-BBS").join("inspector-appearance.json");
            let choices=std::fs::metadata(&path).ok().filter(|m|m.len()<=65536)
                .and_then(|_|std::fs::read(&path).ok()).and_then(|v|serde_json::from_slice(&v).ok()).unwrap_or_default();
            (path,choices)
        });
        if let Some(choice)=choices.get(&snapshot.id) {
            return match choice.as_str(){"female"=>Some(true),"male"=>Some(false),_=>None};
        }
        let female=match names::infer(&snapshot.name){Appearance::Female=>Some(true),Appearance::Male=>Some(false),Appearance::Unknown=>None};
        if let Some(female)=female {
            choices.insert(snapshot.id,if female{"female"}else{"male"}.into());
            if let Some(parent)=path.parent(){let _=std::fs::create_dir_all(parent);}
            if let Ok(bytes)=serde_json::to_vec_pretty(choices){let _=std::fs::write(path,bytes);}
        }
        female
    }
    pub fn update(&mut self,context:Option<&str>,snapshot:Option<&InspectorSnapshot>,
        humans:&mut crate::humans::Humans,bus:&omsi_sim::VehicleInstance,world:&crate::scene::World) {
        let wanted=snapshot.map(|s|s.id);
        if self.context.as_deref()!=context || self.identity!=wanted {
            if let Some(id)=self.person.take(){humans.remove_bcs_inspector(id);}
            self.context=context.map(str::to_owned);self.identity=wanted;
        }
        let (Some(_),Some(snapshot))=(context,snapshot) else{return;};
        if self.person.is_none(){
            let female=self.appearance(snapshot);
            self.person=humans.spawn_bcs_inspector(&*world.data(),bus,snapshot.id,female);
            if let Some(id)=self.person{log::info!("BCS inspector {} ready as character {id}; appearance inferred from name; inspection instance {}",snapshot.id,snapshot.instance);}
        }
        if let Some(id)=self.person {
            let checking=matches!(snapshot.phase.as_str(),"requested"|"checking"|"paused");
            if !humans.step_bcs_inspector(id,checking.then_some(snapshot.checked),snapshot.phase=="paused") {self.person=None;}
        }
    }
}
