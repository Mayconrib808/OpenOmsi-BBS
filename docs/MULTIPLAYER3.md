# OpenOmsi + BBS 3.0.0-dev.1

Primeira implementação separada da ponte 2.x. O executável selecionado pelo
BCS encaminha sua viagem para o openOMSI real após entrar na sessão do mapa.
A avaliação e a comunicação BCS usam o bbs.lua gerado pela PeDePe.
Não distribuir o JAR, a DLL ou uma cópia alterada desse plugin.

## Comportamento

- Convite HTTPS permanente por empresa; configuração local feita uma vez.
- Empresa criada no serviço pelo próprio configurador, sem conta Cloudflare
  de cada empresa/jogador. O operador mantém um único Worker central.
- Mapa extraído de `--map` ou `[map]` da situação `.osn`, incluindo UTF-16LE.
  Caminhos normalizados; SHA-256 truncado a 128 bits identifica a sessão.
- Claim transacional por empresa/mapa. Apenas o vencedor recebe lease_token.
  Renovação a cada aproximadamente 8 segundos; lease de 60 segundos.
  Sem confirmação por 35 segundos, o host para antes de uma nova eleição.
  Credenciais antigas não renovam nem apagam uma sessão nova.
- O primeiro PC inicia o servidor dedicado empacotado. Portas locais escolhidas
  dinamicamente, configurações privadas em diretório temporário, admin secret
  aleatório, processos e túnel pertencentes a um Windows Job Object.
- Endereço público anunciado depois de confirmar mapa, mundo ativo, protocolo
  6, ônibus livres e sincronização do relógio. Outros jogadores usam o túnel;
  o cliente no host usa loopback. Não retransmitir JSON quando o túnel muda.
- Native BCS args, motorista, situação, linha, tour, trip e setvar preservados.
  Somente opções LAN conflitantes são substituídas. Lua copiado sem alterações
  à pasta de conteúdo do jogo real, preservando mods desse jogo. Servidor
  dedicado não carrega plugins de avaliação. Não reinstalar a ponte antiga.
- Sem manifests obrigatórios ou restrição por hash/lista de ônibus. O host
  precisa conseguir carregar o mapa; o jogador precisa de conteúdo suficiente
  para sua própria viagem. A camada não fabrica arquivos ausentes. O motor
  continua responsável pelos substitutos/avisos de conteúdo de outros veículos.
- Relógio civil calculado da timezone explícita mais shift_minutes; inclui DST
  da referência e virada de data. Criador pode editar com todos os mapas parados,
  mantendo o mesmo convite. Guests não alteram o relógio.
- Após o cliente do host sair, o supervisor mantém a sessão para os demais;
  encerra após 60 segundos com zero jogadores. Sem serviço residente quando
  não há sessão. PC desligado perde a sessão; novos jogadores elegem novo host.
  Migração transparente de jogadores ou preservação do mundo não implementada.

## Atualização única do serviço, pelo operador

O Worker existente continua compatível com as rotas 2.x. Antes de testar 3.0,
publique `relay/src/multiplayer3.js` e o `room.js` atualizado junto do Worker:

```sh
cd relay
npx wrangler deploy
```

Use a conta/projeto existentes do diretório. Não trocar ROOM binding, migration
ou HOST_KEY das empresas 2.x. Não enviar HOST_KEY aos jogadores 3.0.
Não há credenciais Cloudflare disponíveis nesta sessão de desenvolvimento;
o código foi preparado, mas o serviço público não foi atualizado aqui.
Convites 2.x não são convertidos automaticamente: crie um convite 3.0 uma vez.

## Build e verificações

```sh
python scripts/build_multiplayer3.py --native-server build/v3-server
```

Servidor reaproveitado do pacote publicado v2.0.5-dev.4, com hashes/proveniência
verificados por build_native_server.verify_package: 0.2.20-bbs-free2, protocolo 6.
O cliente é o openOMSI oficial instalado, preferencialmente 0.2.25 ou posterior
com suporte BCS nativo; compatibilidade posterior depende do protocolo.

Checks: concorrência de 20 claims (um host), mapas independentes, expiração,
fencing/release antigos, convite estável, edição de clock protegida, manutenção
2.x, situação UTF-16, preservação dos argumentos e Lua, offsets -3/-8 e DST,
readiness/world e ônibus livres; Go test/vet, build Windows x64.

## Pendências antes de anunciar versão pública

Publicar o Worker; testar Windows/BCS atualizado em dois PCs com o mesmo mapa,
verificar F9/avaliação nativa, passageiros/tabela e relógio da empresa; testar mapa
separado e saída do host. Verificar o carregamento de Lua no diretório de conteúdo
real e que nenhuma configuração local ativa outro bbs.lua simultaneamente.
O teste de lógica e a compilação não comprovam o fluxo real PeDePe/jogo.
BCS e openOMSI continuam sujeitos às limitações da integração nativa beta.
