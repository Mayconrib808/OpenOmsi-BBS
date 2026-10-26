from pathlib import Path
import hashlib,json,subprocess
root=Path('game')
info=json.loads(Path('support/build-support/coop15-0343/source.json').read_text())
patch=Path('support/build-support/coop15-0343/cumulative.patch')
assert hashlib.sha256(patch.read_bytes()).hexdigest()==info['patch_sha256']
assert subprocess.check_output(['git','rev-parse','HEAD'],cwd=root,text=True).strip()==info['upstream_commit']
subprocess.run(['git','apply','--check',str(patch.resolve())],cwd=root,check=True)
subprocess.run(['git','apply',str(patch.resolve())],cwd=root,check=True)
# The Coop15 distribution deliberately migrates legacy passenger settings to
# the improved animation on first launch. The original 0.3.43 fixture predates
# that migration. Keep round-trip coverage and add a persistent opt-out case.
test=root/'crates/omsi-launcher-core/src/passenger_animation_tests.rs'
fixture=test.read_text(encoding='utf-8')
old_expectations=[
    'assert_eq!(settings_from_text(None)["passenger_animation"], "original");',
    'assert_eq!(settings_from_text(Some("version=2\\npassenger_animation=original\\n"))["passenger_animation"], "original");',
    'assert_eq!(settings_from_text(Some("version=3\\npassenger_animation=enhanced\\n"))["passenger_animation"], "original");',
    'assert_eq!(settings_from_text(Some("version=4\\npassenger_animation=unknown\\n"))["passenger_animation"], "original");',
    'assert_eq!(settings_from_text(Some(&saved))["passenger_animation"], "original");',
]
for old in old_expectations:
    assert fixture.count(old)==1,('Unexpected official fixture drift',old)
    fixture=fixture.replace(old,old.rsplit('"original"',1)[0]+'"enhanced"'+old.rsplit('"original"',1)[1],1)
anchor='        let mut settings = settings_from_text(None);'
assert fixture.count(anchor)==2, 'Unexpected official passenger fixture layout'
opt_out='''        let opt_out = "version=4\\npassenger_effects_221=1\\npassenger_animation=original\\n";
        assert_eq!(settings_from_text(Some(opt_out))["passenger_animation"], "original");
        let persisted = settings_to_text(&settings_from_text(Some(opt_out)), Some(opt_out));
        assert_eq!(settings_from_text(Some(&persisted))["passenger_animation"], "original");
'''
fixture=fixture.replace(anchor,opt_out+anchor,1)
test.write_text(fixture,encoding='utf-8')
assert not subprocess.check_output(['git','diff','--check'],cwd=root,text=True)
print('Coop15 migration-aware fixture: improved defaults and preserved explicit opt-outs.')
print('Genuine upstream 0.3.43 plus all previous Coop/passenger/fuel corrections and street-facing hail arm applied.')
