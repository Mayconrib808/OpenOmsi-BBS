#!/usr/bin/env python3
"""Append three opt-in features to the central 29-language UI catalog exactly once.

The catalog is ~4 MB; editing it in GitHub Actions avoids the GitHub connector's
small-response size limit. The resulting app.yml is committed normally.
"""
import json
import re
from pathlib import Path

CATALOG = Path("crates/omsi-app/locales/app.yml")
TRANSLATIONS = {
    "AI bus drivers wave when greeting": {
        "be":"Кіроўцы аўтобусаў ШІ вітаюць узмахам рукі",
        "ca":"Els conductors dels autobusos IA saluden amb la mà",
        "cs":"Řidiči autobusů AI zdraví mávnutím ruky",
        "de":"KI-Busfahrer winken zur Begrüßung",
        "es":"Los conductores de autobuses de IA saludan con la mano",
        "fr":"Les conducteurs des bus IA saluent de la main",
        "hi":"AI बस चालक हाथ हिलाकर अभिवादन करते हैं",
        "hu":"Az MI-buszok vezetői kézlengetéssel köszönnek",
        "id":"Pengemudi bus AI melambaikan tangan saat menyapa",
        "it":"Gli autisti degli autobus IA salutano con la mano",
        "ja":"AIバスの運転手が手を振って挨拶する",
        "ka":"AI ავტობუსების მძღოლები ხელის დაქნევით ესალმებიან",
        "kk":"ЖИ автобус жүргізушілері қол бұлғап амандасады",
        "ko":"AI 버스 운전자가 손을 흔들어 인사",
        "ms":"Pemandu bas AI melambai ketika menyapa",
        "nl":"AI-buschauffeurs zwaaien bij het groeten",
        "pl":"Kierowcy autobusów AI machają na powitanie",
        "pt":"Motoristas dos ônibus da IA acenam ao cumprimentar",
        "pt-pt":"Motoristas dos autocarros da IA acenam ao cumprimentar",
        "ru":"Водители автобусов ИИ машут рукой при приветствии",
        "sr":"Vozači AI autobusa mašu pri pozdravu",
        "sv":"AI-bussförare vinkar när de hälsar",
        "th":"คนขับรถบัส AI โบกมือทักทาย",
        "tl":"Kumakaway ang mga drayber ng AI bus kapag bumabati",
        "tr":"Yapay zekâ otobüs şoförleri selam verirken el sallar",
        "uk":"Водії автобусів ШІ махають рукою на знак привітання",
        "vi":"Tài xế xe buýt AI vẫy tay chào",
        "zh":"AI公交车司机挥手致意",
        "zh-tw":"AI公車司機揮手致意",
    },
    "Optional passenger conversations inside buses": {
        "be":"Неабавязковыя размовы пасажыраў у аўтобусах",
        "ca":"Converses opcionals entre passatgers dins dels autobusos",
        "cs":"Volitelné rozhovory cestujících v autobusech",
        "de":"Optionale Gespräche von Fahrgästen im Bus",
        "es":"Conversaciones opcionales entre pasajeros dentro de los autobuses",
        "fr":"Conversations facultatives entre passagers dans les bus",
        "hi":"बसों के अंदर यात्रियों की वैकल्पिक बातचीत",
        "hu":"Választható utasbeszélgetések a buszokon",
        "id":"Percakapan penumpang opsional di dalam bus",
        "it":"Conversazioni facoltative tra passeggeri sugli autobus",
        "ja":"バス車内での乗客同士の会話（オプション）",
        "ka":"მგზავრების არასავალდებულო საუბრები ავტობუსებში",
        "kk":"Автобус ішіндегі жолаушылардың қосымша әңгімелері",
        "ko":"버스 안 승객 대화 선택 기능",
        "ms":"Perbualan penumpang pilihan di dalam bas",
        "nl":"Optionele gesprekken tussen passagiers in bussen",
        "pl":"Opcjonalne rozmowy pasażerów w autobusach",
        "pt":"Conversas opcionais entre passageiros dentro dos ônibus",
        "pt-pt":"Conversas opcionais entre passageiros dentro dos autocarros",
        "ru":"Необязательные разговоры пассажиров в автобусах",
        "sr":"Opcioni razgovori putnika u autobusima",
        "sv":"Valfria samtal mellan passagerare i bussar",
        "th":"บทสนทนาของผู้โดยสารภายในรถบัส (เปิดใช้ได้)",
        "tl":"Opsyonal na usapan ng mga pasahero sa loob ng bus",
        "tr":"Otobüslerde isteğe bağlı yolcu sohbetleri",
        "uk":"Необов'язкові розмови пасажирів в автобусах",
        "vi":"Trò chuyện tùy chọn giữa hành khách trên xe buýt",
        "zh":"车内乘客对话（可选）",
        "zh-tw":"車內乘客對話（可選）",
    },
    "AI buses safely pass other stopped buses": {
        "be":"Аўтобусы ШІ бяспечна аб'язджаюць іншыя аўтобусы на прыпынках",
        "ca":"Els autobusos IA avancen amb seguretat altres autobusos aturats",
        "cs":"Autobusy AI bezpečně předjíždějí jiné stojící autobusy",
        "de":"KI-Busse überholen andere haltende Busse sicher",
        "es":"Los autobuses de IA adelantan con seguridad a otros autobuses detenidos",
        "fr":"Les bus IA dépassent en sécurité les autres bus à l'arrêt",
        "hi":"AI बसें रुकी हुई दूसरी बसों को सुरक्षित तरीके से पार करती हैं",
        "hu":"Az MI-buszok biztonságosan megelőzik a megálló buszokat",
        "id":"Bus AI menyalip bus lain yang berhenti dengan aman",
        "it":"Gli autobus IA superano in sicurezza altri autobus fermi",
        "ja":"AIバスが停車中の他のバスを安全に追い越す",
        "ka":"AI ავტობუსები უსაფრთხოდ უსწრებენ გაჩერებულ ავტობუსებს",
        "kk":"ЖИ автобустары тоқтаған басқа автобустарды қауіпсіз басып озады",
        "ko":"AI 버스가 정차한 다른 버스를 안전하게 추월",
        "ms":"Bas AI memotong bas lain yang berhenti dengan selamat",
        "nl":"AI-bussen halen andere stilstaande bussen veilig in",
        "pl":"Autobusy AI bezpiecznie wyprzedzają inne stojące autobusy",
        "pt":"Ônibus da IA ultrapassam outros ônibus parados com segurança",
        "pt-pt":"Autocarros da IA ultrapassam outros autocarros parados em segurança",
        "ru":"Автобусы ИИ безопасно обгоняют другие остановившиеся автобусы",
        "sr":"AI autobusi bezbedno pretiču druge zaustavljene autobuse",
        "sv":"AI-bussar kör säkert om andra stillastående bussar",
        "th":"รถบัส AI แซงรถบัสคันอื่นที่จอดอยู่ได้อย่างปลอดภัย",
        "tl":"Ligtas na lumalampas ang mga AI bus sa ibang nakahintong bus",
        "tr":"Yapay zekâ otobüsleri duran diğer otobüsleri güvenle sollar",
        "uk":"Автобуси ШІ безпечно обганяють інші автобуси на зупинках",
        "vi":"Xe buýt AI vượt an toàn những xe buýt khác đang dừng",
        "zh":"AI公交车安全超越其他停靠的公交车",
        "zh-tw":"AI公車安全超越其他停靠的公車",
    },
}
REQUIRED = set(next(iter(TRANSLATIONS.values())))
text = CATALOG.read_text(encoding="utf-8")
for label, values in TRANSLATIONS.items():
    assert set(values) == REQUIRED, f"Incorrect language set for {label}"
    assert len(REQUIRED) == 29
    if re.search(r"^"+re.escape(json.dumps(label, ensure_ascii=False))+r":\s*$", text, re.M):
        continue
    text += "\n"+json.dumps(label,ensure_ascii=False)+":\n"
    for locale, translated in sorted(values.items()):
        text += f"  {locale}: {json.dumps(translated, ensure_ascii=False)}\n"
CATALOG.write_text(text,encoding="utf-8")
print("Translated AI feature labels in",len(REQUIRED),"languages; bytes",len(text.encode()))
