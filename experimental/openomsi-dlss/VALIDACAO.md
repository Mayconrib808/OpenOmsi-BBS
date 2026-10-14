# Validação da adaptação — 14/10/2026

O código foi adaptado e verificado. O executável Windows de distribuição ainda
não foi gerado, e o processamento em uma GPU RTX permanece pendente.

| Verificação | Resultado |
| --- | --- |
| Base oficial 0.2.50, revisão e hashes dos 26 arquivos alterados | Passou |
| Aplicação do patch em uma cópia nova da base | Passou |
| Protocolo de rede oficial 7 | Preservado; código de omsi-net não foi alterado |
| Renderizador, jogo e launcher-core: cargo check em Linux | Passou |
| Renderizador, jogo e launcher-core: cargo check para Windows x64/GNU | Passou, incluindo os trechos de D3D12/Streamline e o wgpu adaptado |
| Modos de DLSS, jitter e funcionamento sem backend compatível | 3 testes passaram |
| Validação Naga dos shaders e tradução para GLSL | 2 testes passaram |
| Salvar e reler DLSS no launcher | 1 teste passou |
| Salvar e reler DLSS nas configurações do jogo | 1 teste passou |
| Atualizador, incluindo identificação da edição de teste | 8 testes passaram |
| Sintaxe do JavaScript do launcher | Passou |
| Sintaxe do script PowerShell, com BOM para PowerShell 5.1 | Passou pelo parser; não foi executado em Windows |
| Workflow de teste e script Python de compilação | Sintaxe e estrutura verificadas |
| Build release Windows/MSVC e recompilação da interface Tauri | Pendente |
| Execução do cliente Windows, --help/--version e verificação do pacote | Pendente; incluída no script de compilação |
| Imagem, movimento e recuperação do DLSS/DLAA em RTX | Pendente |
| Successful NR frames aumentando no RenoDX/Swapper | Pendente |
| Fechar/reabrir com configurações e ReShade persistidos | Pendente |
| Viagem multiplayer pela Bridge e finalização pelo BCS | Pendente |

Total: 15 testes de regressão focados passaram. Eles foram executados em Linux.
O cargo check para Windows verifica os trechos de código Windows, mas não
executa o jogo e não substitui a geração nem os testes do executável final.

O teste do atualizador que usa um servidor HTTP local inicialmente recebeu
502 porque a configuração de proxy do ambiente encaminhou a conexão local.
Ele passou ao executar o mesmo binário de testes sem proxy, apenas para esse
servidor local. Nenhuma mudança de código foi necessária.

A tentativa de usar cargo-xwin/MSVC parou no download das ferramentas de Visual
Studio: a requisição ao manifesto da Microsoft recebeu conexão recusada.
A verificação Windows/GNU usou MinGW e compilou também as dependências de
DirectX 12. O único aviso do wgpu adaptado é a função de factory_media sem uso,
esperado depois da retirada da chamada que motivou a alteração para ReShade.

A revisão automática bloqueou a publicação do ramo de teste no GitHub por
considerar que a autorização para desenvolver não incluía publicação externa
e que o destino não estava verificado. Nenhum envio alternativo foi feito.
Depois disso, os metadados do repositório Mayconrib808/OpenOmsi-BBS foram
conferidos. O envio continua aguardando autorização explícita do usuário.

O processo de compilação Windows está pronto em scripts/build_openomsi_dlss.py
e .github/workflows/openomsi-dlss-test.yml. Ele verifica as opções CLI exigidas
pela Bridge, a identidade da edição, o marcador OMSI_PLUGIN_HOST32 e a alteração
de wgpu antes de empacotar o cliente.
