"""Coop12 driver handover: absolute VFS paths must not be re-resolved under OMSI_ROOT.

The remote VehicleType already contains the exact trusted mounted bus path.
PreparedPlayer::load(args) takes a relative menu selection, and resolve_path
would duplicate OMSI_ROOT when passed an absolute drive letter.
"""
from pathlib import Path

def replace_one(path: Path, before: str, after: str) -> None:
    data = path.read_text(encoding="utf-8")
    count = data.count(before)
    if count != 1:
        raise RuntimeError(f"Unexpected source in {path}: {count} matches")
    path.write_text(data.replace(before, after), encoding="utf-8", newline="\n")

def apply_hotfix(game: Path) -> None:
    spawn = game / "crates/omsi-app/src/spawn.rs"
    before = """    pub(crate) fn load(args: &Args) -> Result<Self> {
        let bus = args.bus.as_deref().context("no vehicle selected")?;
        let path = player_bus_path(&args.root, bus)?;
        let ty = Arc::new(omsi_sim::VehicleType::load(&args.root, &path)?);
        let parts = load_coupled_types(&args.root, ty.clone());
        Ok(Self { ty, parts, staged: None })
    }
"""
    after = """    pub(crate) fn load(args: &Args) -> Result<Self> {
        let bus = args.bus.as_deref().context("no vehicle selected")?;
        let path = player_bus_path(&args.root, bus)?;
        Self::load_existing_path(args, &path)
    }

    /// Handover from a remote VehicleType that was already resolved and
    /// mounted (possibly by a sealed LAN download). Calling resolve_path
    /// again would incorrectly append the absolute path to OMSI_ROOT.
    pub(crate) fn load_existing_path(args: &Args, path: &Path) -> Result<Self> {
        anyhow::ensure!(path.is_absolute(), "handover vehicle path is not absolute: {}", path.display());
        anyhow::ensure!(omsi_cfg::vfs::is_file(path), "handover bus is unavailable at {}", path.display());
        let ty = Arc::new(omsi_sim::VehicleType::load(&args.root, path)?);
        let parts = load_coupled_types(&args.root, ty.clone());
        Ok(Self { ty, parts, staged: None })
    }
"""
    replace_one(spawn, before, after)

    before_tests = """    /// The NF6D tram is eleven parts long and its rear cab car"""
    test = """    /// A visitor loads the exact bus from another player's LAN content
    /// directory without accidentally joining its absolute path to OMSI_ROOT.
    #[test]
    fn visitor_handover_keeps_exact_absolute_vehicle_path() {
        let sandbox = std::env::temp_dir().join(format!("coop12-handover-path-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&sandbox);
        struct Cleanup(PathBuf);
        impl Drop for Cleanup {
            fn drop(&mut self) { let _ = std::fs::remove_dir_all(&self.0); }
        }
        let _cleanup = Cleanup(sandbox.clone());
        let root = sandbox.join("SteamLibrary/steamapps/common/OMSI 2");
        let remote = sandbox.join("lan-vehicles/mounted/Vehicles/Caio Vip I");
        std::fs::create_dir_all(&root).unwrap();
        std::fs::create_dir_all(&remote).unwrap();
        let bus = remote.join("Vip 1 Manual.bus");
        std::fs::write(&bus, "[friendlyname]\\nCoop\\nVip 1\\nManual\\n[model]\\nempty.cfg\\n").unwrap();
        std::fs::write(remote.join("empty.cfg"), "").unwrap();
        let args = Args::parse_from(["omsi", "--root", root.to_str().unwrap()]);
        let prepared = PreparedPlayer::load_existing_path(&args, &bus).unwrap();
        assert_eq!(prepared.ty.def.path, bus, "LAN bus path was re-rooted into OMSI");
        assert_eq!(prepared.parts.len(), 0);
    }

    /// The NF6D tram is eleven parts long and its rear cab car"""
    replace_one(spawn, before_tests, test)

    drive = game / "crates/omsi-app/src/coop_drive.rs"
    before_drive = """                let local = remote.vehicle().ty.def.path.to_string_lossy().to_string();
                let (Some(world), Some(renderer)) = (self.world.clone(), self.renderer.as_ref())"""
    after_drive = """                // Already mounted and validated for this peer; keep its
                // absolute path instead of running the menu resolver again.
                let local = remote.vehicle().ty.def.path.clone();
                let (Some(world), Some(renderer)) = (self.world.clone(), self.renderer.as_ref())"""
    replace_one(drive, before_drive, after_drive)
    before_arg = """                    bus: Some(local),
                    paint,
                    hof,"""
    after_arg = """                    bus: Some(local.to_string_lossy().into_owned()),
                    paint,
                    hof,"""
    replace_one(drive, before_arg, after_arg)
    before_worker = """                            crate::spawn::PreparedPlayer::load(&worker_args).map(|mut p| {
                                p.prefetch(prefetch, worker_args.paint.as_deref());
                                p
                            });"""
    after_worker = """                            crate::spawn::PreparedPlayer::load_existing_path(&worker_args, &local)
                                .map(|mut p| {
                                    p.prefetch(prefetch, worker_args.paint.as_deref());
                                    p
                                });"""
    replace_one(drive, before_worker, after_worker)
    print("Coop12: exact absolute VFS path for visitor driver transfer, with regression test.")
