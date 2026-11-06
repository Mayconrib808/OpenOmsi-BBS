# Vestuário de inverno dos passageiros padrões — plano de validação da Bridge

**Estado:** recurso experimental, ainda não incluído no Setup público. Há um pacote com 7 malhas OBJ de artistas, texturas e arquivos de influência nos ossos. A conversão automática não comprova ajuste visual correto.

## Comportamento que precisa funcionar

| Clima no jogo | NPC padrão sem CTI especial | Registros esperados |
| --- | --- | --- |
| 20 °C, sem neve | Roupa original | Sem objetos adicionais; peças antigas ocultadas |
| 12 °C, sem neve | Jaqueta ou suéter de artista | `authored_asset_loaded`, `garment_instances_created`, `real_mesh_uploaded_and_instance_visible` |
| 4 °C, sem neve | Jaqueta/suéter + touca + luvas | 3 assets associados ao mesmo ID, com todos os arquivos e textura válidos |
| 10 °C, neve | Roupa pesada e acessórios | Respeitar neve na escolha; nenhuma peça improvisada |
| 16 °C após 4 °C | Volta à roupa normal | Instâncias de inverno ocultas sem recriar o NPC |

Requisitos da opção `weather_clothing`: quando desativada, nenhuma peça extra aparece. CTIs reais já identificadas como `winter/cold/coat/inverno/casaco/snow` prevalecem. Avatares personalizados, puppets e personagens ocultos não recebem acessórios por cima.

## Inspeção visual indispensável em um jogo instalado
1. Abrir uma rota com humanos originais variados. Verificar caminhada, braço balançando, sinal de parada, guarda-chuva, pagamento, embarque, banco, conversa e troca de motorista. Confirmar que nenhuma manga atravessa a mão, nenhuma touca flutua e que as luvas estão junto aos pulsos.
2. Comparar host e cliente LAN no mesmo ponto, inclusive NPCs sentados. O modelo e a textura precisam continuar iguais; não introduzir estados novos na rede apenas para roupa.
3. Conferir duplicação de mesh ao reaproveitar GPU instances depois de um passageiro se retirar. Nunca deixar roupas do passageiro anterior no novo.
4. Anotar FPS médio, 1% low, VRAM, RAM e custo CPU de pose com a opção **desligada** e **ligada**, nas mesmas condições; usar uma estação lotada com 40+ passageiros. Não afirmar melhora ou estabilidade sem medição.
5. Em `game.log`, a presença de `authored_asset_loaded` prova carregamento, mas **não** aplicação. Só `real_mesh_uploaded_and_instance_visible` indica atualização de vértices e visibilidade no renderizador. Nem mesmo isso substitui inspeção visual.

## Controle de desempenho do protótipo
- Até **12 passageiros** com roupas 3D adicionais simultâneas em até **32 m**, no máximo **2** novas aplicações por frame.
- Reutilização de instâncias e meshes GPU; ao reaproveitá-las o cache de vértices e normais precisa ser invalidado.
- Um vestido de inverno já existente no CTI não deve receber jaqueta extra por cima.
- Texturas limitadas a 2048 px; não criar geometria substituta quando um asset real estiver ausente.

## Logs
- `FEATURE EVENT role=host|client feature=... result=... reason=...`: decisões realizadas, com IDs, motivo e fonte.
- `FEATURE SUMMARY`: por feature/role em intervalos de 60 segundos, `evaluated`, `executed`, `completed`, `blocked`, `failed`, `no_opportunity` e causas contadas.
- `passenger_audio` no cliente diferencia `clip_load_timeout`, `clip_decode_failed` e `voice_active_in_engine`. A aceitação pelo mixer **não** prova saída física pelo alto-falante.
- `ai_bus_overtaking` diferencia manobra iniciada, bloqueada, concluída ou cancelada. `bcs_events` diferencia evento recebido, modelo selecionado, despacho, veículo criado e chegada física.

## Liberação
CI Windows do simulador, teste de carregamento dos sete modelos reais, integridade do pacote e testes de gameplay são **portas independentes**. Não assinar manifesto nem substituir a versão publicada enquanto qualquer etapa permanecer sem aprovação. Cachecol real ainda não incluído: não substituir por um objeto cilíndrico ou modelo sem licença para redistribuição.
