//! One persistent appearance per player, whether driver, conductor or pedestrian.
pub(crate) mod catalog;
use catalog::{Choice, Preference};
use crate::{App, driver};
use omsi_sim::human::HumanType;
use std::path::{Path, PathBuf};
use std::sync::Arc;

pub(crate) struct CharacterState {
    pub open: bool,
    pub choices: Vec<Choice>,
    pub query: String,
    pub page: usize,
    pub chosen: String,
    pub error: String,
    applied_driver: Option<(u64, String)>,
}

impl Default for CharacterState {
    fn default() -> Self {
        Self { open:false, choices:Vec::new(), query:String::new(), page:0,
            chosen:catalog::saved_choice().unwrap_or_default(), error:String::new(),
            applied_driver:None }
    }
}

fn resolve(root: &Path, relative: &str) -> Option<PathBuf> {
    let relative=catalog::normalize(relative)?;
    omsi_cfg::find_in_roots(&relative).map(|(_,path)|path).or_else(|| {
        let path=omsi_cfg::resolve_path(root,&relative);
        omsi_cfg::vfs::is_file(&path).then_some(path)
    })
}

pub(crate) fn preferred_type(root: &Path) -> Option<Arc<HumanType>> {
    catalog::saved_choice().and_then(|s|resolve(root,&s)).and_then(|p|driver::cached_type(&p))
}

pub(crate) fn saved_choice() -> Option<String> { catalog::saved_choice() }

impl App {
    pub(crate) fn character_open(&mut self) {
        let mut roots=omsi_cfg::content_dirs("Humans");
        let own=omsi_cfg::resolve_path(&self.args.root,"Humans");
        if omsi_cfg::vfs::is_dir(&own) && !roots.contains(&own) { roots.push(own); }
        let state=&mut self.net.coop.character;
        state.choices=catalog::scan(&roots);
        state.page=0;
        state.error.clear();
        state.open=true;
        self.net.coop.panel=true;
    }

    pub(crate) fn character_choose(&mut self, chosen: &str) {
        let chosen=if chosen.is_empty() {String::new()} else {
            let Some(file)=catalog::normalize(chosen) else {return};
            file
        };
        let ty=if chosen.is_empty() {None} else {
            let Some(ty)=resolve(&self.args.root,&chosen).and_then(|p|driver::cached_type(&p)) else {
                self.net.coop.character.error=crate::coop::text(
                    "Este personagem não pôde ser carregado. A escolha anterior foi mantida.",
                    "This character could not be loaded. Your previous choice was kept.",
                    "Diese Figur konnte nicht geladen werden. Die bisherige Auswahl bleibt.").into();
                return;
            };
            Some(ty)
        };
        let result=catalog::preference_path().ok_or_else(||std::io::Error::new(
            std::io::ErrorKind::NotFound,"User profile unavailable"))
            .and_then(|path|Preference {character:chosen.clone()}.save(&path));
        if result.is_err() {
            self.net.coop.character.error=crate::coop::text(
                "Não foi possível salvar a escolha no seu perfil.",
                "Could not save the choice in your profile.",
                "Die Auswahl konnte nicht im Profil gespeichert werden.").into();
            return;
        }
        let state=&mut self.net.coop.character;
        state.chosen=chosen;
        state.error.clear();
        let actual=ty.or_else(||self.world.as_deref().and_then(|w|driver::driver_type(w,0)));
        if let (Some(ty),Some(h),Some(foot))=(actual,self.session.humans.as_mut(),self.session.on_foot.as_mut()) {
            foot.kind=h.type_index(ty) as u64;
            h.avatar_remove(crate::on_foot::AVATAR_KEY);
        }
        self.character_sync();
    }

    /// Reapply when taking another bus; only the picture changes, never the vehicle.
    pub(crate) fn character_sync(&mut self) {
        let Some(p)=self.player.as_ref() else {return};
        let borrowed = if self.net.coop.drive.owner_active() {
            self.drive_peer().and_then(|peer| {
                self.net.lan.as_ref()?.peers()
                    .find(|p| p.pose.id == peer)
                    .and_then(|p| omsi_net::human_path(&p.pose.figure))
            })
        } else { None };
        let key=(p.uid, borrowed.unwrap_or_else(|| self.net.coop.character.chosen.clone()));
        if self.net.coop.character.applied_driver.as_ref()==Some(&key) {return};
        let (Some(world),Some(renderer),Some(scene))=(self.world.as_deref(),self.renderer.as_ref(),self.scene.as_mut()) else {return};
        let chosen=(!key.1.is_empty()).then_some(key.1.as_str());
        let p=self.player.as_mut().unwrap();
        let expected=chosen.and_then(|s|resolve(&self.args.root,s)).and_then(|p|driver::cached_type(&p))
            .or_else(||driver::driver_type(world,0));
        if p.driver.as_ref().zip(expected.as_ref()).is_some_and(|(d,ty)|d.human_type().def.path==ty.def.path) {
            self.net.coop.character.applied_driver=Some(key);
            return;
        }
        let new=driver::DriverFigure::new_named(world,renderer,scene,&p.vehicle,chosen,0);
        if let Some(new) = new {
            if let Some(mut old)=p.driver.take() {old.hide(renderer,scene);}
            p.driver=Some(new);
            self.net.coop.character.applied_driver=Some(key);
        }
    }

    /// The real on-foot type is also sent when this player owns no vehicle.
    pub(crate) fn character_figure(&self) -> Option<PathBuf> {
        let walking=self.session.on_foot.as_ref().and_then(|f| {
            let humans=self.session.humans.as_ref()?;
            let n=humans.types.len();
            if n==0 {return None};
            humans.types.get((f.kind % n as u64) as usize).map(|ty|ty.def.path.clone())
        });
        walking.or_else(||self.player.as_ref().and_then(|p|p.driver.as_ref()).map(|d|d.human_type().def.path.clone()))
    }
}

