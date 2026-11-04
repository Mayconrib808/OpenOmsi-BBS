# Validação da Beta 2

Estado em 4 de novembro de 2026. O commit exato e o hash do executável estão em `beta-manifest.json`.

## Código e pacote

- Linux: `cargo test --locked -p omsi-sim --lib`: **993 testes passaram, 31 ignorados, zero falhas**.
- Linux: `cargo check --locked -p omsi-app`: passou. Há avisos anteriores do motor.
- A publicação de Windows exige novamente os testes da simulação, compilação nativa, execução de `openomsi.exe --help`, instalação, atualização, coleta de logs e remoção em Windows PowerShell 5.
- O teste de instalação usa uma pasta com espaços e verifica a restauração dos perfis, a preservação de edições posteriores e o changelog acumulado desde a versão instalada.
- Os testes iniciais do instalador usam um arquivo sentinela. Esse arquivo nunca é executado ou incluído no ZIP publicado. O ZIP final usa o executável produzido pela compilação nativa.

## Arquivos de ônibus analisados

Os ensaios abaixo carregaram os `.bus`, modelos, cabine e scripts enviados pelo usuário. Não precisam das texturas. São ensaios de CPU, sem renderização ou viagem no mapa.

| Amostra | Acesso selecionado | Resultado do ensaio |
|---|---|---|
| Ian Santos, `Vip 3.bus`, Vip III B270F Euro 5 | Elevador adicional na porta central; peça original estática | Trajeto e ciclo do controlador aprovados |
| Edu231, `VipIV_OF-1721_Manual.bus` | Elevador adicional na porta central; banco retrátil original | Trajeto e ciclo do controlador aprovados |
| Edu231, `VipIV_OF-1724_Manual.bus` | Elevador adicional na porta central; banco retrátil original | Trajeto e ciclo do controlador aprovados |
| MAN, `MAN_EN92_main.bus` | Rampa original na segunda porta | Trajeto, macro original, animação e controlador aprovados |
| Solaris, `Urbino 12.bus` | Rampa original na porta central | Trajeto, macro original, animação e controlador aprovados |
| Solaris, `Urbino 15.bus` | Rampa original na porta central | Trajeto, macro original, animação e controlador aprovados |
| Millennium IV articulado, `Mi4_UA_F_Voith.bus` | Equipamento original reconhecido | Pendente: nenhuma manobra completa livre foi comprovada |
| Solaris, `Urbino 18 Main.bus` | Equipamento original | Pendente: trailer necessário ausente no conjunto analisado |
| Torino 2007 OF 1722M | Perfil explícito preservado da Beta 1 | Perfil validado; o usuário já testou o ciclo da Beta 1 no jogo |

O detector procura passagem e espaço usando triângulos do modelo e volumes da cadeira e do passageiro, incluindo a varredura das curvas. A rota aprovada não assegura que acessórios opcionais, animações não relacionadas ao acesso ou versões diferentes do mesmo mod deixem o mesmo espaço.

## Ensaios das rampas originais

O exemplo `wheelchair_probe` carrega o programa original, executa a macro do equipamento e atualiza as animações reais, incluindo matrizes dos pais. A simulação usa uma calçada de 18 cm. O Solaris usa um rebaixamento de 10 cm do ônibus para representar o ajoelhamento.

| Amostra | Macro original | Primeiro apoio/alinhamento confirmado | Recolhimento |
|---|---|---|---|
| MAN EN92 | `ramplift_frame` | 8,66 s | Confirmado pela geometria depois de fechar a porta e desativar o comando manual |
| Solaris 12 | `cockpit_frame` | 1,28 s | A variável muda antes da peça; o detector espera a animação terminar |
| Solaris 15 | `cockpit_frame` | 1,28 s | A variável muda antes da peça; o detector espera a animação terminar |

Os três chegaram às etapas `Riding` e `Finished`, liberando a imobilização após recolher. O maior deslocamento entre atualizações a 60 Hz foi 1,41 cm. Nos três Vip, com porta e área liberadas no ensaio, os ciclos do elevador adicional também chegaram a `Riding` e `Finished`; o maior deslocamento foi 0,81 cm. Esses valores demonstram continuidade do controlador nas condições medidas.

Os mods têm avisos/erros de compilação de scripts anteriores ao adaptador, incluindo variáveis de IA e outros subsistemas. O ensaio de equipamento isola a macro indicada; não certifica todos os scripts do ônibus. Os scripts e modelos do usuário não são redistribuídos.

## Postura e bloqueios

Os testes incluem pés separados nos apoios em três escalas e quatro orientações, joelhos separados, saída em ré, varredura diagonal, piso inclinado, porta com animação lenta, apoio da rampa, recolhimento, banco retrátil, obstrução da passagem e recusa de assistência quando o passageiro ficou longe do ônibus.

## Ainda precisa de teste no jogo

- Aparência dos joelhos, roupas e mãos, transições da inclinação, folgas do modelo e encaixe da plataforma nos mapas do usuário.
- Operação completa do painel, freios, portas e suspensão dos ônibus com todos os seus scripts ativos.
- Multiplayer real: reconexão, troca de motorista, equipamento e inclinação vistos pelos observadores.
- Millennium articulado, Solaris 18 com trailer e outros mods não representados nas amostras.

Não foi realizada validação visual ou uma sessão multiplayer desta Beta 2 neste ambiente. O manifesto registra ambas como `false`. A detecção permanece experimental: evidência insuficiente gera uma pendência no log em vez de inventar um acesso.

## Reprodução dos ensaios

Com os mods instalados em um diretório local e o código-fonte desta versão:

```text
cargo build --locked -p omsi-sim --example wheelchair_probe
```

Ative a variável de ambiente `WHEELCHAIR_EXERCISE=1` e execute o exemplo com dois argumentos: a pasta raiz do jogo e o caminho completo do `.bus`. O JSON contém o perfil detectado, os motivos de rejeição e o resultado do ciclo. Para o teste no jogo, use `INICIAR_BETA.cmd` e depois `Recolher_Logs.cmd`.
