# Validação de acessibilidade 2.2.3

Base: **2.2.2.4 Hotfix 5**. A suíte de simulação passou com 1.004 testes aprovados, 31 ignorados e nenhuma falha. O ensaio abaixo usa CPU, os arquivos fornecidos e o controlador de cadeira; não executa renderização, a viagem completa da demanda de passageiros nem uma sessão multiplayer real.

**19 variantes completaram o ciclo automatizado; 9 variantes principais continuam bloqueadas; 3 arquivos são componentes traseiros de articulados.** Nenhum resultado equivale a garantia visual em jogo. As três partes traseiras não são veículos principais independentes.

| Arquivo fornecido | Resultado | Vagas verificadas |
| --- | --- | --- |
| Articulado Scania/Articulado Scania/Mi4_K310UA_F_ZF.bus | Acesso bloqueado | 0 |
| Articulado Scania/Articulado Scania/Mi4_K310UA_T_ZF.bus | Componente traseiro | 0 |
| Articulado(2)/Articulado/Mi4_UA_F_Voith.bus | Acesso bloqueado | 0 |
| Articulado(2)/Articulado/Mi4_UA_F_ZF.bus | Acesso bloqueado | 0 |
| Articulado(2)/Articulado/Mi4_UA_T_Voith.bus | Componente traseiro | 0 |
| Articulado(2)/Articulado/Mi4_UA_T_ZF.bus | Componente traseiro | 0 |
| Man cadeirante/Man cadeirante/MAN_EN92_main.bus | Ciclo do controlador aprovado | 1 |
| Solaris/Solaris/Urbino 12.bus | Ciclo do controlador aprovado | 1 |
| Solaris/Solaris/Urbino 15.bus | Ciclo do controlador aprovado | 2 |
| Solaris/Solaris/Urbino 18 Main.bus | Ciclo do controlador aprovado | 1 |
| Torino 2007/Torino 2007/Torino_2007_VW_T2_BH.bus | Acesso bloqueado | 0 |
| Torino 2025/Torino 2025/TS_1726-AR_Manual.bus | Acesso bloqueado | 0 |
| Torino S/Torino S/Torino_S_Rio_Ita.bus | Acesso bloqueado | 0 |
| Torino2007-MB/2007 Marcopolo Torino OF 1722M/Torino 2007  Manual Real.bus | Ciclo do controlador aprovado | 1 |
| Torino2007-MB/2007 Marcopolo Torino OF 1722M/Torino 2007 Auto.bus | Ciclo do controlador aprovado | 1 |
| Vip 1 3 portas/Vip 1 3 portas/Vip 1 OF 1721 Manual.bus | Acesso bloqueado | 0 |
| Vip 1 4 portass/Vip 1 4 portass/Vip 1 OF 1721 Manual.bus | Acesso bloqueado | 0 |
| Vip 3 Mb/Vip 3 Mb/Vip 3.bus | Ciclo do controlador aprovado | 1 |
| Vip V/Vip V/Vip 5.bus | Ciclo do controlador aprovado | 1 |
| Vip V(1)/Vip V/VipV_9m_151900OD_Manual.bus | Ciclo do controlador aprovado | 1 |
| Vip iii Wolks/Vip iii Wolks/Vip 3.bus | Acesso bloqueado | 0 |
| Vip iv/Vip iv/VipIV_OF-1721_Manual.bus | Ciclo do controlador aprovado | 1 |
| Vip iv/Vip iv/VipIV_OF-1724_Manual.bus | Ciclo do controlador aprovado | 1 |
| torino S Zane/torino S Zane/_TorinoS_17260s_e6.bus | Ciclo do controlador aprovado | 1 |
| torino S Zane/torino S Zane/_TorinoS_17260s_e6_ZF-8AP.bus | Ciclo do controlador aprovado | 1 |
| torino S Zane/torino S Zane/_TorinoS_OF-1721_e6.bus | Ciclo do controlador aprovado | 1 |
| torino S Zane/torino S Zane/_TorinoS_OF-1726_e6.bus | Ciclo do controlador aprovado | 1 |
| torino S Zane/torino S Zane/_ZTorinoS_OF-1721_e6_ZF-8AP.bus | Ciclo do controlador aprovado | 1 |
| torino S Zane/torino S Zane/_ZTorinoS_OF-1726_e6_ZF-8AP.bus | Ciclo do controlador aprovado | 1 |
| vip iii/vip iii/Vip 3.bus | Ciclo do controlador aprovado | 1 |
| vip iii mb 2/vip iii mb 2/Vip 3.bus | Ciclo do controlador aprovado | 1 |

## Critérios

- Rampas nativas MAN/Solaris: os scripts e animações originais são executados contra uma calçada de 18 cm; embarque, desembarque, contato e recolhimento são medidos.
- Elevador auxiliar: porta e área disponíveis são pré-condições do ensaio; o controlador precisa chegar a `Riding`, depois a `Finished`, e liberar o bloqueio após recolher. Esse ensaio não comprova sozinho que uma pessoa gerada naturalmente chegará até a porta.
- Torino 2007 OF 1722M: preserva a calibração já testada no jogo do usuário, reconhecida pelas malhas e dimensões exatas. O pacote não depende da pasta de instalação nem do nome atribuído ao veículo.
- Passagens não comprovadas continuam bloqueadas. Não se removem bancos ou grades fixas para declarar compatibilidade.
- O JSON adjacente conserva perfis, motivos, impressões digitais e resultados por vaga; os modelos comerciais não são redistribuídos.

## Validação de jogo pendente

Confirmar geração natural e sinalização, aproximação à porta acessível, fila e redistribuição de passageiros, operação dos comandos originais, contato dos pés e mãos, fixação, pedido de desembarque, continuidade e conexão. No multiplayer, testar dois ou mais cadeirantes e devolução da mesma pessoa ao host. Os articulados Scania/Mercedes e os demais modelos bloqueados requerem calibração adicional antes de qualquer declaração de compatibilidade.
