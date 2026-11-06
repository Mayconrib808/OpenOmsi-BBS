//go:build windows

package main

import (
    "os"
    "path/filepath"
    "strings"
    "syscall"
    "unsafe"
)

// A native, offline, scrollable, localized release-tour window. Everything
// renders inside Setup; no WebView, external site, registry or asset download.
const changelogSeenVersion = "2.2.3.2"
const (
    logWMPaint=0x000F
    logWMMouseWheel=0x020A
    logWMKeyDown=0x0100
    logWMSize=0x0005
    logIDDone=1401
    logIDSectionBase=1410
)
type logString struct {pt,en,de,es,fr,ru string}
type logEntry struct {title,body logString}
type logSection struct {label logString; accent uintptr; entries []logEntry}
type logPaintStruct struct {
    DC uintptr
    Erase int32
    Rect guiRect
    Restore,IncUpdate int32
    Reserved [32]byte
}
type logWindowState struct {
    section,scroll,maxScroll int
    heading,body,subhead,tiny uintptr
    buttons []uintptr
}
var changelogWindow uintptr
var changelogProcedure=syscall.NewCallback(changelogWindowProc)
var changelogState *logWindowState

func logT(pt,en,de string) logString {return logString{pt:pt,en:en,de:de,es:en,fr:en,ru:en}}
func logT6(pt,en,de,es,fr,ru string) logString {return logString{pt,en,de,es,fr,ru}}
func logL(lang string,t logString) string {
    switch normalizeLanguage(lang) {
    case "es": return t.es
    case "fr": return t.fr
    case "ru": return t.ru
    default: return localText(lang,t.pt,t.en,t.de)
    }
}
func logAccent(r,g,b byte)uintptr{return guiRGB(r,g,b)}

func allLogSections() []logSection {
    return []logSection{
        {
            label:logT("🛠️ HOTFIX 3 — SERVIDOR", "🛠️ HOTFIX 3 — SERVER", "🛠️ HOTFIX 3 — SERVER"),
            accent:logAccent(112,219,164),
            entries:[]logEntry{
                {logT("🌍 BCS em qualquer idioma", "🌍 BCS in any language", "🌍 BCS in jeder Sprache"),
                 logT("O adaptador não restringe mais o nome do motorista ao inglês, português brasileiro ou espanhol: perfis do BCS em japonês, alemão, português europeu e demais idiomas são aceitos por sua estrutura, preservando o nome original em UTF-16. Arquivos legados como native.odr também funcionam; arquivos inválidos continuam bloqueados.",
                      "The adapter no longer allowlists translated driver names. Japanese, German, Spanish, European Portuguese and other BCS personnel records are accepted by validated structure, preserving the original UTF-16 name. Legacy filenames such as native.odr work; malformed records remain blocked.",
                      "Der Adapter beschränkt übersetzte Fahrernamen nicht mehr. Japanische, deutsche, spanische und andere BCS-Profile werden über die gültige Struktur erkannt, unter Erhalt des UTF-16-Namens. Ältere Dateinamen wie native.odr bleiben unterstützt; fehlerhafte Dateien werden abgewiesen.")},
                {logT("Salvar e iniciar empresas com muitos arquivos", "Save and start large company inventories", "Große Firmeninventare speichern und starten"),
                 logT("Corrige ‘host preferences exceed size limit’: o gerenciador abria perfis maiores que 4 MB e recusava salvá-los. Leitura, preservação de campos extras, salvamento e migração agora usam o mesmo limite local de 32 MiB, com leitura limitada e sem apagar inventários, mapas, chaves, voz ou senhas. Testes no Windows salvam e reabrem 22 mil arquivos cadastrados. Mantém P16, sinalização Original/Enhanced e integridade das oito vozes V2 da Hotfix 2.",
                 "Fixes ‘host preferences exceed size limit’: profiles above 4 MB could open but fail to save. Reading, preserving extra fields, saving and migration now share a bounded 32 MiB local limit, retaining inventories, maps, keys, voice and passwords. Windows tests save and reopen 22,000 inventory files. Retains P16, Original/Enhanced hailing and the eight approved V2 voice hashes from Hotfix 2.",
                 "Behebt ‘host preferences exceed size limit’: Profile über 4 MB ließen sich öffnen, aber nicht speichern. Lesen, Zusatzfelder, Speichern und Migration nutzen dieselbe begrenzte lokale 32-MiB-Grenze. Inventare, Karten, Schlüssel, Sprache und Passwörter bleiben erhalten. Windows-Tests speichern und öffnen 22.000 Inventardateien erneut. P16, Original-/Enhanced-Handzeichen und V2-Sprachprüfsummen aus Hotfix 2 bleiben erhalten.")},
            },
        },
        {
            label:logT("🛠️ CORREÇÕES DO HOTFIX","🛠️ HOTFIX CORRECTIONS","🛠️ HOTFIX-KORREKTUREN"),
            accent:logAccent(255,203,92),
            entries:[]logEntry{
                {logT("✅ Fiscal agora percorre o corredor","✅ Inspector now patrols the aisle","✅ Kontrolleur läuft durch den Gang"),
                 logT("Corrigimos o fiscal que levantava e ficava parado perto da porta. Durante a fiscalização, ele usa os caminhos internos do ônibus para visitar diferentes regiões e fazer pequenas pausas perto dos passageiros, sem depender de o contador do BCS mudar a cada passo.",
                 "Fixes the inspector standing still by the door after getting up. During inspection, the character walks through the bus using its interior paths, visiting passenger areas with short pauses instead of waiting for the BCS count to change at every step.",
                 "Behebt den Kontrolleur, der nach dem Aufstehen an der Tür stehen blieb. Während der Kontrolle nutzt er die Innenwege des Busses, besucht die Fahrgastbereiche und legt kurze Pausen ein, ohne ständig auf den BBS-Zähler zu warten.")},
                {logT("🚓 Polícia e bombeiros em tamanho correto","🚓 Police and fire engines at proper scale","🚓 Polizei und Feuerwehr in richtiger Größe"),
                 logT("Os carros substitutos incluídos na Bridge tinham um erro de conversão de eixos dos modelos 3D. A carroceria da viatura aparecia como se tivesse apenas 1,8 metro de comprimento. A geometria da polícia e dos bombeiros foi corrigida para preservar o tamanho real; o posicionamento em mapas diferentes ainda precisa ser confirmado.",
                 "Bundled emergency replacements had an incorrect 3D axis conversion. The police car could be interpreted as only 1.8 metres long. The geometry for police and fire vehicles now uses the proper dimensions. In-game placement still needs verification on different maps.",
                 "Die mitgelieferten Ersatzfahrzeuge hatten eine falsche Umrechnung der 3D-Achsen. Das Polizeiauto konnte nur als 1,8 Meter lang erkannt werden. Die Geometrie von Polizei und Feuerwehr wurde berichtigt; die Positionierung muss noch auf verschiedenen Karten getestet werden.")},
                {logT("🚨 Emergências com mais tempo e diagnóstico","🚨 Longer response window and diagnostics","🚨 Längere Einsatzzeit und Protokollierung"),
                 logT("O sistema não deve mais encerrar a tentativa de atendimento depois de apenas 30 segundos. O tempo máximo foi ampliado para 180 segundos, com registros no host sobre o recebimento da ocorrência, despacho, chegada, cancelamentos e dificuldades de rota. A cena só aparece quando o BCS realmente informar um evento compatível.",
                 "The responder is no longer cancelled after just 30 seconds; the limit increases to 180 seconds. Host logs now report received events, dispatch, arrival, cancellation and routing problems. A visual scene still requires a compatible real BCS event.",
                 "Ein Einsatz wird nicht mehr schon nach 30 Sekunden beendet; die Grenze wurde auf 180 Sekunden erhöht. Das Host-Protokoll dokumentiert empfangene Ereignisse, Entsendung, Ankunft, Abbruch und Routenprobleme. Voraussetzung bleibt ein passendes BBS-Ereignis.")},
                {logT("🚗 Trânsito mais completo no multiplayer","🚗 Better multiplayer traffic coverage","🚗 Bessere Verkehrsabdeckung im Multiplayer"),
                 logT("Corrigida a regra que considerava áreas do host e dos convidados sobrepostas mesmo quando parte da área do jogador ainda estava sem carros. O servidor pode preencher essa região respeitando os limites de densidade, espaço livre e desempenho; não significa tráfego infinito ou aumento forçado das configurações.",
                 "The host previously treated nearby guest coverage as already populated even if part of the guest's area was empty. The server can fill the remaining area while honoring traffic settings, free space and performance limits, rather than forcing extra cars everywhere.",
                 "Der Host betrachtete die Umgebung anderer Spieler teilweise als bereits versorgt, obwohl Abschnitte leer waren. Jetzt kann der Server diese Bereiche unter Beachtung von Verkehrsmenge, freiem Platz und Leistung füllen, ohne überall mehr Autos zu erzwingen.")},
                {logT("✅ Banco reservado e retorno ao lugar","✅ Reserved seat and return","✅ Reservierter Sitz und Rückkehr"),
                 logT("O banco continua reservado enquanto o fiscal caminha. Ao encerrar a inspeção, ele procura voltar ao mesmo assento. O trajeto depende dos caminhos internos definidos no modelo do ônibus; ele não deve se teletransportar nem sair automaticamente no ponto final.",
                 "The seat remains reserved while the inspector walks. When inspection ends, the character attempts to return to the same seat. This requires a usable interior path; the inspector should neither teleport nor automatically leave at the final stop.",
                 "Der Sitz bleibt während des Rundgangs reserviert. Nach der Kontrolle versucht die Figur, zu diesem Sitz zurückzukehren. Dafür ist ein gültiger Innenweg nötig; sie soll sich weder teleportieren noch am Endpunkt automatisch aussteigen.")},
                {logT("🚪 Desembarque e portas de ônibus personalizados","🚪 Alighting and addon bus door mapping","🚪 Ausstieg und Türen von Add-on-Bussen"),
                 logT("Correção adicional para ônibus que registram apenas uma entrada e uma saída, mas usam outra animação física para a porta traseira. A saída agora pode reconhecer essa porta realmente aberta. Reclamações de porta ficaram menos repetitivas e o log distingue pedidos de embarque e desembarque. O funcionamento no Apache VIP III ainda precisa ser confirmado numa viagem.",
                 "Additional fix for buses declaring only one entrance and one exit while the rear door uses another physical animation. A genuinely open rear door can now be recognized for alighting. Door complaints are less repetitive and logs distinguish boarding from alighting requests. Apache VIP III gameplay still requires verification.",
                 "Zusätzliche Korrektur für Busse mit nur einem gemeldeten Ein- und Ausgang, deren hintere Tür anders animiert wird. Ein tatsächlich geöffneter Hinterausgang kann nun erkannt werden. Türbeschwerden wiederholen sich seltener, und das Protokoll unterscheidet Ein- und Ausstiegswünsche. Der Apache VIP III muss im Spiel noch getestet werden.")},
                {logT("✨ Novidades da 2.2.2 reapresentadas","✨ Full 2.2.2 changes shown again","✨ Alle Neuerungen von 2.2.2 erneut"),
                 logT("Mesmo quem já fechou a janela da 2.2.2 verá este changelog novamente após instalar o hotfix. Use as categorias à esquerda e a roda do mouse para consultar todas as mudanças em português, inglês ou alemão.",
                 "The full release tour appears again after installing this hotfix, even if you dismissed the 2.2.2 notes earlier. Browse categories on the left and scroll with the mouse wheel. Available in Portuguese, English and German.",
                 "Nach dem Hotfix erscheint die vollständige Änderungsliste erneut, auch wenn die 2.2.2-Hinweise bereits geschlossen wurden. Links Kategorie wählen und mit dem Mausrad blättern. Auf Portugiesisch, Englisch und Deutsch.")},
            },
        },
        {
            label:logT("🚨 EVENTOS DO BCS","🚨 BCS INCIDENTS","🚨 BBS-EREIGNISSE"),
            accent:logAccent(255,112,122),
            entries:[]logEntry{
                {logT("✅ Polícia e bombeiros","✅ Police and fire brigade","✅ Polizei und Feuerwehr"),
                 logT("Ocorrências compatíveis do BCS podem aparecer no cenário com veículos de atendimento, sirenes e luzes. O sistema prioriza modelos disponíveis no jogo e inclui carro policial e caminhão de bombeiros genéricos como substitutos quando os modelos esperados não estiverem instalados.",
                 "Compatible BCS incidents can appear in the world with emergency vehicles, sirens and flashing lights. Installed vehicle models are preferred; a generic police car and fire truck are bundled when the expected models are unavailable.",
                 "Passende BBS-Ereignisse erscheinen in der Spielwelt mit Einsatzfahrzeugen, Sirenen und Blaulicht. Vorhandene Modelle werden bevorzugt; ein generisches Polizeiauto und ein Feuerwehrfahrzeug sind als Ersatz enthalten.")},
                {logT("✅ Ambulâncias para emergências médicas","✅ Ambulances for medical incidents","✅ Rettungswagen bei medizinischen Notfällen"),
                 logT("Desmaios, mal-estar e outras ocorrências médicas reconhecidas pelo BCS podem solicitar atendimento usando os modelos de ambulância encontrados no OpenOMSI. O resgate visual acompanha a ocorrência, sem substituir o resultado oficial do BCS.",
                 "Fainting and other recognized medical incidents can call for an ambulance using models found in OpenOMSI. The visual response follows the incident and does not override BCS outcomes.",
                 "Ohnmacht und andere erkannte medizinische Notfälle können einen Rettungswagen aus den OpenOMSI-Modellen anfordern. Der sichtbare Einsatz begleitet das Ereignis, ohne das BBS-Ergebnis zu ersetzen.")},
                {logT("✅ Óleo, fumaça e incêndio","✅ Oil, smoke and fire","✅ Öl, Rauch und Feuer"),
                 logT("Mancha de óleo na pista, fumaça no ônibus e lixeira pegando fogo recebem representações visuais próximas ao veículo. Antes, esses acontecimentos eram mostrados principalmente nos avisos do BCS; agora também podem ser vistos no cenário.",
                 "Oil spills on the road, smoke on the bus and a burning bin now have visual representations near the vehicle. Previously these events were mostly shown through BCS notifications.",
                 "Ölflecken auf der Fahrbahn, Rauch im Bus und ein brennender Abfalleimer erhalten sichtbare Effekte in Fahrzeugnähe. Zuvor erschienen sie hauptsächlich als BBS-Meldungen.")},
                {logT("✅ Alternativa pela calçada","✅ Sidewalk parking fallback","✅ Halten am Gehweg als Alternative"),
                 logT("Se não houver um espaço viável na rua, veículos de emergência podem procurar uma área de calçada suficientemente larga e livre para parar. A alternativa verifica espaço e obstáculos; não significa que o veículo possa atravessar qualquer calçada.",
                 "If no suitable road space is available, responders may use a sufficiently wide, unobstructed sidewalk area for parking. The fallback checks clearance and obstacles; it does not allow driving through arbitrary sidewalks.",
                 "Ist auf der Straße kein geeigneter Platz frei, können Einsatzfahrzeuge auf einem ausreichend breiten und freien Gehwegbereich halten. Platz und Hindernisse werden geprüft; beliebige Gehwege dürfen nicht befahren werden.")},
                {logT("✅ Captura automática do BCS","✅ Automatic BCS event capture","✅ Automatische Erkennung aus BBS"),
                 logT("A Bridge observa os eventos do BCS ao iniciar normalmente, tanto em viagens individuais quanto no multiplayer da empresa. Eventos puramente informativos, como rádio ou contagem de passageiros, continuam no BCS sem uma cena visual desnecessária.",
                 "The Bridge observes BCS events during normal startup, both on solo routes and in company multiplayer. Informational events, such as radio and passenger counts, stay within BCS without unnecessary world effects.",
                 "Die Bridge erkennt BBS-Ereignisse beim normalen Start im Einzelspieler und Firmen-Mehrspieler. Rein informative Ereignisse wie Funk oder Fahrgastzählung bleiben ohne unnötige Szeneneffekte im BBS.")},
            },
        },
        {
            label:logT("👮 FISCAIS E PASSAGEIROS","👮 INSPECTORS & RIDERS","👮 KONTROLLE & FAHRGÄSTE"),
            accent:logAccent(93,215,188),
            entries:[]logEntry{
                {logT("✅ Fiscal já dentro do ônibus","✅ Inspector already on board","✅ Kontrolleur bereits im Bus"),
                 logT("Quando o BCS identifica uma fiscalização, o OpenOMSI cria um personagem adulto num banco disponível, preferindo lugares próximos ao motorista. O visual é escolhido entre os humanos instalados; a identificação do fiscal orienta a escolha masculina ou feminina, quando possível.",
                 "When BCS identifies an inspector, OpenOMSI places an adult character in an available seat, preferring one near the driver. The model comes from installed human assets and the inspector's name can guide male or female appearance when possible.",
                 "Erkennt BBS einen Kontrolleur, setzt OpenOMSI eine erwachsene Figur auf einen freien Sitz, möglichst in Fahrernähe. Das Modell stammt aus vorhandenen Figuren; der Name kann die männliche oder weibliche Darstellung beeinflussen.")},
                {logT("✅ Inspeção com caminhada e retorno","✅ Walking inspection and return","✅ Rundgang und Rückkehr zum Sitz"),
                 logT("Ao iniciar a fiscalização, o fiscal deve levantar e circular pelo ônibus, acompanhando os passageiros. Ao terminar, deve procurar o banco reservado. Este hotfix corrige o caso em que ele ficava parado na porta depois de levantar.",
                 "Once inspection starts, the inspector should stand up, walk through the bus and visit passenger areas. After finishing, the character returns towards the reserved seat. This hotfix addresses the inspector remaining at the door.",
                 "Bei Kontrollbeginn steht die Figur auf, läuft durch den Bus und besucht die Fahrgastbereiche. Danach kehrt sie zum reservierten Sitz zurück. Dieser Hotfix behebt das Stehenbleiben an der Tür.")},
                {logT("✅ Embarque até 18 metros do ônibus","✅ Boarding within 18 metres","✅ Einstieg bis zu 18 Meter entfernt"),
                 logT("O motorista não precisa acertar exatamente a marca do ponto. Com o ônibus parado, porta de entrada aberta e linha correta, passageiros próximos podem caminhar até a porta, respeitando o limite de 18 metros e a posição do ponto.",
                 "Drivers no longer need to stop exactly on the marker. If the bus is stopped, the correct service is selected and an entrance door is open, nearby passengers can walk to it from up to 18 metres away, subject to stop constraints.",
                 "Der Bus muss nicht exakt auf der Haltestellenmarkierung stehen. Bei stehendem Bus, richtiger Linie und offener Einstiegstür können nahe Fahrgäste bis zu 18 Meter zur Tür laufen, sofern die Haltestellenbedingungen passen.")},
                {logT("✅ Passageiros do lado correto","✅ Passengers on the correct side","✅ Fahrgäste auf der richtigen Straßenseite"),
                 logT("A lógica de embarque confere a direção do ônibus, a altura, a posição do ponto e o lado físico da porta. Isso evita convidar passageiros do lado oposto da rua para atravessá-la apenas porque o ônibus parou por perto.",
                 "Boarding checks bus direction, height, stop position and the physical door side. This prevents people across the road from being invited to cross traffic just because a bus stopped nearby.",
                 "Beim Einstieg werden Fahrtrichtung, Höhe, Haltestelle und tatsächliche Türseite geprüft. So werden Fahrgäste auf der gegenüberliegenden Straßenseite nicht einfach durch den Verkehr zum Bus geführt.")},
                {logT("✅ Agradecimentos na saída","✅ Thanks when leaving","✅ Dank beim Aussteigen"),
                 logT("Alguns passageiros agradecem ao desembarcar usando as falas já existentes no pacote de vozes instalado. A reação não acontece com todos, para não ficar repetitiva.",
                 "Some passengers thank the driver when getting off, using the already installed voice pack. The reaction is occasional, rather than repeated for every passenger.",
                 "Einige Fahrgäste bedanken sich beim Aussteigen mit Stimmen aus dem installierten Sprachpaket. Das passiert gelegentlich und nicht bei jedem Fahrgast.")},
                {logT("✅ Pedidos para abrir a porta","✅ Requests to open the door","✅ Bitten, die Tür zu öffnen"),
                 logT("Quem espera para entrar ou sair pode pedir atenção quando a porta permanece fechada por tempo demais; há também reação ao fechamento junto ao desembarque. As falas são buscadas entre os áudios instalados e há intervalo entre reclamações para evitar repetição.",
                 "Waiting passengers may ask for attention when a suitable door stays closed too long; a door closing during an exit can also trigger a reaction. The game searches installed voice clips and applies cooldowns to avoid spam.",
                 "Wartende Fahrgäste können sich melden, wenn eine passende Tür lange geschlossen bleibt; auch das Schließen beim Ausstieg kann eine Reaktion auslösen. Vorhandene Sprachdateien werden genutzt, mit Pausen gegen ständige Wiederholungen.")},
            },
        },
        {
            label:logT("🚗 TRÂNSITO E CIDADE","🚗 TRAFFIC & CITY","🚗 VERKEHR UND STADT"),
            accent:logAccent(124,176,255),
            entries:[]logEntry{
                {logT("✅ Melhor distribuição dos veículos","✅ Better vehicle distribution","✅ Gleichmäßigere Fahrzeugverteilung"),
                 logT("A inteligência artificial considera a extensão das faixas disponíveis e a ocupação dos diferentes sentidos da via ao decidir onde criar carros. Isso busca reduzir concentrações estranhas num único lado de avenidas largas, respeitando os limites de tráfego configurados.",
                 "Traffic AI considers available lane length and occupancy by travel direction when placing cars. This aims to reduce strange clusters on one side of wide roads while preserving your configured traffic limits.",
                 "Die Verkehrs-KI berücksichtigt verfügbare Fahrstreifenlängen und die Auslastung je Richtung. Dadurch sollen ungewöhnliche Ansammlungen auf einer Seite breiter Straßen reduziert werden, ohne die eingestellten Verkehrslimits zu ändern.")},
                {logT("✅ Atendimento sem bloquear tudo","✅ Responders avoid unnecessary obstruction","✅ Rettungseinsätze ohne unnötige Blockaden"),
                 logT("Os atendimentos procuram um local seguro próximo ao incidente. Quando a pista não oferece espaço adequado, o posicionamento pode usar a alternativa de calçada livre e larga. A lógica evita locais com obstáculos e passagem insuficiente.",
                 "Responders look for a suitable position near the incident. Where the road lacks space, placement may use a free, wide sidewalk instead. Obstructions and insufficient clearance are considered.",
                 "Einsatzfahrzeuge suchen eine geeignete Stelle in Ereignisnähe. Reicht der Platz auf der Fahrbahn nicht aus, kann ein freier, breiter Gehweg genutzt werden. Hindernisse und zu geringe Durchfahrt werden berücksichtigt.")},
            },
        },
        {
            label:logT("👥 MULTIPLAYER E DIREÇÃO","👥 MULTIPLAYER & DRIVING","👥 MULTIPLAYER & FAHREN"),
            accent:logAccent(190,142,255),
            entries:[]logEntry{
                {logT("✅ Troca de motorista mais consistente","✅ More reliable driver handover","✅ Zuverlässigerer Fahrerwechsel"),
                 logT("O sistema de cooperação recebeu ajustes para conservar melhor o controle do ônibus e as informações associadas à viagem quando outro jogador assume o volante, reduzindo divergências entre motorista e passageiro.",
                 "Cooperative driving received improvements intended to preserve bus control and trip information when another player takes the wheel, reducing inconsistencies between driver and passenger views.",
                 "Der kooperative Fahrbetrieb wurde verbessert, damit Fahrzeugsteuerung und Fahrtdaten beim Fahrerwechsel möglichst erhalten bleiben und Ansichten zwischen Fahrer und Mitfahrer weniger voneinander abweichen.")},
                {logT("✅ GPS e sequência de pontos","✅ GPS and stop sequence","✅ GPS und Haltestellenfolge"),
                 logT("Há mudanças destinadas a conservar o ponto atual da rota na troca de motorista, evitando reiniciar a indicação desde o primeiro ponto. Esse cenário ainda precisa de confirmação em viagens compartilhadas completas.",
                 "Changes aim to retain the active route stop during handover instead of jumping back to the first one. This still needs verification in complete shared journeys.",
                 "Änderungen sollen beim Fahrerwechsel die aktuelle Haltestelle erhalten, statt zur ersten zurückzuspringen. Das muss noch in vollständigen gemeinsamen Fahrten geprüft werden.")},
                {logT("✅ Áudio restaurado ao voltar ao próprio ônibus","✅ Audio restored on returning to your bus","✅ Ton beim Zurückkehren zum eigenen Bus"),
                 logT("Ao reassumir o próprio veículo, o OpenOMSI tenta restabelecer os sons associados a ele caso estejam ausentes. A mudança trata o problema de retornar à direção sem o áudio normal do ônibus.",
                 "When returning to your own vehicle, OpenOMSI attempts to reload its sounds if missing. This targets the case where a returned driver cannot hear the normal bus audio.",
                 "Bei Rückkehr zum eigenen Fahrzeug versucht OpenOMSI, fehlende Fahrzeuggeräusche erneut zu laden. Das behebt den Fall, dass nach der Rückkehr der normale Buston fehlt.")},
                {logT("✅ Personagens sentados nos outros PCs","✅ Seated characters on other PCs","✅ Sitzende Figuren auf anderen PCs"),
                 logT("A sincronização passa a utilizar o estado real da posição sentada de passageiros, em vez de confiar apenas num estado antigo de animação. Isso ajuda a evitar figuras aparecendo de pé para um jogador enquanto estão sentadas para outro.",
                 "Synchronization uses the actual seated passenger state rather than an outdated animation cache. This helps prevent people appearing to stand for one player while sitting for another.",
                 "Die Synchronisierung nutzt den tatsächlichen Sitzstatus statt eines veralteten Animationszustands. So sollen Figuren nicht bei einem Spieler stehen, während sie bei einem anderen sitzen.")},
                {logT("✅ Respostas compartilhadas no servidor","✅ Server-coordinated shared events","✅ Vom Server koordinierte Ereignisse"),
                 logT("Em sessões cooperativas, o host coordena os atendimentos de emergência e retransmite dados visuais para os demais jogadores. A intenção é que todos vejam a mesma ocorrência, sem criar um veículo de resgate separado em cada PC.",
                 "In co-op sessions, the host coordinates emergency responders and relays visual incident data to connected players. The goal is a shared scene rather than separately spawned rescue vehicles on each PC.",
                 "In kooperativen Sitzungen koordiniert der Host die Einsatzfahrzeuge und verteilt sichtbare Ereignisdaten. Ziel ist eine gemeinsame Szene statt separat erzeugter Rettungsfahrzeuge auf jedem PC.")},
            },
        },
        {
            label:logT("⚙️ ATUALIZAÇÃO E VERSÕES","⚙️ UPDATES & VERSIONS","⚙️ UPDATES & VERSIONEN"),
            accent:logAccent(255,179,119),
            entries:[]logEntry{
                {logT("✅ OpenOMSI 0.3.46 e servidor P15","✅ OpenOMSI 0.3.46 and P15 server","✅ OpenOMSI 0.3.46 und P15-Server"),
                 logT("O OpenOMSI modificado e o executável do servidor dinâmico P15 foram compilados sobre a base 0.3.46. A Bridge permanece integrada ao BCS e conserva o caminho de instalação já usado pelos jogadores.",
                 "The modified client and dynamic P15 server were built on the 0.3.46 base. Bridge-to-BCS integration and the existing installation path are preserved.",
                 "Der modifizierte Client und der dynamische P15-Server basieren auf 0.3.46. Die BBS-Anbindung und der bisherige Installationspfad bleiben erhalten.")},
                {logT("✅ Novidades completas em três idiomas","✅ Full release tour in three languages","✅ Vollständige Neuerungen in drei Sprachen"),
                 logT("O antigo resumo foi substituído por este painel navegável, com emojis, seções coloridas e explicações de cada mecânica. Disponível em português, inglês e alemão. O hotfix mostra a janela novamente, mesmo para quem já viu a versão anterior.",
                 "The previous short list is replaced by this navigable panel with emoji, colored sections and detailed mechanics. Available in Portuguese, English and German. The hotfix opens it again even if the original was dismissed.",
                 "Die alte Kurzliste wird durch dieses durchsuchbare Fenster mit Emoji, farbigen Bereichen und detaillierten Erklärungen ersetzt. Auf Portugiesisch, Englisch und Deutsch. Der Hotfix zeigt die Hinweise erneut.")},
                {logT("✅ Arquivos antigos preservados","✅ Previous components preserved","✅ Bisherige Komponenten bleiben erhalten"),
                 logT("A montagem da atualização mantém os hosts completos, o servidor P14 e recursos anteriores, enquanto troca somente os componentes necessários do cliente, servidor P15 e Setup. Os pacotes são verificados por hashes e assinatura.",
                 "The release assembly preserves the complete hosts, P14 server and earlier assets, replacing only the necessary client, P15 server and Setup components. Packages are verified by hashes and publisher signature.",
                 "Die Update-Erstellung bewahrt vollständige Hosts, den P14-Server und bisherige Dateien. Nur notwendige Client-, P15- und Setup-Komponenten werden ersetzt. Pakete werden mit Hashes und Signatur geprüft.")},
                {logT("📌 Testes em viagens reais","📌 Real gameplay testing","📌 Tests während echter Fahrten"),
                 logT("Compilação e verificações automatizadas não substituem testes jogando. Eventos do BCS, movimentação do fiscal em diferentes modelos de ônibus, GPS após troca de motorista e sincronização multiplayer precisam ser confirmados durante viagens reais.",
                 "Compilation and automated checks do not replace gameplay tests. BCS incidents, inspector walking on different bus models, handover GPS and multiplayer synchronization still require verification during real journeys.",
                 "Kompilierung und automatisierte Tests ersetzen keine Fahrten im Spiel. BBS-Ereignisse, der Kontrolleur in verschiedenen Busmodellen, GPS beim Fahrerwechsel und Mehrspieler-Synchronisierung müssen noch geprüft werden.")},
            },
        },
    }
}

// Release notes are chosen from the version installed BEFORE updating. On
// legacy packages that did not store this marker, we deliberately show the
// whole known history rather than misidentify what the player already has.
var shownLogSections []logSection

func hotfix2LogSection() logSection {
    return logSection{
        label:logT("🛠️ HOTFIX 2 • DESEMBARQUE","🛠️ HOTFIX 2 • ALIGHTING","🛠️ HOTFIX 2 • AUSSTIEG"),
        accent:logAccent(255,203,92),
        entries:[]logEntry{
            {logT("🚪 Portas de saída reconhecidas","🚪 Exit doors recognized","🚪 Ausstiegstüren erkannt"),
             logT("Ônibus personalizados podem manter o indicador PAX_Exit como fechado mesmo com a porta fisicamente aberta. Agora a simulação também verifica a animação real da folha correspondente; abrir só a porta dianteira não libera uma saída traseira.",
                  "Some custom buses report PAX_Exit closed even while the physical leaf is open. The simulator also checks the matching real door animation; opening only the front door does not unlock rear exits.",
                  "Manche Zusatzbusse melden PAX_Exit geschlossen, obwohl die Tür tatsächlich offen ist. Das Spiel prüft nun auch die passende reale Türanimation; nur die Vordertür zu öffnen entriegelt keine Hintertür.")},
            {logT("🚏 Parada certa e rota de saída","🚏 Correct stop and exit path","🚏 Richtige Haltestelle und Ausstiegsweg"),
             logT("Em corredores com vários pontos próximos, a parada agendada tem prioridade sobre IDs arbitrários. Caminhos são recalculados quando o tempo chega a zero; passageiros que já alcançaram a saída aberta podem concluir a caminhada sem teleporte.",
                  "In corridors with nearby stops, the scheduled stop takes precedence over arbitrary IDs. Exit paths retry when the timer reaches zero, and a rider already at an open exit may complete the walk without teleporting.",
                  "Bei nahen Haltestellen hat die planmäßige Haltestelle Vorrang vor beliebigen IDs. Ausstiegswege werden bei Timer null neu berechnet; Fahrgäste an einer offenen Tür steigen ohne Teleport aus.")},
            {logT("🚶 Embarque fora da marca exata","🚶 Boarding around the stop pole","🚶 Einstieg neben der Haltestellenmarke"),
             logT("Tolerância controlada permite embarcar alguns metros antes ou depois da marca, no ponto certo e do lado correto da via. Quando falta o alvo da porta, o passageiro tenta outra entrada válida e continua caminhando normalmente.",
                  "A bounded allowance lets riders board a few metres before or after the marker, at the correct stop and side of the road. If the door target is missing, riders reselect a valid entry and walk normally.",
                  "Ein begrenzter Abstand vor oder hinter der Markierung ist erlaubt, nur an der richtigen Haltestelle und Straßenseite. Fehlt der Türeinstiegspunkt, wird ein gültiger Eingang neu gewählt.")},
            {logT("👮 Fiscal sem ciclo de rota infinito","👮 No inspector pathfinding loop","👮 Keine Endlosschleife beim Kontrolleur"),
             logT("Ao encontrar um corredor desconectado, o fiscal procura um ponto realmente alcançável e limita novas tentativas. Isso evita milhares de avisos por minuto e preserva os passageiros e o banco reservado.",
                  "When the aisle graph is disconnected, the inspector looks for a reachable point and throttles retries instead of issuing thousands of warnings. Passenger and seat states are preserved.",
                  "Bei unterbrochenen Innenwegen sucht der Kontrolleur einen erreichbaren Punkt und begrenzt neue Versuche. Tausende Warnungen werden vermieden; Fahrgäste und Sitzreservierung bleiben erhalten.")},
            {logT("🚗 Tráfego para cada jogador","🚗 Traffic around each player","🚗 Verkehr um jeden Spieler"),
             logT("O servidor calcula a meta de carros pelas ruas da região de cada participante, mesmo quando outro ônibus está quilômetros distante. O diagnóstico identifica casos sem faixas elegíveis; limites de densidade continuam valendo.",
                  "The server calculates traffic targets from the roads around each participant, even when buses are kilometres apart. Diagnostics report missing eligible lanes; density caps remain in force.",
                  "Der Server berechnet die Verkehrsdichte anhand der Straßen jedes Spielers, auch bei großer Entfernung. Diagnosen zeigen fehlende geeignete Spuren; Verkehrslimits gelten weiter.")},
            {logT("🧭 COOPROUTE1 e diagnóstico LAN","🧭 COOPROUTE1 and LAN diagnostics","🧭 COOPROUTE1 und LAN-Diagnose"),
             logT("Comandos de rota já processados deixam de aparecer falsamente como rejeitados. O registro de transferência de passageiros identifica ponto, ônibus, tarefa e posição; problemas de saída agora mostram o motivo individualmente.",
                  "Already handled route commands no longer appear falsely rejected. Passenger ownership logs show stop, bus, task and position; stuck exits now identify the reason per rider.",
                  "Bereits verarbeitete Routenbefehle erscheinen nicht mehr als abgewiesen. Fahrgast-Logs zeigen Haltestelle, Bus, Aufgabe und Position; blockierte Ausstiege erhalten Gründe.")},
            {logT("🎫 Portas e avisos menos repetitivos","🎫 Door warnings with cooldowns","🎫 Türmeldungen mit Pausen"),
             logT("Pedidos de abertura usam intervalos para evitar falas repetidas. Abrir a saída cancela reclamações pendentes; sair da parada com a saída fechada pode produzir uma reação mais irritada.",
                  "Door requests are rate-limited. Opening an exit cancels pending complaints, and leaving the stop with the exit closed may trigger a stronger response.",
                  "Türbitten werden begrenzt. Das Öffnen eines Ausstiegs beendet ausstehende Beschwerden; die Abfahrt mit geschlossener Tür kann eine stärkere Reaktion auslösen.")},
        },
    }
}

func hotfix3LogSection() logSection {
    return logSection{
        label:logT("🛠️ HOTFIX 3 • FISCAIS E TRÁFEGO","🛠️ HOTFIX 3 • INSPECTORS AND TRAFFIC","🛠️ HOTFIX 3 • KONTROLLEURE UND VERKEHR"),
        accent:logAccent(255,203,92),
        entries:[]logEntry{
            {logT("👮 Fiscal com aparência profissional","👮 Inspector with staff appearance","👮 Kontrolleur mit Dienstkleidung"),
             logT("O fiscal utiliza preferencialmente personagens uniformizados originais do OMSI, com opções masculina e feminina. A seleção não modifica as aparências dos passageiros comuns. Caso um modelo não esteja instalado, existe alternativa.",
                  "Inspectors prefer stock OMSI staff characters, with male and female options. Ordinary passengers keep their own appearances. A fallback is used if the preferred model is unavailable.",
                  "Kontrolleure verwenden bevorzugt originale OMSI-Dienstfiguren, mit männlicher und weiblicher Auswahl. Normale Fahrgäste bleiben unverändert; fehlende Modelle haben eine Alternative.")},
            {logT("🚌 Fiscal não age como passageiro comum","🚌 Inspector no longer acts as a regular passenger","🚌 Kontrolleur verhält sich nicht wie ein normaler Fahrgast"),
             logT("Durante a fiscalização, o personagem não reclama da direção, não pede parada nem tenta desembarcar no ponto final pela rotina normal dos passageiros. O assento fica reservado até o encerramento da inspeção.",
                  "During inspection the character does not complain about driving, press the stop button or automatically get off at the terminus. The seat remains reserved until the inspection ends.",
                  "Während der Kontrolle gibt es keine Fahrbeschwerden, Haltewünsche oder automatisches Aussteigen an der Endhaltestelle. Der Sitzplatz bleibt reserviert.")},
            {logT("🚗 Tráfego por sentido da avenida","🚗 Traffic in both avenue directions","🚗 Verkehr in beiden Fahrtrichtungen"),
             logT("O gerador reconsidera trechos longos que parecem sem saída enquanto o servidor carrega as ruas. O ajuste respeita faixas proibidas, densidade do mapa e espaço livre; diagnósticos por direção ficam disponíveis. A distribuição precisa ser conferida em viagem.",
                  "The generator can reconsider long lanes appearing disconnected while map tiles stream. The change respects prohibited lanes, map density and free space, with directional diagnostics available. In-game traffic distribution still needs checking.",
                  "Lange Spuren, die während des Karten-Nachladens als Sackgasse erscheinen, können erneut berücksichtigt werden. Sperren, Verkehrsdichte und Freiraum bleiben gültig; Richtungsdiagnosen helfen bei der Spielprüfung.")},
        },
    }
}

func earlierBridgeLogSection() logSection {
    return logSection{
        label:logT("📚 VERSÕES ANTERIORES","📚 EARLIER RELEASES","📚 FRÜHERE VERSIONEN"),
        accent:logAccent(119,187,244),
        entries:[]logEntry{
            {logT("📦 2.2.0 • Base cooperativa","📦 2.2.0 • Coop foundation","📦 2.2.0 • Koop-Grundlage"),
             logT("Integração multiplayer ampliada, início manual do ônibus, sincronização de combustível e melhorias de servidor, mapas e HostAgent.",
             "Broader multiplayer integration, manual bus startup, fuel synchronization and improved server, map and HostAgent handling.",
             "Erweiterter Mehrspielerbetrieb, manueller Busstart, Kraftstoff-Synchronisierung und Verbesserungen an Servern, Karten und HostAgent.")},
            {logT("🤝 2.2.1 • Motorista e cobrador","🤝 2.2.1 • Driver and conductor","🤝 2.2.1 • Fahrer und Schaffner"),
             logT("Painel cooperativo, troca de motorista e cobrador, compartilhamento das interações de bilhetes, acompanhamento de rota, ajustes de gestos e guarda-chuvas dos passageiros.",
             "Coop panel, driver/conductor handover, shared ticket interactions, route progress, passenger gestures and umbrellas.",
             "Koop-Anzeige, Fahrer-/Schaffnerwechsel, gemeinsame Ticketbedienung, Routenfortschritt, Fahrgastgesten und Regenschirme.")},
            {logT("🔧 2.2.1.8–2.2.1.9 • Compatibilidade","🔧 2.2.1.8–2.2.1.9 • Compatibility","🔧 2.2.1.8–2.2.1.9 • Kompatibilität"),
             logT("Correções de passageiro/cobrador e sincronização, BCS em idiomas diferentes, reparos de atualização e melhorias na integração PeDePe. O histórico técnico completo continua no repositório.",
             "Passenger/conductor and synchronization fixes, localized BCS compatibility, update repairs and PeDePe integration. The complete technical history remains in the repository.",
             "Fahrgast-/Schaffner- und Synchronisierungsfehler behoben, lokalisiertes BBS, Update-Reparaturen und PeDePe-Integration. Der vollständige technische Verlauf steht im Repository.")},
        },
    }
}

// Newer releases must be listed before the old cumulative sections. Each
// section is shown only when that release is newer than the last acknowledged
// changelog; installing several releases at once still shows their full history.
// Every release, including hotfixes, has its own cumulative changelog section.
// A separate cumulative changelog is mandatory for every hotfix. Existing
// entries are retained; players updating from 2.2.2.2 see only this new entry.
// Independent TEST release notes; all prior hotfix notes remain cumulative.
func winterTest2LogSection() logSection {
    return logSection{
        label:logT("🧪 2.2.2.2-test.2 • TESTE","🧪 2.2.2.2-test.2 • TEST","🧪 2.2.2.2-test.2 • TEST"),
        accent:logAccent(134,218,194),
        entries:[]logEntry{
            {logT("🧥 Roupas ligadas ao corpo real","🧥 Clothes attached to the actual body","🧥 Kleidung an der echten Körperoberfläche"),
             logT("Novo ajuste pela superfície do personagem carregado. Casacos, toucas e luvas acompanham a malha já animada ao andar, sentar ou mover os braços; a adaptação é reaproveitada por modelo. O teste visual no jogo ainda precisa ser confirmado.",
                  "New fitting against the loaded passenger's surface. Coats, hats and gloves follow the final animated mesh during walking, sitting and arm movements; each model's fit is cached. In-game visual validation is still required.",
                  "Neue Anpassung an die Oberfläche des geladenen Fahrgasts. Jacken, Mützen und Handschuhe folgen dem fertig animierten Körper beim Laufen, Sitzen und Bewegen der Arme. Die Anpassung wird je Modell gespeichert. Visuelle Prüfung im Spiel steht noch aus.")},
            {logT("🔄 Atualização com início confirmado","🔄 Confirmed updater startup","🔄 Bestätigter Updater-Start"),
             logT("O Setup aguarda a confirmação do atualizador mantendo a janela responsiva. Reconhece o aceite enviado pelo Windows e retorna erro se a instalação não for autorizada pelo Setup. Tentativas canceladas não podem instalar depois. Bloqueios temporários ao substituir executáveis são repetidos automaticamente; os limites do ZIP e dos arquivos foram alinhados ao pacote cumulativo.",
                  "Setup waits for the updater's acknowledgement while remaining responsive. Windows acceptance messages are recognized, and missing Setup acceptance returns an error. Canceled attempts cannot install later. Temporary executable locks are retried automatically; ZIP and member limits now match the cumulative package.",
                  "Setup wartet bei weiterhin bedienbarer Oberfläche auf die Bestätigung des Updaters. Windows-Bestätigungen werden erkannt; bei fehlender Setup-Bestätigung wird ein Fehler zurückgegeben. Abgebrochene Versuche installieren später nichts. Temporäre Dateisperren werden automatisch erneut versucht; ZIP- und Dateigrenzen entsprechen dem Gesamtpaket.")},
            {logT("📋 Diagnóstico e histórico acumulado","📋 Diagnostics and cumulative history","📋 Diagnose und vollständiger Änderungsverlauf"),
             logT("O log identifica cada etapa da atualização e o motivo de falha. As roupas registram o modelo real, o método de fixação e os motivos de bloqueio. Este changelog é exclusivo da test 2 e mantém as mudanças anteriores desde sua última versão vista.",
                  "Logs identify update stages and failure reasons. Clothing diagnostics include the actual model, attachment method and rejection reasons. Test 2 has its own changelog and retains changes since the last release you viewed.",
                  "Protokolle zeigen Updatephasen und Fehlergründe sowie das tatsächliche Fahrgastmodell, die Befestigungsmethode und Ablehnungsgründe. Test 2 hat einen eigenen Änderungsverlauf mit allen seit der zuletzt angesehenen Version hinzugekommenen Änderungen.")},
        },
    }
}

func winterTest1LogSection() logSection {
    return logSection{
        label:logT("🧪 2.2.2.2-test.1 • TESTE","🧪 2.2.2.2-test.1 • TEST","🧪 2.2.2.2-test.1 • TEST"),
        accent:logAccent(218,174,255),
        entries:[]logEntry{
            {logT("🧥 Roupas 3D: teste de encaixe","🧥 3D clothing: fit preview","🧥 3D-Kleidung: Anprobe-Test"),
             logT("VERSÃO EXPERIMENTAL. A adaptação de casacos, toucas e luvas foi refeita por segmentos do esqueleto. Ative manualmente Roupas conforme o clima nas opções do jogo para testar. Os modelos ainda NÃO foram aprovados visualmente; fotografe frente, costas, caminhada, sentados e movimentação dos braços. Desative a opção se houver defeitos.",
                  "EXPERIMENTAL BUILD. Coats, hats and gloves have been refitted to skeleton segments. Manually enable Weather-appropriate passenger clothing in game settings to test. Models have NOT been visually approved. Take front, rear, walking, seated and arm-motion screenshots. Disable this option if malformed.",
                  "EXPERIMENTELLE VERSION. Jacken, Mützen und Handschuhe werden an Skelettsegmente angepasst. Wettergerechte Fahrgastkleidung in den Spieleinstellungen manuell einschalten. Noch NICHT visuell freigegeben. Bilder von Vorder-/Rückseite, Laufen, Sitzen und Armbewegungen machen. Bei Fehlern ausschalten.")},
            {logT("🛡️ Controle de qualidade e retorno","🛡️ Safety and rollback","🛡️ Sicherheit und Rückkehr"),
             logT("Nenhuma mecânica adicional é ativada automaticamente. A checagem de triângulos e coordenadas continua bloqueando geometrias inválidas e mantém o personagem original. A Hotfix 1 está preservada para recuperação; o jogo, Bridge, eventos BCS, servidor P15 e logs não foram propositalmente alterados.",
                  "No additional game feature is enabled automatically. The triangle/coordinate checks continue to reject invalid geometry and preserve original passengers. Hotfix 1 remains available for rollback; unrelated Bridge, BCS, P15 and logging functionality is unchanged.",
                  "Weitere Spielmechaniken werden nicht automatisch aktiviert. Ungültige Geometrie wird blockiert, Originalfahrgäste bleiben erhalten. Hotfix 1 bleibt für Rückkehr erhalten; Bridge, BBS, P15 und Protokollierung bleiben unverändert.")},
        },
    }
}

func winterSafetyHotfixLogSection() logSection {
    return logSection{
        label:logT("🛡️ 2.2.2.2 • HOTFIX 1","🛡️ 2.2.2.2 • HOTFIX 1","🛡️ 2.2.2.2 • HOTFIX 1"),
        accent:logAccent(145,211,245),
        entries:[]logEntry{
            {logT("🧥 Roupas deformadas desativadas","🧥 Broken clothing hidden","🧥 Verformte Kleidung ausgeblendet"),
             logT("Corrigida a renderização de roupas de frio não adaptadas aos passageiros. Modelos experimentais sem aprovação de encaixe e animação são bloqueados, evitando triângulos gigantes, casacos flutuantes e toucas deslocadas. Os humanos usam suas roupas originais até existirem modelos realmente ajustados.",
                  "Unfitted experimental winter garments can no longer render as giant triangles, detached coats or misplaced hats. Until actual passenger-rig visual validation is complete, original passenger clothing remains visible.",
                  "Nicht angepasste experimentelle Winterkleidung erzeugt keine riesigen Dreiecke, schwebenden Jacken oder verschobenen Mützen mehr. Bis zur visuellen Prüfung bleibt die Originalkleidung sichtbar.")},
            {logT("🔒 Demais funções preservadas","🔒 Other features preserved","🔒 Andere Funktionen bleiben"),
             logT("Mantidos o servidor P15, o launcher com 14 opções, os logs detalhados, os eventos BCS e as demais mecânicas da 2.2.2.2. Esta hotfix não anuncia roupas novas como prontas: o encaixe real aos esqueletos continua pendente de validação visual.",
                  "P15 server, 14 launcher options, detailed logs, BCS events and all other 2.2.2.2 features are preserved. This hotfix does not claim the winter garments are visually fixed; stock-rig fitting remains unapproved.",
                  "P15-Server, 14 Launcher-Optionen, ausführliche Protokolle, BBS-Ereignisse und übrige 2.2.2.2-Funktionen bleiben. Die Winterkleidung ist noch nicht visuell angepasst und freigegeben.")},
        },
    }
}

func release2222LogSection() logSection {
    return logSection{
        label: logT("🧥 2.2.2.2 • ROUPAS E LOGS","🧥 2.2.2.2 • WINTER & LOGS","🧥 2.2.2.2 • WINTER & PROTOKOLLE"),
        accent: logAccent(126,211,180),
        entries: []logEntry{
            {logT("🧣 Roupas 3D reais para passageiros","🧣 Artist-made 3D passenger clothes","🧣 Echte 3D-Winterkleidung"),
             logT("Modelos de artistas com texturas originais: jaquetas, suéteres, toucas e luvas para NPCs padrões. De 6 a 15 °C: casaco; até 5 °C, ou neve a até 12 °C: touca e luvas também. Roupas são opcionais, com limites de distância e quantidade para proteger o FPS. O cachecol ainda não está incluído e o caimento visual precisa de teste em mapas reais.",
                  "Original artist-authored jackets, sweaters, hats and gloves for stock NPCs. From 6 to 15 °C: outerwear. At or below 5 °C, or snow at or below 12 °C: hats and gloves too. Optional feature, distance and count limits. Scarves are not included yet; in-game fit still requires validation.",
                  "Originale Künstler-Modelle für Jacken, Pullover, Mützen und Handschuhe bei Standard-Fahrgästen. Bei 6–15 °C Jacken, bei maximal 5 °C oder Schnee bis 12 °C auch Mützen und Handschuhe. Optional, mit Leistungsgrenzen. Schal und Praxistest stehen noch aus.")},
            {logT("📋 Logs com ações reais e resultados","📋 Actual actions and results in logs","📋 Tatsächliche Aktionen und Ergebnisse"),
             logT("Os logs distinguem oportunidade ausente, tentativa, execução, conclusão, bloqueio e falha. Incluem IDs e motivos para ultrapassagem de ônibus IA, carros cedendo passagem, estacionamento irregular, conversas, roupas por temperatura, trânsito e eventos BCS.",
                  "Feature logs distinguish no opportunity, attempt, execution, completion, blockage and failure. IDs and reasons cover AI overtaking, yielding cars, illegal parking, conversations, winter clothing, traffic density and BCS response events.",
                  "Funktionsprotokolle unterscheiden keine Gelegenheit, Versuch, Ausführung, Abschluss, Sperre und Fehler. Kennungen und Gründe für KI-Überholen, Vorfahrt, Falschparken, Gespräche, Winterkleidung, Verkehr und BBS-Einsätze.")},
            {logT("⏱️ Resumos e separação host/cliente","⏱️ Periodic summaries, host and client","⏱️ Zusammenfassungen für Host und Client"),
             logT("A cada 60 segundos, FEATURE SUMMARY reúne avaliados, executados, concluídos, bloqueados e motivos. O host relata decisões de simulação; o cliente relata carregamento 3D e áudio aceito no mixer. O registro de áudio não garante som fisicamente ouvido.",
                  "FEATURE SUMMARY every 60 seconds groups evaluations, execution, completion and blockers. Host reports simulation actions; client reports 3D loading and audio accepted by the mixer. Mixer acceptance does not guarantee speaker output.",
                  "FEATURE SUMMARY fasst alle 60 Sekunden Prüfungen, Ausführungen, Abschlüsse und Sperrgründe zusammen. Host: Simulation; Client: 3D-Grafik und Audiomixer. Mixer-Annahme bedeutet nicht zwingend hörbaren Ton.")},
            {logT("🛡️ Compatibilidade e limites","🛡️ Compatibility and limitations","🛡️ Kompatibilität und Grenzen"),
             logT("Mantidos os eventos BCS, veículos 3D de polícia/bombeiro, funções existentes, launcher com 14 opções e o servidor P15. Roupas extras não substituem os humanos originais e ficam desativadas se a opção Roupas conforme o clima estiver desligada. Esta versão ainda exige confirmação de aparência e FPS no jogo.",
                  "BCS events, 3D police/fire vehicles, existing functions, 14 launcher settings and P15 server are retained. Clothing overlays preserve original humans and follow the weather-clothing toggle. Visual fitting and FPS require in-game confirmation.",
                  "BBS-Ereignisse, 3D-Polizei/Feuerwehr, bisherige Funktionen, 14 Launcher-Optionen und P15-Server bleiben. Kleidung ergänzt die Originalmenschen nur mit aktivierter Wetter-Option. Sichtprüfung und FPS-Test stehen aus.")},
        },
    }
}

func hotfix2221SecondLogSection() logSection {
    return logSection{
        label:logT("🚓 2.2.2.1 • HOTFIX 2","🚓 2.2.2.1 • HOTFIX 2","🚓 2.2.2.1 • HOTFIX 2"),
        accent:logAccent(255,154,105),
        entries:[]logEntry{
            {logT("🚨 Polícia e bombeiro 3D","🚨 3D police and fire responders","🚨 3D-Polizei und Feuerwehr"),
             logT("Acrescentados veículos 3D de substituição aos eventos BCS quando o modelo adequado não estiver instalado. O movimento e a posição ainda exigem teste visual por mapa.",
                  "Original 3D fallback response vehicles for BCS incidents when preferred map models are missing. Movement and placement require in-game verification.",
                  "Originale 3D-Ersatzfahrzeuge für BBS-Ereignisse bei fehlenden Kartenmodellen. Bewegung und Position müssen geprüft werden.")},
            {logT("🚗 Estacionamento e interface","🚗 Parking frequency and launcher","🚗 Parkhäufigkeit und Launcher"),
             logT("Frequência do estacionamento irregular entre 0 e 5%, passo de 0,1%, com persistência. Lobby recompilado, preservando 14 configurações opcionais.",
                  "Illegal-parking frequency now persists from 0 to 5% in 0.1% steps; rebuilt Windows lobby retains all 14 optional settings.",
                  "Falschparkhäufigkeit von 0 bis 5 % in 0,1-%-Schritten wird gespeichert; der Windows-Launcher behält 14 Optionen.")},
        },
    }
}

func hotfix2221LogSection() logSection {
    return logSection{
        label:logT("🛠️ 2.2.2.1 • HOTFIX 1","🛠️ 2.2.2.1 • HOTFIX 1","🛠️ 2.2.2.1 • HOTFIX 1"),
        accent:logAccent(255,203,92),
        entries:[]logEntry{
            {logT("🖥️ Lobby atualizado e controles visíveis","🖥️ Updated launcher and visible settings","🖥️ Aktualisierter Launcher und Optionen"),
             logT("A 2.2.2.1 anterior instalou o jogo, mas manteve um launcher antigo. O pacote agora inclui o launcher Windows recompilado, com as 14 opções de mecânicas extras em Jogabilidade > Passageiros / Tráfego; nenhuma é ativada sem consentimento.",
                  "The previous 2.2.2.1 installed the game but retained an outdated launcher. The package now includes the rebuilt Windows launcher with all 14 optional feature controls under Gameplay > Passengers / Traffic; nothing is silently enabled.",
                  "Die bisherige 2.2.2.1 installierte das Spiel, behielt aber einen veralteten Launcher. Das Paket enthält nun den neu erstellten Windows-Launcher mit 14 optionalen Einstellungen unter Gameplay > Fahrgäste / Verkehr. Nichts wird automatisch aktiviert.")},
            {logT("🪑 Passageiros sentados no multiplayer","🪑 Correct remote seated passenger poses","🪑 Sitzende Fahrgäste im Mehrspieler"),
             logT("Corrigido o passageiro que aparecia em pé dentro da cadeira para o outro jogador. O cliente respeita o estado de sentado e o banco enviados pela rede nas animações originais e aprimoradas; levantar e desembarcar continuam possíveis.",
                  "Fixed riders appearing to stand inside seats on another player's screen. The client reconstructs the transmitted seated state and seat for original and enhanced animation; standing up and alighting remain supported.",
                  "Fahrgäste, die auf einem anderen PC im Sitz stehend dargestellt wurden, erhalten die übertragene Sitzpose in ursprünglicher und verbesserter Animation. Aufstehen und Aussteigen bleiben möglich.")},
            {logT("📜 Histórico acumulado de atualizações","📜 Cumulative release notes restored","📜 Vollständiger Änderungsverlauf"),
             logT("O changelog volta a abrir no Setup depois da instalação. A janela mostra a Hotfix 1, a 2.2.2.1 e as versões anteriores ainda não vistas, sem repetir as mudanças já confirmadas. A seleção depende do último changelog visualizado, não apenas do número do jogo.",
                  "The release tour opens again in Setup after installation. It shows Hotfix 1, the original 2.2.2.1 and any older updates not yet acknowledged, without repeating versions already read.",
                  "Der Änderungsverlauf öffnet sich nach der Installation erneut. Er zeigt Hotfix 1, 2.2.2.1 und frühere noch nicht bestätigte Änderungen, ohne bereits gelesene Versionen zu wiederholen.")},
            {logT("🔍 Fiscal e tráfego: ativação e diagnóstico","🔍 Inspector and traffic: enabling and diagnosis","🔍 Kontrolleur und Verkehr: Aktivierung"),
             logT("As opções Fiscal do BCS e Tráfego extra permanecem desligadas por padrão. Agora podem ser encontradas no launcher atualizado. A presença de carros IA normais não significa que a densidade extra esteja habilitada; a qualidade da distribuição em mapas reais ainda exige verificação.",
                  "BCS inspectors and extra traffic remain disabled by default and can now be enabled in the updated launcher. Normal AI traffic can be present without extra density; real-map traffic distribution still needs verification.",
                  "BBS-Kontrolleure und zusätzlicher Verkehr bleiben standardmäßig aus und sind jetzt im Launcher sichtbar. Normale KI-Fahrzeuge fahren auch ohne Extradichte; die Verteilung auf echten Karten muss noch geprüft werden.")},
        },
    }
}

func release2221LogSection() logSection {
    return logSection{
        label:logT("🚌 2.2.2.1 • NOVIDADES","🚌 2.2.2.1 • FEATURES","🚌 2.2.2.1 • NEU"),
        accent:logAccent(110,190,251),
        entries:[]logEntry{
            {logT("👋 Acenos e conversas nos ônibus","👋 Driver greetings and passenger talk","👋 Fahrergrüße und Gespräche"),
             logT("Motoristas IA podem acenar ao cruzar com você, enquanto passageiros podem conversar ocasionalmente usando vozes já instaladas. As duas mecânicas são opcionais e não devem produzir sons ou gestos continuamente.",
                  "AI drivers can wave when passing your bus; riders can occasionally talk using installed voice clips. Both are optional, with cooldowns to avoid constant gestures or sounds.",
                  "KI-Fahrer können beim Vorbeifahren winken, Fahrgäste gelegentlich mit installierten Stimmen sprechen. Beides ist optional und hat Pausen gegen Wiederholungen.")},
            {logT("🚌 Ultrapassagem e preferência ao sair do ponto","🚌 Passing stopped buses and yielding","🚌 Überholen und Vorfahrt am Halt"),
             logT("Ônibus IA podem tentar ultrapassar outros parados, com verificações de faixa e distância. Carros podem facilitar a saída do ônibus quando houver espaço seguro. São comportamentos experimentais; colisões e FPS precisam de testes em mapas diferentes.",
                  "AI buses can attempt to pass stopped buses with lane and gap checks, while cars may yield when buses leave stops safely. These are experimental and need collision and performance tests on different maps.",
                  "KI-Busse können stehende Busse mit Spur- und Abstandsprüfung überholen; Autos können beim Ausfahren Platz machen. Experimentell: Kollisionen und Leistung sind auf weiteren Karten zu testen.")},
            {logT("🚏 Baias e estacionamento irregular","🚏 Shared bays and illegal parking","🚏 Haltebuchten und Falschparken"),
             logT("A IA considera melhor o tamanho dos ônibus em paradas longas; o uso integral e simultâneo de todas as vagas ainda não está completo. Alguns carros podem parar irregularmente, ligar o pisca-alerta e reagir à buzina após um atraso.",
                  "AI buses account for vehicle length at extended bays, but full multi-bus bay management remains incomplete. Some cars can stop illegally, use hazard lights and respond to honks after a delay.",
                  "Die KI berücksichtigt Buslängen an langen Haltestellen; die vollständige Mehrfachbelegung ist noch nicht fertig. Manche Autos halten falsch, blinken und reagieren verzögert auf Hupen.")},
            {logT("🚶 Passageiros chegam ao ponto ao longo do tempo","🚶 Independent stop passenger demand","🚶 Unabhängige Fahrgastnachfrage"),
             logT("Passageiros podem chegar gradualmente, esperar por linhas compatíveis e desistir após uma espera longa, em vez de surgir uma nova multidão a cada ônibus. A opção é independente das configurações de lotação e demanda do mapa.",
                  "Passengers can arrive over time, wait for suitable lines and give up after long delays instead of an instant refill for each bus. This is an independent opt-in feature.",
                  "Fahrgäste können allmählich kommen, auf passende Linien warten und nach langer Zeit aufgeben, statt bei jedem Bus neu zu erscheinen. Die Funktion ist separat zuschaltbar.")},
            {logT("🧥 Roupas conforme o clima","🧥 Clothing adapted to the weather","🧥 Kleidung passend zum Wetter"),
             logT("Em chuva, neve, calor ou frio, personagens podem escolher variantes de roupas CTI existentes. Sem variantes adequadas, o modelo permanece igual; o recurso não cria novas roupas tridimensionais.",
                  "Characters may select existing CTI clothing variants to suit rain, snow, warm or cold weather. Models without compatible variants stay unchanged; no new 3D clothes are generated.",
                  "Figuren können vorhandene CTI-Kleidungsvarianten je nach Regen, Schnee oder Temperatur verwenden. Ohne passende Varianten bleibt das Modell unverändert.")},
            {logT("⚙️ Opções separadas, cliente e servidor P15","⚙️ Independent settings and matched P15","⚙️ Einzelne Schalter und P15-Version"),
             logT("As opções novas ficam desligadas por padrão e podem ser ligadas individualmente. O pacote traz cliente e servidor P15 da mesma base; para o multiplayer funcionar corretamente, todos devem usar versões compatíveis.",
                  "New options default to off and can be enabled separately. Client and P15 server belong to the same build line; multiplayer users must run compatible versions.",
                  "Neue Optionen sind standardmäßig aus und können einzeln aktiviert werden. Client und P15-Server müssen aus derselben kompatiblen Version stammen.")},
        },
    }
}

func engine0347LogSection() logSection {
 return logSection{
  label:logT("🚌 2.2.2.4 • OPENOMSI 0.3.47","🚌 2.2.2.4 • OPENOMSI 0.3.47","🚌 2.2.2.4 • OPENOMSI 0.3.47"),accent:logAccent(85,190,255),
  entries:[]logEntry{
   {logT("Oito áudios em Português (Brasil)","Eight Brazilian Portuguese recordings","Acht brasilianisch-portugiesische Aufnahmen"),
    logT("Incluídas as oito gravações fornecidas: três pedidos de porta, duas reclamações de ponto perdido e três agradecimentos/despedidas. Em PT-BR, o passageiro pronto para descer reclama uma vez ao passar do ponto. Com a frente aberta e a saída fechada, pede após dois segundos; quem espera junto à entrada também pede se houver vaga. Áudio normalizado e pedidos com alcance ampliado no ônibus. A primeira migração PT-BR ativa as falas; o silêncio de passageiros e escolhas posteriores continuam respeitados.","Includes the eight supplied recordings: three door requests, two missed-stop complaints and three thanks/farewells. In PT-BR, a rider ready to alight complains once after passing the stop. A closed exit with an open entrance prompts a request after two seconds; a waiting rider by the entrance asks only when capacity is available. Audio is normalized and requests have cabin-wide reach. First PT-BR migration enables these reactions; passenger mute and later choices are preserved.","Enthält acht bereitgestellte Aufnahmen: drei Türbitten, zwei Beschwerden über verpasste Haltestellen und drei Dankes-/Abschiedsaufnahmen. Bei PT-BR beschwert sich ein aussteigebereiter Fahrgast einmal nach der verpassten Haltestelle. Offener Einstieg und geschlossener Ausstieg lösen nach zwei Sekunden eine Bitte aus; Einsteigende bitten nur bei freier Kapazität. Normalisierte Lautstärke und größere Reichweite im Bus. Erste PT-BR-Migration aktiviert die Reaktionen; Stummschaltung und spätere Entscheidungen bleiben erhalten.")},
   {logT("Motor e lobby realmente na 0.3.47","Game engine and lobby upgraded to 0.3.47","Spiel und Lobby auf 0.3.47 aktualisiert"),
    logT("A 2.2.2.3 trouxe as três rotinas de passageiros, mas ainda usava a base 0.3.46. Esta versão integra a base completa da 0.3.47 e recompila jogo, launcher e servidor com a mesma origem. O lobby recebe a versão real da API do launcher.","2.2.2.3 backported the three passenger routines while retaining the 0.3.46 base. This release integrates the full 0.3.47 base and rebuilds game, launcher and server from the same source. The lobby receives the actual launcher API version.","2.2.2.3 übernahm die drei Fahrgastroutinen auf Basis 0.3.46. Jetzt wird die vollständige Basis 0.3.47 integriert; Spiel, Launcher und Server stammen aus derselben Quelle. Die Lobby erhält die tatsächliche Version der Launcher-API.")},
   {logT("Servidor compatível P16","Matching P16 server","Passender P16-Server"),
    logT("O jogo 0.3.47 utiliza protocolo 16. Um host P16 separado acompanha a atualização; os executáveis completos dos hosts antigos e os servidores P14/P15 são preservados. Preferências privadas são copiadas para o host P16 sem sobrescrever o arquivo antigo. Todos os jogadores da mesma sessão precisam usar a versão compatível.","0.3.47 uses protocol 16. A separate P16 host is included; complete legacy host executables and P14/P15 servers are retained. Private preferences are copied to the P16 host without overwriting old files. Players in one session need matching versions.","0.3.47 nutzt Protokoll 16. Ein separater P16-Host wird ergänzt; bisherige vollständige Hosts und P14/P15-Server bleiben erhalten. Private Einstellungen werden in separate P16-Dateien kopiert. Alle Spieler einer Sitzung benötigen kompatible Versionen.")},
   {logT("Correções anteriores preservadas","Previous fixes retained","Bisherige Korrekturen bleiben erhalten"),
    logT("Continuam as três opções de passageiros ativadas na primeira migração, as conexões cooperativas, as correções de troca de motorista, câmera, skins e áudio, e a remoção das roupas extras. O atualizador x64 mantém restauração em disco e verificação completa da instalação. A validação visual no jogo e a sessão real com dois PCs ainda dependem de teste.","Retains first-migration passenger defaults, cooperative connections, driver transfer, camera, skins and audio fixes, and removal of extra clothes. The x64 updater keeps disk rollback and complete installation verification. Visual gameplay and a real two-PC session still require testing.","Erhält aktive Fahrgastoptionen bei erster Migration, Koop-Anschlüsse, Fahrerwechsel-, Kamera-, Skin- und Audiokorrekturen sowie die Entfernung zusätzlicher Kleidung. Der x64-Updater behält Datenträgersicherung und vollständige Prüfung. Sichtprüfung im Spiel und echte Zwei-PC-Sitzung stehen aus.")},
  },
 }
}

func passengers0347LogSection() logSection {
    return logSection{
        label:logT("🚌 2.2.2.3 • VERSÃO FINAL","🚌 2.2.2.3 • FINAL","🚌 2.2.2.3 • FINAL"),accent:logAccent(85,190,255),
        entries:[]logEntry{
            {logT("Roupas de frio removidas","Winter clothes removed","Winterkleidung entfernt"),
             logT("Removido o desenho das roupas extras que deformavam os passageiros. A opção antiga é ignorada, mesmo se estava ativada.","Removed the additional clothes that deformed passengers. The old switch is ignored even when saved on.","Die zusätzlichen, verformten Kleidungsmodelle wurden entfernt. Die alte Einstellung wird auch bei gespeichertem Ein ignoriert.")},
            {logT("Passageiros da 0.3.47","Passengers from 0.3.47","Fahrgäste aus 0.3.47"),
             logT("Usamos as rotinas originais dos PRs #3427, #3428 e #3429: sinalização com braço e olhar, reação visual a ônibus cheio e conexões entre linhas com caminhada verificada. As três opções e as animações naturais são ativadas na primeira migração; depois, sua escolha é preservada. A chance de conexão continua em 20%.","Uses original PRs #3427, #3428 and #3429: arm/gaze hailing, full-bus visual reactions and onward connections with checked walking ground. First migration enables all three and Natural animation; subsequent choices are preserved. Connection chance remains 20%.","Originalroutinen aus PR #3427, #3428 und #3429: Winken/Blick, Reaktion auf volle Busse und geprüfte Fußwege zum Anschluss. Die erste Migration aktiviert alle drei und natürliche Animationen; spätere Einstellungen bleiben erhalten. Anschlusschance: 20%." )},
            {logT("Conexões no cooperativo","Cooperative connections","Anschlüsse im Koop"),
             logT("A lógica original de escolha e caminhada é integrada à autoridade do multiplayer. O cliente mantém o passageiro até a confirmação do host; o histórico acompanha a transferência para manter o limite de duas conexões e excluir o ônibus anterior. Requer cliente e servidor desta versão. A viagem real com dois PCs ainda precisa ser validada.","Original selection/walking logic is integrated with multiplayer ownership. The client retains the passenger until the host acknowledges handback. Transfer history preserves the two-connection lifetime limit and excludes the previous bus. Requires matching client/server. Real paired gameplay remains unverified.","Originale Auswahl und Fußwege nutzen die Multiplayer-Zuständigkeit. Der Client behält den Fahrgast bis zur Host-Bestätigung. Der Verlauf erhält die Grenze von zwei Anschlüssen und schließt den vorherigen Bus aus. Gleiche Client-/Serverversion erforderlich. Ein realer Test mit zwei PCs steht aus.")},
            {logT("Atualizador: instalação completa","Updater: complete installation","Updater: vollständige Installation"),
             logT("O instalador passa a ser x64 e guarda a cópia de restauração em disco, reduzindo o uso de memória ao substituir arquivos grandes. A janela permanece aberta durante a instalação e verificação. O teste substitui todo um runtime anterior; falhas fatais passam a ser registradas no log do Setup. A correção no seu computador ainda precisa ser confirmada.","The installer is now x64 and keeps rollback files on disk, reducing memory when replacing large files. Progress remains visible during install and verification. The test replaces an entire old runtime; fatal errors are captured in Setup logs. Confirmation on your computer is still needed.","Der Installer nutzt x64 und legt Sicherungen auf Datenträger ab, um beim Austausch großer Dateien Speicher zu sparen. Fortschritt bleibt bei Installation und Prüfung sichtbar. Der Test ersetzt eine komplette alte Laufzeit; fatale Fehler landen im Setup-Protokoll. Bestätigung auf Ihrem PC steht aus.")},
        },
    }
}

func hotfix2224VoiceIntegrityLogSection() logSection {
    return logSection{
        label: logT("🔊 2.2.2.4 HOTFIX 2 • ÁUDIO V2", "🔊 2.2.2.4 HOTFIX 2 • V2 VOICES", "🔊 2.2.2.4 HOTFIX 2 • V2-STIMMEN"),
        accent: logAccent(255,205,75),
        entries: []logEntry{
            {logT("🙋 Passageiros finalmente sinalizam", "🙋 Passengers can now hail approaching buses", "🙋 Fahrgäste zeigen Haltezeichen"),
             logT("Corrigimos dois bloqueios no motor: os gestos estavam limitados às animações Enhanced e só começavam a 60 metros, quando o passageiro já mudava de tarefa para embarcar. Agora a sinalização também funciona com animações originais, começa de 60 a 90 metros da parada e respeita linhas, lotação e preferências. Passageiros de outro jogador aceitam rota correspondente mesmo sem texto do destino pronto. O registro PASSENGER_HAIL_AUDIT ajuda a verificar as condições.",
                  "Two native blockers fixed: signaling was disabled in original animation mode, and the visual request began only at 60 m, just as boarding began. Both original and enhanced human poses now support arm raising 60-90 m before the stop, keeping line, capacity, and player settings intact. Host-matched routes work before the destination text is ready; PASSENGER_HAIL_AUDIT helps diagnose eligibility.",
                  "Zwei Fehler korrigiert: Original-Animationen unterdrückten das Armzeichen; die Anfrage begann erst bei 60 m, als Fahrgäste schon zum Einstieg aufbrachen. Nun funktionieren Original und Enhanced bereits 60-90 m vor der Haltestelle. Route, Kapazität und Einstellungen bleiben unverändert. PASSENGER_HAIL_AUDIT zeigt die Bedingungen.")},
            {logT("✅ Salvar e ativar não rejeita gravações V2", "✅ Save and activate accepts approved V2 voices", "✅ Aktivieren akzeptiert geprüfte V2-Stimmen"),
             logT("O erro SHA256 mismatch em ExitThanks_1.wav e nas outras gravações brasileiras V2 foi corrigido. O Setup aceita exclusivamente os oito arquivos conhecidos por nome, pasta e SHA-256, sem reduzir a segurança dos demais arquivos. A atualização não altera o volume das suas vozes.",
                  "Fixes SHA256 mismatch for ExitThanks_1.wav and the seven other approved Brazilian V2 recordings. Only those exact files and hashes are accepted; other corrupt assets remain rejected. Your voice volume is unchanged.",
                  "Behebt SHA256 mismatch bei ExitThanks_1.wav und den sieben anderen geprüften V2-Aufnahmen. Nur exakte Dateinamen und SHA-256-Werte werden akzeptiert. Die Lautstärke bleibt unverändert.")},
            {logT("🗄️ Backup e restauração respeitam os áudios", "🗄️ Backups preserve V2 voice audio", "🗄️ Sicherungen erhalten V2-Audio"),
             logT("O pacote Cooperativo, a verificação de integridade, o backup e a restauração reconhecem os WAVs V2 já instalados e não exigem baixar novamente o pacote inteiro.",
                  "Cooperative inventory, checksum checks, backup and restore recognize the approved V2 audio without a needless full redownload.",
                  "Koop-Paketprüfung, Sicherung und Wiederherstellung erkennen die freigegebenen V2-Dateien ohne erneuten Komplett-Download.")},
            {logT("🚌 Mantidas as melhorias anteriores", "🚌 Previous Hotfix 1 improvements retained", "🚌 Vorherige Hotfix-1-Änderungen bleiben"),
             logT("Permanece a correção do servidor P16, sinalização dos passageiros, troca de motorista, câmeras do cobrador, resposta da IA à buzina e opções adicionais ativadas por padrão (respeitando escolhas individuais).",
                  "Retains P16, passenger hailing, conductor cameras, handover visibility, AI horn replies and supported enabled defaults, respecting manual opt-outs.",
                  "Erhält P16, Haltezeichen, Fahrerwechsel, Schaffnerkameras, KI-Hupantworten und aktivierte Standardoptionen; manuelle Einstellungen bleiben erhalten.")},
        },
    }
}

func logSections() []logSection {
    if shownLogSections!=nil{return shownLogSections}
    return selectLogSections("")
}

// Show newest unseen releases first. The saved marker is the last changelog
// actually acknowledged, not merely the last executable installed. In
// particular 2.2.2.1 never opened its tour, so users with a Hotfix 3 marker
// must see BOTH the original 2.2.2.1 features and Hotfix 1 corrections.
func selectLegacyLogSections(from string) []logSection {
    from=strings.TrimSpace(from)
    if from==changelogSeenVersion { return []logSection{allLogSections()[0]} } // Manually opened tour
    // Hotfix 3 has its own chapter, shown even to someone who has already
    // acknowledged Hotfix 2. Earlier users get cumulative history.
    hotfix3 := allLogSections()[0]
    if from=="2.2.2.4-hotfix.2" { return []logSection{hotfix3} }
    if from=="2.2.2.4-hotfix.1" || from=="2.2.2.4" {
        return []logSection{hotfix3,hotfix2224VoiceIntegrityLogSection()}
    }
    result:=[]logSection{hotfix3,hotfix2224VoiceIntegrityLogSection(),engine0347LogSection()}
    if from=="2.2.2.3"{return result}
    result=append(result,passengers0347LogSection())
    // The unpublished 2.2.2.3 test candidate already included these passenger changes.
    // Its cumulative update should show the final release chapter only.
    if from=="2.2.2.2-test.2" || from=="2.2.2.3-test.1"{return result}
    result=append(result,winterTest2LogSection())
    if from=="2.2.2.2-test.1"{return result}
    result=append(result,winterTest1LogSection())
    if from=="2.2.2.2-hotfix.1"{return result}
    result=append(result,winterSafetyHotfixLogSection())
    if from=="2.2.2.2"{return result}
    result=append(result,release2222LogSection())
    if from=="2.2.2.1-hotfix.2"{return result}
    result=append(result,hotfix2221SecondLogSection())
    if from=="2.2.2.1-hotfix.1"{return result}
    result=append(result,hotfix2221LogSection())
    if from=="2.2.2.1"{return result}
    result=append(result,release2221LogSection())
    if from=="2.2.2-hotfix.3"{return result}
    result=append(result,hotfix3LogSection())
    if from=="2.2.2-hotfix.2"{return result}
    result=append(result,hotfix2LogSection())
    if from=="2.2.2-hotfix.1"{return result}
    old:=allLogSections()
    if from=="2.2.2"{return result}
    if strings.HasPrefix(from,"2.2.1.9"){return append(result,old[1:]...)}
    if strings.HasPrefix(from,"2.2.1"){
        earlier:=earlierBridgeLogSection()
        earlier.entries=earlier.entries[2:]
        return append(append(result,earlier),old[1:]...)
    }
    if strings.HasPrefix(from,"2.2.0"){
        earlier:=earlierBridgeLogSection()
        earlier.entries=earlier.entries[1:]
        return append(append(result,earlier),old[1:]...)
    }
    return append(append(result,earlierBridgeLogSection()),old[1:]...)
}

func logFill(dc uintptr,r guiRect,color uintptr){
    brush,_,_:=guiGDI.NewProc("CreateSolidBrush").Call(color)
    guiUser.NewProc("FillRect").Call(dc,uintptr(unsafe.Pointer(&r)),brush)
    guiGDI.NewProc("DeleteObject").Call(brush)
}
func logTextDraw(dc uintptr,content string,r guiRect,font,color,flags uintptr) int32 {
    old,_,_:=guiGDI.NewProc("SelectObject").Call(dc,font)
    guiGDI.NewProc("SetBkMode").Call(dc,1)
    guiGDI.NewProc("SetTextColor").Call(dc,color)
    height,_,_:=guiUser.NewProc("DrawTextW").Call(dc,uintptr(unsafe.Pointer(winPtr(content))),^uintptr(0),uintptr(unsafe.Pointer(&r)),flags|0x0800)
    guiGDI.NewProc("SelectObject").Call(dc,old)
    return int32(height)
}
func logMeasure(dc uintptr,content string,r guiRect,font uintptr)int32{
    old,_,_:=guiGDI.NewProc("SelectObject").Call(dc,font)
    // DT_CALCRECT|DT_WORDBREAK|DT_NOPREFIX
    guiUser.NewProc("DrawTextW").Call(dc,uintptr(unsafe.Pointer(winPtr(content))),^uintptr(0),uintptr(unsafe.Pointer(&r)),0x0400|0x0010|0x0800)
    guiGDI.NewProc("SelectObject").Call(dc,old)
    return r.Bottom-r.Top
}
func logWindowPaint(window uintptr){
    state:=changelogState
    if state==nil{return}
    g:=activeSetupGUI
    if g==nil{return}
    var ps logPaintStruct
    dc,_,_:=guiUser.NewProc("BeginPaint").Call(window,uintptr(unsafe.Pointer(&ps)))
    if dc==0{return}
    defer guiUser.NewProc("EndPaint").Call(window,uintptr(unsafe.Pointer(&ps)))
    var client guiRect
    guiUser.NewProc("GetClientRect").Call(window,uintptr(unsafe.Pointer(&client)))
    w,h:=client.Right,client.Bottom
    if w<500||h<400{return}
    navy:=guiRGB(13,21,35)
    white:=guiRGB(239,246,255)
    muted:=guiRGB(160,179,201)
    logFill(dc,client,navy)
    logFill(dc,guiRect{0,0,w,113},guiRGB(21,37,61))
    logFill(dc,guiRect{0,109,w,113},guiRGB(255,202,84))
    logTextDraw(dc,"OpenOMSI  BCS BRIDGE",guiRect{26,15,w-22,44},state.heading,white,0x0000|0x0020)
    logTextDraw(dc,logL(g.config.Language,logT("✨ ATUALIZAÇÃO "+changelogSeenVersion,"✨ UPDATE "+changelogSeenVersion,"✨ AKTUALISIERUNG "+changelogSeenVersion)),guiRect{26,51,w-20,81},state.subhead,guiRGB(255,211,107),0x0020)
    logTextDraw(dc,logL(g.config.Language,logT("Escolha um assunto à esquerda. Role para ler todas as mudanças.","Choose a category on the left. Scroll to read the complete notes.","Links ein Thema wählen und für alle Änderungen scrollen.")),guiRect{26,83,w-24,107},state.tiny,muted,0x0020)
    logFill(dc,guiRect{0,114,268,h-76},guiRGB(17,29,47))
    logFill(dc,guiRect{0,h-77,w,h},guiRGB(21,37,61))
    logFill(dc,guiRect{0,h-77,w,h-74},guiRGB(43,63,89))
    logTextDraw(dc,logL(g.config.Language,logT("📌  Algumas mudanças ainda precisam de testes em viagens reais.","📌  Some changes still need testing in real journeys.","📌  Einige Änderungen benötigen noch Tests im Spiel.")),guiRect{26,h-65,w-245,h-28},state.tiny,muted,0x0010)
    sections:=logSections()
    if state.section<0||state.section>=len(sections){state.section=0}
    selected:=sections[state.section]
    x0,x1:=int32(292),w-28
    top,bottom:=int32(133),h-96
    logFill(dc,guiRect{x0-9,top-10,x1+9,bottom+10},guiRGB(24,36,55))
    save,_,_:=guiGDI.NewProc("SaveDC").Call(dc)
    guiGDI.NewProc("IntersectClipRect").Call(dc,uintptr(x0),uintptr(top),uintptr(x1),uintptr(bottom))
    y:=top+8-int32(state.scroll)
    logFill(dc,guiRect{x0,y,x0+5,y+42},selected.accent)
    logTextDraw(dc,logL(g.config.Language,selected.label),guiRect{x0+16,y,x1-18,y+37},state.heading,white,0x0020)
    y+=52
    for _,item:=range selected.entries{
        head:=logL(g.config.Language,item.title)
        body:=logL(g.config.Language,item.body)
        bodyRect:=guiRect{x0+24,y+52,x1-26,y+52}
        bodyHeight:=logMeasure(dc,body,guiRect{Left:bodyRect.Left,Top:bodyRect.Top,Right:bodyRect.Right,Bottom:bodyRect.Top+500},state.body)
        cardHeight:=int32(71)+bodyHeight
        logFill(dc,guiRect{x0+2,y,x1-9,y+cardHeight},guiRGB(31,46,68))
        logFill(dc,guiRect{x0+2,y,x0+7,y+cardHeight},selected.accent)
        logTextDraw(dc,head,guiRect{x0+22,y+12,x1-20,y+47},state.subhead,guiRGB(232,245,255),0x0010)
        logTextDraw(dc,body,guiRect{x0+24,y+51,x1-28,y+52+bodyHeight},state.body,guiRGB(186,204,224),0x0010)
        y+=cardHeight+14
    }
    y+=12
    guiGDI.NewProc("RestoreDC").Call(dc,save)
    fullHeight:=int(y-top)+state.scroll
    windowHeight:=int(bottom-top)
    state.maxScroll=fullHeight-windowHeight
    if state.maxScroll<0{state.maxScroll=0}
    if state.scroll>state.maxScroll{state.scroll=state.maxScroll}
    if state.maxScroll>0{
        track:=guiRect{x1-5,top+8,x1-1,bottom-8}
        logFill(dc,track,guiRGB(52,68,88))
        railHeight:=int(track.Bottom-track.Top)
        thumbHeight:=railHeight*windowHeight/fullHeight
        if thumbHeight<36{thumbHeight=36}
        thumbTop:=int(track.Top)+state.scroll*(railHeight-thumbHeight)/state.maxScroll
        logFill(dc,guiRect{x1-5,int32(thumbTop),x1-1,int32(thumbTop+thumbHeight)},selected.accent)
    }
}
func changelogWindowProc(window uintptr,message uint32,wparam,lparam uintptr)uintptr{
    g:=activeSetupGUI
    if g==nil{result,_,_:=guiUser.NewProc("DefWindowProcW").Call(window,uintptr(message),wparam,lparam);return result}
    switch message{
    case logWMPaint:
        logWindowPaint(window)
        return 0
    case logWMMouseWheel:
        if changelogState!=nil {
            ticks:=int(int16(uint16(wparam>>16)))
            changelogState.scroll-=ticks*95/120
            if changelogState.scroll<0{changelogState.scroll=0}
            if changelogState.scroll>changelogState.maxScroll{changelogState.scroll=changelogState.maxScroll}
            guiUser.NewProc("InvalidateRect").Call(window,0,0)
        }
        return 0
    case logWMKeyDown:
        if changelogState!=nil {
            switch wparam {
            case 0x26:changelogState.scroll-=64 // Up
            case 0x28:changelogState.scroll+=64 // Down
            case 0x21:changelogState.scroll-=460 // PageUp
            case 0x22:changelogState.scroll+=460 // PageDown
            case 0x24:changelogState.scroll=0   // Home
            case 0x23:changelogState.scroll=changelogState.maxScroll // End
            default:break
            }
            if changelogState.scroll<0{changelogState.scroll=0}
            if changelogState.scroll>changelogState.maxScroll{changelogState.scroll=changelogState.maxScroll}
            guiUser.NewProc("InvalidateRect").Call(window,0,0)
        }
        return 0
    case logWMSize:
        guiUser.NewProc("InvalidateRect").Call(window,0,0)
        return 0
    case guiWMCommand:
        id:=int(wparam&0xffff)
        if id>=logIDSectionBase && id<logIDSectionBase+len(logSections()){
            if changelogState!=nil{
                changelogState.section=id-logIDSectionBase
                changelogState.scroll=0
                for _,button:=range changelogState.buttons{guiUser.NewProc("InvalidateRect").Call(button,0,1)}
                guiUser.NewProc("InvalidateRect").Call(window,0,0)
            }
            return 0
        }
        if id!=logIDDone{break}
        fallthrough
    case guiWMClose:
        if err:=os.MkdirAll(appDir(g.dir),0755);err==nil&&!g.smoke{
            _=os.WriteFile(filepath.Join(appDir(g.dir),"changelog-seen.txt"),[]byte(changelogSeenVersion+"\n"),0644)
            _=os.Remove(filepath.Join(appDir(g.dir),"changelog-from.txt"))
        }
        if changelogState!=nil {
            for _,font:=range []uintptr{changelogState.heading,changelogState.body,changelogState.subhead,changelogState.tiny}{
                guiGDI.NewProc("DeleteObject").Call(font)
            }
            changelogState=nil
            shownLogSections=nil
        }
        guiUser.NewProc("DestroyWindow").Call(window)
        changelogWindow=0
        guiUser.NewProc("EnableWindow").Call(g.window,1)
        guiUser.NewProc("SetForegroundWindow").Call(g.window)
        return 0
    case guiWMDrawItem:
        if changelogState==nil{break}
        item:=(*guiDrawItem)(unsafe.Pointer(lparam))
        if item==nil{break}
        dc:=item.DC
        chosen:=int(item.ID)-logIDSectionBase==changelogState.section
        bgcolor:=guiRGB(27,43,63)
        fgcolor:=guiRGB(186,203,225)
        if chosen{bgcolor=guiRGB(48,71,95);fgcolor=guiRGB(255,244,204)}
        if int(item.ID)==logIDDone{bgcolor=guiRGB(255,206,86);fgcolor=guiRGB(22,31,45)}
        logFill(dc,item.Rect,bgcolor)
        if chosen&&int(item.ID)!=logIDDone {
            logFill(dc,guiRect{item.Rect.Left,item.Rect.Top,item.Rect.Left+5,item.Rect.Bottom},logSections()[changelogState.section].accent)
        }
        logTextDraw(dc,guiText(item.Window),guiRect{item.Rect.Left+12,item.Rect.Top,item.Rect.Right-8,item.Rect.Bottom},g.font,fgcolor,0x0000|0x0004|0x0020)
        return 1
    }
    result,_,_:=guiUser.NewProc("DefWindowProcW").Call(window,uintptr(message),wparam,lparam)
    return result
}
func (g *setupGUI) showReleaseChangelog(force bool) uintptr {
    if changelogWindow!=0{return changelogWindow}
    if !force{
        if bridgeVersion!=changelogSeenVersion{return 0}
        seen,_:=os.ReadFile(filepath.Join(appDir(g.dir),"changelog-seen.txt"))
        if strings.TrimSpace(string(seen))==changelogSeenVersion{return 0}
    }
    from, _ := os.ReadFile(filepath.Join(appDir(g.dir),"changelog-from.txt"))
    if len(from)==0 {
        // Older updaters did not create changelog-from.txt: use the marker
        // left by the last release window if available.
        from,_=os.ReadFile(filepath.Join(appDir(g.dir),"changelog-seen.txt"))
    }
    shownLogSections=selectLogSections(string(from))
    instance,_,_:=setupKernel.NewProc("GetModuleHandleW").Call(0)
    cursor,_,_:=guiUser.NewProc("LoadCursorW").Call(0,32512)
    class:=guiWindowClass{Procedure:changelogProcedure,Instance:instance,Icon:g.icon,SmallIcon:g.smallIcon,Cursor:cursor,Background:g.background,Name:winPtr("OpenOMSIReleaseNotesTour")}
    class.Size=uint32(unsafe.Sizeof(class))
    guiUser.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&class)))
    style:=uintptr(0x00C00000|0x00080000|0x02000000)
    rect:=guiRect{Right:1080,Bottom:780}
    guiUser.NewProc("AdjustWindowRectEx").Call(uintptr(unsafe.Pointer(&rect)),style,0,0)
    var parent guiRect
    guiUser.NewProc("GetWindowRect").Call(g.window,uintptr(unsafe.Pointer(&parent)))
    width,height:=int(rect.Right-rect.Left),int(rect.Bottom-rect.Top)
    x,y:=int(parent.Left)+(int(parent.Right-parent.Left)-width)/2,int(parent.Top)
    if x<0{x=0};if y<0{y=0}
    window,_,_:=guiUser.NewProc("CreateWindowExW").Call(0,uintptr(unsafe.Pointer(class.Name)),uintptr(unsafe.Pointer(winPtr("OpenOMSI BCS Bridge  •  "+changelogSeenVersion))),style,uintptr(x),uintptr(y),uintptr(width),uintptr(height),g.window,0,instance,0)
    if window==0{return 0}
    // Windows may constrain the requested window to the display work area.
    // Position child controls using the resulting client area, as painting does.
    var client guiRect
    guiUser.NewProc("GetClientRect").Call(window,uintptr(unsafe.Pointer(&client)))
    clientWidth,clientHeight:=int(client.Right),int(client.Bottom)
    changelogWindow=window
    changelogState=&logWindowState{heading:guiFont(26,700),subhead:guiFont(19,700),body:guiFont(18,400),tiny:guiFont(16,400)}
    control:=func(label string,x,y,w,h,id int)uintptr{
        hwnd,_,_:=guiUser.NewProc("CreateWindowExW").Call(0,uintptr(unsafe.Pointer(winPtr("BUTTON"))),uintptr(unsafe.Pointer(winPtr(label))),guiWSChild|guiWSVisible|guiWSTabStop|0x000B,uintptr(x),uintptr(y),uintptr(w),uintptr(h),window,uintptr(id),instance,0)
        guiUser.NewProc("SendMessageW").Call(hwnd,guiWMSetFont,g.font,1)
        return hwnd
    }
    // Fit every cumulative release section above the footer, including the
    // complete history shown to users upgrading from an old installation.
    // Keep ALL section buttons above the footer at the actual client height.
    tabTop,tabStep,tabHeight:=152,65,54
    if len(logSections())>8{tabTop,tabStep,tabHeight=141,48,42}
    if len(logSections())>11{tabTop,tabStep,tabHeight=130,33,30}
    if len(logSections())>15{tabTop,tabStep,tabHeight=118,26,24}
    if count:=len(logSections());count>0{
        if step:=(clientHeight-90-tabTop)/count;step<tabStep{
            tabStep=step
            tabHeight=tabStep-2
        }
    }
    for i,s:=range logSections(){
        hwnd:=control(logL(g.config.Language,s.label),17,tabTop+i*tabStep,232,tabHeight,logIDSectionBase+i)
        changelogState.buttons=append(changelogState.buttons,hwnd)
    }
    done:=control(logL(g.config.Language,logT("✓ VAMOS JOGAR","✓ LET'S PLAY","✓ LOS GEHT'S")),clientWidth-270,clientHeight-64,230,44,logIDDone)
    guiUser.NewProc("EnableWindow").Call(g.window,0)
    guiUser.NewProc("ShowWindow").Call(window,1)
    guiUser.NewProc("UpdateWindow").Call(window)
    guiUser.NewProc("SetFocus").Call(done)
    return window
}

// Every release, including both preceding hotfixes, has its own history boundary.
func selectPreviousLogSections(from string) []logSection {
    from=strings.TrimSpace(from)
    result:=[]logSection{passenger223Hotfix2LogSection()}
    if from=="2.2.3-hotfix.2" || from=="2.2.3-hotfix.1" {return result}
    result=append(result,accessibility223Hotfix1LogSection())
    if from=="2.2.3" {return result}
    result=append(result,accessibility223LogSection())
    if from=="2.2.2.4-hotfix.5" {return result}
    result=append(result,boardingHotfix5LogSection())
    if from=="2.2.2.4-hotfix.4" {return result}
    result=append(result,routeHotfix4LogSection())
    if from=="2.2.2.4-hotfix.3" {return result}
    return append(result,selectLegacyLogSections(from)...)
}
func accessibility223Hotfix1LogSection() logSection {
    return logSection{
        label:logT("♿ 2.2.3 HOTFIX 1","♿ 2.2.3 HOTFIX 1","♿ 2.2.3 HOTFIX 1"),
        accent:logAccent(112,219,164),
        entries:[]logEntry{
            {logT("Desembarque disponível no Alt","Alighting available in Alt","Ausstieg im Alt-Menü verfügbar"),
             logT("O pedido de desembarque permanece disponível até a assistência terminar. Corrige o ciclo de ‘cadeira fixada’ que fazia a opção desaparecer. Pare no ponto, abra a porta acessível e use a assistência do menu Alt.",
                  "The alighting request remains available until assistance completes. Fixes the secured-chair loop that hid the action. Stop at the bus stop, open the accessible door and use assistance in the Alt menu.",
                  "Der Ausstiegswunsch bleibt bis zum Ende der Unterstützung verfügbar. Behebt die Schleife des gesicherten Rollstuhls, durch die die Option verschwand. Anhalten, Zugangstür öffnen und die Unterstützung im Alt-Menü verwenden.")},
            {logT("Prioridade com a área ocupada","Priority with an occupied bay","Vorrang bei belegtem Stellplatz"),
             logT("Os passageiros tentam ceder espaço e, após uma espera curta, a cadeira recebe prioridade de passagem mesmo sem outro lugar em pé disponível. O banco retrátil acompanha o atendimento. Ônibus parado e porta aberta continuam obrigatórios.",
                  "Passengers try to yield; after a short wait the chair gains passage priority even when no other standing place is free. The folding seat follows assistance. The bus must remain stopped with the door open.",
                  "Fahrgäste versuchen Platz zu machen; nach kurzer Wartezeit erhält der Rollstuhl auch ohne freien anderen Stehplatz Vorrang. Der Klappsitz folgt der Unterstützung. Stillstand und geöffnete Tür bleiben erforderlich.")},
            {logT("Localizador universal e nove ônibus","Universal discovery and nine buses","Universelle Erkennung und neun Busse"),
             logT("O acesso assistido usa a porta detectada e o piso da cabine; a regra também atende estruturas semelhantes. Os nove modelos enviados completaram o ciclo automatizado: Scania articulado, Mi4 Voith/ZF, Torino 2007 VW T2 BH, Torino 2025, Torino S Rio Ita, VIP I de 3/4 portas e VIP III Volkswagen.",
                  "Assisted access uses the detected door and cabin floor; the same rule handles similar layouts. All nine supplied models completed automated boarding, alighting and stowing: articulated Scania, Mi4 Voith/ZF, Torino 2007 VW T2 BH, Torino 2025, Torino S Rio Ita, three/four-door VIP I and VIP III Volkswagen.",
                  "Der unterstützte Zugang nutzt die erkannte Tür und den Kabinenboden; die Regel gilt auch für ähnliche Strukturen. Alle neun eingesandten Modelle haben den automatisierten Zyklus abgeschlossen: Scania-Gelenkbus, Mi4 Voith/ZF, Torino 2007 VW T2 BH, Torino 2025, Torino S Rio Ita, VIP I mit drei/vier Türen und VIP III Volkswagen.")},
            {logT("Sinalização, lobby e histórico","Hailing, lobby and history","Haltezeichen, Lobby und Verlauf"),
             logT("Inclui as correções de sinalização Original/Enhanced e de rota no multiplayer preparadas desde a 2.2.3. Jogo, servidor P16 e lobby usam a mesma revisão. Quem vem de versões anteriores recebe também os capítulos ainda não vistos. As 41 regressões de acessibilidade fazem parte dos 1.013 testes de simulação aprovados.",
                  "Includes Original/Enhanced passenger hailing and multiplayer route fixes prepared since 2.2.3. Game, P16 server and lobby share the same revision. Earlier installations also receive unseen release chapters. The 41 accessibility regressions are included in 1,013 passing simulation tests.",
                  "Enthält die seit 2.2.3 vorbereiteten Original-/Enhanced-Handzeichen- und Mehrspielerroutenkorrekturen. Spiel, P16-Server und Lobby nutzen dieselbe Revision. Frühere Installationen erhalten ungelesene Kapitel. Die 41 Barrierefreiheitsregressionen sind in 1.013 bestandenen Simulationstests enthalten.")},
        },
    }
}
func accessibility223LogSection() logSection {
    return logSection{
        label:logT("♿ 2.2.3 • CADEIRANTES","♿ 2.2.3 • WHEELCHAIR PASSENGERS","♿ 2.2.3 • ROLLSTUHLFAHRGÄSTE"),
        accent:logAccent(85,190,255),
        entries:[]logEntry{
            {logT("Cadeirantes no fluxo normal","Wheelchair passengers in normal demand","Rollstuhlfahrgäste im normalen Fahrgastaufkommen"),
             logT("Cadeirantes podem surgir nos pontos, aproximar-se de um ônibus compatível e dar sinal. A geração vem ativada com chance de 3% por passageiro novo, ajustável de 0% a 100%, e tem opção própria para desativar. O lobby desta versão foi recompilado para mostrar esses controles.",
                  "Wheelchair users may appear at stops, approach a compatible bus and hail it. Spawning defaults to 3% per new passenger, adjustable from 0% to 100%, with an independent off switch. This release rebuilds the actual lobby to show these controls.",
                  "Rollstuhlfahrgäste können an Haltestellen erscheinen, sich einem geeigneten Bus nähern und ein Haltezeichen geben. Die Chance beträgt standardmäßig 3% je neuem Fahrgast, ist von 0% bis 100% einstellbar und separat abschaltbar. Die neu gebaute Lobby enthält diese Einstellungen.")},
            {logT("Acesso e assistência no menu Alt","Access and assistance in the Alt menu","Zugang und Unterstützung im Alt-Menü"),
             logT("O localizador examina portas, equipamento, piso, espaço e manobras. Rampas originais esperam sua animação real; ônibus com acesso verificado sem equipamento animado recebem assistência pelo botão de cadeira no menu Alt. Um acesso sem passagem comprovada permanece indisponível. Os textos novos têm tradução nos 30 idiomas do jogo.",
                  "Discovery examines doors, equipment, floor, space and maneuvers. Original ramps wait for real animation feedback; verified accesses without animated equipment use the wheelchair button in the Alt menu. An unverified route remains unavailable. New controls and messages cover all 30 game languages.",
                  "Die Erkennung prüft Türen, Ausrüstung, Boden, Platz und Rangierbewegungen. Originalrampen warten auf echte Animationsrückmeldung; geprüfte Zugänge ohne animierte Ausrüstung nutzen die Rollstuhltaste im Alt-Menü. Ungeprüfte Wege bleiben gesperrt. Neue Bedienfelder und Meldungen sind in allen 30 Spielsprachen verfügbar.")},
            {logT("Vagas, desembarque e conexões","Bays, alighting and connections","Stellplätze, Ausstieg und Anschlüsse"),
             logT("Cada vaga verificada tem ocupação independente. Passageiros em pé podem liberar o corredor; bloqueios reais não são atravessados. O cadeirante usa o sinal específico quando disponível ou o sinal comum com aviso ao motorista. A mesma pessoa conserva a cadeira após descer e nas conexões, inclusive na troca de autoridade do multiplayer P16.",
                  "Verified bays have independent occupancy. Standing passengers may clear the aisle; real obstructions are never bypassed. Alighting uses a verified wheelchair signal or the ordinary signal with a driver message. The same passenger keeps the chair after alighting and through connections, including P16 authority handback.",
                  "Geprüfte Stellplätze sind unabhängig belegt. Stehende Fahrgäste können den Gang freigeben; echte Hindernisse werden nicht durchquert. Der Ausstieg nutzt ein geprüftes Rollstuhlsignal oder das normale Signal mit Fahrerhinweis. Der Fahrgast behält seinen Rollstuhl beim Ausstieg und bei Anschlüssen, auch bei der P16-Autoritätsübergabe.")},
            {logT("Base e validação","Base and validation","Basis und Validierung"),
             logT("Usa a base exata da 2.2.2.4 Hotfix 5 e preserva HostAgent, servidores, vozes e correções anteriores. Os testes automatizados e o relatório de ônibus acompanham o código. Compilação aprovada não confirma aparência, animações nem o ciclo real em cada ônibus: a validação visual em jogo continua indicada separadamente.",
                  "Uses the exact 2.2.2.4 Hotfix 5 base, retaining hosts, servers, voices and earlier fixes. Automated tests and the supplied-vehicle report accompany the source. A successful build does not establish visual or full in-game compatibility for every bus; gameplay validation is recorded separately.",
                  "Verwendet die genaue Basis von 2.2.2.4 Hotfix 5 und erhält Hosts, Server, Stimmen und frühere Korrekturen. Automatische Tests und der Fahrzeugbericht begleiten den Quelltext. Ein erfolgreicher Build bestätigt nicht die Darstellung oder vollständige Spielkompatibilität jedes Busses; Spieltests werden getrennt ausgewiesen.")},
        },
    }
}
func boardingHotfix5LogSection() logSection {
    return logSection{label:logT("2.2.2.4 HOTFIX 5 • EMBARQUE","2.2.2.4 HOTFIX 5 • BOARDING","2.2.2.4 HOTFIX 5 • EINSTIEG"),accent:logAccent(112,219,164),entries:[]logEntry{
        {logT("Pontos deslocados e diagnóstico","Offset stops and diagnostics","Versetzte Haltestellen und Diagnose"),logT("O embarque final reaproveita a aproximação segura de plataforma já autorizada. Mantém ônibus parado e alinhado, porta aberta, lado da calçada, altura, destino e lotação; PASSENGER_BOARDING_AUDIT registra aceites e recusas.","Final boarding reuses an already authorized safe curb-offset approach, retaining stationary-bus, alignment, open-door, platform-side, height, destination and capacity checks. PASSENGER_BOARDING_AUDIT records acceptance and refusal.","Der Einstieg nutzt die bereits geprüfte sichere Annäherung bei versetzter Plattform. Stillstand, Ausrichtung, offene Tür, Straßenseite, Höhe, Ziel und Belegung bleiben geprüft. PASSENGER_BOARDING_AUDIT protokolliert Annahme und Ablehnung.")},
    }}
}
func routeHotfix4LogSection() logSection {
    return logSection{label:logT("2.2.2.4 HOTFIX 4 • ROTA P16","2.2.2.4 HOTFIX 4 • P16 ROUTE","2.2.2.4 HOTFIX 4 • P16-ROUTE"),accent:logAccent(112,219,164),entries:[]logEntry{
        {logT("Rota e passageiros multiplayer","Multiplayer route and passengers","Mehrspielerroute und Fahrgäste"),logT("COOPROUTE1 deixa de ser enviado ao servidor dedicado sem veículo. PASSENGER_LAN_AUDIT compara geração autoritativa, quadros de rede, passageiros espelhados e embarques sem mudar a escala do BCS.","Navigator-only COOPROUTE1 no longer targets the headless server. PASSENGER_LAN_AUDIT compares authoritative demand, network frames, mirrored passengers and boarding without changing the BCS timetable.","Die reine Navigator-Meldung COOPROUTE1 wird nicht mehr an den Server ohne Fahrzeug gesendet. PASSENGER_LAN_AUDIT vergleicht autoritative Nachfrage, Netzwerkdaten, gespiegelte Fahrgäste und Einstiege ohne Änderung des BCS-Fahrplans.")},
    }}
}


func passenger223Hotfix2LogSection() logSection {
    return logSection{
        label:logT("🚌 2.2.3 HOTFIX 2","🚌 2.2.3 HOTFIX 2","🚌 2.2.3 HOTFIX 2"),
        accent:logAccent(85,190,255),
        entries:[]logEntry{
            {logT("Fila inicial dos pontos","Existing stop queues","Anfangswarteschlangen"),
             logT("Remove o corte fixo de 70% da fila inicial, que podia deixar pontos vazios por minutos. Ao carregar uma região, a demanda do mapa pode colocar até quatro pessoas que já aguardavam em cada ponto. Novas chegadas continuam graduais, sem encher novamente a fila quando um ônibus chega.",
                  "Removes the fixed 70% initial-queue reduction that could leave stops empty for minutes. A newly loaded stop may immediately show up to four existing waiters according to map demand. Later arrivals remain gradual and a bus arriving does not refill the queue.",
                  "Entfernt die pauschale Kürzung der Anfangswarteschlange um 70%. Neu geladene Haltestellen dürfen entsprechend dem Kartenbedarf sofort bis zu vier bereits wartende Personen anzeigen. Spätere Ankünfte bleiben schrittweise; ein ankommender Bus füllt die Schlange nicht neu.")},
            {logT("Embarque no multiplayer","Multiplayer boarding","Einstieg im Mehrspieler"),
             logT("Quando a identificação de linha do servidor não é reconhecida localmente, a viagem pública ativa pode comprovar que atende o ponto e depois o destino exato do passageiro. O host continua autorizando a transferência e o destino é preservado após o embarque ser autorizado.",
                  "When a host line key cannot be resolved locally, an active public duty may prove that it calls at this stop and subsequently at the passenger's exact destination. The host still authorizes ownership transfer and the destination is retained after grant.",
                  "Kann der Client einen Linienschlüssel des Hosts nicht auflösen, darf ein aktiver öffentlicher Fahrplan den aktuellen Halt und das anschließende genaue Fahrgastziel belegen. Der Host bestätigt weiterhin die Übergabe; das Ziel bleibt erhalten.")},
            {logT("Diagnóstico e compatibilidade","Diagnostics and compatibility","Diagnose und Kompatibilität"),
             logT("O log registra pedidos de embarque, linhas não reconhecidas, pontos registrados, destino e situação da viagem. A contagem de sinalização inclui passageiros recebidos do servidor. Mantém as correções do Alt, da área ocupada e do localizador da Hotfix 1. A suíte automatizada acompanha a entrada na cabine; a nova viagem interativa será confirmada pelo próximo log do jogador.",
                  "Logs record boarding claims, unresolved line keys, registered stops, destination and duty state. Hailing counts include mirrored passengers. Preserves Hotfix 1 Alt, occupied-bay and discovery fixes. Automated coverage follows cabin entry; the reported interactive trip awaits a new player capture.",
                  "Protokolle erfassen Einstiegsanfragen, unbekannte Linienschlüssel, registrierte Haltestellen, Ziel und Fahrplanstatus. Haltezeichenzähler enthalten gespiegelte Fahrgäste. Erhält die Alt-, Stellplatz- und Erkennungskorrekturen von Hotfix 1. Der automatische Test prüft den Kabineneinstieg; die gemeldete Spielreise benötigt einen neuen Mitschnitt.")},
        },
    }
}

func select2231LogSections(from string) []logSection {
 hotfix2 := logSection{
  label: logT6("2.2.3.1 HOTFIX 2 — INTEGRIDADE DA ATIVAÇÃO",
      "2.2.3.1 HOTFIX 2 — ACTIVATION INTEGRITY",
      "2.2.3.1 HOTFIX 2 — AKTIVIERUNGSINTEGRITÄT",
      "2.2.3.1 HOTFIX 2 — INTEGRIDAD DE ACTIVACIÓN",
      "2.2.3.1 HOTFIX 2 — INTÉGRITÉ DE L’ACTIVATION",
      "2.2.3.1 HOTFIX 2 — ПРОВЕРКА АКТИВАЦИИ"),
  accent: logAccent(112,219,164),
  entries: []logEntry{
   {logT6("Correção de SHA256 mismatch: Setup.exe", "Fix Setup.exe SHA-256 mismatch",
       "SHA-256-Konflikt bei Setup.exe behoben",
       "Corregido SHA256 mismatch: Setup.exe", "Correction du SHA-256 de Setup.exe",
       "Исправлено несовпадение SHA-256 Setup.exe"),
    logT6("A ativação agora usa os hashes seguros do pacote cumulativo P17, mesmo se a instalação ainda possuir docs/SHA256.txt antigo. Executáveis realmente alterados continuam bloqueados.",
       "Activation now verifies pinned P17 cumulative runtime hashes even when an older docs/SHA256.txt remains installed. Tampered binaries are still rejected.",
       "Die Aktivierung prüft jetzt die festgelegten P17-Laufzeit-Hashes, auch wenn eine veraltete docs/SHA256.txt vorhanden ist. Manipulierte Dateien werden weiter blockiert.",
       "La activación verifica los hashes auténticos del paquete P17 aunque quede un docs/SHA256.txt antiguo. Los ejecutables modificados siguen bloqueados.",
       "L’activation vérifie les empreintes P17 approuvées même si un ancien fichier docs/SHA256.txt est toujours présent. Les fichiers modifiés restent bloqués.",
       "Активация использует проверенные хеши P17, даже если остался старый docs/SHA256.txt. Подменённые файлы блокируются.")},
  },
 }
 if from == "2.2.3.1-hotfix.1" {return []logSection{hotfix2}}
 hotfix := logSection{
  label: logT6("2.2.3.1 HOTFIX 1 — SETUP E ATIVAÇÃO",
      "2.2.3.1 HOTFIX 1 — SETUP AND ACTIVATION",
      "2.2.3.1 HOTFIX 1 — SETUP UND AKTIVIERUNG",
      "2.2.3.1 HOTFIX 1 — INSTALACIÓN Y ACTIVACIÓN",
      "2.2.3.1 HOTFIX 1 — INSTALLATION ET ACTIVATION",
      "2.2.3.1 HOTFIX 1 — УСТАНОВКА И АКТИВАЦИЯ"),
  accent:logAccent(112,219,164),
  entries:[]logEntry{
   {logT6("Ativação sem bloqueio do Registro", "Activation without Registry blockage",
           "Aktivierung ohne Registry-Sperre",
           "Activación sin bloqueo del Registro", "Activation sans blocage du Registre", "Активация без блокировки реестра"),
    logT6("Quando o Windows recusar a gravação IFEO, o Setup usa a integração nativa do BCS e salva um backup das preferências. Sem alterar permissões do Windows.",
          "When Windows denies the IFEO write, Setup uses native BCS integration and backs up preferences. Windows permissions remain unchanged.",
          "Wenn Windows das Schreiben in IFEO verweigert, nutzt Setup die native BBS-Integration und sichert die Einstellungen.",
          "Si Windows deniega la escritura IFEO, Setup utiliza la integración nativa de BCS y guarda una copia de los ajustes. No cambia los permisos.",
          "Si Windows refuse l'écriture IFEO, Setup utilise l'intégration native de BBS et sauvegarde les préférences sans changer les permissions.",
          "Если Windows запрещает запись IFEO, мастер использует встроенную интеграцию BBS и сохраняет резервную копию настроек, не меняя права доступа.")},
   {logT6("Integridade e idiomas", "Package integrity and languages", "Paketintegrität und Sprachen",
          "Integridad e idiomas", "Intégrité et langues", "Целостность и языки"),
    logT6("Corrigida a verificação do pacote cumulativo 2.2.3.1. Interface em português, inglês, alemão, espanhol, francês e russo. OpenOMSI 0.3.49 e HostAgent P17 preservados.",
          "Fixed cumulative 2.2.3.1 package verification. Setup supports Portuguese, English, German, Spanish, French and Russian. OpenOMSI 0.3.49 and HostAgent P17 are preserved.",
          "Die Paketprüfung wurde korrigiert. Setup unterstützt Portugiesisch, Englisch, Deutsch, Spanisch, Französisch und Russisch. OpenOMSI 0.3.49 und HostAgent P17 bleiben erhalten.",
          "Corregida la validación del paquete 2.2.3.1. Setup ahora admite portugués, inglés, alemán, español, francés y ruso. Se conservan OpenOMSI 0.3.49 y HostAgent P17.",
          "Vérification du paquet 2.2.3.1 corrigée. Setup propose le portugais, l'anglais, l'allemand, l'espagnol, le français et le russe. OpenOMSI 0.3.49 et HostAgent P17 sont préservés.",
          "Исправлена проверка пакета 2.2.3.1. Доступны португальский, английский, немецкий, испанский, французский и русский языки. OpenOMSI 0.3.49 и HostAgent P17 сохранены.")},
  },
 }
 if from == "2.2.3.1" {return []logSection{hotfix2, hotfix}}
 newest := logSection{label:logT("2.2.3.1 • OPENOMSI 0.3.49", "2.2.3.1 • OPENOMSI 0.3.49", "2.2.3.1 • OPENOMSI 0.3.49"), accent:logAccent(112,219,164), entries:[]logEntry{
 {logT("OpenOMSI 0.3.49 e protocolo 17", "OpenOMSI 0.3.49 and protocol 17", "OpenOMSI 0.3.49 und Protokoll 17"),
  logT("Atualização integral do jogo, Launcher, servidor e HostAgent P17. Configurações, mapas, voz, senhas e inventários anteriores são preservados.", "Full game, Launcher, server and HostAgent P17 update. Existing settings, maps, voice, passwords and inventories are retained.", "Vollständiges Update von Spiel, Launcher, Server und HostAgent P17. Einstellungen, Karten, Sprache, Kennwörter und Inventare bleiben erhalten.")},
 {logT("Espaço para cadeirantes", "Wheelchair clearance", "Platz für Rollstühle"),
  logT("Os passageiros liberam continuamente o trajeto e a área reservada durante a assistência, evitando que voltem a bloquear a cadeira.", "Passengers keep the path and reserved bay clear throughout assistance, preventing the wheelchair from being blocked again.", "Fahrgäste halten Weg und Stellplatz während der Hilfe frei, damit der Rollstuhl nicht erneut blockiert wird.")},
 {logT("Buzina e aceno dos motoristas IA", "AI driver horns and waves", "Hupen und Winken der KI-Fahrer"),
  logT("A saudação usa dois toques rápidos no comando da buzina, com reprodução completa do conjunto de áudio do ônibus. O aceno é aplicado ao braço do motorista.", "The greeting uses two quick horn button taps, with the bus model's complete horn audio. The wave is applied to the driver's arm.", "Die Begrüßung nutzt zwei kurze Huptastendrücke mit vollständigem Hupklang des Busmodells. Das Winken wird am Fahrerarm angewendet.")},
 }}
 result:=[]logSection{newest}
 if from==changelogSeenVersion || from=="2.2.3-hotfix.2" {return result}
 return append(result,selectPreviousLogSections(from)...)
}

func selectLogSections(from string) []logSection {
 newest := logSection{label:logT("2.2.3.2 • OPENOMSI 0.3.50", "2.2.3.2 • OPENOMSI 0.3.50", "2.2.3.2 • OPENOMSI 0.3.50"), accent:logAccent(112,219,164), entries:[]logEntry{
 {logT6("OpenOMSI 0.3.50 completo", "Complete OpenOMSI 0.3.50", "Vollständiges OpenOMSI 0.3.50", "OpenOMSI 0.3.50 completo", "OpenOMSI 0.3.50 complet", "Полное обновление OpenOMSI 0.3.50"),
 logT6("Jogo, Launcher e servidor atualizados com todas as alterações oficiais da 0.3.50. HostAgent e migração mantêm o protocolo P17, mapas, voz, senhas e preferências.", "Game, Launcher and server include all official 0.3.50 changes. HostAgent and migration retain P17, maps, voice, passwords and preferences.", "Spiel, Launcher und Server enthalten alle offiziellen Änderungen von 0.3.50. HostAgent und Migration behalten P17, Karten, Sprache und Einstellungen bei.", "Juego, Launcher y servidor incluyen todos los cambios oficiales de 0.3.50. Se conservan P17 y los ajustes del HostAgent.", "Le jeu, le Launcher et le serveur intègrent toutes les modifications officielles de 0.3.50. P17 et les réglages du HostAgent sont conservés.", "Игра, Launcher и сервер включают все официальные изменения 0.3.50. Сохранены P17 и настройки HostAgent.")},
 {logT6("Cadeirante: aviso e sinal", "Wheelchair request and signal", "Rollstuhlanzeige und Haltewunsch", "Aviso y señal de silla de ruedas", "Demande et signal du fauteuil roulant", "Уведомление и сигнал коляски"),
 logT6("O pedido de desembarque aparece na tela mesmo em ônibus sem indicador próprio. O sinal da cadeira é limpo ao concluir o desembarque e ao encerrar a assistência.", "The alighting request appears on screen even on buses without a dedicated indicator. The wheelchair signal resets after alighting and assistance.", "Der Ausstiegswunsch erscheint auch ohne eigene Fahrzeuganzeige. Das Rollstuhlsignal wird nach Ausstieg und Hilfe zurückgesetzt.", "La solicitud de bajada aparece en pantalla aunque no haya indicador propio. La señal se apaga al terminar la asistencia.", "La demande de descente apparaît même sans indicateur dédié. Le signal est réinitialisé après la descente et l'assistance.", "Запрос выхода виден даже без отдельного индикатора. Сигнал сбрасывается после выхода и помощи.")},
 {logT6("Ativação corrigida", "Activation fixes retained", "Aktivierungskorrekturen erhalten", "Activación corregida", "Activation corrigée", "Исправления активации"),
 logT6("Inclui as correções de pacote incompleto ou alterado e de ativação do BCS publicadas na 2.2.3.1 Hotfix 2.", "Includes the incomplete or modified package and BCS activation fixes from 2.2.3.1 Hotfix 2.", "Enthält die Paket- und BBS-Aktivierungskorrekturen aus 2.2.3.1 Hotfix 2.", "Incluye las correcciones de paquete y activación de BCS de 2.2.3.1 Hotfix 2.", "Inclut les corrections du paquet et de l'activation BBS de 2.2.3.1 Hotfix 2.", "Включены исправления пакета и активации BBS из 2.2.3.1 Hotfix 2.")},
 }}
 result:=[]logSection{newest}
 from=strings.TrimSpace(from)
 if from==changelogSeenVersion || from=="2.2.3.1-hotfix.2" {return result}
 return append(result,select2231LogSections(from)...)
}
