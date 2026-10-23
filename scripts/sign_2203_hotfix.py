"""Sign a proven hotfix; the publisher key is never copied into the package."""
import argparse,base64,json
from pathlib import Path
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
ROOT=Path(__file__).resolve().parents[1]
VERSION='2.2.0.3-hotfix.1';SEQUENCE=2026102302
ORIGIN='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev'
def main():
 p=argparse.ArgumentParser();p.add_argument('--publisher-key',type=Path,required=True);p.add_argument('--candidate-report',type=Path,required=True);p.add_argument('--candidate-run',type=int,required=True);p.add_argument('--candidate-commit',required=True);a=p.parse_args()
 report=json.loads(a.candidate_report.read_text(encoding='utf-8-sig'))
 if report['version']!=VERSION or report['sequence']!=SEQUENCE or not report['native_setup_smoke'] or not report['native_server_lamp'] or not report['native_manual_and_timer'] or len(report['retained_components_sha256'])!=6:raise ValueError('Unvalidated hotfix candidate')
 material=json.loads(a.publisher_key.read_text());key=Ed25519PrivateKey.from_private_bytes(base64.b64decode(material['private_seed'],validate=True))
 if key.public_key().public_bytes_raw()!=base64.b64decode('5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs='):raise ValueError('Trusted publisher differs')
 if material['last_sequence']>=SEQUENCE:raise ValueError('Publisher sequence already used')
 digest=report['updater_sha256']
 notes={'pt':'Hotfix do Setup: luz verde/amarela de Status do Servidor, atualização automática a cada 5 segundos e botão Verificar se o servidor está online. Confirma o HostAgent local e a disponibilidade no diretório, incluindo espera sob demanda. A consulta não inicia mapas nem altera configurações. HostAgent completo P15, Coop 9 e servidor dinâmico preservados da 2.2.0.3.',
 'en':'Setup hotfix: green/yellow Server status lamp, automatic checks every five seconds and a manual refresh button. Confirms local HostAgent readiness and directory presence, including on-demand standby, without waking maps or changing settings. Complete P15 host, Coop 9 and dynamic server retained from 2.2.0.3.',
 'de':'Setup-Hotfix: grüne/gelbe Serverstatus-Anzeige, automatische Prüfung alle fünf Sekunden und manuelle Aktualisierung. Prüft lokalen HostAgent und Verzeichnispräsenz einschließlich Bereitschaft auf Anfrage, ohne Karten zu starten oder Einstellungen zu ändern. Vollständiger P15-Host, Coop 9 und dynamischer Server aus 2.2.0.3 bleiben erhalten.'}
 manifest=dict(schema=1,version=VERSION,sequence=SEQUENCE,notes=notes,installer=dict(url=ORIGIN+'/packages/'+digest+'.exe',sha256=digest,size=report['installer_size']))
 raw=json.dumps(manifest,ensure_ascii=False,separators=(',',':')).encode();envelope=dict(payload=base64.b64encode(raw).decode(),signature=base64.b64encode(key.sign(raw)).decode())
 out=ROOT/'build-support/bridge2203-hotfix';(out/'stable.json').write_text(json.dumps(envelope,separators=(',',':'))+'\n')
 publication=dict(version=VERSION,sequence=SEQUENCE,candidate_run_id=a.candidate_run,candidate_commit=a.candidate_commit,installer_sha256=digest,installer_size=report['installer_size'],full_package_sha256=report['full_package_sha256'],coop_archive_sha256=report['coop_archive_sha256'])
 (out/'publication.json').write_text(json.dumps(publication,indent=2)+'\n');print(json.dumps(publication))
if __name__=='__main__':main()
