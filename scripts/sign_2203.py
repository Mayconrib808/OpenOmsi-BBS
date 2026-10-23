"""Sign the exact approved Windows candidate without exposing or copying the publisher key."""
import argparse,base64,hashlib,json
from pathlib import Path
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
ROOT=Path(__file__).resolve().parents[1]
ORIGIN='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev'
def main():
    p=argparse.ArgumentParser();p.add_argument('--publisher-key',type=Path,required=True);p.add_argument('--candidate-report',type=Path,required=True)
    p.add_argument('--candidate-run',type=int,required=True);p.add_argument('--candidate-commit',required=True);a=p.parse_args()
    report=json.loads(a.candidate_report.read_text(encoding='utf-8-sig'))
    if report['version']!='2.2.0.3' or report['sequence']!=2026102301 or not report['complete_p15_host_code_preserved'] or not report['native_p15_controls_tested'] or not report['host_installer_powershell_5_and_7_tested']:raise ValueError('Candidate identity or native evidence differs')
    keys=json.loads(a.publisher_key.read_text(encoding='utf-8-sig'))
    private=Ed25519PrivateKey.from_private_bytes(base64.b64decode(keys['private_seed'],validate=True))
    if private.public_key().public_bytes_raw()!=base64.b64decode('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs='):raise ValueError('Publisher key differs from the trusted Setup key')
    notes={
      'pt':'Bridge 2.2.0.3 + Coop 9: HostAgent completo com voz, rádio, alcance, lista pública, banimento por identificador e senha admin por mapa preservados. Servidor dinâmico P14 mantém a base 0.3.39 e o carregamento por regiões. Ônibus IA da mesma linha se cumprimentam em sentidos opostos e respondem à buzina ou ao lampejo curto do jogador. Farol baixo e farol alto fixo não contam como lampejo. Atualize também o host. Viagem gráfica real com BCS e dois jogadores ainda requer validação no jogo.',
      'en':'Bridge 2.2.0.3 + Coop 9 retains the complete P15 HostAgent controls and the protocol-14 dynamic server on its 0.3.39 base with regional streaming. Oncoming AI buses on the same line greet one another and answer a player horn or brief high-beam flash. Dipped or steady high beams do not trigger a reply. Update the host as well. A real graphical two-player BCS trip still requires in-game validation.',
      'de':'Bridge 2.2.0.3 + Coop 9 bewahrt die vollständigen P15-HostAgent-Optionen und den dynamischen P14-Server auf Basis 0.3.39 mit regionalem Streaming. Entgegenkommende KI-Busse derselben Linie grüßen einander und antworten auf Hupe oder kurze Lichthupe des Spielers. Dauerlicht löst keine Antwort aus. Auch den Host aktualisieren. Ein echter grafischer BCS-Test mit zwei Spielern steht noch aus.'}
    digest=report['updater_sha256']
    manifest=dict(schema=1,sequence=2026102301,version='2.2.0.3',notes=notes,installer=dict(url=ORIGIN+'/packages/'+digest+'.exe',sha256=digest,size=report['installer_size']))
    raw=json.dumps(manifest,ensure_ascii=False,separators=(',',':')).encode()
    envelope=dict(payload=base64.b64encode(raw).decode(),signature=base64.b64encode(private.sign(raw)).decode())
    out=ROOT/'build-support/bridge2203'
    (out/'stable.json').write_text(json.dumps(envelope,separators=(',',':'))+'\n',encoding='utf-8')
    publication=dict(version='2.2.0.3',sequence=2026102301,candidate_run_id=a.candidate_run,candidate_commit=a.candidate_commit,installer_sha256=digest,installer_size=report['installer_size'],full_package_sha256=report['full_package_sha256'],coop_archive_sha256=report['coop_archive_sha256'])
    (out/'publication.json').write_text(json.dumps(publication,indent=2)+'\n',encoding='utf-8')
    print(json.dumps(publication))
if __name__=='__main__':main()
