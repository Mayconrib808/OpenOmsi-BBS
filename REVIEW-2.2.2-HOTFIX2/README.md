# OpenOMSI 2.2.2 Hotfix 2 — revisão corrigida para o desenvolvedor

Esta revisão acrescenta as correções de idiomas e opções ao código nativo da Hotfix 2. **É uma revisão posterior ao pacote publicado no Buscar atualizações.** O instalador e o feed de atualização não foram alterados nesta entrega. A fonte original permanece identificada no histórico e em [provenance.json](provenance.json).

As falas de porta, reclamação de ponto perdido, pedido de desembarque e agradecimento usam o catálogo nativo completo: inglês e as 29 traduções do OpenOMSI. São legendas/textos; o áudio continua usando os pacotes de voz instalados. Os novos controles e suas ajudas usam o mesmo catálogo no jogo e no launcher.

| Parte | Código completo e patch |
| --- | --- |
| Eventos do BCS e veículos de emergência | [01-bcs-events](01-bcs-events/README.md) |
| Fiscais: aparência, assento, patrulha e retorno | [02-inspectors](02-inspectors/README.md) |
| Passageiros: falas, portas, embarque e desembarque | [03-passengers](03-passengers/README.md) |
| Melhorias no trânsito: densidade, vias longas e multiplayer | [04-traffic-improvements](04-traffic-improvements/README.md) |
| Configurações, traduções, menus, protocolo e dependências comuns | [00-shared-integration](00-shared-integration/README.md) |

O código integral de cada arquivo está na raiz deste branch. Os patches contêm o delta completo sobre a base oficial 0.3.46, dividido por arquivo sem sobreposição. Eles incluem contexto cumulativo de contribuições anteriores; não são cinco PRs independentes nem devem ser aplicados novamente sobre o código já materializado.

## Opções disponíveis

[OPTIONS.md](OPTIONS.md) descreve os sete novos controles independentes, além dos controles existentes de animações, guarda-chuvas, sinalização e compra de bilhetes. Os novos controles começam desligados nos dois leitores/escritores de configurações e exigem reiniciar o jogo, conforme a ajuda traduzida. A migração preserva escolhas explícitas, inclusive desativadas.

Com recursos desligados, não se inicia a captura BCS sem necessidade, não se carrega a cena correspondente, não se procura ônibus para comentários opcionais, não se consome RNG de tarefas para agradecimentos e não se calcula o comprimento extra de vias. As correções de portas, caminhos e segurança da simulação continuam ativas.

## Código e evidência desta revisão

- Commit de código testado: `44d2e919b505133be401869552c108b03ec48a70`.
- Árvore nativa testada: `43b7d5ffa5b2e3d4c7901399011f21658b6790c0`.
- [Compilação Windows e testes desta revisão](https://github.com/Mayconrib808/OpenOmsi-BBS/actions/runs/41169027280).
- [Validação e limites](VALIDATION.md), [auditoria atualizada](REVIEW.md), [proveniência](provenance.json) e [inventário das fontes](source-files.json).
- Base oficial: `17be22987d7c01c813a1473e7696fc71fb3b1d89`.
- Fonte original da Hotfix 2 publicada: `b6c22b1b993b0126ab799c7634105c2f1b9d0d87`, árvore `990d5300185280c6f209d72ddaa0666a07f9d482`.

[REVIEW.md](REVIEW.md) distingue correções concluídas de verificações ainda pendentes. Há limites conhecidos para testes de desempenho, cenas complexas e viagens em dois PCs; não há afirmação de ausência de bugs ou de ganho de FPS medido.

## Reproduzir os patches

Prefira clonar este branch para examinar ou compilar. Para reconstruir o código nativo a partir da base oficial, aplique os cinco patches listados em [series.txt](series.txt), em ordem, com `git apply --index`. [verify.py](verify.py) prova a reconstrução em um índice temporário e verifica hashes sem modificar o código.

```sh
git fetch https://github.com/openOMSI-org/openOMSI.git 17be22987d7c01c813a1473e7696fc71fb3b1d89
python3 REVIEW-2.2.2-HOTFIX2/verify.py
```

O upstream atual já incorporou contribuições anteriores. Para preparar PRs novos, porte as mudanças necessárias sobre o estado atual, preservando traduções, opções e testes. Os patches históricos em `ci/release222` já estão materializados: não os reaplique. `UPSTREAM-REVIEW` é documentação de uma exportação anterior; esta entrega usa `REVIEW-2.2.2-HOTFIX2`.

## Escopo e licenças

Bridge externa, Setup, HostAgent, CompanyHost, assinatura/atualizador e servidor P14 não fazem parte desta modificação de código nativo. Não há JAR proprietário do BCS nem ativos originais do OMSI 2 nesta entrega. Os avisos existentes foram preservados: workspace AGPL-3.0-only e dados de nomes com seus próprios avisos/GFDL-1.2. O gerador das viaturas originais está em `tools/generate_bcs_assets.py`.
