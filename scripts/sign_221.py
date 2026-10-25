"""Sign the verified hotfix with the publisher already trusted by installed Setups."""
import argparse,base64,json
from pathlib import Path
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
ROOT=Path(__file__).resolve().parents[1]
VERSION='2.2.1'
SEQUENCE=2026102515
p=argparse.ArgumentParser()
p.add_argument('--publisher-key',type=Path,required=True)
p.add_argument('--candidate-report',type=Path,required=True)
p.add_argument('--candidate-run',type=int,required=True)
p.add_argument('--candidate-commit',required=True)
a=p.parse_args()
r=json.loads(a.candidate_report.read_text(encoding='utf-8-sig'))
assert r['version']==VERSION and r['sequence']==SEQUENCE
assert r['complete_p15_ui_tested'] and all(x['all_other_bytes_preserved'] for x in r['complete_host_patch'].values())
assert r['official_041_p15'] and r['app_server_native_041'] and r['custom_closing_signal']
assert r['coop_protocol']==15 and r['base_package_sha256']=='74e1c44bfe88ab10bb16c31daaa36a72794d6a91893994badd03db785604fe45'
assert r['installer_size']<=25<<20 and len(a.candidate_commit)==40
assert r['complete_hosts_unchanged'] and r['servers_unchanged'] and r['passenger_migration_tested']
assert r['native_candidate_commit']=='4510587e4aff1abfbd7555394d9b6f59a9da4c90'
assert r['native_version']=='0.3.41-coop.14-2.2.1'
material=json.loads(a.publisher_key.read_text(encoding='utf-8-sig'))
key=Ed25519PrivateKey.from_private_bytes(base64.b64decode(material['private_seed'],validate=True))
assert key.public_key().public_bytes_raw()==base64.b64decode('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=')
assert material['last_sequence']<SEQUENCE
notes={'pt': '2.2.1: sinal de parada durante a aproximacao, mantido ate o onibus parar; caminhada com menor puladinha e apoio na plataforma. Guarda-chuva preparado a maior distancia, aberto ate entrar, fechado apontando para baixo. Agua com sistema de gotas dos vidros e gotejamento pelas bordas/ponta. Efeitos ativos por padrao tambem ao abrir pelo BCS nos mapas brasileiros/poloneses selecionados, com opcao de desativar. Controles dos efeitos tambem dentro do jogo; escolha Natural/OMSI apenas no lobby. Preserva os hosts completos, servidores P14/P15, audio Fechou e correcoes Coop anteriores. Feche jogo e Setup antes de atualizar. Versao para o pessoal testar os modelos dos addons.', 'en': '2.2.1: passengers hail an approaching compatible bus until it stops; reduced walking bounce and platform support. Umbrellas appear earlier, stay open until boarding, and hang down when folded. Native window raindrop film and gravity-driven drips. Effects enabled by default for direct BCS starts on the selected maps; later opt-outs remain saved. Effects can also be toggled in game; Natural/OMSI stays in the lobby. Complete hosts, P14/P15 servers, custom audio and cumulative Coop fixes preserved. Close game and Setup before updating. Addon models need community visual testing.', 'de': '2.2.1: Fahrgaeste geben einem passenden herannahenden Bus bis zum Anhalten ein Zeichen; weniger Huepfen und stabilerer Bodenkontakt. Schirme werden frueher sichtbar, bleiben bis zum Einstieg offen und zeigen geschlossen nach unten. Regentropfen wie auf Scheiben und fallende Tropfen. Effekte bei direktem BCS-Start auf den ausgewaehlten Karten standardmaessig aktiv; spaetere Abschaltung bleibt gespeichert. Effekte auch im Spiel schaltbar, Natural/OMSI nur in der Lobby. Vollstaendige Hosts, P14/P15-Server, Ton und bisherige Coop-Korrekturen erhalten. Spiel und Setup vor dem Update schliessen. Visueller Addon-Test durch die Community erforderlich.'}
m=dict(schema=1,version=VERSION,sequence=SEQUENCE,notes=notes,installer=dict(url='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev/packages/'+r['updater_sha256']+'.exe',sha256=r['updater_sha256'],size=r['installer_size']))
raw=json.dumps(m,ensure_ascii=False,separators=(',',':')).encode()
out=ROOT/'build-support/bridge221';out.mkdir(exist_ok=True)
(out/'stable.json').write_text(json.dumps(dict(payload=base64.b64encode(raw).decode(),signature=base64.b64encode(key.sign(raw)).decode()),separators=(',',':'))+'\n')
publication=dict(version=VERSION,sequence=SEQUENCE,candidate_run_id=a.candidate_run,candidate_commit=a.candidate_commit,installer_sha256=r['updater_sha256'],installer_size=r['installer_size'],coop_archive_sha256=r['coop_archive_sha256'])
(out/'publication.json').write_text(json.dumps(publication,indent=2)+'\n')
print('Signed verified hotfix with the existing publisher; no private key included in the project.')

