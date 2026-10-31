#!/usr/bin/env python3
"""Append the new 2.2.2.1 traffic-courtesy option to all 29 app locales."""
import json
import re
from pathlib import Path

catalog = Path("crates/omsi-app/locales/app.yml")
label = "Cars change lanes for buses leaving stops"
translations = {
    "be": "Аўтамабілі перастройваюцца, каб прапусціць аўтобусы ад прыпынкаў",
    "ca": "Els cotxes canvien de carril per deixar sortir els autobusos de les parades",
    "cs": "Auta mění jízdní pruh, aby pustila autobusy ze zastávek",
    "de": "Autos wechseln die Spur, um Bussen das Ausfahren aus Haltestellen zu ermöglichen",
    "es": "Los coches cambian de carril para dejar salir a los autobuses de las paradas",
    "fr": "Les voitures changent de voie pour laisser sortir les bus des arrêts",
    "hi": "स्टॉप से निकलने वाली बसों को रास्ता देने के लिए कारें लेन बदलती हैं",
    "hu": "Az autók sávot váltanak, hogy kiengedjék a buszokat a megállókból",
    "id": "Mobil berpindah jalur untuk memberi jalan kepada bus yang keluar dari halte",
    "it": "Le auto cambiano corsia per far uscire gli autobus dalle fermate",
    "ja": "停留所から発進するバスに道を譲るため車が車線変更する",
    "ka": "მანქანები იცვლიან ზოლს, რათა ავტობუსები გაჩერებებიდან გამოვიდნენ",
    "kk": "Автокөліктер аялдамадан шығатын автобустарға жол беру үшін жолақ ауыстырады",
    "ko": "정류장에서 출발하는 버스를 위해 자동차가 차선을 변경합니다",
    "ms": "Kereta menukar lorong untuk memberi laluan kepada bas yang keluar dari hentian",
    "nl": "Auto's wisselen van rijstrook om bussen uit haltes te laten vertrekken",
    "pl": "Samochody zmieniają pas, aby przepuścić autobusy wyjeżdżające z przystanków",
    "pt": "Carros mudam de faixa para dar passagem a ônibus saindo dos pontos",
    "pt-pt": "Os carros mudam de faixa para dar passagem a autocarros que saem das paragens",
    "ru": "Автомобили перестраиваются, пропуская автобусы от остановок",
    "sr": "Automobili menjaju traku kako bi propustili autobuse sa stajališta",
    "sv": "Bilar byter körfält för att släppa ut bussar från hållplatser",
    "th": "รถยนต์เปลี่ยนเลนเพื่อให้ทางรถบัสที่ออกจากป้ายจอด",
    "tl": "Lumilipat ng linya ang mga kotse upang padaanin ang bus mula sa hintuan",
    "tr": "Otomobiller duraklardan çıkan otobüslere yol vermek için şerit değiştirir",
    "uk": "Автомобілі змінюють смугу, щоб пропустити автобуси від зупинок",
    "vi": "Ô tô chuyển làn để nhường đường cho xe buýt rời trạm",
    "zh": "汽车变道礼让驶离车站的公交车",
    "zh-tw": "汽車變換車道禮讓駛離站牌的公車"
}
assert len(translations) == 29
text = catalog.read_text(encoding="utf-8")
if not re.search(r"^" + re.escape(json.dumps(label)) + r":\\s*$", text, re.M):
    text += "\\n" + json.dumps(label) + ":\\n"
    for lang, value in sorted(translations.items()):
        text += f"  {lang}: {json.dumps(value, ensure_ascii=False)}\\n"
    catalog.write_text(text, encoding="utf-8")
print("Courtesy label languages", len(translations))
