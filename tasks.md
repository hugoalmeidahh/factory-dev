# FactoryDev — Tasks para Codex

> **Como usar este arquivo:**
> 1. Envie o bloco `## PROJECT CONTEXT` uma vez no início de cada sessão.
> 2. Envie uma task por vez. Aguarde a entrega e revise antes de passar para a próxima.
> 3. Sempre inclua o contexto junto com a task se iniciar uma nova sessão.

---

## PROJECT CONTEXT

```
Projeto: FactoryDev
Tipo: App local de produtividade para desenvolvedores
Linguagem: Go (1.22+)
UI: HTMX + html/template (sem React, sem SPA)
Storage: JSON (~/.fdev/state.json), interface preparada para SQLite futuro
Distribuição: Binário único (go:embed), sem CGO, Linux amd64 e arm64
Porta padrão: 7331, host: 127.0.0.1

Estrutura de pastas:
  cmd/factorydev/main.go
  internal/app/app.go
  internal/config/config.go
  internal/storage/storage.go
  internal/storage/json_storage.go
  internal/ssh/ssh_config.go
  internal/git/service.go
  internal/handler/
  web/templates/
  web/static/

Regras globais do projeto:
- Sem globals: tudo injetado via AppContext (struct App)
- Sem frameworks web externos (apenas net/http ou chi)
- Sem ORM
- CGO_ENABLED=0 em todos os builds
- Nunca logar chaves privadas, senhas ou tokens
- Toda referência a caminho de arquivo usa o struct Paths — nunca string solta
- Erros nunca aparecem como stack trace na UI — apenas no log
```

---

---

## EPIC 0 — Bootstrap do Projeto

---

### TASK 0.1 — Init do repositório e estrutura de pastas

**Objetivo:**
Criar o módulo Go, estrutura de pastas padrão e garantir que o projeto compile e rode com um `main.go` mínimo.

**Arquivos a criar:**
- `go.mod`
- `cmd/factorydev/main.go`
- `internal/app/app.go` (stub)
- `internal/config/config.go` (stub)
- `internal/storage/storage.go` (stub)
- `internal/ssh/ssh_config.go` (stub)
- `internal/git/service.go` (stub)
- `internal/handler/.gitkeep`
- `web/templates/.gitkeep`
- `web/static/.gitkeep`
- `.gitignore`

**O que fazer:**
- Rodar `go mod init github.com/seuusuario/factorydev`
- Criar `main.go` mínimo que imprime `"FactoryDev iniciando..."` e encerra
- Criar stubs (arquivos com `package` e comentário `// TODO`) para todos os internals
- Criar `.gitignore` para Go: ignorar `bin/`, `dist/`, `*.env`, `*.log`
- Garantir que `go build ./...` passa sem erros

**Estrutura de referência:**
```
factorydev/
├── cmd/
│   └── factorydev/
│       └── main.go
├── internal/
│   ├── app/
│   │   └── app.go
│   ├── config/
│   │   └── config.go
│   ├── storage/
│   │   └── storage.go
│   │   └── json_storage.go
│   ├── ssh/
│   │   └── ssh_config.go
│   ├── git/
│   │   └── service.go
│   └── handler/
├── web/
│   ├── templates/
│   └── static/
├── go.mod
├── Makefile
└── .gitignore
```

**Regras:**
- Nenhum pacote externo ainda nesta task — apenas stdlib
- Nenhum `init()` global
- `go vet ./...` deve passar limpo

---

### TASK 0.2 — Makefile e scripts de build

**Objetivo:**
Automatizar dev, build e release com um Makefile simples.

**Arquivos a criar:**
- `Makefile`

**O que fazer:**
- `make dev` → `go run ./cmd/factorydev/...`
- `make build` → compila para `bin/factorydev` (sem embed)
- `make release` → compila para `dist/factorydev-linux-amd64` e `dist/factorydev-linux-arm64` com build tag `release`, `CGO_ENABLED=0` e `-ldflags "-s -w -X main.Version=$(VERSION)"`
- `make clean` → remove `bin/` e `dist/`
- `make lint` → roda `golangci-lint run ./...` se disponível, senão `go vet ./...`

**Estrutura de referência:**
```makefile
APP     = factorydev
VERSION ?= dev
LDFLAGS = -ldflags "-s -w -X main.Version=$(VERSION)"

.PHONY: dev build release clean lint

dev:
	go run ./cmd/factorydev/...

build:
	mkdir -p bin
	go build $(LDFLAGS) -o bin/$(APP) ./cmd/factorydev/...

release:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -tags release $(LDFLAGS) \
		-o dist/$(APP)-linux-amd64 ./cmd/factorydev/...
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
		go build -tags release $(LDFLAGS) \
		-o dist/$(APP)-linux-arm64 ./cmd/factorydev/...

clean:
	rm -rf bin/ dist/

lint:
	@which golangci-lint > /dev/null && golangci-lint run ./... || go vet ./...
```

**Regras:**
- `make release` nunca usa CGO
- Version deve ser injetada via ldflags

---

### TASK 0.3 — Config: flags de linha de comando

**Objetivo:**
Ler configuração via flags nativas do Go e expor um struct `Config` para o app inteiro.

**Arquivos a criar/editar:**
- `internal/config/config.go`
- `cmd/factorydev/main.go` (atualizar para usar ParseFlags)

**O que fazer:**
- Criar struct `Config` com campos: `Host`, `Port`, `Debug`
- Criar `FDevDir string` resolvido (resultado do Paths, preenchido depois)
- Criar função `ParseFlags() *Config` usando o pacote `flag` nativo
- Validar Port (1024–65535) — fatal se inválida
- Host default: `"127.0.0.1"`
- Port default: `7331`
- Debug default: `false`
- Logar host e porta no startup

**Estrutura de referência:**
```go
package config

import (
    "flag"
    "fmt"
    "log"
)

type Config struct {
    Host    string
    Port    int
    Debug   bool
    FDevDir string // preenchido depois pelo Paths
}

func ParseFlags() *Config {
    cfg := &Config{}
    flag.StringVar(&cfg.Host, "host", "127.0.0.1", "Endereço de escuta")
    flag.IntVar(&cfg.Port, "port", 7331, "Porta HTTP (1024-65535)")
    flag.BoolVar(&cfg.Debug, "debug", false, "Modo debug")
    flag.Parse()

    if cfg.Port < 1024 || cfg.Port > 65535 {
        log.Fatalf("porta inválida: %d", cfg.Port)
    }
    return cfg
}

func (c *Config) Addr() string {
    return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
```

**Regras:**
- Porta inválida → `log.Fatal` (impede o app de iniciar)
- Host nunca deve ser `0.0.0.0` por padrão (segurança: não expor na rede local)

---

### TASK 0.4 — AppContext e injeção de dependências

**Objetivo:**
Criar o struct central `App` que carrega todas as dependências. Zero globals. Tudo injetado.

**Arquivos a criar/editar:**
- `internal/app/app.go`
- `cmd/factorydev/main.go` (atualizar para inicializar App)

**O que fazer:**
- Criar struct `App` com campos: `Config`, `Storage`, `Logger`, `Paths`, `SSHService`, `GitService`
- Criar função `New(cfg *Config) (*App, error)` que inicializa cada dependência em ordem
- Se qualquer dependência falhar na inicialização, retornar erro (main faz `log.Fatal`)
- Handlers receberão `*App` via closure ou como receiver de um struct `Handler`

**Estrutura de referência:**
```go
package app

import (
    "log/slog"
    "github.com/seuusuario/factorydev/internal/config"
    "github.com/seuusuario/factorydev/internal/storage"
)

type App struct {
    Config  *config.Config
    Storage storage.Storage
    Logger  *slog.Logger
    Paths   *config.Paths
    // SSHService *ssh.Service  — adicionar nas tasks do Epic 5
    // GitService *git.Service  — adicionar nas tasks futuras
}

func New(cfg *config.Config) (*App, error) {
    // 1. resolver paths
    // 2. garantir diretórios ~/.fdev
    // 3. inicializar logger
    // 4. inicializar storage
    // retornar App ou erro
    return &App{Config: cfg}, nil // expandir nas próximas tasks
}
```

**Regras:**
- Nenhum pacote interno pode importar `app.App` diretamente (evitar ciclo de importação)
- Services recebem suas dependências via construtor próprio, não via `*App`
- `New()` deve retornar erro descritivo se algo falhar, nunca `panic`

---

---

## EPIC 1 — Filesystem e Diretório ~/.fdev

---

### TASK 1.1 — Resolver diretório base do usuário e struct Paths

**Objetivo:**
Obter o home directory de forma portável e definir todos os caminhos do app em um único struct `Paths`.

**Arquivos a criar/editar:**
- `internal/config/paths.go`

**O que fazer:**
- Usar `os.UserHomeDir()` para obter home
- Definir `baseDir = ~/.fdev`
- Criar struct `Paths` com todos os caminhos derivados
- Criar função `NewPaths() (*Paths, error)`
- Criar métodos para paths de chaves SSH por alias

**Estrutura de referência:**
```go
package config

import (
    "os"
    "path/filepath"
    "regexp"
)

type Paths struct {
    Base    string // ~/.fdev
    Keys    string // ~/.fdev/keys
    Logs    string // ~/.fdev/logs
    Backups string // ~/.fdev/backups
    State   string // ~/.fdev/state.json
}

func NewPaths() (*Paths, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return nil, err
    }
    base := filepath.Join(home, ".fdev")
    return &Paths{
        Base:    base,
        Keys:    filepath.Join(base, "keys"),
        Logs:    filepath.Join(base, "logs"),
        Backups: filepath.Join(base, "backups"),
        State:   filepath.Join(base, "state.json"),
    }, nil
}

var validAlias = regexp.MustCompile(`^[a-z0-9_-]+$`)

func (p *Paths) KeyDir(alias string) string {
    if !validAlias.MatchString(alias) {
        panic("alias inválido: " + alias) // deve ser validado antes de chegar aqui
    }
    return filepath.Join(p.Keys, alias)
}

func (p *Paths) PrivateKey(alias string) string {
    return filepath.Join(p.KeyDir(alias), "id_ed25519")
}

func (p *Paths) PublicKey(alias string) string {
    return filepath.Join(p.KeyDir(alias), "id_ed25519.pub")
}

func (p *Paths) SSHConfig() string {
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".ssh", "config")
}
```

**Regras:**
- Nunca hardcodar `/home/user` ou `/Users/user`
- `alias` deve passar por `validAlias` antes de gerar paths — protege contra path traversal (`../`)
- Toda string de caminho no projeto vem daqui, nunca literal

---

### TASK 1.2 — Criar estrutura padrão de diretórios no startup

**Objetivo:**
Garantir que `~/.fdev` e subpastas existam com as permissões corretas toda vez que o app iniciar.

**Arquivos a criar/editar:**
- `internal/config/fs.go`
- `internal/app/app.go` (chamar EnsureDirectories no New())

**O que fazer:**
- Criar função `EnsureDirectories(paths *Paths) error`
- Criar diretórios com permissões específicas:
  - `~/.fdev/` → `0700`
  - `~/.fdev/keys/` → `0700` (mais restritivo — protege chaves)
  - `~/.fdev/logs/` → `0755`
  - `~/.fdev/backups/` → `0755`
- Usar `os.MkdirAll` — idempotente (não falha se já existe)
- Chamar no startup, antes de qualquer outra operação

**Estrutura de referência:**
```go
package config

import (
    "fmt"
    "io/fs"
    "os"
)

func EnsureDirectories(paths *Paths) error {
    dirs := []struct {
        path string
        perm fs.FileMode
    }{
        {paths.Base,    0700},
        {paths.Keys,    0700},
        {paths.Logs,    0755},
        {paths.Backups, 0755},
    }
    for _, d := range dirs {
        if err := os.MkdirAll(d.path, d.perm); err != nil {
            return fmt.Errorf("criar diretório %s: %w", d.path, err)
        }
    }
    return nil
}
```

**Regras:**
- `keys/` DEVE ter `0700` — nunca `0755` (SSH recusa chaves em diretórios com permissão aberta)
- Falha nesta função deve impedir o app de iniciar (`log.Fatal` no main)
- Função deve ser idempotente — chamá-la várias vezes não causa erro

---

---

## EPIC 2 — Storage v0 (JSON)

---

### TASK 2.1 — Interface Storage e modelos de dados

**Objetivo:**
Definir a interface `Storage` e os modelos `State` e `Account` com schema versionado.

**Arquivos a criar:**
- `internal/storage/storage.go`

**O que fazer:**
- Definir interface `Storage` com métodos `LoadState` e `SaveState`
- Definir struct `State` com `SchemaVersion`, `Accounts` e `UpdatedAt`
- Definir struct `Account` com todos os campos necessários
- Constante `CurrentSchema = 1`

**Estrutura de referência:**
```go
package storage

import "time"

const CurrentSchema = 1

type Storage interface {
    LoadState() (*State, error)
    SaveState(*State) error
}

type State struct {
    SchemaVersion int       `json:"schemaVersion"`
    Accounts      []Account `json:"accounts"`
    UpdatedAt     time.Time `json:"updatedAt"`
}

type Account struct {
    ID           string    `json:"id"`
    Name         string    `json:"name"`           // ex: "work", "personal"
    Provider     string    `json:"provider"`        // "github", "gitlab", "bitbucket", "other"
    HostName     string    `json:"hostName"`        // ex: "github.com"
    HostAlias    string    `json:"hostAlias"`       // ex: "github-work"
    IdentityFile string    `json:"identityFile"`    // path absoluto da chave privada
    GitUserName  string    `json:"gitUserName"`
    GitUserEmail string    `json:"gitUserEmail"`
    CreatedAt    time.Time `json:"createdAt"`
    UpdatedAt    time.Time `json:"updatedAt"`
}
```

**Regras:**
- Interface `Storage` deve ser o único contrato — nunca acessar `JSONStorage` diretamente fora do pacote storage
- `SchemaVersion` começa em `1`
- `ID` gerado externamente (no handler/service) via `crypto/rand` ou `google/uuid`

---

### TASK 2.2 — Implementar JSONStorage com escrita atômica

**Objetivo:**
Implementar a interface `Storage` em JSON com escrita atômica (tmp → fsync → rename) para nunca corromper `state.json`.

**Arquivos a criar:**
- `internal/storage/json_storage.go`

**O que fazer:**
- Struct `JSONStorage` com campo `path string`
- `LoadState()`: ler e deserializar `state.json`; se arquivo não existe, retornar `State` vazio com `SchemaVersion = CurrentSchema`
- `SaveState()`: escrever em `<path>.tmp` → `f.Sync()` → `os.Rename()` para o path final
- Usar `json.MarshalIndent` (legível pelo usuário)
- Garantir perm `0600` no arquivo final

**Estrutura de referência:**
```go
package storage

import (
    "encoding/json"
    "errors"
    "os"
    "time"
)

type JSONStorage struct {
    path string
}

func NewJSONStorage(path string) *JSONStorage {
    return &JSONStorage{path: path}
}

func (s *JSONStorage) LoadState() (*State, error) {
    data, err := os.ReadFile(s.path)
    if errors.Is(err, os.ErrNotExist) {
        return &State{SchemaVersion: CurrentSchema}, nil
    }
    if err != nil {
        return nil, err
    }
    var state State
    if err := json.Unmarshal(data, &state); err != nil {
        return nil, err
    }
    return &state, nil
}

func (s *JSONStorage) SaveState(state *State) error {
    state.UpdatedAt = time.Now()
    data, err := json.MarshalIndent(state, "", "  ")
    if err != nil {
        return err
    }

    tmp := s.path + ".tmp"
    f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
    if err != nil {
        return err
    }

    if _, err = f.Write(data); err != nil {
        f.Close()
        return err
    }
    if err = f.Sync(); err != nil {
        f.Close()
        return err
    }
    f.Close()

    return os.Rename(tmp, s.path)
}
```

**Regras:**
- `state.json` e `state.json.tmp` DEVEM ter perm `0600`
- `f.Sync()` é obrigatório antes do `Rename` (garante flush para disco)
- Nunca sobrescrever diretamente o arquivo — sempre via `tmp → rename`
- `os.Rename` é atômico no Linux (mesmo filesystem)

---

### TASK 2.3 — Migrations de schema

**Objetivo:**
Preparar uma estratégia de migration simples e extensível para quando o schema do `state.json` evoluir.

**Arquivos a criar:**
- `internal/storage/migrations.go`

**O que fazer:**
- Mapa de funções `migrations: map[int]func(*State) error`
- Função `migrate(s *State) error` que aplica migrations em cadeia até atingir `CurrentSchema`
- Chamar `migrate()` logo após o `LoadState()` bem-sucedido
- Antes de migrar: fazer backup do `state.json` (chamar função do Epic 1 de backup)

**Estrutura de referência:**
```go
package storage

var migrations = map[int]func(*State) error{
    // Exemplo de migration futura:
    // 1: func(s *State) error {
    //     // adicionar campo novo, preencher default, etc.
    //     return nil
    // },
}

func migrate(s *State) error {
    for s.SchemaVersion < CurrentSchema {
        fn, ok := migrations[s.SchemaVersion]
        if !ok {
            break
        }
        if err := fn(s); err != nil {
            return err
        }
        s.SchemaVersion++
    }
    return nil
}
```

**Regras:**
- Migration NUNCA é destrutiva sem backup prévio
- Se não existe migration para a versão atual, parar silenciosamente (não é erro — pode ser versão futura)
- Salvar o state com a nova versão após migrar com sucesso

---

---

## EPIC 3 — Web UI (Templates + HTMX)

---

### TASK 3.1 — Servidor HTTP com rotas base e graceful shutdown

**Objetivo:**
Subir o servidor HTTP na porta configurada com rotas mínimas, middleware de logging e graceful shutdown.

**Arquivos a criar/editar:**
- `internal/handler/handler.go`
- `internal/handler/routes.go`
- `cmd/factorydev/main.go` (atualizar)

**Dependências Go a adicionar:**
- `github.com/go-chi/chi/v5` (router)

**O que fazer:**
- Criar struct `Handler` com campo `app *app.App`
- Registrar rotas:
  - `GET /` → render layout principal
  - `GET /health` → JSON `{"status":"ok"}`
  - `GET /assets/*` → servir arquivos estáticos
- Middleware: `chi/middleware.Logger`, `chi/middleware.Recoverer`
- Graceful shutdown: escutar `SIGINT`/`SIGTERM`, shutdown com timeout de 5s

**Estrutura de referência:**
```go
// routes.go
func (h *Handler) Routes() http.Handler {
    r := chi.NewRouter()
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)

    r.Get("/", h.Index)
    r.Get("/health", h.Health)
    r.Handle("/assets/*", http.StripPrefix("/assets/", h.staticHandler()))

    // Tool routes
    r.Get("/tools/ssh/accounts", h.ListAccounts)
    r.Post("/tools/ssh/accounts", h.CreateAccount)
    r.Get("/tools/ssh/accounts/new", h.NewAccountDrawer)
    r.Get("/tools/ssh/accounts/{id}/edit", h.EditAccountDrawer)
    r.Post("/tools/ssh/accounts/{id}", h.UpdateAccount)
    r.Delete("/tools/ssh/accounts/{id}", h.DeleteAccount)
    r.Post("/tools/ssh/accounts/{id}/generate-key", h.GenerateKey)
    r.Post("/tools/ssh/accounts/{id}/apply-ssh", h.ApplySSHConfig)
    r.Post("/tools/ssh/accounts/{id}/test", h.TestConnection)

    return r
}

// graceful shutdown em main.go:
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit

ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
srv.Shutdown(ctx)
```

**Regras:**
- `Recoverer` deve capturar panics e retornar `500` com mensagem amigável — nunca stack trace na resposta HTTP
- Timeout de shutdown: 5 segundos

---

### TASK 3.2 — Template base: layout, sidebar e drawer container

**Objetivo:**
Criar o layout HTML base com sidebar de tools, área de conteúdo principal e container do drawer lateral.

**Arquivos a criar:**
- `web/templates/layout.html`
- `web/templates/partials/sidebar.html`
- `web/templates/partials/drawer.html`
- `web/static/css/app.css` (CSS mínimo)

**O que fazer:**
- Layout com 3 zonas: sidebar fixa à esquerda, conteúdo principal, drawer lateral direito
- Sidebar com itens de tool; item ativo marcado via classe CSS baseada em `{{.ActiveTool}}`
- HTMX: incluir via CDN `https://unpkg.com/htmx.org@2.0.0`
- Drawer: `div#drawer` com `div#drawer-content` — hidden por padrão, JS mínimo para abrir/fechar
- Toast container: `div#toast-container` fixo no canto superior direito
- Rota `GET /` serve o layout; conteúdo inicial é a tela de SSH Accounts

**Estrutura de referência:**
```html
<!-- layout.html -->
<!DOCTYPE html>
<html lang="pt-BR">
<head>
    <meta charset="UTF-8">
    <title>FactoryDev</title>
    <script src="https://unpkg.com/htmx.org@2.0.0"></script>
    <link rel="stylesheet" href="/assets/css/app.css">
</head>
<body class="fdev-body">

  <!-- Sidebar -->
  <aside class="fdev-sidebar">
    <div class="fdev-logo">FactoryDev</div>
    <nav>
      <a class="fdev-nav-item {{if eq .ActiveTool "ssh"}}active{{end}}"
         hx-get="/tools/ssh/accounts"
         hx-target="#main-content"
         hx-push-url="/tools/ssh/accounts">
        SSH / Git Accounts
      </a>
    </nav>
  </aside>

  <!-- Main content -->
  <main id="main-content" class="fdev-main">
    {{template "content" .}}
  </main>

  <!-- Drawer container -->
  <div id="drawer-overlay" class="fdev-overlay hidden" onclick="closeDrawer()"></div>
  <aside id="drawer" class="fdev-drawer fdev-drawer--closed">
    <div id="drawer-content"></div>
  </aside>

  <!-- Toast container -->
  <div id="toast-container" class="fdev-toasts"></div>

  <script src="/assets/js/app.js"></script>
</body>
</html>
```

```javascript
// web/static/js/app.js — JS mínimo
function openDrawer() {
    document.getElementById('drawer').classList.remove('fdev-drawer--closed')
    document.getElementById('drawer-overlay').classList.remove('hidden')
}
function closeDrawer() {
    document.getElementById('drawer').classList.add('fdev-drawer--closed')
    document.getElementById('drawer-overlay').classList.add('hidden')
    document.getElementById('drawer-content').innerHTML = ''
}

// Toast via HX-Trigger
document.body.addEventListener('showToast', function(e) {
    const { msg, type } = e.detail
    const toast = document.createElement('div')
    toast.className = `fdev-toast fdev-toast--${type}`
    toast.textContent = msg
    document.getElementById('toast-container').appendChild(toast)
    setTimeout(() => toast.remove(), 3000)
})
document.body.addEventListener('closeDrawer', closeDrawer)
```

**Regras:**
- Drawer nunca recarrega a página completa — apenas injeta partial via HTMX
- Item ativo na sidebar sempre reflete a tool atual via `{{.ActiveTool}}`
- Toast desaparece automaticamente após 3 segundos

---

### TASK 3.3 — Partials reutilizáveis: Drawer wrapper, Toast e Confirm

**Objetivo:**
Criar os componentes HTML reutilizáveis que todas as tools vão usar.

**Arquivos a criar:**
- `web/templates/partials/drawer-wrapper.html`
- `web/templates/partials/toast.html`
- `internal/handler/helpers.go`

**O que fazer:**
- `drawer-wrapper.html`: envelope genérico do drawer com header (título + botão fechar) e slot de conteúdo
- Helper Go `h.renderDrawer(w, title, templateName, data)` que renderiza o wrapper + conteúdo e retorna o header `HX-Trigger: openDrawer`
- Helper Go `h.successResponse(w, msg, retarget, rerenderTemplate, data)` que:
  - Seta `HX-Trigger: {"showToast":{"msg":"...","type":"success"}, "closeDrawer":true}`
  - Seta `HX-Retarget` e `HX-Reswap` se fornecido
  - Renderiza o template de resposta
- Helper Go `h.errorResponse(w, errs)` para erros de validação inline no form

**Estrutura de referência:**
```go
// helpers.go
func (h *Handler) renderDrawer(w http.ResponseWriter, title, tmpl string, data any) {
    w.Header().Set("HX-Trigger", `openDrawer`)
    h.render(w, "partials/drawer-wrapper.html", map[string]any{
        "Title":   title,
        "Content": tmpl,
        "Data":    data,
    })
}

func (h *Handler) successToast(w http.ResponseWriter, msg string) {
    trigger := fmt.Sprintf(`{"showToast":{"msg":%q,"type":"success"},"closeDrawer":true}`, msg)
    w.Header().Set("HX-Trigger", trigger)
}

func (h *Handler) render(w http.ResponseWriter, tmpl string, data any) {
    // executa o template com layout ou sem, dependendo do header HX-Request
}
```

**Regras:**
- Respostas de POST bem-sucedido NUNCA redirecionam (sem `http.Redirect`) — usam HX-Trigger
- Erros de validação retornam o drawer com os erros inline, sem fechar

---

### TASK 3.4 — Embed de assets no binário (build tag release)

**Objetivo:**
Usar `//go:embed` para embutir templates e assets no binário final, mantendo dev sem embed para edição ao vivo.

**Arquivos a criar:**
- `web/embed.go` (com build tag `release`)
- `web/embed_dev.go` (sem build tag — usa `os.DirFS`)
- `internal/handler/static.go`

**O que fazer:**
- `web/embed.go` (compilado apenas com `-tags release`):
  ```go
  //go:build release
  package web
  import "embed"
  //go:embed templates/* static/*
  var FS embed.FS
  ```
- `web/embed_dev.go` (compilado em dev):
  ```go
  //go:build !release
  package web
  import "os"
  var FS = os.DirFS("web")
  ```
- Handler de static serve de `web.FS` — não precisa saber qual FS está usando

**Regras:**
- `//go:embed` nunca usa caminho absoluto
- Em dev, arquivos são lidos do disco a cada request (edição ao vivo)
- Em release, tudo dentro do binário — sem dependência de arquivos externos

---

---

## EPIC 4 — Tool: Git Accounts (SSH Multi-conta)

---

### TASK 4.1 — Validação de Account

**Objetivo:**
Criar a função de validação de Account com erros por campo, usada antes de qualquer operação de save.

**Arquivos a criar:**
- `internal/storage/validation.go`

**O que fazer:**
- Struct `ValidationError{Field, Message string}`
- Função `Validate(a Account, existing []Account) []ValidationError`
- Validar:
  - `Name` não vazio
  - `Provider` deve ser um dos: `github`, `gitlab`, `bitbucket`, `other`
  - `HostName` não vazio e formato válido (sem `http://`, sem espaço)
  - `HostAlias` não vazio, apenas `[a-z0-9_-]`, único no State (exceto o próprio ID em edição)
  - `GitUserEmail` com formato de email válido
  - `GitUserName` não vazio

**Estrutura de referência:**
```go
package storage

import (
    "regexp"
    "strings"
)

type ValidationError struct {
    Field   string
    Message string
}

var (
    validAliasRe = regexp.MustCompile(`^[a-z0-9_-]+$`)
    validEmailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
    validProviders = map[string]bool{
        "github": true, "gitlab": true, "bitbucket": true, "other": true,
    }
)

func Validate(a Account, existing []Account) []ValidationError {
    var errs []ValidationError

    if strings.TrimSpace(a.Name) == "" {
        errs = append(errs, ValidationError{"name", "obrigatório"})
    }
    if !validProviders[a.Provider] {
        errs = append(errs, ValidationError{"provider", "valor inválido"})
    }
    if !validAliasRe.MatchString(a.HostAlias) {
        errs = append(errs, ValidationError{"hostAlias", "apenas letras minúsculas, números, - e _"})
    }
    for _, e := range existing {
        if e.HostAlias == a.HostAlias && e.ID != a.ID {
            errs = append(errs, ValidationError{"hostAlias", "já existe"})
            break
        }
    }
    if !validEmailRe.MatchString(a.GitUserEmail) {
        errs = append(errs, ValidationError{"gitUserEmail", "email inválido"})
    }

    return errs
}
```

**Regras:**
- `HostAlias` deve ser globalmente único — não apenas por provider
- Erros retornados em slice — coletar TODOS antes de retornar (não parar no primeiro)

---

### TASK 4.2 — Template: listar accounts (tela principal da tool SSH)

**Objetivo:**
Criar o template e handler que renderiza a lista de Git Accounts na área de conteúdo principal.

**Arquivos a criar:**
- `web/templates/ssh/accounts-list.html`
- `internal/handler/ssh_accounts.go` (função `ListAccounts`)

**O que fazer:**
- Handler `GET /tools/ssh/accounts`:
  - Carregar accounts do Storage
  - Para cada account, checar se a chave privada existe em `~/.fdev/keys/<alias>/id_ed25519`
  - Passar `ActiveTool: "ssh"` no PageData
  - Renderizar template
- Template: tabela/lista com colunas: Name, Provider, Host Alias, Git Email, Status da Chave
- Botões por linha:
  - **Edit** → `hx-get="/tools/ssh/accounts/{id}/edit"` + `hx-target="#drawer-content"`
  - **Generate Key** (se sem chave) → `hx-post="/tools/ssh/accounts/{id}/generate-key"`
  - **Apply SSH Config** → `hx-post="/tools/ssh/accounts/{id}/apply-ssh"`
  - **Test** → `hx-post="/tools/ssh/accounts/{id}/test"` (mostra output)
  - **Delete** → `hx-delete` + `hx-confirm="Deletar {name}?"`
- Botão "Nova Conta" no header → `hx-get="/tools/ssh/accounts/new"` + `hx-target="#drawer-content"`
- Se accounts vazia: renderizar empty state (título + subtítulo + botão CTA)

**Regras:**
- Empty state nunca é uma tabela vazia — é um componente visual separado
- Status da chave calculado no handler (não no template)
- `hx-confirm` para ações destrutivas (delete)

---

### TASK 4.3 — Template e handler: drawer de criar/editar account

**Objetivo:**
Renderizar o formulário de criação/edição de account dentro do drawer lateral.

**Arquivos a criar:**
- `web/templates/ssh/account-drawer.html`
- Funções `NewAccountDrawer` e `EditAccountDrawer` em `ssh_accounts.go`

**O que fazer:**
- `GET /tools/ssh/accounts/new` → drawer com form vazio
- `GET /tools/ssh/accounts/{id}/edit` → drawer com form preenchido
- Campos do form: `name`, `provider` (select), `hostName`, `hostAlias`, `gitUserName`, `gitUserEmail`
- JS mínimo no template: ao alterar `provider` ou `name`, sugerir `hostAlias` automaticamente
- Botões: **Cancel** (`onclick="closeDrawer()"`) e **Save** (`hx-post` ou `hx-put`)
- Resposta do handler seta header `HX-Trigger: openDrawer` para abrir o drawer

**Estrutura de referência — sugestão de alias:**
```html
<script>
function suggestAlias() {
    const provider = document.getElementById('provider').value
    const name = document.getElementById('name').value.toLowerCase().replace(/\s+/g, '-')
    if (provider && name) {
        const alias = provider + '-' + name
        const field = document.getElementById('hostAlias')
        if (!field.dataset.userEdited) field.value = alias
    }
}
</script>
<input id="hostAlias" oninput="this.dataset.userEdited='1'" ...>
```

**Regras:**
- Sugestão de alias não sobrescreve se o usuário editou manualmente (`data-user-edited`)
- Em edição, se o alias mudar e a chave existir → exibir aviso inline no form

---

### TASK 4.4 — Handlers: criar, editar e deletar account

**Objetivo:**
Endpoints POST/DELETE para CRUD completo de accounts com resposta HTMX.

**Arquivos a editar:**
- `internal/handler/ssh_accounts.go`

**O que fazer:**
- `POST /tools/ssh/accounts` (criar):
  - Parse do form
  - Gerar `ID` via `crypto/rand` (UUID v4 simples)
  - Validar com `Validate()`
  - Se erros: retornar drawer com erros inline (não fechar)
  - Se ok: salvar, setar `HX-Trigger` de toast + closeDrawer, re-renderizar lista
- `POST /tools/ssh/accounts/{id}` (editar):
  - Mesmo fluxo, mas atualizar account existente
- `DELETE /tools/ssh/accounts/{id}`:
  - Verificar se chave existe → logar aviso (não deletar a chave automaticamente)
  - Remover account do State
  - Re-renderizar lista com toast de confirmação

**Estrutura de referência:**
```go
func (h *Handler) CreateAccount(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()
    a := accountFromForm(r.Form)
    a.ID = newUUID()
    a.CreatedAt = time.Now()
    a.UpdatedAt = time.Now()

    state, _ := h.app.Storage.LoadState()
    if errs := storage.Validate(a, state.Accounts); len(errs) > 0 {
        h.renderDrawerWithErrors(w, "ssh/account-drawer.html", a, errs)
        return
    }

    state.Accounts = append(state.Accounts, a)
    h.app.Storage.SaveState(state)

    h.successToast(w, "Conta criada com sucesso!")
    w.Header().Set("HX-Retarget", "#main-content")
    h.renderAccountList(w, state)
}
```

**Regras:**
- Erros de validação: retornar status `422 Unprocessable Entity`, não `200`
- Delete não deleta arquivos de chave — apenas remove do state.json (usuário pode querer reusá-las)
- Toast deve ser exibido após qualquer operação bem-sucedida

---

---

## EPIC 5 — SSH Key Generation + SSH Config Writer

---

### TASK 5.1 — Parser do ~/.ssh/config existente

**Objetivo:**
Ler e parsear o `~/.ssh/config` identificando blocos gerenciados pelo FactoryDev vs blocos manuais do usuário.

**Arquivos a criar:**
- `internal/ssh/parser.go`

**O que fazer:**
- Ler `~/.ssh/config` linha a linha
- Identificar início de bloco: linha que começa com `Host ` (case insensitive)
- Identificar blocos FDEV: bloco precedido por comentário `# BEGIN FDEV <alias>`
- Preservar todos os blocos não-FDEV intactos (linhas originais, incluindo comentários)
- Retornar struct `ParsedSSHConfig` com lista de `SSHConfigBlock`

**Estrutura de referência:**
```go
package ssh

import (
    "bufio"
    "os"
    "strings"
)

type SSHConfigBlock struct {
    Alias  string
    IsFDev bool
    Lines  []string
}

type ParsedSSHConfig struct {
    Blocks      []SSHConfigBlock
    HeaderLines []string // linhas antes do primeiro Host (comentários globais)
}

func ParseSSHConfig(path string) (*ParsedSSHConfig, error) {
    f, err := os.Open(path)
    if os.IsNotExist(err) {
        return &ParsedSSHConfig{}, nil // arquivo não existe é ok
    }
    if err != nil {
        return nil, err
    }
    defer f.Close()

    var result ParsedSSHConfig
    var currentBlock *SSHConfigBlock
    var pendingFDevAlias string // se a linha anterior foi "# BEGIN FDEV <alias>"

    scanner := bufio.NewScanner(f)
    for scanner.Scan() {
        line := scanner.Text()

        if strings.HasPrefix(line, "# BEGIN FDEV ") {
            pendingFDevAlias = strings.TrimPrefix(line, "# BEGIN FDEV ")
            continue
        }

        if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "host ") {
            if currentBlock != nil {
                result.Blocks = append(result.Blocks, *currentBlock)
            }
            alias := strings.Fields(line)[1]
            currentBlock = &SSHConfigBlock{
                Alias:  alias,
                IsFDev: pendingFDevAlias == alias,
                Lines:  []string{line},
            }
            pendingFDevAlias = ""
            continue
        }

        if currentBlock != nil {
            currentBlock.Lines = append(currentBlock.Lines, line)
        } else {
            result.HeaderLines = append(result.HeaderLines, line)
        }
    }
    if currentBlock != nil {
        result.Blocks = append(result.Blocks, *currentBlock)
    }
    return &result, scanner.Err()
}
```

**Regras:**
- Se `~/.ssh/config` não existe → retornar config vazia (não é erro)
- NUNCA modificar blocos não-FDEV
- Esta é a task de maior risco — escrever testes unitários com fixtures de ssh_config com entradas mistas

---

### TASK 5.2 — Backup automático do ~/.ssh/config

**Objetivo:**
Antes de qualquer escrita no `~/.ssh/config`, criar backup timestampado em `~/.fdev/backups/`.

**Arquivos a criar:**
- `internal/ssh/backup.go`

**O que fazer:**
- Função `BackupSSHConfig(paths *config.Paths) error`
- Se `~/.ssh/config` não existe, retornar `nil` (nada a fazer)
- Copiar para `~/.fdev/backups/ssh_config_<YYYYMMDD_HHMMSS>`
- Manter apenas os 10 backups mais recentes (apagar os mais antigos)
- Logar que backup foi criado (com o path)

**Estrutura de referência:**
```go
func BackupSSHConfig(paths *config.Paths) error {
    src := paths.SSHConfig()
    if _, err := os.Stat(src); os.IsNotExist(err) {
        return nil
    }

    ts := time.Now().Format("20060102_150405")
    dst := filepath.Join(paths.Backups, "ssh_config_"+ts)

    if err := copyFile(src, dst, 0600); err != nil {
        return fmt.Errorf("backup ssh_config: %w", err)
    }

    slog.Info("backup criado", "path", dst)
    return pruneOldBackups(paths.Backups, "ssh_config_", 10)
}
```

**Regras:**
- Backup DEVE acontecer antes de qualquer escrita, sem exceção
- Perm do backup: `0600`
- `pruneOldBackups`: ordenar por nome (timestamp no nome garante ordem), remover os mais antigos além do limite

---

### TASK 5.3 — Writer seguro do ~/.ssh/config

**Objetivo:**
Inserir ou atualizar blocos FDEV no `~/.ssh/config` de forma atômica, preservando todos os blocos manuais do usuário.

**Arquivos a criar:**
- `internal/ssh/writer.go`

**O que fazer:**
- Função `ApplyAccount(account storage.Account, paths *config.Paths) error`
- Fluxo: `BackupSSHConfig` → `ParseSSHConfig` → upsert do bloco FDEV → serializar → escrever atômico
- Upsert: se bloco com `HostAlias` já existe e `IsFDev == true`, substituir. Se não existe, adicionar ao final
- Serializar: reescrever todos os blocos na ordem original, blocos FDEV com marcadores `# BEGIN FDEV` / `# END FDEV`
- Escrita atômica: mesmo padrão do Storage (tmp → fsync → rename)
- Garantir que `~/.ssh/` existe antes de escrever (criar se não existe com perm `0700`)

**Formato do bloco gerado:**
```
# BEGIN FDEV github-work
Host github-work
  HostName github.com
  User git
  IdentityFile /home/user/.fdev/keys/github-work/id_ed25519
  IdentitiesOnly yes
# END FDEV github-work
```

**Regras:**
- `IdentityFile` usa caminho absoluto (via `paths.PrivateKey(alias)`) — nunca `~`
- `IdentitiesOnly yes` é obrigatório — sem isso o SSH pode tentar outras chaves e falhar
- Nunca remover linhas de blocos não-FDEV
- `~/.ssh/config` final deve ter perm `0600`

---

### TASK 5.4 — Gerador de chave ed25519

**Objetivo:**
Gerar par de chaves ed25519 em Go puro (sem chamar `ssh-keygen` externo) e salvar em `~/.fdev/keys/<alias>/`.

**Arquivos a criar:**
- `internal/ssh/keygen.go`

**Dependências Go a adicionar:**
- `golang.org/x/crypto/ssh`

**O que fazer:**
- Função `GenerateKey(alias, comment string, paths *config.Paths) error`
- Verificar se chave já existe → retornar `ErrKeyExists` (handler decide se sobrescreve)
- Criar diretório `~/.fdev/keys/<alias>/` com perm `0700`
- Gerar par ed25519 com `crypto/ed25519`
- Serializar chave privada: PEM OpenSSH via `x/crypto/ssh.MarshalPrivateKey`
- Serializar chave pública: formato OpenSSH via `x/crypto/ssh.MarshalAuthorizedKey`
- Salvar: privada com perm `0600`, pública com perm `0644`

**Estrutura de referência:**
```go
package ssh

import (
    "crypto/ed25519"
    "crypto/rand"
    "errors"
    "os"
    "golang.org/x/crypto/ssh"
)

var ErrKeyExists = errors.New("chave já existe para este alias")

func GenerateKey(alias, comment string, paths *config.Paths) error {
    privPath := paths.PrivateKey(alias)
    if _, err := os.Stat(privPath); err == nil {
        return ErrKeyExists
    }

    if err := os.MkdirAll(paths.KeyDir(alias), 0700); err != nil {
        return err
    }

    pub, priv, err := ed25519.GenerateKey(rand.Reader)
    if err != nil {
        return err
    }

    sshPub, err := ssh.NewPublicKey(pub)
    if err != nil {
        return err
    }

    privPEM, err := ssh.MarshalPrivateKey(priv, comment)
    if err != nil {
        return err
    }

    if err := os.WriteFile(privPath, pem.EncodeToMemory(privPEM), 0600); err != nil {
        return err
    }

    pubBytes := ssh.MarshalAuthorizedKey(sshPub)
    return os.WriteFile(paths.PublicKey(alias), pubBytes, 0644)
}
```

**Regras:**
- Perm `0600` na chave privada — SSH recusa chaves com perm mais aberta
- Nunca logar a chave privada
- Usar apenas stdlib + `x/crypto/ssh` — sem dependência do binário `ssh-keygen`

---

### TASK 5.5 — Testar conexão SSH

**Objetivo:**
Executar `ssh -T git@<hostAlias>` com timeout e retornar o output para a UI via partial HTMX.

**Arquivos a criar:**
- `internal/ssh/test.go`
- `web/templates/ssh/test-result.html`

**O que fazer:**
- Função `TestConnection(alias string) (output string, err error)`
- Usar `exec.CommandContext` com timeout de 10 segundos
- Capturar stdout + stderr combinados (`CombinedOutput`)
- GitHub retorna exit code 1 com `"Hi <user>!"` → isso é SUCESSO (não tratar exit code 1 como erro)
- Handler `POST /tools/ssh/accounts/{id}/test` retorna partial com o output (sucesso ou falha)

**Estrutura de referência:**
```go
func TestConnection(alias string) (string, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "ssh",
        "-T",
        "-o", "StrictHostKeyChecking=accept-new",
        "-o", "BatchMode=yes",
        "git@"+alias,
    )

    out, _ := cmd.CombinedOutput() // ignora exit code intencionalmente
    if ctx.Err() != nil {
        return "", errors.New("timeout: conexão demorou mais de 10s")
    }
    return string(out), nil
}
```

**Regras:**
- Ignorar exit code — verificar o conteúdo do output para determinar sucesso
- `StrictHostKeyChecking=accept-new` — aceita host novo automaticamente, rejeita se mudou
- `BatchMode=yes` — impede que SSH peça senha interativamente
- NUNCA usar `StrictHostKeyChecking=no` (inseguro)

---

---

## EPIC 6 — Onboarding e First Run

---

### TASK 6.1 — Doctor: checklist de pré-requisitos

**Objetivo:**
Verificar se o ambiente está pronto para o FactoryDev funcionar e exibir resultado na UI e no terminal.

**Arquivos a criar:**
- `internal/doctor/doctor.go`
- `web/templates/doctor.html`
- Adicionar subcomando `doctor` no `main.go`

**O que fazer:**
- Struct `Check{Name string, OK bool, Message string}`
- Função `RunDoctor(paths *config.Paths) []Check`
- Checks:
  - `~/.ssh/` existe
  - `~/.ssh/` tem perm adequada (`0700`)
  - `ssh` disponível no PATH (`exec.LookPath("ssh")`)
  - `~/.fdev/` foi criado (checado pelos Paths/EnsureDirectories)
- CLI: `factorydev doctor` imprime checks no terminal (sem subir servidor)
- UI: `GET /doctor` renderiza template com resultado dos checks

**Estrutura de referência:**
```go
func RunDoctor(paths *config.Paths) []Check {
    checks := []Check{}

    // Check: ~/.ssh existe
    sshDir := filepath.Join(filepath.Dir(paths.Base), ".ssh")
    _, err := os.Stat(sshDir)
    checks = append(checks, Check{
        Name:    "~/.ssh/ existe",
        OK:      err == nil,
        Message: ifElse(err == nil, "OK", "Crie com: mkdir -m 700 ~/.ssh"),
    })

    // Check: ssh no PATH
    _, err = exec.LookPath("ssh")
    checks = append(checks, Check{
        Name:    "ssh disponível no PATH",
        OK:      err == nil,
        Message: ifElse(err == nil, "OK", "Instale o OpenSSH"),
    })

    return checks
}
```

**Regras:**
- Falhas no doctor NÃO impedem o app de iniciar — apenas informam o usuário
- CLI doctor deve retornar exit code 1 se qualquer check falhar (para uso em scripts)

---

### TASK 6.2 — Empty state na listagem de accounts

**Objetivo:**
Quando não há accounts, exibir tela amigável com CTA em vez de tabela vazia.

**Arquivos a editar:**
- `web/templates/ssh/accounts-list.html` (adicionar bloco `{{if eq (len .Accounts) 0}}`)

**O que fazer:**
- No template de listagem: verificar se accounts está vazio
- Se vazio: renderizar empty state com:
  - SVG inline de ícone (chave ou terminal)
  - Título: `"Nenhuma conta configurada ainda"`
  - Subtítulo: `"Adicione sua primeira conta Git para gerenciar múltiplas identidades no mesmo host."`
  - Botão CTA: mesmo `hx-get` do botão "Nova Conta" do header
- Se não vazio: renderizar tabela normal

**Regras:**
- Empty state e tabela preenchida nunca aparecem juntos
- O botão CTA do empty state deve ser idêntico ao "Nova Conta" do header (mesmo hx-get, mesmo drawer)

---

---

## EPIC 7 — Qualidade e Segurança

---

### TASK 7.1 — Logger estruturado com slog

**Objetivo:**
Configurar `log/slog` com output para arquivo e terminal, adequado para cada modo.

**Arquivos a criar/editar:**
- `internal/app/logger.go`
- `internal/app/app.go` (inicializar logger no `New()`)

**O que fazer:**
- Em `debug=true`: log no terminal com `slog.NewTextHandler(os.Stdout, ...)`
- Em produção: log em `~/.fdev/logs/app.log` com `slog.NewJSONHandler`
- Level: `Info` em produção, `Debug` quando `--debug`
- Setar `slog.SetDefault(logger)` para uso global
- Rotação simples: se `app.log` > 10MB, renomear para `app.log.1` antes de criar novo

**Estrutura de referência:**
```go
func NewLogger(paths *config.Paths, debug bool) (*slog.Logger, error) {
    level := slog.LevelInfo
    if debug {
        level = slog.LevelDebug
    }

    opts := &slog.HandlerOptions{Level: level}

    if debug {
        return slog.New(slog.NewTextHandler(os.Stdout, opts)), nil
    }

    logPath := filepath.Join(paths.Logs, "app.log")
    if err := rotateIfNeeded(logPath, 10*1024*1024); err != nil {
        return nil, err
    }

    f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return nil, err
    }

    return slog.New(slog.NewJSONHandler(f, opts)), nil
}
```

**Regras:**
- Nunca logar: chaves privadas, conteúdo de `state.json`, tokens, senhas
- Em produção, log sempre em JSON (facilita parsing futuro)

---

### TASK 7.2 — Modo dry-run com diff

**Objetivo:**
Simular operações no `~/.ssh/config` mostrando o diff antes de escrever, sem efetivamente alterar o arquivo.

**Arquivos a criar:**
- `internal/ssh/dryrun.go`
- `web/templates/ssh/diff-preview.html`

**O que fazer:**
- Função `PreviewApply(account storage.Account, paths *config.Paths) ([]DiffLine, error)`
- Gerar o conteúdo que seria escrito (sem escrever)
- Comparar linha a linha com o conteúdo atual do `~/.ssh/config`
- Retornar slice de `DiffLine{Type string, Text string}` onde Type é `"added"`, `"removed"` ou `"unchanged"`
- Handler retorna o partial de diff para o usuário confirmar ou cancelar

**Estrutura de referência:**
```go
type DiffLine struct {
    Type string // "added", "removed", "unchanged"
    Text string
}

func PreviewApply(account storage.Account, paths *config.Paths) ([]DiffLine, error) {
    current, _ := os.ReadFile(paths.SSHConfig())
    // gerar o novo conteúdo (lógica do Writer sem escrever)
    next := generateSSHConfig(account, paths, string(current))
    return diffLines(string(current), next), nil
}
```

**Regras:**
- Dry-run ainda faz backup (para o usuário poder inspecionar o estado atual)
- Linhas adicionadas aparecem em verde no template, removidas em vermelho

---

### TASK 7.3 — Tratamento de erros amigável na UI

**Objetivo:**
Garantir que todos os erros chegam ao usuário com mensagem clara e contextual — sem stack trace, sem JSON cru.

**Arquivos a criar:**
- `internal/app/errors.go`
- `web/templates/partials/error-toast.html`

**O que fazer:**
- Struct `AppError{Code, Message string, Err error}`
- `AppError.Error() string` retorna `Message`
- Mapa de erros conhecidos para mensagens amigáveis:
  - `permission denied` → `"Permissão negada: verifique as permissões de ~/.ssh/config"`
  - `no such file` → `"Arquivo não encontrado"`
  - `ErrKeyExists` → `"Chave já existe para este alias. Deseja sobrescrever?"`
  - `ssh: no reachable address` → `"Não foi possível conectar ao host"`
- Middleware `Recoverer` customizado: captura panic, loga com stack trace (só no log), retorna mensagem amigável na UI

**Estrutura de referência:**
```go
type AppError struct {
    Code    string
    Message string
    Err     error // apenas para log — nunca expor na UI
}

func WrapError(err error) *AppError {
    if err == nil { return nil }
    msg := friendlyMessage(err)
    return &AppError{Message: msg, Err: err}
}

func friendlyMessage(err error) string {
    s := err.Error()
    switch {
    case strings.Contains(s, "permission denied"):
        return "Permissão negada. Verifique as permissões do arquivo."
    case errors.Is(err, ErrKeyExists):
        return "Chave já existe para este alias."
    default:
        return "Ocorreu um erro inesperado. Verifique os logs."
    }
}
```

**Regras:**
- Stack traces NUNCA aparecem na UI — apenas em `~/.fdev/logs/app.log`
- Erros de validação são exibidos inline no form (não como toast)
- Erros de operação (SSH, filesystem) são exibidos como toast de erro

---

---

## EPIC 8 — Distribuição

---

### TASK 8.1 — Build release: binário único sem CGO

**Objetivo:**
Garantir que `make release` gera binários portáveis para Linux amd64 e arm64, sem dependências de runtime.

**Arquivos a editar:**
- `Makefile` (já criado na Task 0.2 — verificar e completar)

**O que fazer:**
- Confirmar que `CGO_ENABLED=0` está em todos os targets de release
- Confirmar que build tag `-tags release` está presente (ativa o `//go:embed`)
- Confirmar que `-ldflags "-s -w"` reduz o tamanho do binário
- Adicionar target `make verify-release` que:
  - Compila o binário
  - Roda `./dist/factorydev-linux-amd64 --help` ou `version`
  - Confirma que o binário funciona

**Teste de validação:**
```bash
# Após make release, verificar:
file dist/factorydev-linux-amd64
# deve mostrar: ELF 64-bit LSB executable, statically linked

ldd dist/factorydev-linux-amd64
# deve mostrar: not a dynamic executable
```

**Regras:**
- `ldd` no binário final NÃO pode mostrar dependências de `.so`
- Testar o binário em uma máquina sem Go instalado antes de considerar pronto

---

### TASK 8.2 — CLI: flags, subcomandos e versão

**Objetivo:**
Implementar a CLI completa com subcomandos `doctor` e `version`.

**Arquivos a editar:**
- `cmd/factorydev/main.go`

**O que fazer:**
- `factorydev` → inicia servidor (comportamento padrão)
- `factorydev doctor` → roda `RunDoctor()`, imprime resultado, exit 0 (tudo ok) ou exit 1 (algum check falhou)
- `factorydev version` → imprime versão injetada via ldflags e exit 0
- Flags continuam funcionando: `--port`, `--host`, `--debug`

**Estrutura de referência:**
```go
var Version = "dev" // sobrescrito por: -ldflags "-X main.Version=1.0.0"

func main() {
    if len(os.Args) > 1 {
        switch os.Args[1] {
        case "doctor":
            runDoctorCLI()
            return
        case "version":
            fmt.Printf("FactoryDev %s\n", Version)
            return
        }
    }
    runServer()
}

func runDoctorCLI() {
    paths, _ := config.NewPaths()
    checks := doctor.RunDoctor(paths)
    allOK := true
    for _, c := range checks {
        status := "✓"
        if !c.OK { status = "✗"; allOK = false }
        fmt.Printf("%s  %s: %s\n", status, c.Name, c.Message)
    }
    if !allOK {
        os.Exit(1)
    }
}
```

**Regras:**
- `factorydev doctor` deve ter exit code 1 se qualquer check falhar (útil em scripts de CI/setup)
- `Version` deve ser `"dev"` quando compilado sem ldflags (modo desenvolvimento)

---

---

## ORDEM DE EXECUÇÃO RECOMENDADA

```
Fase 1 → Epic 0 (Tasks 0.1 → 0.4)   — Bootstrap completo
Fase 2 → Epic 1 (Tasks 1.1 → 1.2)   — Filesystem ~/.fdev
Fase 3 → Epic 2 (Tasks 2.1 → 2.3)   — Storage JSON
Fase 4 → Epic 3 (Tasks 3.1 → 3.5)   — Web UI base
Fase 5 → Epic 4 (Tasks 4.1 → 4.4)   — Tool Git Accounts (CRUD)
Fase 6 → Epic 5 (Tasks 5.1 → 5.5)   — SSH Key Gen + Config Writer
Fase 7 → Epic 6 (Tasks 6.1 → 6.2)   — Onboarding
Fase 8 → Epic 7 (Tasks 7.1 → 7.3)   — Qualidade e Segurança
Fase 9 → Epic 8 (Tasks 8.1 → 8.2)   — Distribuição
```

> **Dica:** Ao iniciar uma nova sessão com o Codex, sempre reenvie o bloco `## PROJECT CONTEXT` antes da task. Isso garante que o agente tem o contexto completo mesmo sem memória da sessão anterior.