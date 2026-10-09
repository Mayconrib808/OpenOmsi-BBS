# Servidor automático — 2.0.5-dev.2

Abra **Setup → Empresa / servidor**, ou **HostAgent.exe**. O aplicativo configura cada mapa e mantém um agente em segundo plano. Quando alguém inicia uma viagem pelo BCS, a bridge pede esse mapa; o agente abre o servidor dedicado, aguarda o mundo carregar, sincroniza o relógio e publica o endereço validado. A conexão do jogo continua sendo o multiplayer nativo do openOMSI.

O PC do anfitrião precisa estar ligado, conectado e com o agente ativo. A opção **Iniciar agente ao entrar no Windows** evita abrir o agente manualmente depois de entrar na sua conta. Fechar a janela de configuração deixa o agente ativo. **Parar agente** encerra os servidores que ele abriu.

Em telas menores, use as barras de rolagem ou a roda do mouse para alcançar os controles. A tecla Tab também traz o campo selecionado para a área visível.

## Configurar os mapas

1. Informe a pasta original do OMSI 2. Por padrão, o aplicativo seleciona o servidor incluído nesta atualização, em `app/server/openomsi.exe`, inclusive ao importar uma configuração anterior. Esse servidor deriva do openOMSI 0.2.20 e permite ônibus livres. Se você já salvou outro executável na dev.1, use **Procurar** para escolher o `app/server/openomsi.exe` deste novo ZIP.
2. Se já tiver o JSON da empresa, clique **Importar JSON atual**. Os mapas e as opções cadastradas são preservados. A antiga frota passa a ser apenas uma preferência de modelos substitutos. Os caminhos anteriores do CompanyHost são importados automaticamente quando disponíveis.
3. Para cadastrar outro mapa, clique **Detectar mapas**, escolha o mapa instalado e clique **Adicionar mapa**. O nome vem do `global.cfg`; o aplicativo cria a sessão e atribui portas separadas.
4. Escolha o **Mapa configurado**. Ajuste porta UDP/HTTP, máximo de jogadores, tráfego, passageiros, tabela de horários e tempo para fechar vazio. `0` no tempo vazio mantém esse mapa aberto até você parar o agente.
5. **Substitutos opcionais** pode ficar vazio. Qualquer jogador pode dirigir o seu ônibus, mesmo privado ou ausente no host, sem cadastrá-lo aqui. Se quiser, use Ctrl/Shift para indicar modelos locais que o servidor tentará usar como substitutos. Se nenhum funcionar, tenta os MAN originais e depois outros ônibus locais. Isso nunca limita a entrada.
6. Configure o fuso e o ajuste em minutos da empresa, conferindo a hora usada pelo BCS. O exemplo Transfort usa `Europe/Berlin` e `-480` minutos.

**Hospedar este mapa** controla se o agente pode iniciá-lo. Dois mapas podem ficar abertos ao mesmo tempo, cada um com suas portas e configurações. O aplicativo detecta portas repetidas. Ele cria configurações próprias por mapa e preserva as opções não controladas do `server.cfg` anterior.

O modo permissivo continua ativo: diferenças de hashes, arquivos extras, `Holidays.txt`, repaints e ônibus de outros jogadores não bloqueiam a entrada. O jogador precisa conseguir carregar seu mapa e seu próprio ônibus. O host não precisa ter o ônibus do jogador nem autorizar novos modelos. Quem não tiver os recursos do ônibus remoto poderá ver um modelo substituto; o interior exato depende dos recursos locais. Não se transfere conteúdo privado. Os hashes presentes em perfis antigos são metadados para auditoria administrativa; não viram uma barreira multiplayer.

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

Atualize a bridge para 2.0.3 dev3. No Setup, ative o multiplayer, escolha o novo `.players.json` ou link `/profile`, informe seu nome e clique **Salvar e ativar**. Inicie a viagem pelo BCS normalmente.

A bridge consulta o perfil atualizado, solicita o mapa e espera o carregamento. O servidor só é anunciado online depois de confirmar o mundo ativo, o relógio e o endereço HTTPS. Se o anfitrião estiver offline, a bridge informa isso. Não é necessário reenviar o JSON quando o túnel mudar. Perfis anteriores sem `directory_url` continuam funcionando pelo modo manual.

Um mapa vazio fecha após o tempo configurado. A próxima viagem pode abri-lo novamente. Um mapa ocupado permanece aberto. Pedidos simultâneos para o mesmo mapa reutilizam o servidor em carregamento ou já aberto.

## Verificação desta versão

O pacote inclui testes de rotação de endereço, identidade da empresa, segredo privado, pedidos duplicados, portas separadas, ônibus privado fora da lista do host, seleção vazia de substitutos e desligamento por inatividade. O workflow executa os testes Go, do diretório e do servidor nativo, compila todos os executáveis e verifica os controles Windows, incluindo capturas do painel em PT/EN/DE.

O teste final com dois jogadores, BCS real e um Worker publicado deve confirmar abertura sob demanda, reinício com novo túnel e reconexão usando o mesmo JSON. Não substitua os arquivos da instalação durante uma viagem.

## Trocar o servidor

Pare o agente, escolha outro `openomsi.exe` em **Servidor dedicado → Procurar** e clique em **Salvar e iniciar agente**. O caminho fica salvo também para o início em segundo plano. Escolher o executável incluído em `app/server/openomsi.exe` restaura a seleção automática nas próximas atualizações do pacote. Os mapas e o Cloudflare são preservados. Servidores externos precisam ser compatíveis com o protocolo da bridge e com o suporte a ônibus livres; selecionar um executável não aplica essas adaptações nele.

## English

Open **Setup → Company / server** or **HostAgent.exe**. Import your existing profile or scan/add installed maps. The bundled server is selected automatically. Configure each map's UDP/HTTP ports, player limit, traffic, passengers, timetable and idle timeout. Substitute models are optional: players may drive private buses absent on the host without registration. Missing remote models use local substitutes; no private assets transfer.

Deploy the service in `relay/` once, set its `HOST_KEY` secret to the private key copied from the application, and enter the Worker's HTTPS URL in the application. Start the agent, then export a player profile. Players enroll that profile or the room's `/profile` URL once. The stable directory handles wake requests and refreshed tunnel addresses; actual gameplay uses native openOMSI multiplayer. The host PC must be on and the background agent running. Local files and hashes do not gate multiplayer entry. Real two-player validation remains necessary.

## Deutsch

Öffne **Setup → Firma / Server** oder **HostAgent.exe**. Importiere das vorhandene Firmenprofil oder suche und registriere installierte Karten. Der enthaltene Server wird automatisch gewählt. Jede Karte hat eigene UDP-/HTTP-Ports, Spielergrenze, Verkehr, Fahrgäste, Fahrplan und Leerlaufzeit. Ersatzmodelle sind optional; private Spielerbusse müssen beim Host weder installiert noch registriert sein. Fehlende Modelle werden lokal ersetzt, private Inhalte werden nicht übertragen.

Veröffentliche den Dienst aus `relay/` einmal, setze `HOST_KEY` auf den privaten Schlüssel aus der Anwendung und trage die HTTPS-URL des Workers ein. Starte den Agenten und exportiere das Spielerprofil. Spieler richten das Profil oder die `/profile`-URL einmal ein. Das feste Verzeichnis verarbeitet Startanfragen und aktualisierte Tunneladressen; das Spiel verwendet den nativen openOMSI-Multiplayer. Der Host-PC muss eingeschaltet sein und der Agent laufen. Lokale Dateien und Hashes blockieren den Multiplayer-Beitritt nicht. Ein realer Test mit zwei Spielern steht noch aus.

## Clima na 2.0.3 dev1

Atualize o HostAgent e publique novamente o código de `relay/` no Worker já configurado. As chaves, URL e perfil existentes continuam válidos. O pedido de abertura passa a carregar apenas os valores numéricos validados do clima preparado pelo BCS, sem caminhos de arquivos locais.

O primeiro pedido para um mapa fechado define o clima inicial do mundo. Outros jogadores entram no mesmo clima. Um mapa já aberto não reinicia nem troca de clima por causa de outra previsão do BCS. Ao fechar por inatividade, o próximo pedido pode definir um novo clima. Se não houver uma situação e um `.owt` recentes do mapa/data, o clima padrão do servidor permanece e a bridge registra a razão. O Worker antigo aceita pedidos sem transferir esse clima; a atualização do serviço é necessária para habilitá-lo.
