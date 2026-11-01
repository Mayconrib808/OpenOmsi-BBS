# Política permanente de changelogs — OpenOMSI BCS Bridge

**Regra de release:** TODA atualização publicada, incluindo versões menores, hotfixes e correções emergenciais, deve trazer changelog próprio. Não abrir exceções por tamanho da mudança.

## Antes de publicar
1. Criar `docs/releases/<versão>.md` com mudanças reais, correções, compatibilidade e limitações.
2. Criar seção equivalente no painel de changelog do Setup (localizado em `source/setup_changelog_windows.go`).
3. Atualizar o marcador `changelogSeenVersion` para a versão publicada.
4. Manter histórico cumulativo com **todas** as releases e hotfixes ainda não visualizadas, desde o último changelog confirmado pelo usuário; nunca pular hotfixes ou repetir releases já lidas.
5. Abrir automaticamente o changelog após a instalação e preservar o painel navegável existente, sem abrir Bloco de Notas.
6. Incluir textos pt/en/de do Setup. Não assumir que novas mecânicas exclusivas da Bridge exigem tradução para os 29 idiomas do projeto OpenOMSI.
7. Fazer os testes de UI e do algoritmo cumulativo antes de assinar o manifesto e enviar para o `Buscar atualizações`.
8. Jamais anunciar uma versão como publicada antes de o deploy e a verificação do manifesto assinado e dos arquivos ao vivo terem sido concluídos com sucesso.

Esta política faz parte do fluxo de lançamento da Bridge e se aplica também a novas hotfixes.
