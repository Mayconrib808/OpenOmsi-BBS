# Teste da 2.2.3 Hotfix 2

1. Atualize a Bridge do motorista e a instalação que executa o HostAgent. Feche e reinicie o servidor do mapa para carregar o novo executável P16.
2. Confirme no começo dos logs a versão `0.3.47-coop.16-2.2.3-hotfix.2`.
3. Compare uma sessão hospedada pelo próprio jogo e outra pelo HostAgent, usando o mesmo mapa, região e horário. A quantidade total de IA no servidor não é a quantidade dentro do alcance do cliente.
4. Confira a fila dos primeiros pontos após carregar a região. Ela segue a demanda do mapa e os limites de vagas/população; novas chegadas não exigem a presença de um ônibus.
5. Com uma viagem pública válida, confirme a aproximação, o sinal, a autorização de embarque e a entrada pela porta. O destino do passageiro precisa permanecer o mesmo após a autorização do host.
6. No cliente, `PASSENGER_CLAIM_AUDIT` informa pontos registrados, linhas não reconhecidas, recusas, pedidos e estado da viagem. `PASSENGER_HAIL_AUDIT` inclui as pessoas recebidas do servidor.
7. Verifique o desembarque da cadeira pelo Alt e a prioridade de espaço da Hotfix 1.
8. O Setup deve mostrar primeiro o capítulo Hotfix 2. Quem vem da Hotfix 1 vê esse capítulo; instalações anteriores também veem as alterações ainda não lidas.
