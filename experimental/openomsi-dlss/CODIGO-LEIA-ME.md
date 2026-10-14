# openOMSI 0.2.50 com DLSS/DLAA — código de teste

Esta adaptação usa o cliente oficial 0.2.50, protocolo 7, com a integração pública
de DLSS Super Resolution e DLAA do fork Criisko/openOMSI 0.2.314. O efeito DLSS 5
Neural Rendering depende da instalação separada do RenoDX/DLSS 5 Swapper.

Este pacote contém código, instalador e instruções de compilação. Ainda não
contém um executável Windows validado. O estado dos testes está em VALIDACAO.md.

## O que mudou

- Integração Streamline/D3D12, vetores de movimento, jitter e histórico por quadro.
- Composição do HUD na resolução da janela depois do DLSS.
- DLAA e modos Quality, Balanced, Performance e Ultra Performance no launcher.
- Leitura, gravação e recuperação das configurações de DLSS.
- Funcionamento sem DLSS em plataformas, GPUs ou instalações sem suporte.
- Alteração isolada no wgpu para testar a compatibilidade com ReShade.
- Preparação das DLLs NVIDIA e da gravação do ReShade, com cópia de segurança.
- Verificação da identidade da base, do patch e do protocolo 7 durante a preparação.

O código do protocolo de rede permanece o da base oficial. A Bridge continua
escolhendo mapa, ônibus e argumentos da viagem. A edição de teste possui uma
identidade própria, 0.2.50-dlss-test.1, e não é substituída pela atualização oficial
automática.

## Compilar no Windows

Use Python 3, Git, Rust stable e Visual Studio Build Tools com C++ e Windows SDK.
No diretório que contém as pastas scripts e experimental, execute:

```powershell
python scripts/build_openomsi_dlss.py
```

O script prepara uma cópia nova da base oficial, verifica e aplica o patch,
executa regressões, compila o jogo e os dois launchers, verifica as opções usadas
pela Bridge e produz o ZIP em build. Os arquivos auxiliares vêm do pacote oficial
0.2.50, cujo SHA-256 está fixado no manifesto.

O workflow .github/workflows/openomsi-dlss-test.yml prepara a mesma compilação
em um runner Windows. Ele gera um artefato de teste, sem publicar uma release.
A publicação do ramo de teste no GitHub aguarda autorização do usuário.

Após obter o ZIP compilado, siga LEIA-ME.txt. A aprovação de compilação e dos
testes de código não confirma processamento DLSS/NR na RTX nem uma viagem
multiplayer pela Bridge. Esses testes continuam necessários no PC de destino.

## Origem

- Base oficial: https://github.com/openOMSI-org/openOMSI/tree/v0.2.50
- Integração pública de referência: https://github.com/Criisko/openOMSI/tree/v0.2.314
- Bridge de destino: https://github.com/Mayconrib808/OpenOmsi-BBS

As revisões exatas e os hashes estão em upstream.json. Os créditos e licenças
estão em CREDITS-DLSS.txt e nos arquivos LICENSE do pacote de código.
