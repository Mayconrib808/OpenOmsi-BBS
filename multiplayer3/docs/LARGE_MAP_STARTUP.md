# Servidores de mapas grandes — desempenho (3.x)

## O que este ajuste efetivamente entrega

O tempo-limite anterior de 15 minutos encerrava sessões dedicadas em mapas que
demoravam até 25 minutos. A Bridge agora tolera até 45 minutos de partida e
registra em log as fases de preparação do runtime, catálogo de ônibus e carga
efetiva do mundo. O intervalo longo NÃO acelera o primeiro carregamento.

Uma sessão pesada (mais de 3 minutos até ficar pronta) permanece disponível
por 10 minutos depois que o último jogador sai. Mapas rápidos continuam com
1 minuto. Mapas carregados que nunca receberam jogadores são liberados após
5 minutos. O objetivo é não descarregar e recarregar um mapa enorme entre
viagens do BCS sem manter a RAM ocupada para sempre.

O processo de jogo e o catálogo de frota continuam sendo os mesmos; a Bridge
não muda o protocolo, as versões suportadas ou o tempo e o clima da empresa.

## Por que NÃO mudar radius=0 para radius=2

No motor público do openOMSI, o servidor dedicado trata radius=0 como **mapa
completo**. Um radius pequeno delimita a área inicial e NÃO acompanha
automaticamente a posição de jogadores. Alterá-lo só na Bridge faz bairros
distantes ficarem ausentes da simulação (trânsito, passageiros, paradas e
horários). Não libere isso como otimização.

Referências upstream:

- https://github.com/openOMSI-org/openOMSI/blob/main/docs/SERVER.md
- https://github.com/openOMSI-org/openOMSI/blob/main/crates/omsi-app/src/server.rs
- https://github.com/openOMSI-org/openOMSI/blob/main/crates/omsi-app/src/offscreen/setup.rs
- https://github.com/openOMSI-org/openOMSI/blob/main/crates/omsi-scene/src/tiles.rs

## O que falta para abrir tão rápido quanto o multiplayer normal

É necessário que o **servidor dedicado do openOMSI** carregue inicialmente
apenas os tiles essenciais e faça streaming incremental da união das regiões
ocupadas por todos os jogadores, descarregando as distantes. Isso requer
integrar o streamer da cena com a simulação do host (rede viária, semáforos,
veículos de IA, horários, pessoas e entrega de conteúdo remoto), não apenas
editar server.cfg. Também exige testes em mapas com jogadores separados por
quilômetros, entrada/saída, viagens, BCS e protocolos 14/15. Enquanto o runtime
não informar explicitamente que oferece streaming seguro em servidor dedicado,
a Bridge deve manter radius=0 para evitar regiões ausentes.

## Como avaliar no log

Comparar a abertura direta do openOMSI com uma abertura via Bridge, no mesmo
mapa, estação, tráfego e versão do servidor. Guardar os tempos:

- `host timing: native runtime ready`: download/verificação do executável.
- `host timing: fleet catalog ready`: leitura do catálogo de ônibus.
- `host timing: process and tunnel requested`: subida inicial do processo.
- `host timing: world ready`: tempo de carga do mundo (parte mais pesada).

Ao medir RAM, diferenciar a memória do processo do servidor da memória de
outros processos e arquivos em cache do Windows.

Este ajuste NÃO foi apresentado como solução do primeiro carregamento:
para esse gargalo, precisamos de uma versão do motor com streaming dedicado,
validada e com protocolo compatível.
