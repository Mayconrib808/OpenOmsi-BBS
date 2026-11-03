# Diretório da empresa — correção P16.1 (03/11/2026)

- Corrige HTTP 400 ao publicar a empresa com o HostAgent P16. A validação do diretório ainda exigia protocolo 6, embora a Bridge aceite 6, 7, 14, 15 e 16.
- Aceita somente esses cinco números de protocolo. Mantém autenticação, identidade da empresa, limites do perfil e validação dos mapas.
- Preserva os registros, convites, chaves, armazenamento, heartbeats, pedidos de abertura, clima e rotas v3 existentes. Nenhuma migração de dados ou troca de chave.
- Acrescenta uma consulta pública de capacidades sem configurações ou credenciais para verificar a versão realmente publicada.
- Correção no serviço online: o HostAgent tenta publicar novamente no próximo ciclo; não exige baixar novamente a Bridge.

Validação: 15 testes de regressão, incluindo publicação, heartbeat, descoberta, pedido de abertura, troca de túnel, rejeição de protocolos inválidos e rotas v3. O workflow verifica as capacidades do serviço após publicar. Uma viagem real depende do servidor e dos jogadores.
