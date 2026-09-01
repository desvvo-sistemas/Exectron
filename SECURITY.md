# Política de Segurança

## Versões suportadas

Correções de segurança são aplicadas sempre na versão mais recente publicada em
[Releases](https://github.com/desvvo-sistemas/Exectron/releases). Não há
backport para versões anteriores.

## Reportando uma vulnerabilidade

**Não abra uma issue pública** para vulnerabilidades. Use
[Security Advisories](https://github.com/desvvo-sistemas/Exectron/security/advisories/new)
para reportar em privado.

Inclua: versão do Exectron, sistema operacional, descrição do problema e o passo
a passo para reproduzir. Respondemos em até 7 dias com uma avaliação inicial.

## Superfície de risco conhecida

O Exectron é uma ferramenta de desenvolvimento local. Por natureza ela:

- **executa comandos arbitrários** definidos pelo usuário nos perfis de projeto,
  com as permissões do usuário que abriu o aplicativo;
- **baixa toolchains** de `go.dev` e `nodejs.org` por HTTPS durante a instalação
  e na instalação de versões do Node;
- **invoca a CLI do Docker** quando a aba Docker é usada, com as permissões do
  usuário sobre o daemon.

Nada disso é executado sem ação do usuário. Relatos que dependem de o atacante
já ter acesso de escrita ao `projects.json` ou de executar código na máquina do
usuário estão fora do escopo — nesse ponto, o comprometimento já aconteceu.
