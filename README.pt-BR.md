# openOMSI BBS Bridge

Ponte experimental e não oficial entre **openOMSI** e **Bus Company Simulator / Busbetrieb-Simulator (BCS/BBS)**, mantida por **Mayconrib808**.

[English](README.md) · [Instalação](docs/INSTALL.md) · [Compilação](docs/BUILD.md) · [Créditos](CREDITS.md)

**A v1.1.3 gera um ZIP completo da ponte.** Setup, lançador, fachada de compatibilidade e novo auxiliar de 32 bits são compilados com o código incluído. O auxiliar vem no pacote e é preparado automaticamente pelo Setup. Quem usa não precisa procurar outro download ou instalar ferramentas de desenvolvimento. O auxiliar antigo, cuja origem não foi confirmada, fica excluído.

## Instalar

Tenha OMSI 2, BCS/BBS e **openOMSI 0.2.0 para Windows x64** instalados separadamente. Extraia **todo o ZIP** em uma pasta fixa, abra **Setup.exe**, configure os dois caminhos dos jogos e escolha **2 — Ativar / atualizar**. O pacote traz o guia offline **TUTORIAL.html**.

No BCS/BBS, deixe **“Iniciar o OMSI mais depressa” desmarcado**. Desative a sincronização com o relógio real no openOMSI. No ponto final: **F9 → aguarde pelo menos dois segundos → finalize no BCS/BBS com o openOMSI aberto**. A opção **3** do Setup devolve a inicialização ao OMSI original.

A referência é **BCS/BBS 5.0.0.1**. Berlin-Spandau tem um problema de carregamento registrado no openOMSI 0.2.0; a ponte não corrige esse mapa. A data automática segue a data local do Windows. Veja [as verificações realizadas e seus limites](docs/VALIDATION.md).

## Projeto independente e alterações locais

Não há vínculo, aprovação, patrocínio ou suporte oficial da PeDePe GbR, openOMSI Project, MR-Software GbR, Aerosoft GmbH ou Valve.

Não distribuímos executáveis, DLLs, JARs, plugins, mapas, ônibus, DLCs ou logs reais dos produtos proprietários. O auxiliar carrega somente a `bbs.dll` original já instalada pelo usuário. O `Omsi.exe` incluído é uma fachada feita por este projeto, não o executável do jogo original.

**Os executáveis, JARs e DLLs proprietários originais não são substituídos nem recebem patches. Há gravação de dados locais:** a ponte atualiza `Drivers/bbs.odr`, cria arquivos próprios, coloca seu auxiliar na pasta do OMSI e configura um redirecionamento específico no Registro do Windows. Por isso seria incorreto anunciar que nenhum arquivo da PeDePe é modificado. Veja [os efeitos locais](docs/RUNTIME_EFFECTS.md) e [o aviso técnico](NOTICE_FOR_PEDEPE.md).

Créditos e licenças não representam autorização da PeDePe nem garantia de aceitação pelas regras do BCS/BBS. A tradução dos contadores pode influenciar a avaliação do serviço; não prometemos telemetria completa ou pagamento idêntico ao OMSI original.

## Compilar e baixar o resultado

Com **Go 1.23.2** e **Python 3.10+**:

```sh
python3 scripts/build.py --check-only
python3 scripts/build.py
```

No Windows: `py -3 scripts/build.py` ou `source/BUILD.cmd`. A pasta completa, o ZIP e seu checksum são gerados em `dist/`. Para usar o ZIP, Go e Python não são necessários. O processo verifica testes, análise Go, os quatro executáveis para Windows, layout da fachada e integridade do pacote. Os testes nativos com DLL exigem Visual Studio C++ x86 apenas no ambiente de desenvolvimento.

O [GitHub Actions](https://github.com/Mayconrib808/OpenOmsi-BBS/actions) verifica Linux e Windows e anexa o ZIP completo e seu checksum a cada execução Windows bem-sucedida, como **OpenOMSI-BCS-Bridge-v1.1.3**. A visibilidade do repositório e o acesso pela conta GitHub continuam valendo. Isso não publica automaticamente uma Release pública. CI aprovado não substitui uma viagem real com BCS.

Código próprio sob [MIT](LICENSE). O novo auxiliar adapta o protocolo e a interface MIT do openOMSI, com o copyright **2026 usonskyyyy** e [o aviso original](source/reference/OPENOMSI_LICENSE.txt) preservados. Os avisos Go acompanham os binários. Veja [as licenças](THIRD_PARTY_NOTICES.md) e [todos os créditos](CREDITS.md).

Coordenação, requisitos, testes e manutenção: **Mayconrib808**. Implementação e revisão assistidas por **OpenAI Codex**, creditado como ferramenta de desenvolvimento.

Suporte nas Issues ou Discord **`.zmaycon.`**, com os pontos nas duas pontas. A opção 5 do Setup gera diagnóstico local; [revise antes de compartilhar](SUPPORT.md). Nada é enviado automaticamente.
