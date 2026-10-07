# Descoberta do projeto e manutenção das páginas

O site do projeto é **https://mayconrib808.github.io/OpenOmsi-BBS/**. A versão atual e os downloads oficiais ficam nas [Releases](https://github.com/Mayconrib808/OpenOmsi-BBS/releases). Estas orientações tratam de apresentação e descoberta; nenhuma configuração garante posição no Google, Trending ou recomendações do GitHub.

## O que foi aplicado

- Apresentação em português no site e README em inglês e português, com o nome OpenOmsi + BBS e os nomes completos dos jogos.
- Links diretos para a versão atual, tutorial, histórico de versões, código, suporte e contribuição.
- Títulos e descrições específicos por página; URLs canônicas, imagens de compartilhamento e favicon.
- Conteúdo HTML acessível sem JavaScript, layout para celular e texto útil sobre instalação, multiplayer opcional e limites da versão.
- Dados estruturados descritivos do código e do histórico; nenhum número de avaliações ou popularidade foi inventado.
- Sitemap com as páginas públicas realmente publicadas e datas correspondentes à atualização do conteúdo.
- Preservação da meta de verificação existente do Google Search Console.

## Search Console — etapa do proprietário

1. Entre na conta que verificou o site no [Google Search Console](https://search.google.com/search-console).
2. Selecione a propriedade de prefixo de URL **https://mayconrib808.github.io/OpenOmsi-BBS/**. Se ela ainda não existir, use esse prefixo e a verificação HTML da conta correspondente. A meta existente não prova, por si só, que a propriedade já foi verificada.
3. Em **Sitemaps**, envie **https://mayconrib808.github.io/OpenOmsi-BBS/sitemap.xml**.
4. Em **Inspeção de URL**, confira a página inicial, `TUTORIAL.html` e `releases.html`; solicite indexação quando necessário.
5. Acompanhe indexação e desempenho. A solicitação e o sitemap não garantem inclusão ou posição nos resultados; repetir pedidos não acelera o processo.

Essa etapa depende da conta do proprietário. Não há envio automático à conta do Search Console nesta alteração.

Um arquivo `robots.txt` só controla rastreamento quando fica na raiz do host. Este projeto está em `/OpenOmsi-BBS/`; criar `/OpenOmsi-BBS/robots.txt` não controlaria o host `mayconrib808.github.io`. Por isso, não foi criado um arquivo nesse lugar. A ausência de bloqueio é conferida separadamente.

## GitHub e divulgação

Mantenha a descrição, o link do site e tópicos que realmente representem o projeto, como OpenOMSI, OMSI 2, BBS/BCS, Windows e multiplayer. Tópicos permitem encontrar repositórios por assunto; não certificam qualidade ou compatibilidade.

Publique versões quando houver mudanças reais. Atualize o changelog e as notas, responda aos relatos e diferencie os testes aprovados dos testes ainda pendentes. Compartilhe o projeto em comunidades pertinentes seguindo as regras de cada comunidade. Não crie estrelas, avaliações, commits ou links artificiais para simular atividade.

## Ao publicar a próxima versão

1. Confirme os testes e o pacote da Release antes de mudar os links de download.
2. Atualize versão, data, links, checksum e novidades em `index.html`, `releases.html`, nos READMEs e no tutorial.
3. Acrescente a versão ao histórico e atualize as descrições e os dados estruturados quando necessário.
4. Atualize `lastmod` no sitemap somente nas páginas cujo conteúdo mudou de forma significativa.
5. Confira os links, a apresentação em celular e desktop e a publicação do GitHub Pages.

## Fontes oficiais consultadas

- [GitHub: tópicos do repositório](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/classifying-your-repository-with-topics)
- [GitHub: conteúdo do README](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-readmes)
- [Google: guia inicial de SEO](https://developers.google.com/search/docs/fundamentals/seo-starter-guide)
- [Google: construção e envio de sitemap](https://developers.google.com/search/docs/crawling-indexing/sitemaps/build-sitemap)
- [Google: solicitação de novo rastreamento](https://developers.google.com/search/docs/crawling-indexing/ask-google-to-recrawl)
- [Google: localização de robots.txt](https://developers.google.com/crawling/docs/robots-txt/create-robots-txt)
