<p align="center">
  <img src="docs/assets/readme-banner.png" alt="OpenOmsi + BBS — suas viagens do BBS no openOMSI" width="100%">
</p>

<p align="center">
  <a href="https://github.com/Mayconrib808/OpenOmsi-BBS/releases/tag/v2.0.4"><img alt="Versão 2.0.4" src="https://img.shields.io/badge/version-2.0.4-f47f30?style=for-the-badge"></a>
  <a href="https://github.com/Mayconrib808/OpenOmsi-BBS/actions/workflows/source-checks.yml"><img alt="Verificações de compilação no Linux e Windows" src="https://img.shields.io/github/actions/workflow/status/Mayconrib808/OpenOmsi-BBS/source-checks.yml?branch=main&style=for-the-badge&label=build"></a>
  <a href="https://github.com/Mayconrib808/OpenOmsi-BBS/releases"><img alt="Pacote para Windows" src="https://img.shields.io/badge/platform-Windows-388bfd?style=for-the-badge"></a>
  <a href="LICENSE"><img alt="Licença MIT" src="https://img.shields.io/github/license/Mayconrib808/OpenOmsi-BBS?style=for-the-badge&color=78a81d"></a>
</p>

<p align="center">
  <a href="https://mayconrib808.github.io/OpenOmsi-BBS/">Site do projeto</a> ·
  <b><a href="https://github.com/Mayconrib808/OpenOmsi-BBS/releases">Releases</a></b> ·
  <a href="#instalação">Como instalar</a> ·
  <a href="#documentação">Documentação</a> ·
  <a href="SUPPORT.md">Suporte</a> ·
  <a href="README.md">English</a>
</p>

# OpenOmsi + BBS — integração entre OpenOMSI e Bus Company Simulator

**OpenOmsi + BBS** conecta o **openOMSI** ao **Bus Company Simulator / Busbetrieb-Simulator (BCS/BBS)** no Windows. Escolha a viagem no BBS e abra o openOMSI com o ônibus e os horários preparados pela ponte. O multiplayer da empresa é opcional.

> [!WARNING]
> **Projeto experimental da comunidade.** A integração principal e o ingresso em uma sessão foram observados em viagens reais. A visualização completa entre dois jogadores e a equivalência de avaliação/pagamento ainda precisam de testes. Veja o [registro da validação](docs/VALIDATION.md).

> [!IMPORTANT]
> Você precisa ter **OMSI 2**, **BCS/BBS** e **openOMSI** instalados. Jogos, addons pagos e o plugin original do BBS são instalados separadamente.

## Releases

**[Baixar OpenOmsi + BBS 2.0.4.zip](https://github.com/Mayconrib808/OpenOmsi-BBS/releases/download/v2.0.4/OpenOmsi.%2B.BBS.2.0.4.zip)**

| Pacote | Versão | Download |
| --- | --- | --- |
| Pacote completo para Windows | **2.0.4 · versão atual** | [OpenOmsi + BBS 2.0.4.zip](https://github.com/Mayconrib808/OpenOmsi-BBS/releases/download/v2.0.4/OpenOmsi.%2B.BBS.2.0.4.zip) |
| Verificação SHA-256 | 2.0.4 | [Checksum](https://github.com/Mayconrib808/OpenOmsi-BBS/releases/download/v2.0.4/OpenOmsi.%2B.BBS.2.0.4.zip.sha256) |
| Versão anterior | 2.0.3 | [Release 2.0.3](https://github.com/Mayconrib808/OpenOmsi-BBS/releases/tag/v2.0.3) |
| Histórico e notas | Versões publicadas | [Abrir Releases](https://github.com/Mayconrib808/OpenOmsi-BBS/releases) |

O ZIP completo contém **Setup.exe**, **CompanyHost.exe**, **HostAgent.exe**, lançador da ponte, fachada de compatibilidade, auxiliar de 32 bits instalado automaticamente, servidor dedicado da bridge, tutorial offline, código-fonte, créditos e licenças. **Para jogar, você não precisa baixar outro auxiliar nem instalar Go ou Python.**

**Atualização de 8 de outubro de 2026:** a 2.0.4 corrige a seleção segura de viagens em mapas onde BCS e OMSI usam nomes diferentes para rota/destino, incluindo perfis com `[station_typ2]` e rotas do BCS com pontos intermediários. Também preserva os segundos originais do horário do OMSI quando o BCS informa apenas `HH:MM`, evitando offsets artificiais. [Notas da 2.0.4](https://github.com/Mayconrib808/OpenOmsi-BBS/releases/tag/v2.0.4) · [Antivírus](docs/ANTIVIRUS.md) · [Changelog](CHANGELOG.md).

Em **Assets**, escolha o ZIP para Windows com esse nome. Os arquivos automáticos “Source code” do GitHub contêm apenas o código-fonte e precisam ser compilados.

## O que a ponte oferece

| Recurso | O que faz |
| --- | --- |
| **Setup com janela** | Escolha os jogos e o perfil da empresa e clique em **Salvar e ativar**. |
| **Configuração salva** | Caminhos, nome e cópia do perfil ficam salvos entre atualizações. |
| **Início pelo BBS** | Abre o OpenOMSI com o ônibus e a viagem preparados pela ponte. |
| **Multiplayer da empresa** | Procura a sessão da empresa. Ônibus privados ou desconhecidos no host não bloqueiam a entrada. |
| **Servidor automático** | O HostAgent abre o servidor incluído quando necessário; substitutos de ônibus são opcionais. |
| **Relógio da empresa** | A sessão acompanha o fuso e a mudança de horário configurados pela empresa. |
| **Perfil para os colegas** | O host publica o perfil com o endereço acessível da sessão. |
| **Diagnóstico local** | **Coletar logs** gera um ZIP para investigar problemas. |

## Instalação

1. Instale **OMSI 2**, **BCS/BBS** e [**OpenOMSI**](https://github.com/openOMSI-Project/openOMSI/releases) separadamente, com o mapa da empresa e o ônibus que você dirige.
2. Baixe o ZIP completo e **extraia tudo em uma pasta fixa**.
3. Com os jogos fechados, abra **Setup.exe** e escolha a pasta do OMSI 2 original e o `openomsi.exe`.
4. O multiplayer é opcional. Para jogar sem ele, deixe **Ativar multiplayer da empresa** desmarcado. Para entrar na empresa, marque a caixa, selecione o perfil ou link HTTPS fornecido pelo administrador e digite seu nome.
5. Clique em **Salvar e ativar** e aceite a solicitação de permissão do Windows.
6. No BCS/BBS, deixe **“Iniciar o OMSI mais depressa” desmarcado**. No OpenOMSI, desative a sincronização com o relógio real.
7. Inicie a viagem normalmente pelo BBS. Se ativou o multiplayer, aguarde o servidor da empresa ficar pronto.

**O jogador não precisa editar JSON nem abrir servidor no próprio PC.** O administrador fornece o perfil e mantém a sessão. Instalar os ônibus dos colegas é opcional: serve para melhorar a representação e o interior quando o modelo existe localmente.

Para desligar apenas o multiplayer, feche os jogos, desmarque **Ativar multiplayer da empresa** e clique novamente em **Salvar e ativar**. A ponte permanece ativa. **Desativar ponte** desliga a integração inteira.

No ponto final: **F9 → aguarde pelo menos dois segundos → finalize no BCS/BBS com o OpenOMSI aberto**.

Para voltar ao OMSI original, feche os jogos e clique em **Desativar ponte**. O guia offline está em **TUTORIAL.html**. [Passo a passo simples do jogador →](docs/GUIA_JOGADOR.md)

## Antivírus / Microsoft Defender

Um pacote de desenvolvimento anterior da 2.0.3 teve um alerta relatado como `Trojan:Win32/Wacatac.B!ml` no auxiliar de 32 bits. Na 2.0.3 o auxiliar possui recursos explícitos de produto/autor/versão/descrição/ícone, preservando a identidade normal da compilação Go e sem empacotador ou ofuscação. O Setup atualiza auxiliares conhecidos e restaura o anterior se a ativação falhar. A CI da release analisa o ZIP candidato e os executáveis incluídos com Microsoft Defender. Isso melhora a transparência e reduz a chance de falso positivo, mas nenhum teste do projeto garante a mesma classificação em todos os PCs. Veja [ANTIVIRUS.md](docs/ANTIVIRUS.md).

## Compatibilidade e testes

A referência de jogo testada é **Windows x64 + OpenOMSI 0.2.0 + BCS/BBS 5.0.0.1**. A bridge verifica os recursos necessários da versão escolhida e o protocolo multiplayer compatível. Os executáveis oficiais 0.2.0 e 0.2.11 já passaram pelas verificações de requisitos do projeto; uma viagem real com 0.2.11 ainda precisa de validação.

Em **07/10/2026**, uma viagem em **Carrão City, linha N407**, com **Caio Apache VIP I OF 1721 manual**, confirmou o ingresso no servidor, identificação do ônibus, chat, passageiros compartilhados e conclusão com **67% de avaliação no BCS**. Houve uma queda seguida de reconexão durante o teste.

A continuidade por **Próxima viagem** e o teste com duas pessoas ainda precisam de validação real. **A penalidade específica por furar sinal vermelho ainda não está disponível**: o OpenOMSI não fornece a informação necessária pela interface da ponte. Os contadores reais de colisões e condução continuam sendo transferidos. [Detalhes das penalidades →](docs/TRAFFIC_PENALTIES.md) · [Registro da validação →](docs/VALIDATION.md)

## Documentação

| Guia | Conteúdo |
| --- | --- |
| [Guia simples do jogador](docs/GUIA_JOGADOR.md) | Configurar uma vez, jogar e atualizar |
| [Multiplayer](docs/MULTIPLAYER.md) | Perfis, sessões e tarefas do administrador |
| [Servidor automático](docs/AUTO_HOST.md) | Host por demanda e configuração do diretório |
| [Relógio da empresa](docs/COMPANY_CLOCK.md) | Hospedar com data e hora sincronizadas |
| [Antivírus](docs/ANTIVIRUS.md) | Defender e solução de falso positivo |
| [Penalidades de trânsito](docs/TRAFFIC_PENALTIES.md) | Dados transferidos e limite do sinal vermelho |
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

No multiplayer configurado com relógio da empresa, a data automática acompanha esse relógio. Fora desse modo, ela segue a data local do Windows. A tradução dos contadores pode influenciar a avaliação; não há promessa de telemetria completa ou pagamento idêntico ao OMSI original.

</details>

## Contribuir

Problemas reproduzíveis, correções, documentação e resultados de testes são bem-vindos. Leia [CONTRIBUTING.md](CONTRIBUTING.md) e o [Código de Conduta](CODE_OF_CONDUCT.md).

Para verificar o código, instale **Go 1.23.2** e **Python 3.10+** e execute:

```sh
python3 scripts/build.py --check-only
```

A compilação completa da release também usa o servidor nativo fixado descrito em [BUILD.md](docs/BUILD.md). O resultado fica em `dist/`.

## Suporte

[Relatar um problema](https://github.com/Mayconrib808/OpenOmsi-BBS/issues/new/choose) · [Sugerir um recurso](https://github.com/Mayconrib808/OpenOmsi-BBS/issues/new/choose) · [Política de segurança](SECURITY.md)

Contato com **Mayconrib808** no Discord: **`.zmaycon.`**, com os dois pontos. **Coletar logs** no Setup gera um diagnóstico local; [revise os arquivos antes de compartilhar](SUPPORT.md). Nada é enviado automaticamente.

## Créditos e licença

Mantido e testado por **Mayconrib808**. Implementação e revisão assistidas por **OpenAI Codex**, creditado como ferramenta de desenvolvimento.

Código próprio, documentação e arte do projeto sob [**MIT**](LICENSE). O auxiliar adapta o protocolo e a interface MIT do openOMSI, com o copyright **2026 usonskyyyy** e [o aviso original](source/reference/OPENOMSI_LICENSE.txt) preservados. Os avisos Go acompanham os binários. Veja [todos os créditos](CREDITS.md) e [os avisos de terceiros](THIRD_PARTY_NOTICES.md).

Projeto independente, sem vínculo, aprovação, patrocínio ou suporte oficial da PeDePe GbR, openOMSI Project, MR-Software GbR, Aerosoft GmbH ou Valve.
