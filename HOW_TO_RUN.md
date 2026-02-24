# Como rodar o FactoryDev

## Pré-requisitos

- **Go 1.23+** — [golang.org/dl](https://golang.org/dl/)
- **Git** — para as funcionalidades de repositório
- **Docker Desktop** (opcional) — para o Docker Manager
- macOS, Linux ou Windows (WSL2)

---

## Rodando em desenvolvimento

```bash
# Clone o repositório
git clone <url-do-repo> factory-dev
cd factory-dev

# Instalar dependências Go
go mod download

# Rodar em modo dev (hot-reload manual: Ctrl+C → make dev novamente)
make dev
```

A aplicação sobe em **http://localhost:7337**.

> Em modo `dev` os assets (CSS, JS, templates HTML) são lidos do disco em tempo real.
> Altere um template e recarregue o browser — sem restart.

---

## Comandos disponíveis no Makefile

| Comando | O que faz |
|---|---|
| `make dev` | Roda o servidor em modo desenvolvimento (`go run`) |
| `make build` | Compila o binário em `bin/factorydev` |
| `make release` | Gera binários para Linux amd64 e arm64 em `dist/` |
| `make verify-release` | Compila release e executa o binário linux-amd64 |
| `make clean` | Remove `bin/` e `dist/` |
| `make lint` | Roda `golangci-lint` (ou `go vet` se não estiver instalado) |

---

## Build e execução do binário

```bash
# Compilar
make build

# Executar
./bin/factorydev

# Com versão customizada
VERSION=1.0.0 make build
./bin/factorydev
```

---

## Build de release (Linux, para deploy em servidor)

```bash
make release

# Binários gerados:
# dist/factorydev-linux-amd64
# dist/factorydev-linux-arm64
```

Os binários de release têm assets embutidos (`go:embed`) — são **autocontidos**, sem dependências externas além do sistema operacional.

---

## Configuração

O FactoryDev não precisa de arquivo de configuração. Ele cria automaticamente:

```
~/.fdev/
├── state.json          # Estado persistido (chaves, contas, repos, servidores)
└── keys/               # Chaves SSH geradas/importadas
    └── <alias>/
        ├── id_ed25519
        └── id_ed25519.pub
```

### Variáveis de ambiente

| Variável | Padrão | Descrição |
|---|---|---|
| `FDEV_PORT` | `7337` | Porta HTTP do servidor |
| `FDEV_HOST` | `127.0.0.1` | Host de escuta |
| `TERMINAL` | auto-detect | Terminal preferido no Linux (ex: `export TERMINAL=alacritty`) |

---

## macOS — notas específicas

### Abrir terminal nos repos/servidores
O FactoryDev usa `osascript` para abrir o **Terminal.app** (padrão do macOS). Se você usa **iTerm2**, ele será detectado automaticamente se estiver rodando.

### Iniciar Docker Desktop
Na tela **Docker Manager**, se o Docker não estiver rodando aparece o botão **▶ Iniciar Docker** que executa `open -a Docker` — equivalente a abrir o Docker Desktop pelo Finder/Spotlight.

### Hostname
A função de editar hostname usa `hostnamectl` (Linux/systemd). No macOS, use `scutil --set HostName <nome>` diretamente no terminal.

---

## Linux — Docker sem interface gráfica

Se usar Docker Engine (sem Docker Desktop):

```bash
# Iniciar o serviço
sudo systemctl start docker

# Habilitar na inicialização do sistema
sudo systemctl enable docker

# Adicionar seu usuário ao grupo docker (evita sudo)
sudo usermod -aG docker $USER
# Faça logout e login novamente para aplicar
```

---

## Verificar se está funcionando

Acesse **http://localhost:7337/health** — deve retornar:

```json
{"status":"ok"}
```

Acesse **http://localhost:7337/doctor** para ver um diagnóstico completo das ferramentas detectadas no sistema (git, ssh, Docker, etc.).

---

## Atualizar dependências Go

```bash
go get -u ./...
go mod tidy
go build ./...
```
