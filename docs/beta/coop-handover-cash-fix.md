# Correção cooperativa — motorista, passageiros e cobrador

Base cumulativa: OpenOMSI 0.3.45 / Coop 15, Bridge 2.2.1.9 Hotfix 1,
correções do caixa e cenas BCS beta. O protocolo LAN permanece 15.
Versão desta compilação: `0.3.45-coop.15-2.2.1.9-coop-fix.1`.

## O que mudou

- A troca conserva a cabine lógica dos passageiros, usando a geometria do
  ônibus que o novo motorista controla. A cópia visual não simula outra viagem.
  O contador de passageiros do novo motorista usa o valor da simulação original.
- Ao devolver a direção ou sofrer uma suspensão da conexão, o personagem
  reaparece no ônibus onde ele está agora; a câmera antiga é descartada.
- A câmera a pé acompanha a transformação atual do chassi, depois do recebimento
  da rede, evitando a diferença de um quadro entre cabine e olhos.
- As imagens dos painéis só seguem no estado quando mudam. Os estados periódicos
  aguardam espaço na fila; ações e devolução conservam a ordem e confirmação.
- O servidor sem interface recebe o aviso de autoridade pelo canal de comandos,
  sem acumular transferências de aplicação que ele não consome.
- Moedas e notas são mostradas ao cobrador e ao motorista visitante nas posições
  do caixa original, inclusive em bandejas móveis. São apenas uma representação:
  a coleta sempre acontece na simulação que mantém os passageiros e o BCS.
- Shift+T, botão "Recolher dinheiro" e clique sobre o pagamento usam a coleta
  autorizada. Clicar sobre o pagamento recolhe o conjunto de moedas/notas do
  passageiro; o troco da outra bandeja é preservado. Retransmissões não recolhem
  o pagamento do passageiro seguinte.

## Instalação do candidato

1. Feche o jogo e guarde uma cópia do `openomsi.exe` anterior.
2. Substitua o `openomsi.exe` da pasta `programa` do Coop pelo deste pacote,
   nos dois computadores. Não altere o OMSI 2, mapas, veículos nem configurações.
3. No computador que hospeda o servidor dinâmico, substitua também o executável
   `app/server/openomsi.exe` quando o servidor estiver parado. É o mesmo binário,
   com os argumentos de servidor já configurados pelo HostAgent.
4. Inicie uma sessão nova. Todos os participantes devem mostrar a versão acima.

O pacote contém o jogo Windows, DLLs produzidas pela compilação, fontes
correspondentes e `SOURCE.json` com hashes e commit. Ele não substitui o
Setup/Bridge, perfis do HostAgent ou adaptadores BCS.

## Evidências e limites

Os logs enviados registram a versão `0.3.45-coop.15-2.2.1.9-hotfix.1`,
854 avisos de fila cheia, duas ativações sucessivas da direção e interações
com cobrador autorizado sem controle disponível. A versão enviada ainda não
incluía os commits anteriores de coleta por Shift+T.

A compilação executa testes de perda/reordenação e confirmação do transporte,
estados parciais de painel, posições das moedas, coleta idempotente e retorno
na posição atual do veículo. O resultado desses testes está na execução do
GitHub Actions vinculada ao commit. A sessão real com dois jogadores, o modelo
Apache Vip V OF-1721L TSA e o BCS não pode ser validada neste ambiente.

Verificação em jogo: embarque passageiros, transfira a direção, mova o ônibus,
recolha pagamento com o cobrador por Shift+T e mouse, devolva a direção e confira
passageiros, posição do personagem e continuidade da mesma viagem no BCS.

## Resultado da validação local — 28/10/2026

Código validado: `c0517f07ab164b6d32a1603d77ec40add84b35ad`.

| Verificação | Resultado |
| --- | --- |
| Compilação de verificação do jogo e dos testes (`cargo check --locked -p omsi-app --lib --tests`) | Passou |
| Transporte de handover, perda, reordenação, confirmação e limite de fila | 4 passaram |
| Estado do veículo, imagens preservadas e posições físicas do dinheiro | 8 passaram |
| Cabine emprestada, identidade lógica e ausência de embarque duplicado | 1 passou |
| Cooperativo: câmera, devolução, contador e coleta idempotente | 21 passaram |
| Total de testes focados | **34 passaram; nenhuma falha** |

Os testes executáveis foram feitos em Linux. O ambiente exigiu
`--config 'profile.test.package.omsi-net.codegen-units=1'` para contornar uma
falha do compilador ao criar um arquivo nativo vazio. Isso não alterou os fontes.
A compilação Windows e o teste real com dois jogadores continuam pendentes.
