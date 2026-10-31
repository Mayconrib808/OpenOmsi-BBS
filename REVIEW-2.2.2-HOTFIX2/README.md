# OpenOMSI 2.2.2 Hotfix 2 — código separado para revisão

Código **nativo do OpenOMSI**, exatamente correspondente à compilação Windows usada no Buscar atualizações. A árvore do código foi comparada com `SOURCE.json` e com o arquivo de fontes do artefato; os bytes e a árvore Git coincidem. O arquivo de fontes tem SHA-256 `e83115d0e0523b8667f6d73e664e536273acd29f331f89ff071affc36e325bd8`.

O pedido para abrir a porta, os agradecimentos e as reclamações **estão incluídos** na parte 03. As **melhorias no trânsito têm uma parte própria, 04**.

| Parte | Código completo e patch |
| --- | --- |
| Eventos do BCS: efeitos e veículos de emergência | [01-bcs-events](01-bcs-events/README.md) |
| Fiscais: aparência, assento, patrulha e retorno | [02-inspectors](02-inspectors/README.md) |
| Passageiros: falas, portas, embarque e desembarque | [03-passengers](03-passengers/README.md) |
| Melhorias no trânsito: densidade, vias longas e multiplayer | [04-traffic-improvements](04-traffic-improvements/README.md) |
| Configurações, menus, protocolo e dependências comuns | [00-shared-integration](00-shared-integration/README.md) |

O código integral de cada arquivo está na raiz deste branch, nos caminhos apontados por cada parte. Os patches contêm todo o delta dos arquivos atribuídos à parte, não apenas os trechos novos da hotfix. Suporte cooperativo, guarda-chuvas e outras mudanças anteriores que compartilham arquivos permanecem identificados como contexto cumulativo. Não há código dos aplicativos externos da Bridge, Setup ou hosts neste branch.

## Revisão antes de enviar ao upstream

[REVIEW.md](REVIEW.md) confronta o código com os comentários dos PRs #3427, #3428 e #3429 e registra os problemas observados. **Este é um pacote de referência auditado, não uma declaração de que o código está livre de bugs ou pronto para merge.** Existem pendências de desempenho, opções, traduções e regressões específicas. Nenhum resultado de FPS ou de viagem em dois PCs foi inventado.

[VALIDATION.md](VALIDATION.md) separa compilação/testes Windows existentes, verificação nova desta extração e testes no jogo que continuam pendentes. Este trabalho publica fontes e documentação; não altera a versão oferecida pelo atualizador.

## Origem exata

- Base oficial 0.3.46: `17be22987d7c01c813a1473e7696fc71fb3b1d89`.
- Commit publicado com a árvore exata do código: `b6c22b1b993b0126ab799c7634105c2f1b9d0d87`.
- Árvore Git do artefato Windows: `990d5300185280c6f209d72ddaa0666a07f9d482`.
- [Compilação nativa e testes](https://github.com/Mayconrib808/OpenOmsi-BBS/actions/runs/41157142308).
- [Empacotamento da Bridge](https://github.com/Mayconrib808/OpenOmsi-BBS/actions/runs/41159417197).
- [Publicação no Buscar atualizações](https://github.com/Mayconrib808/OpenOmsi-BBS/actions/runs/41159937109).
- [Proveniência e hashes](provenance.json); [inventário integral das fontes](source-files.json).

O nome de versão interno continua `0.3.46-coop.15-2.2.2`. O script histórico `apply_2223.py` usa a expressão 2.2.3, mas seus ajustes já foram incorporados à **2.2.2 Hotfix 2** publicada; o registro do artefato e a sequência do atualizador são a referência. Os patches históricos em `ci/release222` já estão materializados neste branch: **não os aplique outra vez**. O antigo diretório `UPSTREAM-REVIEW` documenta a exportação 2.2.1.8; para esta entrega, use este diretório.

## Como usar os patches

As cinco partes particionam o delta **por arquivo**, sem sobreposição. Isso permite revisar cada assunto e reconstruir o conjunto sem conflitos entre patches. Não são quatro implementações independentes: eventos, fiscais e passageiros compartilham estruturas e pontos de integração. As etapas intermediárias não são qualificadas como compiláveis separadamente.

Para examinar/compilar, prefira clonar este branch já materializado. Para reproduzir o delta completo, use uma cópia limpa da base oficial indicada, aplique os caminhos de `series.txt` em ordem com `git apply --index` e confira a árvore final. [verify.py](verify.py) realiza essa prova em um índice temporário, sem modificar a árvore de trabalho.

```sh
git fetch https://github.com/openOMSI-org/openOMSI.git 17be22987d7c01c813a1473e7696fc71fb3b1d89
python3 REVIEW-2.2.2-HOTFIX2/verify.py
```

Não aplique esses patches cegamente ao `main` atual: #3427 já foi incorporado ao upstream e parte das contribuições anteriores está nos outros PRs. O desenvolvedor deve portar as partes necessárias sobre o estado atual, preservando as opções e os testes independentes. O relatório explica as dependências e as lacunas.

## Licenças e ativos

Os avisos/licenças existentes das fontes foram preservados. O workspace upstream usa AGPL-3.0-only; os dados de nomes têm seus próprios avisos e GFDL-1.2 em `crates/omsi-app/src/bcs_events/names-data`. Não se deve tratar todo o pacote como MIT. O gerador das viaturas originais está em `tools/generate_bcs_assets.py`; não é necessário redistribuir veículos proprietários do OMSI para revisar o código.

Para o desenvolvedor: [English code audit](REVIEW.md) and [validation evidence](VALIDATION.md).
