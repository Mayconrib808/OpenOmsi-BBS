# Teste da candidata 2.2.3

Esta candidata usa a base da 2.2.2.4 Hotfix 5. O relatório dos ônibus distingue os ciclos automatizados aprovados dos acessos ainda bloqueados. A validação visual de todos os modelos e do multiplayer continua pendente.

Consulte a lista por modelo em [accessibility-223-validation.md](accessibility-223-validation.md), nesta mesma pasta. O arquivo `accessibility-223-vehicle-validation.json` conserva os resultados detalhados, incluindo os motivos de bloqueio.

1. Nas configurações de passageiros, mantenha **Cadeirantes nos pontos de ônibus** ativado. Para encontrar passageiros durante o teste, ajuste **Chance de aparecer um cadeirante** para **100%** e reinicie o jogo. A chance é aplicada a novos passageiros; as pessoas que já estavam no mapa não são convertidas.
2. Use um ônibus com acesso aprovado no relatório e abra o menu **Alt → Assistência ao cadeirante**. O painel informa as vagas verificadas e o lado da porta acessível. Se disser que não há acesso verificado, esse modelo continua bloqueado nesta candidata.
3. Ao chegar ao ponto, observe a aproximação e o sinal do passageiro. Pare junto à calçada e abra a porta acessível. Em ônibus com rampa original, opere a rampa pelos controles do próprio ônibus. Com elevador auxiliar, use **Preparar embarque ou desembarque** no painel de assistência.
4. Aguarde o embarque, a acomodação e o recolhimento do equipamento antes de sair. Confira braços, pernas, rodas, contato com o piso e liberação do corredor. Havendo mais de uma vaga verificada, confira se as cadeiras ocupam lugares diferentes.
5. Continue a viagem até o pedido de desembarque. Confira o sinal específico ou o aviso **Um cadeirante quer descer no próximo ponto**. Pare, abra o acesso e opere o equipamento. A pessoa deve continuar com a cadeira após sair; quando houver uma conexão elegível, deve conservar essa identificação.
6. Para o teste multiplayer, cliente e servidor P16 precisam usar esta mesma candidata. Observe as cadeiras em outro computador, inclusive depois do embarque, do desembarque e de uma conexão devolvida ao host.

Ao terminar, salve o **game.log** da viagem e anote o arquivo `.bus`, mapa, ponto, lado da porta e etapa em que ocorreu qualquer falha. Os registros `WHEELCHAIR` identificam geração, detecção, escolha do acesso, transições, liberação de espaço, ocupação e pedido de desembarque.

Depois do teste, restaure a chance para **3%** ou para o valor desejado. **0%** interrompe a geração de novos cadeirantes; o botão independente permite desativar o recurso.
