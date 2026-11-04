//! Exercise the original equipment macro against a curb, without physics/rendering.
use omsi_sim::{
    VehicleInstance,
    people::wheelchair::{self, Profile},
};
use serde_json::{Value, json};
use std::sync::Arc;

fn animate(v: &mut VehicleInstance, dt: f32) {
    for (i, a) in v.animators.iter_mut().enumerate() {
        v.mesh_transforms[i] = a.update(dt, &v.state.vars);
    }
    omsi_sim::anim::apply_parents(&v.animators, &mut v.mesh_transforms);
    v.mesh_props = omsi_sim::vehicle::compute_mesh_props(&v.ty, &|name| v.var(name));
}
fn snapshot(v: &VehicleInstance, p: &Profile) -> Value {
    let f = wheelchair::discovery::feedback(v, p);
    let variables=p.native.as_ref().and_then(|n|n.index).and_then(|i|v.animators.get(i)).map(|a|a.anims.iter().map(|(a,_,state)|
        json!({"name":a.variable,"script":v.var(&a.variable),"mesh":state.value})).collect::<Vec<_>>());
    json!({"door_open":f.door_open,"ground_ready":f.ground_ready,"floor_ready":f.floor_ready,"stowed":f.stowed,"surface":f.surface,"variables":variables})
}
pub fn run(v: &mut VehicleInstance, p: &Profile) -> Value {
    let Some(binding) = &p.native else {
        return json!({"skipped":"additional equipment"});
    };
    let man = binding.trigger.as_deref() == Some("ramplift_wunsch");
    let solaris = binding
        .trigger
        .as_deref()
        .is_some_and(|t| t.starts_with("pandus_"));
    if !man && !solaris {
        return json!({"skipped":"manual original equipment requires a gameplay test"});
    }
    let mut program = (*v.ty.program).clone();
    let name = if man {
        "ramplift_frame"
    } else {
        "cockpit_frame"
    };
    let Some(&block) = program.macros.get(name) else {
        return json!({"error":"original equipment macro missing"});
    };
    program.frame = vec![block];
    let body_z = if solaris { -0.10 } else { 0.0 };
    v.position.z = body_z;
    v.ground = Some(Arc::new(|x, _| Some(if x > 1.25 { 0.18 } else { 0.0 })));
    v.host.ground_probe = Some(Arc::new(move |x, _, z| {
        z + body_z as f32 - if x > 1.25 { 0.18 } else { 0.0 }
    }));
    for (name, value) in [
        ("elec_busbar_main", 24.0),
        ("bremse_halte", 1.0),
        ("door_handsteuerung", 1.0),
        ("cp_hub_mstr_sw", 1.0),
        ("door_2", 1.0),
        ("door_3", 1.0),
    ] {
        v.set_var(name, value);
    }
    for name in std::iter::once(&p.door_var).chain(&p.door_vars) {
        v.set_var(name, 1.0);
    }
    animate(v, 0.0);
    let initial = snapshot(v, p);
    let fired = program
        .trigger(binding.trigger.as_deref().unwrap())
        .is_some();
    v.vm.run_trigger(
        &program,
        binding.trigger.as_deref().unwrap(),
        &mut v.state,
        &mut v.host,
    );
    let mut first_ready = None;
    for step in 0..1200 {
        v.host.clock.advance(0.02);
        v.vm.run_frame(&program, &mut v.state, &mut v.host);
        animate(v, 0.02);
        let f = wheelchair::discovery::feedback(v, p);
        if f.ground_ready && f.floor_ready && first_ready.is_none() {
            first_ready = Some((step + 1) as f32 * 0.02);
        }
    }
    let deployed = snapshot(v, p);
    if man {
        v.set_var("door_handsteuerung", 0.0);
        v.set_var("door_2", 0.0);
        v.set_var("door_3", 0.0);
    } else {
        v.vm.run_trigger(
            &program,
            binding.trigger.as_deref().unwrap(),
            &mut v.state,
            &mut v.host,
        );
    }
    let immediately_after_stow = snapshot(v, p);
    for _ in 0..1200 {
        v.host.clock.advance(0.02);
        v.vm.run_frame(&program, &mut v.state, &mut v.host);
        animate(v, 0.02);
    }
    let stowed = snapshot(v, p);
    json!({"macro":name,"trigger_found":fired,"curb_height":0.18,"body_z":body_z,"initial":initial,"first_ready_seconds":first_ready,
        "deployed":deployed,"immediately_after_stow":immediately_after_stow,"stowed":stowed})
}
