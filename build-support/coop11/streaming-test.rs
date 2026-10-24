
#[cfg(test)]
mod dedicated_streaming_tests {
    use super::*;

    #[test]
    fn dedicated_612_tiles_keeps_global_data_and_streams_two_moving_regions() {
        use std::time::{Duration,Instant};
        let root=std::env::temp_dir().join(format!("bbs-stream-612-{}",std::process::id()));
        let _=std::fs::remove_dir_all(&root);std::fs::create_dir_all(root.join("maps/Test")).unwrap();
        struct Cleanup(PathBuf);impl Drop for Cleanup{fn drop(&mut self){let _=std::fs::remove_dir_all(&self.0);}}
        let _cleanup=Cleanup(root.clone());
        let mut global=String::from("[name]\nStreaming 612\n");
        for x in 0..34 { for y in 0..18 {
            let name=format!("tile_{x}_{y}.map");
            global.push_str(&format!("[map]\n{x}\n{y}\n{name}\n"));
            std::fs::write(root.join("maps/Test").join(name),"[terrain]\n").unwrap();
        }}
        std::fs::write(root.join("maps/Test/global.cfg"),global).unwrap();
        let mut args=Args::parse_from(["openomsi","--root",root.to_str().unwrap(),"--map","maps/Test/global.cfg","--all","--radius","1"]);
        args.server=Some(root.join("server.cfg"));args.traffic=0;args.passengers=false;args.schedule=false;
        args.cam=Some("150,150,10,0,0".into());args.size="64x36".into();
        crate::server::SERVER_MODE.store(true,std::sync::atomic::Ordering::Relaxed);
        let out=root.join("unused.png");
        let mut run=Offscreen::setup(&args,&out,None,lan::LanGame::default()).unwrap();
        assert_eq!(run.world.global.tiles.len(),612);
        assert!(run.streamer.is_some(),"saved radius or --all disabled dedicated streaming");
        assert!(run.world.loaded_tiles().len()<100,"all map tiles were prepared at startup");
        let ts=omsi_map::tile_size();
        let near=DVec3::new(150.,150.,0.);let far=DVec3::new(33.*ts+150.,17.*ts+150.,0.);
        let started=Instant::now();
        loop {
            run.streamer.as_mut().unwrap().update(&run.renderer,&mut run.scene,&[near,far],Duration::from_millis(10),None);
            let loaded=run.world.loaded_tiles();
            if loaded.contains(&(0,0))&&loaded.contains(&(33,17)){break;}
            assert!(started.elapsed()<Duration::from_secs(30),"second player's region never loaded: {loaded:?}");
            std::thread::sleep(Duration::from_millis(3));
        }
        assert!(run.world.loaded_tiles().len()<612);
        let started=Instant::now();
        while run.world.loaded_tiles().contains(&(0,0)) {
            run.streamer.as_mut().unwrap().update(&run.renderer,&mut run.scene,&[far],Duration::from_millis(10),None);
            assert!(started.elapsed()<Duration::from_secs(30),"abandoned region never unloaded");
            std::thread::sleep(Duration::from_millis(3));
        }
        assert!(run.world.loaded_tiles().contains(&(33,17)));
        assert_eq!(run.world.global.tiles.len(),612,"global map/timetable metadata was discarded");
    }
}
