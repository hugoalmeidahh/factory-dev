# FactoryDev - Contexto do Projeto

## O que é o projeto
O **FactoryDev** é uma ferramenta de desenvolvimento local que fornece uma interface web para centralizar as tarefas mais rotineiras e repetitivas de um desenvolvedor. O objetivo é unificar o gerenciamento de chaves SSH, contas em plataformas Git, clonagem de repositórios, servidores remotos, informações do sistema e containers Docker num único painel, sem a necessidade de decorar dezenas de comandos no terminal.

## Principais Funcionalidades
1. **Key Manager (Gerenciador de Chaves):** Geração (ed25519/RSA/ECDSA), importação, exportação de chaves SSH e recuperação de chaves públicas.
2. **SSH / Git Accounts:** Vincula hosts, chaves SSH e identidades, aplicando as configurações localmente em `~/.ssh/config` e testando a conexão SSH.
3. **Repositories (Repositórios):** Permite clonagem via SSH com progresso ao vivo, visualização de status local (branch, diff) e gestão do histórico de commits. Abertura direta do terminal respectivo.
4. **Git Identities (Identidades do Git):** Permite criar perfis usando as regras `includeIf gitdir:` no `~/.gitconfig` para associar de forma automática o email/nome/assinatura de commit para diretórios específicos.
5. **Server Manager:** Cadastro de servidores remotos para conexão fácil e envio rápido de arquivos via protocolo SCP.
6. **System Info (Métricas):** Dashboard que exibe uso de CPU (por core e geral), RAM, Discos, e redes com atualizações ao vivo.
7. **Docker Manager:** Gerenciamento dos daemon do Docker (iniciar o app Desktop com um clique), containers (play, stop, listagem com logs ao vivo), imagens e templates pré-configurados (PostgreSQL, MySQL, Redis, MongoDB, Adminer).

## Como funciona (Arquitetura Técnica)
O FactoryDev foi pensado para ser rápido, local, seguro e não necessitar de um processo de compilação contínuo (build steps) caso não seja para lançamento.
- **Backend:** Go (Golang) na versão 1.24. Utiliza o roteador `chi`. A ferramenta invoca diretamente binários do sistema e APIs (como Docker SDK e o módulo `gopsutil`).
- **Frontend:** Desenvolvido nativamente em HTML e formatado em arquivos sob a pasta `web/templates`. O motor de interatividade é promovido na sua grande parte por **HTMX** (para invocações dinâmicas sem escrever Javascript manualmente) associado ao **Alpine.js** (para controles visuais rápidos e comportamentos do lado do cliente).
- **Armazenamento de Dados (Persistência):** Todos os cadastros e mapeamentos ficam salvos estritamente de forma local do utilizador em formato JSON (`~/.fdev/state.json`), sem dependência de banco de dados SQL para funcionar, garantindo privacidade a quem o utiliza.
- **Distribuição:** A interface web e os assets estáticos de CSS/JS (localizados na diretoria `web/`) são embutidos usando a instrução `go:embed` e servidos como um único executável binário.

## Estrutura do Repositório (Resumo)
- `cmd/factorydev/`: O diretório do comando principal, local de entrypoint (função main).
- `internal/`: Possui toda a lógica isolada em pacotes Go (`config`, `docker`, `git`, `ssh`, `handler`, etc). Estes pacotes não são exportados publicamente.
- `web/`: Contém arquivos que compõem a UI. `web/static` contém os assets (CSS, JS) e os arquivos estáticos. `web/templates` armazena os blocos e páginas HTML que o servidor Go processa e envia via HTMX para o cliente.
- `Makefile`: Automação da compilação e execução.
