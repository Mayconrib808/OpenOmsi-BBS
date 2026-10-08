# Servidor automático — 2.0.2-dev.4

Abra **Setup → Empresa / servidor**, ou **HostAgent.exe**. O aplicativo configura cada mapa e mantém um agente em segundo plano. Quando alguém inicia uma viagem pelo BCS, a bridge pede esse mapa; o agente abre o servidor dedicado, aguarda o mundo carregar, sincroniza o relógio e publica o endereço validado. A conexão do jogo continua sendo o multiplayer nativo do openOMSI.

O PC do anfitrião precisa estar ligado, conectado e com o agente ativo. A opção **Iniciar agente ao entrar no Windows** evita abrir o agente manualmente depois de entrar na sua conta. Fechar a janela de configuração deixa o agente ativo. **Parar agente** encerra os servidores que ele abriu.

## Configurar os mapas

1. Informe a pasta original do OMSI 2 e o `openomsi.exe` do pacote oficial do servidor dedicado. Para os testes atuais, use o servidor openOMSI 0.2.11.
2. Se já tiver o JSON da empresa, clique **Importar JSON atual**. A frota e os mapas cadastrados são preservados. Os caminhos anteriores do CompanyHost são importados automaticamente quando disponíveis.
3. Para cadastrar outro mapa, clique **Detectar mapas**, escolha o mapa instalado e clique **Adicionar mapa**. O nome vem do `global.cfg`; o aplicativo cria a sessão e atribui portas separadas.
4. Escolha o **Mapa configurado**. Ajuste porta UDP/HTTP, máximo de jogadores, tráfego, passageiros, tabela de horários e tempo para fechar vazio. `0` no tempo vazio mantém esse mapa aberto até você parar o agente.
5. Na lista da frota, use Ctrl/Shift para selecionar vários `.bus`. O campo acima filtra por nome ou pasta. **Sugerir dirigíveis** seleciona os ônibus visíveis no filtro e exclui variantes com `AI` no nome; elas continuam disponíveis para seleção manual. A frota é independente em cada mapa.
6. Configure o fuso e o ajuste em minutos da empresa, conferindo a hora usada pelo BCS. O exemplo Transfort usa `Europe/Berlin` e `-480` minutos.

**Hospedar este mapa** controla se o agente pode iniciá-lo. Dois mapas podem ficar abertos ao mesmo tempo, cada um com suas portas e configurações. O aplicativo detecta portas repetidas. Ele cria configurações próprias por mapa e preserva o `server.cfg` original do pacote oficial.

O modo permissivo continua ativo: diferenças de hashes, arquivos extras, `Holidays.txt`, repaints e ônibus de outros jogadores não bloqueiam a entrada. O jogo ainda precisa conseguir carregar o mapa e o ônibus escolhido, e o servidor precisa oferecer esse ônibus. Os hashes presentes em perfis antigos são metadados para auditoria administrativa; não viram uma barreira multiplayer.

## Conectar o diretório online uma vez

O pacote inclui o serviço em **relay/**. Ele precisa ser publicado em um endereço HTTPS fixo; nenhum diretório público foi criado automaticamente por esta atualização. A pasta contém um Cloudflare Worker e um Durable Object que guardam o perfil, o estado dos mapas e os pedidos de abertura. O tráfego do jogo passa pelo túnel nativo do openOMSI, sem atravessar esse serviço.

Para quem administra o diretório:

1. Entre na sua conta Cloudflare e instale Node.js no computador que fará a publicação. Dentro de `relay/`, execute `npx wrangler login`.
2. No aplicativo, clique **Copiar chave**. Execute `npx wrangler secret put HOST_KEY` e cole essa chave no pedido do Wrangler. Ela fica no serviço e nas configurações privadas do host; não envie aos jogadores.
3. Execute `npx wrangler deploy`. Copie a URL HTTPS do Worker para **URL fixa do diretório online**. O aplicativo acrescenta um código de sala próprio automaticamente.
4. Clique **Salvar e iniciar agente** e confira o estado do agente. Se aparecer `HTTP 401`, a chave configurada no Worker difere da chave do aplicativo. Se aparecer `HTTP 404`, confira a publicação e a URL.
5. Clique **Exportar perfil dos jogadores**. Envie esse `.players.json` uma vez, ou compartilhe a URL da sala acrescida de `/profile`.

O serviço usa um código de sala de 32 caracteres como convite. O perfil público contém apenas os dados da empresa. A chave do host, caminhos locais, configurações de inicialização do Windows e senha administrativa do servidor não são exportados. Pedidos remotos só podem citar mapas previamente cadastrados; não carregam comandos ou executáveis.

Documentação da plataforma: [primeiros passos com Durable Objects](https://developers.cloudflare.com/durable-objects/get-started/) e [armazenamento](https://developers.cloudflare.com/durable-objects/best-practices/access-durable-objects-storage/).

## Jogadores

Atualize a bridge para dev.4. No Setup, ative o multiplayer, escolha o novo `.players.json` ou link `/profile`, informe seu nome e clique **Salvar e ativar**. Inicie a viagem pelo BCS normalmente.

A bridge consulta o perfil atualizado, solicita o mapa e espera o carregamento. O servidor só é anunciado online depois de confirmar o mundo ativo, o relógio e o endereço HTTPS. Se o anfitrião estiver offline, a bridge informa isso. Não é necessário reenviar o JSON quando o túnel mudar. Perfis anteriores sem `directory_url` continuam funcionando pelo modo manual.

Um mapa vazio fecha após o tempo configurado. A próxima viagem pode abri-lo novamente. Um mapa ocupado permanece aberto. Pedidos simultâneos para o mesmo mapa reutilizam o servidor em carregamento ou já aberto.

## Verificação desta versão

O pacote inclui testes de rotação de endereço, identidade da empresa, segredo privado, pedidos duplicados, portas separadas, seleção de frota e desligamento por inatividade. O workflow executa os testes Go e do diretório, compila todos os executáveis e verifica os controles Windows, incluindo capturas do painel em PT/EN/DE.

O teste final com dois jogadores, BCS real e um Worker publicado deve confirmar abertura sob demanda, reinício com novo túnel e reconexão usando o mesmo JSON. Não substitua os arquivos da instalação durante uma viagem.

## English

Open **Setup → Company / server** or **HostAgent.exe**. Import your existing profile or scan/add installed maps. Configure each map's fleet, UDP/HTTP ports, player limit, traffic, passengers, timetable and idle timeout. Ctrl/Shift selects multiple buses; the filter and suggestion button exclude AI variants from automatic suggestions. Each map keeps its own fleet.

Deploy the service in `relay/` once, set its `HOST_KEY` secret to the private key copied from the application, and enter the Worker's HTTPS URL in the application. Start the agent, then export a player profile. Players enroll that profile or the room's `/profile` URL once. The stable directory handles wake requests and refreshed tunnel addresses; actual gameplay uses native openOMSI multiplayer. The host PC must be on and the background agent running. Local files and hashes do not gate multiplayer entry. Real two-player validation remains necessary.

## Deutsch

Öffne **Setup → Firma / Server** oder **HostAgent.exe**. Importiere das vorhandene Firmenprofil oder suche und registriere installierte Karten. Jede Karte hat eine eigene Flotte, UDP-/HTTP-Ports, Spielergrenze, Verkehr, Fahrgäste, Fahrplan und Leerlaufzeit. Strg/Umschalt ermöglicht Mehrfachauswahl; Filter und Vorschläge lassen KI-Varianten bei automatischen Vorschlägen aus.

Veröffentliche den Dienst aus `relay/` einmal, setze `HOST_KEY` auf den privaten Schlüssel aus der Anwendung und trage die HTTPS-URL des Workers ein. Starte den Agenten und exportiere das Spielerprofil. Spieler richten das Profil oder die `/profile`-URL einmal ein. Das feste Verzeichnis verarbeitet Startanfragen und aktualisierte Tunneladressen; das Spiel verwendet den nativen openOMSI-Multiplayer. Der Host-PC muss eingeschaltet sein und der Agent laufen. Lokale Dateien und Hashes blockieren den Multiplayer-Beitritt nicht. Ein realer Test mit zwei Spielern steht noch aus.
