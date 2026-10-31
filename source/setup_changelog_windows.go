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
const changelogSeenVersion = "2.2.2-hotfix.2"
const (
    logWMPaint=0x000F
    logWMMouseWheel=0x020A
    logWMKeyDown=0x0100
    logWMSize=0x0005
    logIDDone=1401
    logIDSectionBase=1410
)
type logString struct {pt,en,de string}
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

func logT(pt,en,de string) logString {return logString{pt,en,de}}
func logL(lang string,t logString) string{return localText(lang,t.pt,t.en,t.de)}
func logAccent(r,g,b byte)uintptr{return guiRGB(r,g,b)}

func allLogSections() []logSection {
    return []logSection{
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

func logSections() []logSection {
    if shownLogSections!=nil{return shownLogSections}
    return allLogSections()
}

func selectLogSections(from string) []logSection {
    current:=hotfix2LogSection()
    old:=allLogSections()
    switch strings.TrimSpace(from) {
    case "2.2.2-hotfix.1":
        return []logSection{current}
    case "2.2.2":
        // Hotfix 1 sections + the changes added by Hotfix 2.
        return []logSection{current,old[0]}
    default:
        // First update, unsupported source marker or older installed version:
        // show the full 2.2.x history and the current release.
        return append([]logSection{current,earlierBridgeLogSection()},old...)
    }
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
    logTextDraw(dc,logL(g.config.Language,logT("✨ ATUALIZAÇÃO 2.2.2  ·  HOTFIX 2","✨ UPDATE 2.2.2  ·  HOTFIX 2","✨ UPDATE 2.2.2  ·  HOTFIX 1")),guiRect{26,51,w-20,81},state.subhead,guiRGB(255,211,107),0x0020)
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
        if bridgeVersion!="2.2.2"{return 0}
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
    window,_,_:=guiUser.NewProc("CreateWindowExW").Call(0,uintptr(unsafe.Pointer(class.Name)),uintptr(unsafe.Pointer(winPtr("OpenOMSI BCS Bridge  •  2.2.2 Hotfix 2"))),style,uintptr(x),uintptr(y),uintptr(width),uintptr(height),g.window,0,instance,0)
    if window==0{return 0}
    changelogWindow=window
    changelogState=&logWindowState{heading:guiFont(26,700),subhead:guiFont(19,700),body:guiFont(18,400),tiny:guiFont(16,400)}
    control:=func(label string,x,y,w,h,id int)uintptr{
        hwnd,_,_:=guiUser.NewProc("CreateWindowExW").Call(0,uintptr(unsafe.Pointer(winPtr("BUTTON"))),uintptr(unsafe.Pointer(winPtr(label))),guiWSChild|guiWSVisible|guiWSTabStop|0x000B,uintptr(x),uintptr(y),uintptr(w),uintptr(h),window,uintptr(id),instance,0)
        guiUser.NewProc("SendMessageW").Call(hwnd,guiWMSetFont,g.font,1)
        return hwnd
    }
    for i,s:=range logSections(){
        hwnd:=control(logL(g.config.Language,s.label),17,152+i*65,232,54,logIDSectionBase+i)
        changelogState.buttons=append(changelogState.buttons,hwnd)
    }
    done:=control(logL(g.config.Language,logT("✓ VAMOS JOGAR","✓ LET'S PLAY","✓ LOS GEHT'S")),810,716,230,44,logIDDone)
    guiUser.NewProc("EnableWindow").Call(g.window,0)
    guiUser.NewProc("ShowWindow").Call(window,1)
    guiUser.NewProc("UpdateWindow").Call(window)
    guiUser.NewProc("SetFocus").Call(done)
    return window
}
