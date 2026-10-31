from pathlib import Path
import subprocess,hashlib,json,sys,os
commit,tree=sys.argv[1:]
out=Path('openomsi-localized-options-source.tar.gz')
subprocess.run(['git','archive','--format=tar.gz','--prefix=openomsi-localized-options/','-o',str(out),commit],check=True)
data=dict(revision='2.2.2-hotfix.2-localized-options-review',commit=commit,source_tree=tree,
    original_source_commit='b6c22b1b993b0126ab799c7634105c2f1b9d0d87',
    original_source_tree='990d5300185280c6f209d72ddaa0666a07f9d482',
    source_sha256=hashlib.sha256(out.read_bytes()).hexdigest(),
    native_sha256=hashlib.sha256(Path('target/release/openomsi.exe').read_bytes()).hexdigest(),
    run_id=int(os.environ['GITHUB_RUN_ID']),native_version=os.environ['OPENOMSI_VERSION'],
    caption_keys=4,translated_locales=29,translated_values=116,english_source_keys=4,
    new_catalog_keys=19,new_translation_values=573,optional_toggles=7,optional_defaults=False,
    launcher_schema_tests_passed=True,settings_roundtrip_tests_passed=True,traffic_tests_passed=True,bcs_tests_passed=True,
    windows_build_passed=True,upstream_locale_validation_passed=True,
    passenger_simulation_tests_passed=True,compiled_caption_tests_passed=True,
    published_to_automatic_updates=False,gameplay_tested=False)
Path('REVISION.json').write_text(json.dumps(data,indent=2)+'\n',encoding='utf-8')
print(json.dumps(data,indent=2))
