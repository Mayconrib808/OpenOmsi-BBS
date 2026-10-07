<p align="center">
  <img src="docs/assets/readme-banner.png" alt="OpenOmsi + BBS — suas viagens do BBS no openOMSI" width="100%">
</p>

<p align="center">
  <a href="https://github.com/Mayconrib808/OpenOmsi-BBS/releases/tag/v1.1.3"><img alt="Versão 1.1.3" src="https://img.shields.io/badge/version-1.1.3-f47f30?style=for-the-badge"></a>
  <a href="https://github.com/Mayconrib808/OpenOmsi-BBS/actions/workflows/source-checks.yml"><img alt="Verificações de compilação no Linux e Windows" src="https://img.shields.io/github/actions/workflow/status/Mayconrib808/OpenOmsi-BBS/source-checks.yml?branch=main&style=for-the-badge&label=build"></a>
  <a href="https://github.com/Mayconrib808/OpenOmsi-BBS/releases"><img alt="Pacote para Windows" src="https://img.shields.io/badge/platform-Windows-388bfd?style=for-the-badge"></a>
  <a href="LICENSE"><img alt="Licença MIT" src="https://img.shields.io/github/license/Mayconrib808/OpenOmsi-BBS?style=for-the-badge&color=78a81d"></a>
</p>

<p align="center">
  <b><a href="https://github.com/Mayconrib808/OpenOmsi-BBS/releases">Releases</a></b> ·
  <a href="#instalação">Como instalar</a> ·
  <a href="#documentação">Documentação</a> ·
  <a href="SUPPORT.md">Suporte</a> ·
  <a href="README.md">English</a>
</p>

**OpenOmsi + BBS** conecta o **openOMSI** ao **Bus Company Simulator / Busbetrieb-Simulator (BCS/BBS)** no Windows. Escolha a viagem no BBS e abra o openOMSI com o ônibus e os horários preparados pela ponte.

> [!WARNING]
> **Projeto experimental da comunidade.** A 1.1.3 passou pelos testes automatizados e por uma viagem real no BBS. Outros mapas e ônibus, além da equivalência completa da avaliação e do pagamento, ainda precisam de testes. Veja o [registro da validação](docs/VALIDATION.md).

> [!IMPORTANT]
> Você precisa ter **OMSI 2**, **BCS/BBS** e **openOMSI** instalados. Os jogos, addons pagos e o plugin original do BBS são instalados separadamente.

## Releases

**[Baixar OpenOmsi + BBS 1.1.3.zip](https://github.com/Mayconrib808/OpenOmsi-BBS/releases/download/v1.1.3/OpenOmsi.%2B.BBS.1.1.3.zip)**

| Pacote | Versão | Download |
| --- | --- | --- |
| Pacote completo para Windows | **1.1.3 · experimental** | [OpenOmsi + BBS 1.1.3.zip](https://github.com/Mayconrib808/OpenOmsi-BBS/releases/download/v1.1.3/OpenOmsi.%2B.BBS.1.1.3.zip) |
| Verificação SHA-256 | 1.1.3 | [Checksum](https://github.com/Mayconrib808/OpenOmsi-BBS/releases/download/v1.1.3/OpenOmsi.%2B.BBS.1.1.3.zip.sha256) |
| Histórico e notas das versões | Versões publicadas | [Abrir Releases](https://github.com/Mayconrib808/OpenOmsi-BBS/releases) |

O ZIP completo contém **Setup.exe**, lançador da ponte, fachada de compatibilidade, auxiliar de 32 bits instalado automaticamente, tutorial offline, código-fonte, créditos e licenças. **Para jogar, você não precisa baixar outro auxiliar nem instalar Go ou Python.**

Em **Assets**, escolha o ZIP para Windows com esse nome. Os arquivos automáticos “Source code” do GitHub contêm o código-fonte e precisam ser compilados.

## O que a ponte oferece

| Recurso | O que faz |
| --- | --- |
| **Um único Setup** | Configura os caminhos, ativa a ponte e restaura a abertura do OMSI original. |
| **Início pelo BBS** | Abre a sessão preparada do openOMSI pelo fluxo habitual de viagem do BBS. |
| **Integração da viagem** | Conecta o plugin original instalado e traduz horários, paradas e bilhetes. |
| **Diagnóstico local** | Gera um ZIP na opção 5 do Setup para investigar problemas. |

## Instalação

A referência testada é **Windows x64 + openOMSI 0.2.0 + BCS/BBS 5.0.0.1**. Confira a compatibilidade antes de trocar o openOMSI por uma versão mais recente.

1. Instale **OMSI 2**, **BCS/BBS** e [**openOMSI**](https://github.com/openOMSI-Project/openOMSI/releases) separadamente.
2. Baixe o ZIP completo e **extraia tudo em uma pasta fixa**.
3. Com os jogos fechados, abra **Setup.exe** e configure os dois caminhos dos jogos.
4. Escolha **2 — Ativar / atualizar** e aceite a solicitação de permissão do Windows.
5. No BCS/BBS, deixe **“Iniciar o OMSI mais depressa” desmarcado**. No openOMSI, desative a sincronização com o relógio real.
6. Inicie a viagem normalmente pelo BBS.

No ponto final: **F9 → aguarde pelo menos dois segundos → finalize no BCS/BBS com o openOMSI aberto**.

Para voltar ao OMSI original, feche os jogos e use a opção **3** do Setup. O guia ilustrado offline está em **TUTORIAL.html**, no ZIP extraído. [Guia detalhado de instalação →](docs/INSTALL.md)

## Viagem testada

Uma viagem real no Windows em **06/10/2026** usou **Carrão City**, linha **2201**, tour **02**, partida **09:20**, com **Caio Apache VIP I OF 1721 manual**. O plugin original carregou, o horário ficou alinhado, seis paradas e oito bilhetes foram gravados, o BBS recebeu a avaliação e concluiu a viagem, e o openOMSI encerrou normalmente.

Esse resultado cobre a combinação testada. O problema conhecido de **carregamento de Berlin-Spandau no openOMSI 0.2.0** continua documentado. [Detalhes da validação e testes pendentes →](docs/VALIDATION.md)

## Documentação

| Guia | Conteúdo |
| --- | --- |
| [Instalação](docs/INSTALL.md) | Caminhos, ativação e restauração do OMSI original |
| [Solução de problemas](docs/TROUBLESHOOTING.md) | Inicialização, painel do plugin e horários |
| [Suporte e diagnóstico](SUPPORT.md) | Como relatar problemas e revisar o diagnóstico |
| [Teste no Windows](docs/TEST_ON_WINDOWS.md) | Verificações reproduzíveis da integração |
| [Compilação](docs/BUILD.md) | Ferramentas, montagem do pacote e testes nativos |
| [Efeitos locais](docs/RUNTIME_EFFECTS.md) | Arquivos e configurações do Windows usados pela ponte |
| [Créditos e licenças](CREDITS.md) | Colaboradores, projetos de origem e avisos |
| [Acessibilidade](ACCESSIBILITY.md) | Interfaces disponíveis e como relatar barreiras |

<details>
<summary><b>Como a ponte funciona e quais dados ela grava</b></summary>

O `Omsi.exe` incluído é uma fachada de compatibilidade deste projeto, não o executável original do jogo. O auxiliar carrega somente a `bbs.dll` original já instalada pelo usuário. Executáveis, DLLs, JARs, plugins, mapas, ônibus, DLCs ou logs reais dos produtos proprietários não são distribuídos.

Os executáveis, JARs e DLLs proprietários originais não são substituídos nem recebem patches. A ponte **grava dados locais**: atualiza `Drivers/bbs.odr`, cria arquivos próprios, instala seu auxiliar e configura um redirecionamento específico no Registro do Windows. Veja [os efeitos locais](docs/RUNTIME_EFFECTS.md) e [o aviso técnico](NOTICE_FOR_PEDEPE.md).

A data automática segue o dia local do Windows. A tradução dos contadores pode influenciar a avaliação; não há promessa de telemetria completa ou pagamento idêntico ao OMSI original.

</details>

## Contribuir

Problemas reproduzíveis, correções, documentação e resultados de testes são bem-vindos. Leia [CONTRIBUTING.md](CONTRIBUTING.md) e o [Código de Conduta](CODE_OF_CONDUCT.md).

Para verificar o código, instale **Go 1.23.2** e **Python 3.10+** e execute:

```sh
python3 scripts/build.py --check-only
```

Para montar o ZIP, execute `python3 scripts/build.py` (no Windows: `py -3 scripts/build.py` ou `source/BUILD.cmd`). O resultado fica em `dist/`. [Instruções completas de compilação →](docs/BUILD.md)

## Suporte

[Relatar um problema](https://github.com/Mayconrib808/OpenOmsi-BBS/issues/new/choose) · [Sugerir um recurso](https://github.com/Mayconrib808/OpenOmsi-BBS/issues/new/choose) · [Política de segurança](SECURITY.md)

Contato com **Mayconrib808** no Discord: **`.zmaycon.`**, com os dois pontos. A opção **5** do Setup gera um diagnóstico local; [revise os arquivos antes de compartilhar](SUPPORT.md). Nada é enviado automaticamente.

## Créditos e licença

Mantido e testado por **Mayconrib808**. Implementação e revisão assistidas por **OpenAI Codex**, creditado como ferramenta de desenvolvimento.

Código próprio, documentação e arte do projeto sob [**MIT**](LICENSE). O auxiliar adapta o protocolo e a interface MIT do openOMSI, com o copyright **2026 usonskyyyy** e [o aviso original](source/reference/OPENOMSI_LICENSE.txt) preservados. Os avisos Go acompanham os binários. Veja [todos os créditos](CREDITS.md) e [os avisos de terceiros](THIRD_PARTY_NOTICES.md).

Projeto independente, sem vínculo, aprovação, patrocínio ou suporte oficial da PeDePe GbR, openOMSI Project, MR-Software GbR, Aerosoft GmbH ou Valve.
