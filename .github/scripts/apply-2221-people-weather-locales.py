#!/usr/bin/env python3
"""Add two opt-in passenger mechanics to all installed UI languages."""
import json
import re
from pathlib import Path

catalog = Path("crates/omsi-app/locales/app.yml")
translations = {
    "Independent passenger demand at each stop": {
        "be": "Незалежны попыт пасажыраў на кожным прыпынку",
        "ca": "Demanda independent de passatgers a cada parada",
        "cs": "Nezávislá poptávka cestujících na každé zastávce",
        "de": "Unabhängige Fahrgastnachfrage an jeder Haltestelle",
        "es": "Demanda independiente de pasajeros en cada parada",
        "fr": "Demande indépendante de passagers à chaque arrêt",
        "hi": "हर स्टॉप पर स्वतंत्र यात्री मांग",
        "hu": "Önálló utasigény minden megállóban",
        "id": "Permintaan penumpang mandiri di setiap halte",
        "it": "Domanda autonoma di passeggeri a ogni fermata",
        "ja": "停留所ごとに独立した乗客需要",
        "ka": "მგზავრების დამოუკიდებელი მოთხოვნა თითოეულ გაჩერებაზე",
        "kk": "Әр аялдамадағы дербес жолаушылар сұранысы",
        "ko": "정류장별 독립적인 승객 수요",
        "ms": "Permintaan penumpang bebas di setiap hentian",
        "nl": "Onafhankelijke passagiersvraag per halte",
        "pl": "Niezależny popyt pasażerów na każdym przystanku",
        "pt": "Demanda independente de passageiros em cada ponto",
        "pt-pt": "Procura independente de passageiros em cada paragem",
        "ru": "Независимый пассажиропоток на каждой остановке",
        "sr": "Nezavisna potražnja putnika na svakom stajalištu",
        "sv": "Oberoende passagerarefterfrågan vid varje hållplats",
        "th": "ความต้องการผู้โดยสารอิสระในแต่ละป้าย",
        "tl": "Hiwalay na dami ng pasahero sa bawat hintuan",
        "tr": "Her durakta bağımsız yolcu talebi",
        "uk": "Незалежний попит пасажирів на кожній зупинці",
        "vi": "Nhu cầu hành khách độc lập tại từng điểm dừng",
        "zh": "各站点独立乘客需求",
        "zh-tw": "各站點獨立乘客需求"
    },
    "Weather-appropriate passenger clothing": {
        "be": "Адзенне пасажыраў паводле надвор'я",
        "ca": "Roba dels passatgers adequada al temps",
        "cs": "Oblečení cestujících podle počasí",
        "de": "Wettergerechte Kleidung der Fahrgäste",
        "es": "Ropa de pasajeros adecuada al clima",
        "fr": "Tenues des passagers adaptées à la météo",
        "hi": "मौसम के अनुकूल यात्री कपड़े",
        "hu": "Időjáráshoz illő utasruházat",
        "id": "Pakaian penumpang sesuai cuaca",
        "it": "Abbigliamento dei passeggeri adatto al meteo",
        "ja": "天候に応じた乗客の服装",
        "ka": "მგზავრების ამინდის შესაფერისი ტანსაცმელი",
        "kk": "Ауа райына сай жолаушы киімі",
        "ko": "날씨에 맞는 승객 복장",
        "ms": "Pakaian penumpang mengikut cuaca",
        "nl": "Passagierskleding passend bij het weer",
        "pl": "Ubrania pasażerów dopasowane do pogody",
        "pt": "Roupas dos passageiros conforme o clima",
        "pt-pt": "Roupa dos passageiros adequada ao tempo",
        "ru": "Одежда пассажиров по погоде",
        "sr": "Odeća putnika prilagođena vremenu",
        "sv": "Passagerarkläder efter väder",
        "th": "เสื้อผ้าผู้โดยสารตามสภาพอากาศ",
        "tl": "Damit ng mga pasahero ayon sa panahon",
        "tr": "Hava durumuna uygun yolcu kıyafetleri",
        "uk": "Одяг пасажирів відповідно до погоди",
        "vi": "Trang phục hành khách theo thời tiết",
        "zh": "乘客服装随天气变化",
        "zh-tw": "乘客服裝隨天氣變化"
    }
}
assert len(translations) == 2
text = catalog.read_text(encoding="utf-8")
for label, entries in translations.items():
    assert len(entries) == 29
    if not re.search(r"^" + re.escape(json.dumps(label)) + r":\s*$", text, re.M):
        text += "\n" + json.dumps(label) + ":\n"
        for language, translated in sorted(entries.items()):
            text += f"  {language}: {json.dumps(translated, ensure_ascii=False)}\n"
catalog.write_text(text, encoding="utf-8")
print("Weather clothing and independent demand localized in 29 languages each")
