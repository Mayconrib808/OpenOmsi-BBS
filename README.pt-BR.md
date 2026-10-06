# openOMSI BBS Bridge

Ponte experimental e não oficial entre **openOMSI** e **Bus Company Simulator / Busbetrieb-Simulator (BCS/BBS)**, mantida por **Mayconrib808**.

[English](README.md) · [Instalação](docs/INSTALL.md) · [Compilação](docs/BUILD.md) · [Créditos](CREDITS.md)

**Este repositório contém o código da v1.1.2, sem uma versão completa pronta para jogar.** Os três programas Go podem ser compilados. O auxiliar customizado de 32 bits do pacote anterior não foi enviado: sua origem exata e licença de redistribuição ainda precisam ser confirmadas. Veja [a situação do auxiliar](docs/HELPER_PROVENANCE.md).

## Projeto independente

Não existe vínculo, aprovação, patrocínio ou suporte oficial da PeDePe GbR, openOMSI Project, MR-Software GbR, Aerosoft GmbH ou Valve. Os nomes identificam os programas com os quais a ponte busca compatibilidade.

O repositório contém nosso código, exemplos fictícios para testes e uma referência de protocolo do openOMSI sob licença MIT, com os avisos originais preservados. Não contém executáveis, DLLs, JARs, plugins, mapas, ônibus ou DLCs proprietários, nem seus logs de conta. OMSI 2, BCS/BBS e openOMSI devem ser instalados separadamente.

**A ponte não aplica patches nem substitui os executáveis proprietários do OMSI 2 ou do BCS/BBS. Ela grava dados de execução:** sincroniza o perfil local `Drivers/bbs.odr`, cria arquivos próprios e, na ativação, configura um redirecionamento específico no Registro do Windows. Por isso não afirmamos que "nenhum arquivo da PeDePe é modificado". Veja [o que muda no computador](docs/RUNTIME_EFFECTS.md).

Os créditos e avisos não representam autorização da PeDePe nem uma garantia de cumprimento das regras online do BCS/BBS. As licenças e os termos dos produtos continuam aplicáveis.

## Funcionamento e requisitos

A ponte tenta identificar a viagem escolhida no log local do BCS/BBS, iniciar o openOMSI com mapa, ônibus, pintura e horário correspondentes, oferecer as interfaces locais de compatibilidade e traduzir os contadores salvos durante a viagem.

A referência desta importação é **Windows + openOMSI 0.2.0 x64 + BCS/BBS 5.0.0.1**. Atualizações podem alterar a compatibilidade. O auxiliar nativo necessário não está neste repositório. Não prometemos funcionamento em todos os mapas, viagens ou versões.

Em um pacote completo compatível, deixe **"Iniciar o OMSI mais depressa" desmarcado** no BCS/BBS e desative a sincronização do relógio real no openOMSI. No ponto final: **F9 → aguarde pelo menos dois segundos → finalize no BCS/BBS com o jogo aberto**.

O pacote importado registra problema de carregamento de Berlin-Spandau no openOMSI 0.2.0, ainda sem correção por esta ponte. A data automática segue a data local do Windows; o log disponível não informa de forma confiável a data da empresa.

## Código e verificações

Com Go 1.23.2 e Python 3.10+:

```sh
python3 scripts/build.py --check-only
python3 scripts/build.py
```

No Windows: `py -3 scripts/build.py` ou `source/BUILD.cmd`. Os testes, a análise Go, a compilação dos três executáveis e a conferência do layout de memória podem rodar sem possuir o jogo. A compilação gera componentes para desenvolvimento em `dist/source-build`, sem auxiliar e sem ZIP instalável. A integração real do painel e da avaliação continua dependendo de testes no Windows com os produtos legítimos.

Código próprio sob [MIT](LICENSE); referências e dependências mantêm [suas licenças e avisos](THIRD_PARTY_NOTICES.md). Créditos de coordenação e testes: **Mayconrib808**. Implementação e organização assistidas por **OpenAI Codex**.

Problemas da ponte podem ser relatados nas Issues ou no contato já usado pelo projeto, **`.zmaycon.`** no Discord, com os pontos nas duas pontas. [Revise os dados de diagnóstico antes de compartilhar](SUPPORT.md).
