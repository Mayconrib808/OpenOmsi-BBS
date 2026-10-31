# Controles opcionais

Os sete novos controles aparecem nas configurações do jogo e do launcher, com nome e ajuda em inglês e nas 29 traduções. São independentes e começam desligados. As alterações valem no próximo início do jogo: é necessário sair e reiniciar a sessão.

| Recurso | Chave salva | Efeito ao desligar |
| --- | --- | --- |
| Cenas de eventos do BCS | `bcs_event_scenes` | Desativa efeitos, atores e respostas de emergência desta integração. Em multiplayer, a IA compartilhada depende da escolha do host. |
| Fiscais do BCS | `bcs_inspectors` | Não inicia a atualização/criação do fiscal. A captura BCS só inicia se este controle ou o de cenas estiver ligado. |
| Comentários de porta e ponto perdido | `passenger_door_comments` | Não executa as buscas e reações adicionais de porta/ponto. |
| Agradecimentos ao desembarcar | `passenger_exit_thanks` | Não executa o agradecimento extra. Agradecimentos comuns de pagamento do jogo têm seu comportamento original. |
| Densidade extra em vias largas | `extra_traffic_density` | Mantém o cálculo original de densidade, sem somar comprimentos de vias. O mesmo cálculo atende host e áreas de jogadores LAN. |
| Cumprimentos dos ônibus da IA | `ai_bus_greetings` | Não procura candidatos nem executa o cumprimento adicional por buzina/luz. Em multiplayer, a IA compartilhada depende do host. |
| Conexões de passageiros entre linhas | `passenger_connections_enabled` | Probabilidade efetiva de conexão é zero, mesmo se o controle de porcentagem salvo for 100%. |

A integração não substitui nem altera contadores de negócio do BCS. Desligar a cena no OpenOMSI não cancela um evento no aplicativo externo. Em multiplayer, desligar cenas localmente impede o processamento desta integração nessa instância; um cliente não pode ordenar que a IA do host mude sem respeitar a autoridade do host.

## Controles existentes preservados

- Modo de animação de passageiros: selecione Original OMSI para desativar as animações Natural. Novas configurações usam Original; uma escolha explícita válida é preservada, respeitando a migração histórica da versão 3.
- Guarda-chuvas e passageiros dando sinal: controles próprios, desligados por padrão; aparecem quando o modo de animação compatível está ativo. A sinalização também possui seleção por mapa.
- Compra de bilhetes: a substituição de probabilidade tem controle próprio; desligada, mantém a chance normal do mapa. A porcentagem de conexão continua separada do botão de habilitar conexões.
- Reclamações por condução ruim e vozes dos passageiros: controles já existentes permanecem disponíveis. As novas opções de comentários/agradecimentos não desligam os sistemas originais.

A correção de portas, recuperação de caminhos, seleção de ponto e regras de propriedade da rede continua ativa. Elas corrigem a simulação e não são efeitos opcionais.

## Persistência e execução

Os dois escritores registram explicitamente `0` ou `1` para as sete chaves. Testes percorrem as 128 combinações de opções, salvam/reabrem nos dois sentidos entre jogo e launcher, preservam valores desativados e testam arquivos antigos e configurações parciais. As reações desativadas não alteram esperas nem o RNG da simulação; agradecimentos ativos também usam uma seleção cosmética separada.

A tradução cobre texto/legendas e ajuda dos controles. Não foram criadas gravações dubladas: as falas audíveis dependem dos arquivos de voz instalados.
