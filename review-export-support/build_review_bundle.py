from pathlib import Path
import subprocess, hashlib, json, re, shutil, os

ROOT=Path(os.environ.get('OPENOMSI_REVIEW_ROOT','/workspace/scratch/b003ee0e9009/hotfix2-source'))
OUT=ROOT/'REVIEW-2.2.2-HOTFIX2'
BASE='17be22987d7c01c813a1473e7696fc71fb3b1d89'
EXACT=os.environ.get('OPENOMSI_REVIEW_EXACT','8612e3284b1b7367c8be8cfbf92c49f777a23d89')
TREE=os.environ['OPENOMSI_REVIEW_TREE']
BRANCH='source/openomsi-0.3.46-coop15-2.2.2-hotfix2'

def git(*args): return subprocess.check_output(['git',*args],cwd=ROOT)
def sha(b): return hashlib.sha256(b).hexdigest()
def write(path,text):
    p=OUT/path;p.parent.mkdir(parents=True,exist_ok=True)
    p.write_text(text,encoding='utf-8',newline='\n')

groups={
 '00-shared-integration': ('Shared native integration / Integração compartilhada',
   'Previous cooperative/weather contributions, settings, menu integration, packet hooks and other native dependencies. This is native OpenOMSI code; external Bridge/Setup/hosts are not present.'),
 '01-bcs-events': ('BCS events / Eventos do BCS',
   'Capture reader, event controller, fainting and drunk/aggressive actor poses, oil/fire/bin visuals, ambulance/police/fire dispatch, host-authoritative response messages and original fallback asset generator.'),
 '02-inspectors': ('Inspectors / Fiscais',
   'An inspector already inside the bus, a reserved passenger seat, appearance selection and international name data, physical aisle patrol, pause, return to the reserved seat and bounded retry on disconnected cabin paths.'),
 '03-passengers': ('Passengers and speech / Passageiros e falas',
   'Closed-door requests, missed-stop reactions, optional cosmetic 22% exit thanks, real exit detection, stop tolerance and path recovery. Cumulative hailing, umbrella render support and onward connections are included where they share these complete source files; they are not all new Hotfix 2 features.'),
 '04-traffic-improvements': ('Traffic improvements / Melhorias no trânsito',
   'Street population weighted by physical lane length, separate population around LAN players, guest traffic activation, population diagnostics and retained emergency-priority and double-horn/light greeting support. Emergency dispatch itself is in part 01; network/menu glue is in part 00.'),
}

def owner(p):
    if p in ['crates/omsi-app/src/bcs_events/inspector.rs','crates/omsi-app/src/bcs_events/names.rs','crates/omsi-sim/src/people/inspector.rs'] or p.startswith('crates/omsi-app/src/bcs_events/names-data/'):
        return '02-inspectors'
    if p.startswith('crates/omsi-app/src/bcs_events/') or p in ['crates/omsi-app/src/traffic/responder.rs','crates/omsi-sim/src/ai_traffic/response.rs','tools/generate_bcs_assets.py','crates/omsi-sim/examples/bcs_models.rs','ci/bcs-capture-reader-tests.rs','scripts/stage-bcs-police.py','docs/beta/bcs-incidents.md','docs/beta/emergency-models.json']:
        return '01-bcs-events'
    if p.startswith(('crates/omsi-app/src/app_events/frame/people/','crates/omsi-sim/src/people/','crates/omsi-sim/src/human/','crates/omsi-app/src/view_sync/people/')) or p in ['crates/omsi-sim/src/people.rs','crates/omsi-app/src/view_sync/people.rs','crates/omsi-sim/src/view_events.rs','crates/omsi-app/src/app_events/frame/people.rs'] or p.startswith('crates/omsi-launcher-core/src/passenger') or p=='launcher/ui/js/pages/passenger-settings.js':
        return '03-passengers'
    if p.startswith(('crates/omsi-app/src/traffic/','crates/omsi-sim/src/ai_traffic/')) or p in ['crates/omsi-app/src/traffic.rs','crates/omsi-sim/src/ai_traffic.rs','crates/omsi-app/src/view_sync/traffic.rs','crates/omsi-app/src/view_sync/traffic_audio.rs','crates/omsi-audio/src/greeting.rs']:
        return '04-traffic-improvements'
    return '00-shared-integration'

OUT.mkdir(exist_ok=True)
assert git('rev-parse',EXACT+'^{tree}').decode().strip()==TREE
raw=git('diff','--binary','--full-index','--no-renames',BASE,EXACT)
sections=re.findall(rb'^diff --git .*?(?=^diff --git |\Z)',raw,flags=re.S|re.M)
assert b''.join(sections)==raw
bygroup={k:[] for k in groups}; paths={k:[] for k in groups}
for section in sections:
    header=section.split(b'\n',1)[0].decode()
    path=header.split(' b/',1)[1]
    group=owner(path)
    bygroup[group].append(section);paths[group].append(path)

source=[]
for row in git('ls-tree','-rz',EXACT).split(b'\0'):
    if not row:continue
    meta,path=row.split(b'\t',1);mode,kind,blob=meta.decode().split()
    assert kind=='blob'
    data=git('cat-file','blob',blob)
    source.append(dict(path=path.decode(),mode=mode,blob_sha1=blob,sha256=sha(data),size=len(data)))
index={f['path']:f for f in source}
write(Path('source-files.json'),json.dumps(source,ensure_ascii=False,indent=2)+'\n')

series=[]
for key,(title,scope) in groups.items():
    patch=b''.join(bygroup[key]);folder=OUT/key;folder.mkdir(exist_ok=True)
    (folder/'changes.patch').write_bytes(patch)
    files=[dict(path=p,source=index.get(p)) for p in paths[key]]
    write(Path(key)/'files.json',json.dumps(files,ensure_ascii=False,indent=2)+'\n')
    series.append(dict(group=key,path=key+'/changes.patch',sha256=sha(patch),size=len(patch),files=len(files)))
    links=[]
    for p in paths[key]:
        if p in index: links.append(f'- [{p}](../../{p})')
        else: links.append(f'- `{p}` — removed in this native tree; deletion is retained in the patch.')
    write(Path(key)/'README.md',f'''# {title}

{scope}

This part contains **complete file-level diffs** from official openOMSI 0.3.46, not illustrative snippets. Every link below opens the full corrected developer-review source based on 2.2.2 Hotfix 2; this revision is later than the automatic-update installer. The complete native workspace is at the repository root.

- [Complete changes.patch](changes.patch)
- [File identities and SHA-256](files.json)
- [Integration and dependency rules](../README.md)
- [Code audit and remaining issues](../REVIEW.md)
- [Recorded validation](../VALIDATION.md)

These are **review partitions**, not independently buildable upstream pull requests. Shared glue lives in part 00. Part 01 references part 02's inspector, and part 03 contains shared passenger structures. Apply the whole series for an exact reconstruction; do not copy one changed file over current upstream main.

## Complete source files

'''+ '\n'.join(links)+'\n')


SUPPORT=Path(__file__).resolve().parent
original=json.loads((SUPPORT/'original-SOURCE.json').read_text())
revision=json.loads(Path(os.environ['OPENOMSI_REVIEW_REVISION']).read_text())
assert revision['commit']==EXACT and revision['source_tree']==TREE
assert revision['windows_build_passed'] and revision['compiled_caption_tests_passed'] and revision['settings_roundtrip_tests_passed']
provenance=dict(release='2.2.2-hotfix.2',revision='localized-options-review',
    published_to_automatic_updates=False,official_base=BASE,
    exact_source_commit=EXACT,exact_source_tree=TREE,revision_build_record=revision,
    original_source_commit='b6c22b1b993b0126ab799c7634105c2f1b9d0d87',
    original_source_tree='990d5300185280c6f209d72ddaa0666a07f9d482',
    original_build_record=original,original_release_sequence=2026103003,
    original_native_build_run=41157142308,original_native_build_artifact=13025264907,
    original_bridge_packaging_run=41159417197,original_publication_run=41159937109,
    original_published_installer_sha256='686c4188c5d69d23deb0debf163f8fcf23020eb934623e37146022a8cbcd0123',
    code_changed_since_original_export=True,feature_patches_are_independent_prs=False,
    exact_source_files=len(source),patches=series,
    exclusions=['external BCS Bridge','Setup','HostAgent','CompanyHost','server P14','update/signing service','proprietary BCS JAR','original OMSI 2 game assets'])
write(Path('provenance.json'),json.dumps(provenance,ensure_ascii=False,indent=2)+'\n')
write(Path('revision-build.json'),json.dumps(revision,ensure_ascii=False,indent=2)+'\n')
write(Path('series.txt'),'\n'.join(s['path'] for s in series)+'\n')
(OUT/'all-native-vs-official-0.3.46.patch').write_bytes(raw)
for filename,target in [('README.md','README.md'),('OPTIONS.md','OPTIONS.md'),('REVIEW.md','REVIEW.md'),('VALIDATION.md','VALIDATION.md')]:
    text=(SUPPORT/filename).read_text().replace('@CODE@',EXACT).replace('@TREE@',TREE).replace('@RUN@',str(revision['run_id']))
    write(Path(target),text)
shutil.copyfile(SUPPORT/'verify.py',OUT/'verify.py')
print(json.dumps(dict(code_commit=EXACT,code_tree=TREE,source_files=len(source),patches=series),indent=2))
