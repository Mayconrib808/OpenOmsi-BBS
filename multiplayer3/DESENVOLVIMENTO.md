# BBS+OpenOmsi Multiplayer 3.0.2-dev.1

Base recuperada da 3.0.1 e correções de lançamento/dados reais portadas da 2.1.5.
O cliente padrão para uma nova instalação é o openOMSI 0.2.29; um cliente existente configurado é preservado.

- Setup PT-BR/EN/DE: empresa ou convite, detecção de OMSI pelo Steam, pastas avançadas opcionais.
- Abrir carrega dados; não ativa nem verifica o BCS. A configuração é um clique explícito, exige BCS fechado e cria backup.
- A edição de config.cfg usa o formato observado, não uma API oficial. Formatos desconhecidos são recusados e oferecem a pasta exata na interface.
- Lua da PeDePe encaminhado byte por byte, motorista com contadores realmente salvos e proteção contra mudança de identidade/arquivo.
- Viagens BCS completas usam autostart e expansão da situação para inicialização de timetable/IBIS; saves manuais e probes não são convertidos.
- Auxiliar próprio da 3.x na raiz OMSI; o auxiliar da 2.x, bbs.start, JAR, DLL e Omsi.exe ficam preservados.
- Convite por empresa, lease por empresa/mapa, host automático, portas dinâmicas, sincronização de relógio e validação de capacidades nativas.
- Preferência de atualização independente do idioma; busca manual sempre disponível. Ed25519, SHA-256, produto/canal e origem separados da 2.x.
- Atualizador transacional só substitui seus componentes; configuração e documentos não são pré-requisitos nem são substituídos.

## Validação

`go test -race ./...` e `go vet ./...`; workflow próprio executa os testes em Windows e gera os executáveis sem recompilar Rust.
`python scripts/build_preview.py --server CAMINHO_SERVIDOR_3.0.1 --output SAIDA` verifica os hashes do servidor já compilado.
O ZIP completo contém a fonte MIT da integração e a licença do runtime openOMSI.

## Antes da versão pública

1. Implementar Próxima viagem por uma interface permitida e confirmar os requisitos da PeDePe; não portar o patch de JAR da 2.x.
2. Validar BCS real, finalizar viagem, troca de mapa, host/guest e saída do host em pelo menos dois computadores.
3. Publicar e testar o canal Cloudflare específico da 3.x depois da validação, com assinatura. O canal 2.x não é usado nem alterado.
4. Migrar as opções da empresa para temperatura e limite de passageiros por mapa.

O servidor empacotado mantém a proveniência bbs-server.json da 3.0.1 (0.2.20-bbs-free2). Não é apresentado como o cliente 0.2.29 nem como a última versão oficial.
