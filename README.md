# FactoryDev

**FactoryDev** é uma ferramenta de desenvolvimento local com interface web que centraliza as tarefas mais repetitivas do dia-a-dia de um desenvolvedor: gerenciamento de chaves SSH, contas git, repositórios, servidores remotos, containers Docker e informações do sistema — tudo em um único lugar, sem precisar decorar comandos ou abrir terminais para cada tarefa.

---

## O que resolve

| Problema | Solução no FactoryDev |
|---|---|
| Gerenciar múltiplas chaves SSH (pessoal, trabalho, cliente) | **Key Manager** — gera, importa e organiza chaves ed25519/RSA/ECDSA |
| Configurar SSH para vários hosts/contas GitHub/GitLab | **SSH/Git Accounts** — aplica `~/.ssh/config` automaticamente |
| Clonar repos com a conta certa | **Repositories** — clona via SSH com a conta selecionada, status ao vivo |
| Manter identidades git separadas por projeto | **Git Identities** — configura `user.name`/`email` e commit signing por diretório via `includeIf` |
| Conectar e enviar arquivos para servidores | **Server Manager** — testa SSH, abre terminal, envia via SCP |
| Monitorar CPU/RAM/disco da máquina | **System Info** — dashboard atualizado a cada 5s |
| Subir containers de dev rápido | **Docker Manager** — inicia Docker, lança PostgreSQL/MySQL/Redis/MongoDB/Adminer com um clique |

---

## Funcionalidades

### Key Manager
- Gera chaves SSH: `ed25519`, `RSA` (2048/3072/4096 bits), `ECDSA`
- Importa chaves existentes do disco
- Regenera chave pública a partir da privada
- Exporta chave pública em Base64

### SSH / Git Accounts
- Cria contas vinculando host, chave e identidade git
- Aplica configuração no `~/.ssh/config` com preview de diff
- Testa conexão SSH (com feedback em tempo real)
- Importa contas existentes de um `~/.ssh/config` já configurado

### Repositories
- Clona repositórios via SSH com progresso assíncrono
- Escaneia diretórios para importar repos git já existentes
- Status ao vivo por repositório (branch atual, arquivos modificados)
- Tabs por repositório: **Overview** (último commit, pull, novo branch, terminal), **Config** (git local), **Commits** (histórico com paginação)
- Abre terminal no diretório do repo (macOS: Terminal.app, Linux: detecta automaticamente)

### Git Identities
- Cria perfis de identidade (nome + e-mail + chave para signing)
- Configura `~/.gitconfig` global (user, gpg, signingkey)
- Gerencia regras `includeIf gitdir:` para identidades por diretório
- Configura commit signing via SSH com instruções para GitHub/GitLab

### Server Manager
- Cadastra servidores SSH com host, porta, usuário, chave e tags
- Testa conexão com feedback assíncrono
- Abre terminal SSH diretamente (usa Terminal.app no macOS)
- Envia arquivos via SCP com progresso

### System Info
- Dashboard com CPU (por core + overall), RAM, Swap, discos e interfaces de rede
- Atualização automática a cada 5s
- Go runtime info (goroutines, heap)
- Editar hostname (Linux com systemd)

### Docker Manager
- Detecta e inicia o Docker Desktop (macOS/Windows) com um clique
- Status do daemon em tempo real no header (versão, containers rodando, memória total)
- Lista containers com ações: iniciar, parar, reiniciar, remover
- Logs e detalhes de cada container
- Gerencia imagens (listar, pull assíncrono, remover)
- Lança containers a partir de templates pré-configurados:
  - PostgreSQL 16, MySQL 8, Redis 7, MongoDB 7, Adminer
  - Ou configuração totalmente manual

---

## Tecnologias

- **Backend:** Go 1.24 + [chi](https://github.com/go-chi/chi) (roteamento)
- **Frontend:** [HTMX](https://htmx.org) + [Alpine.js](https://alpinejs.dev) (sem build step)
- **Persistência:** JSON local em `~/.fdev/state.json`
- **Sistema:** [gopsutil](https://github.com/shirou/gopsutil) (métricas), [docker/docker](https://github.com/docker/docker) (SDK Docker)
- **Distribuição:** binário único com assets embutidos via `go:embed`

---

## Screenshots

> A interface roda em `http://localhost:7337` por padrão.

---

## Licença

MIT
