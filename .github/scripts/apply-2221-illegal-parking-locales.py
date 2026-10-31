#!/usr/bin/env python3
"""Append the opt-in physical illegal-parking label to all 29 game languages."""
import json
import re
from pathlib import Path

catalog = Path("crates/omsi-app/locales/app.yml")
label = "Temporary illegally parked AI cars"
translations = {
    "be": "Часова няправільна прыпаркаваныя аўтамабілі ШІ",
    "ca": "Cotxes d'IA aparcats temporalment de manera irregular",
    "cs": "Dočasně nesprávně zaparkovaná vozidla AI",
    "de": "Vorübergehend verbotswidrig parkende KI-Autos",
    "es": "Coches de IA estacionados temporalmente de forma irregular",
    "fr": "Voitures de l'IA temporairement garées en infraction",
    "hi": "अस्थायी रूप से गलत जगह खड़ी एआई कारें",
    "hu": "Ideiglenesen szabálytalanul parkoló MI-autók",
    "id": "Mobil AI yang parkir liar sementara",
    "it": "Auto IA temporaneamente parcheggiate in modo irregolare",
    "ja": "一時的に違法駐車するAI車両",
    "ka": "AI მანქანები, რომლებიც დროებით არასწორად ჩერდებიან",
    "kk": "Уақытша заңсыз тұрақтайтын ЖИ көліктері",
    "ko": "일시적으로 불법 주차하는 AI 차량",
    "ms": "Kereta AI yang parkir secara tidak sah buat sementara waktu",
    "nl": "Tijdelijk foutgeparkeerde AI-auto's",
    "pl": "Samochody AI parkujące chwilowo w niedozwolonym miejscu",
    "pt": "Carros da IA estacionados irregularmente por pouco tempo",
    "pt-pt": "Carros da IA estacionados irregularmente por pouco tempo",
    "ru": "Автомобили ИИ, временно припаркованные с нарушением правил",
    "sr": "AI automobili koji se privremeno nepropisno parkiraju",
    "sv": "AI-bilar som tillfälligt felparkerar",
    "th": "รถ AI ที่จอดผิดกฎชั่วคราว",
    "tl": "Mga AI na kotse na pansamantalang ilegal ang pagparada",
    "tr": "Geçici olarak usulsüz park eden yapay zekâ araçları",
    "uk": "Автомобілі ШІ, тимчасово припарковані з порушенням правил",
    "vi": "Xe AI tạm thời đỗ sai quy định",
    "zh": "临时违规停车的AI车辆",
    "zh-tw": "暫時違規停車的AI車輛"
}
assert len(translations) == 29
text = catalog.read_text(encoding="utf-8")
if not re.search(r"^" + re.escape(json.dumps(label)) + r":\s*$", text, re.M):
    text += "\n" + json.dumps(label) + ":\n"
    for lang, value in sorted(translations.items()):
        text += f"  {lang}: {json.dumps(value, ensure_ascii=False)}\n"
    catalog.write_text(text, encoding="utf-8")
print("Illegal parking label languages", len(translations))

frequency_label = "Temporary illegal parking frequency"
frequency_translations = {
    "be": "Частата часовых парушэнняў правілаў паркоўкі",
    "ca": "Freqüència d'aparcament irregular temporal",
    "cs": "Četnost dočasného nesprávného parkování",
    "de": "Häufigkeit vorübergehender Falschparker",
    "es": "Frecuencia del estacionamiento irregular temporal",
    "fr": "Fréquence du stationnement irrégulier temporaire",
    "hi": "अस्थायी अवैध पार्किंग की आवृत्ति",
    "hu": "Ideiglenes szabálytalan parkolás gyakorisága",
    "id": "Frekuensi parkir liar sementara",
    "it": "Frequenza di parcheggio irregolare temporaneo",
    "ja": "一時的な違法駐車の発生頻度",
    "ka": "დროებითი არასწორი პარკირების სიხშირე",
    "kk": "Уақытша заңсыз тұрақ жиілігі",
    "ko": "일시적 불법 주차 빈도",
    "ms": "Kekerapan parkir tidak sah sementara",
    "nl": "Frequentie van tijdelijk foutparkeren",
    "pl": "Częstotliwość tymczasowego nielegalnego parkowania",
    "pt": "Frequência de estacionamento irregular temporário",
    "pt-pt": "Frequência de estacionamento irregular temporário",
    "ru": "Частота временной неправильной парковки",
    "sr": "Učestalost privremenog nepropisnog parkiranja",
    "sv": "Frekvens för tillfällig felparkering",
    "th": "ความถี่ของการจอดผิดกฎชั่วคราว",
    "tl": "Dalas ng pansamantalang ilegal na pagparada",
    "tr": "Geçici usulsüz park sıklığı",
    "uk": "Частота тимчасового неправильного паркування",
    "vi": "Tần suất đỗ xe trái quy định tạm thời",
    "zh": "临时违规停车频率",
    "zh-tw": "暫時違規停車頻率"
}
assert len(frequency_translations) == 29
text = catalog.read_text(encoding="utf-8")
if not re.search(r"^" + re.escape(json.dumps(frequency_label)) + r":\s*$", text, re.M):
    text += "\n" + json.dumps(frequency_label) + ":\n"
    for lang, value in sorted(frequency_translations.items()):
        text += f"  {lang}: {json.dumps(value, ensure_ascii=False)}\n"
    catalog.write_text(text, encoding="utf-8")
print("Illegal parking frequency languages", len(frequency_translations))
