# Diretório de empresas — compatibilidade com o protocolo 17 (directory-p17.1)

## PT-BR

- O serviço de descoberta de empresas passa a aceitar o protocolo LAN 17 introduzido pelo openOMSI 0.3.49.
- Preserva suporte aos protocolos 6, 7, 14, 15 e 16, incluindo os perfis e sessões já registrados.
- Não converte pacotes de multiplayer entre versões. Cliente e servidor de uma sessão continuam exigindo o mesmo protocolo.
- Mantém a autenticação, o armazenamento existente, as rotas v3 e as verificações de integridade do perfil.
- Validação inclui publicação, heartbeat, consulta, despertar e atualização de sessões com protocolo 17, rejeição de protocolo 18 e formatos inválidos, e verificação pública de capacidades.

**Nota:** isto atualiza apenas o diretório. A versão Bridge 2.2.3.1, o HostAgent e o servidor nativo P17 exigem uma publicação completa e separada.

## English

- Company discovery now accepts LAN protocol 17 (openOMSI 0.3.49) alongside 6/7/14/15/16.
- Existing authentication, persistence, sessions and v3 routes are preserved.
- This is metadata discovery only, not a network protocol translator; paired clients and servers must still agree on their wire protocol.
- The Bridge 2.2.3.1 runtime and P17 HostAgent are separate release gates.
