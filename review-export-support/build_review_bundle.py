from pathlib import Path
import subprocess, hashlib, json, re, shutil, os

ROOT=Path(os.environ.get('OPENOMSI_REVIEW_ROOT','/workspace/scratch/b003ee0e9009/hotfix2-source'))
OUT=ROOT/'REVIEW-2.2.2-HOTFIX2'
BASE='17be22987d7c01c813a1473e7696fc71fb3b1d89'
EXACT=os.environ.get('OPENOMSI_REVIEW_EXACT','8612e3284b1b7367c8be8cfbf92c49f777a23d89')
TREE='990d5300185280c6f209d72ddaa0666a07f9d482'
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
   'Closed-door requests, missed-stop reactions, 22% exit thanks, real exit detection, stop tolerance and path recovery. Cumulative hailing, umbrella render support and onward connections are included where they share these complete source files; they are not all new Hotfix 2 features.'),
 '04-traffic-improvements': ('Traffic improvements / Melhorias no trânsito',
   'Street population weighted by physical lane length, separate population around LAN players, guest traffic activation, population diagnostics and retained emergency-priority and double-horn/light greeting support. Emergency dispatch itself is in part 01; network/menu glue is in part 00.'),
}

def owner(p):
    if p in ['crates/omsi-app/src/bcs_events/inspector.rs','crates/omsi-app/src/bcs_events/names.rs','crates/omsi-sim/src/people/inspector.rs'] or p.startswith('crates/omsi-app/src/bcs_events/names-data/'):
        return '02-inspectors'
    if p.startswith('crates/omsi-app/src/bcs_events/') or p in ['crates/omsi-app/src/traffic/responder.rs','crates/omsi-sim/src/ai_traffic/response.rs','tools/generate_bcs_assets.py','crates/omsi-sim/examples/bcs_models.rs','ci/bcs-capture-reader-tests.rs','scripts/stage-bcs-police.py','docs/beta/bcs-incidents.md','docs/beta/emergency-models.json']:
        return '01-bcs-events'
    if p.startswith(('crates/omsi-sim/src/people/','crates/omsi-sim/src/human/','crates/omsi-app/src/view_sync/people/')) or p in ['crates/omsi-sim/src/people.rs','crates/omsi-app/src/view_sync/people.rs','crates/omsi-sim/src/view_events.rs','crates/omsi-app/src/app_events/frame/people.rs'] or p.startswith('crates/omsi-launcher-core/src/passenger') or p=='launcher/ui/js/pages/passenger-settings.js':
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

This part contains **complete file-level diffs** from official openOMSI 0.3.46, not illustrative snippets. Every link below opens the full source as shipped in 2.2.2 Hotfix 2. The complete native workspace is at the repository root.

- [Complete changes.patch](changes.patch)
- [File identities and SHA-256](files.json)
- [Integration and dependency rules](../README.md)
- [Code audit and remaining issues](../REVIEW.md)
- [Recorded validation](../VALIDATION.md)

These are **review partitions**, not independently buildable upstream pull requests. Shared glue lives in part 00. Part 01 references part 02's inspector, and part 03 contains shared passenger structures. Apply the whole series for an exact reconstruction; do not copy one changed file over current upstream main.

## Complete source files

'''+ '\n'.join(links)+'\n')

meta=json.loads(Path(os.environ.get('OPENOMSI_REVIEW_METADATA','/workspace/scratch/b003ee0e9009/native-SOURCE.json')).read_text())
provenance=dict(release='2.2.2-hotfix.2',release_sequence=2026103003,native_version=meta['version'],
    official_base=BASE,exact_source_commit=EXACT,exact_source_tree=TREE,
    original_build_record=meta,native_build_run=41157142308,native_build_artifact=13025264907,
    bridge_packaging_run=41159417197,publication_run=41159937109,
    published_installer_sha256='686c4188c5d69d23deb0debf163f8fcf23020eb934623e37146022a8cbcd0123',
    code_changed_for_this_export=False,feature_patches_are_independent_prs=False,
    exact_source_files=len(source),patches=series,
    exclusions=['external BCS Bridge','Setup','HostAgent','CompanyHost','update/signing service','proprietary BCS JAR','original OMSI 2 game assets'])
write(Path('provenance.json'),json.dumps(provenance,ensure_ascii=False,indent=2)+'\n')
write(Path('series.txt'),'\n'.join(s['path'] for s in series)+'\n')
write(Path('all-native-vs-official-0.3.46.patch'),raw.decode('utf-8'))

write(Path('README.md'),f'''# OpenOMSI 2.2.2 Hotfix 2 — código separado para revisão

Código **nativo do OpenOMSI**, exatamente correspondente à compilação Windows usada no Buscar atualizações. A árvore do código foi comparada com `SOURCE.json` e com o arquivo de fontes do artefato; os bytes e a árvore Git coincidem. O arquivo de fontes tem SHA-256 `{meta['source_sha256']}`.

O pedido para abrir a porta, os agradecimentos e as reclamações **estão incluídos** na parte 03. As **melhorias no trânsito têm uma parte própria, 04**.

| Parte | Código completo e patch |
| --- | --- |
| Eventos do BCS: efeitos e veículos de emergência | [01-bcs-events](01-bcs-events/README.md) |
| Fiscais: aparência, assento, patrulha e retorno | [02-inspectors](02-inspectors/README.md) |
| Passageiros: falas, portas, embarque e desembarque | [03-passengers](03-passengers/README.md) |
| Melhorias no trânsito: densidade, vias longas e multiplayer | [04-traffic-improvements](04-traffic-improvements/README.md) |
| Configurações, menus, protocolo e dependências comuns | [00-shared-integration](00-shared-integration/README.md) |

O código integral de cada arquivo está na raiz deste branch, nos caminhos apontados por cada parte. Os patches contêm todo o delta dos arquivos atribuídos à parte, não apenas os trechos novos da hotfix. Suporte cooperativo, guarda-chuvas e outras mudanças anteriores que compartilham arquivos permanecem identificados como contexto cumulativo. Não há código dos aplicativos externos da Bridge, Setup ou hosts neste branch.

## Revisão antes de enviar ao upstream

[REVIEW.md](REVIEW.md) confronta o código com os comentários dos PRs #3427, #3428 e #3429 e registra os problemas observados. **Este é um pacote de referência auditado, não uma declaração de que o código está livre de bugs ou pronto para merge.** Existem pendências de desempenho, opções, traduções e regressões específicas. Nenhum resultado de FPS ou de viagem em dois PCs foi inventado.

[VALIDATION.md](VALIDATION.md) separa compilação/testes Windows existentes, verificação nova desta extração e testes no jogo que continuam pendentes. Este trabalho publica fontes e documentação; não altera a versão oferecida pelo atualizador.

## Origem exata

- Base oficial 0.3.46: `{BASE}`.
- Commit publicado com a árvore exata do código: `{EXACT}`.
- Árvore Git do artefato Windows: `{TREE}`.
- [Compilação nativa e testes](https://github.com/Mayconrib808/OpenOmsi-BBS/actions/runs/41157142308).
- [Empacotamento da Bridge](https://github.com/Mayconrib808/OpenOmsi-BBS/actions/runs/41159417197).
- [Publicação no Buscar atualizações](https://github.com/Mayconrib808/OpenOmsi-BBS/actions/runs/41159937109).
- [Proveniência e hashes](provenance.json); [inventário integral das fontes](source-files.json).

O nome de versão interno continua `0.3.46-coop.15-2.2.2`. O script histórico `apply_2223.py` usa a expressão 2.2.3, mas seus ajustes já foram incorporados à **2.2.2 Hotfix 2** publicada; o registro do artefato e a sequência do atualizador são a referência. Os patches históricos em `ci/release222` já estão materializados neste branch: **não os aplique outra vez**. O antigo diretório `UPSTREAM-REVIEW` documenta a exportação 2.2.1.8; para esta entrega, use este diretório.

## Como usar os patches

As cinco partes particionam o delta **por arquivo**, sem sobreposição. Isso permite revisar cada assunto e reconstruir o conjunto sem conflitos entre patches. Não são quatro implementações independentes: eventos, fiscais e passageiros compartilham estruturas e pontos de integração. As etapas intermediárias não são qualificadas como compiláveis separadamente.

Para examinar/compilar, prefira clonar este branch já materializado. Para reproduzir o delta completo, use uma cópia limpa da base oficial indicada, aplique os caminhos de `series.txt` em ordem com `git apply --index` e confira a árvore final. [verify.py](verify.py) realiza essa prova em um índice temporário, sem modificar a árvore de trabalho.

```sh
git fetch https://github.com/openOMSI-org/openOMSI.git {BASE}
python3 REVIEW-2.2.2-HOTFIX2/verify.py
```

Não aplique esses patches cegamente ao `main` atual: #3427 já foi incorporado ao upstream e parte das contribuições anteriores está nos outros PRs. O desenvolvedor deve portar as partes necessárias sobre o estado atual, preservando as opções e os testes independentes. O relatório explica as dependências e as lacunas.

## Licenças e ativos

Os avisos/licenças existentes das fontes foram preservados. O workspace upstream usa AGPL-3.0-only; os dados de nomes têm seus próprios avisos e GFDL-1.2 em `crates/omsi-app/src/bcs_events/names-data`. Não se deve tratar todo o pacote como MIT. O gerador das viaturas originais está em `tools/generate_bcs_assets.py`; não é necessário redistribuir veículos proprietários do OMSI para revisar o código.

Para o desenvolvedor: [English code audit](REVIEW.md) and [validation evidence](VALIDATION.md).
''')

write(Path('VALIDATION.md'),'''# Validation and limits

## Exact released build (existing evidence, re-read for this audit)

Native Windows run [41157142308](https://github.com/Mayconrib808/OpenOmsi-BBS/actions/runs/41157142308), job `native`, completed successfully. Its source archive SHA-256 and source tree are recorded in `provenance.json`.

| Check actually executed | Result |
| --- | --- |
| Windows release build of `omsi-app` and native `--version` | Passed |
| `ai_traffic::response::tests` | 8 passed |
| `people::inspector::patrol_tests` | 1 passed |
| `tolerant_stop_box_tests` | 2 passed |
| `one_entry_one_exit_with_separate_rear_leaf_can_open_for_alighting` | 1 passed |
| `omsi-app bcs_events::` | 14 passed |
| Standalone international inspector name test (also present in the 14 app tests) | 1 passed |
| Generated police/fire geometry axis and actual model loading | Passed |

There were 27 test executions here, including the repeated name test, not a fresh run of every workspace regression. The simulation commands filtered out hundreds of other tests. These focused tests do not cover all dialogue timers, every cabin topology, visual appearance or guest traffic density.

The Bridge packaging run 41159417197 also reported passing Bridge regressions, update/changelog Windows UI checks and unchanged complete hosts/P14. Those are packaging evidence, not additional native gameplay tests.

## Checks newly performed for this source delivery

- Downloaded the exact native artifact; verified `openomsi-source.tar.gz` against its recorded SHA-256.
- Reproduced the published source tree byte-for-byte, including Windows line endings in six files written by the layering script.
- Exposed that exact tree in a normal Git commit; the materialized CI-only commit is no longer the only reference to these sources.
- Partitioned the complete official-base delta into five file-disjoint patches, retaining binary patch records and licenses.
- Applied every patch in order to a temporary Git index at the official base and compared the final Git tree with the artifact tree.
- Verified every source file SHA-256 and blob identity; verified no file is omitted or assigned to two parts.
- Read the initial and follow-up reviews of upstream PRs #3427, #3428 and #3429. Findings and remaining work are in `REVIEW.md`.

Run `python3 REVIEW-2.2.2-HOTFIX2/verify.py` to repeat the extraction/integrity checks. `verification.json` records their result; it does not certify gameplay or performance.

## Still not established

No new Rust compilation was run locally for this documentation-only export; the recorded successful Windows build belongs to the exact code tree included here. No zero-bug, zero-allocation or zero-FPS-impact claim is made. Real-map busy stops, articulated/addon buses, long avenues with separated LAN peers, disabled-option RNG equivalence, VR/mobile visuals, GPU/VRAM growth, p95/p99 frame time and a complete two-PC BCS journey remain to be validated.
''')

print(json.dumps(dict(exact_source_commit=EXACT,exact_source_tree=TREE,source_files=len(source),patches=series),indent=2))
