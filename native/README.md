# Servidor com ônibus livres

Derivado do openOMSI 0.2.20, licença MIT. O aplicativo cliente continua usando o protocolo 6 original. Esta modificação não é uma versão oficial do openOMSI.

`free_player_vehicles = 1` permite dirigir ônibus fora da instalação ou da antiga lista do host. O servidor anuncia `free_player_vehicles: true` e uma lista de permissões vazia em `/status`, preservando o menu de ônibus locais dos clientes oficiais. No modo livre, não expulsa jogadores por modelo.

Quando não consegue carregar um ônibus remoto, o servidor usa um modelo local substituto. `fallback_vehicles` é uma preferência opcional de substitutos; falhas nessa preferência passam aos ônibus originais MAN e depois aos demais modelos locais disponíveis. Isso não muda o ônibus que o dono dirige nem transfere arquivos privados. A visualização e entrada no interior dependem dos recursos locais do cliente, conforme o multiplayer original.

O modo padrão do servidor original permanece intacto quando `free_player_vehicles = 0`. Mapas, relógio, clima, passageiros, porta de administração e protocolo de rede conservam os mecanismos originais.

Para reproduzir em Windows x64 com Python, Git, Rust e Visual Studio C++:

```
python scripts/build_native_server.py
python scripts/build.py --native-server build/native-server
```

O script verifica o SHA-256 do arquivo-fonte oficial definido em `upstream.json`, aplica `free-player-vehicles.patch`, verifica cada arquivo alterado, executa os testes de substituição, configuração e rede, compila o servidor e testa sua interface de linha de comando. Inclui o runtime Microsoft local e a biblioteca Steam distribuídos também no pacote upstream. `bbs-server.json` identifica fontes, compilador, commit e os hashes dos arquivos finais.

O pacote não inclui conteúdo do OMSI 2 nem de ônibus particulares. A instalação original do jogo e o mapa continuam necessários no host e nos jogadores.
