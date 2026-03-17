# FactoryDev - Padrões de Desenvolvimento (Go / HTMX)

Este documento define os padrões arquiteturais e de codificação para o backend e interface web do **FactoryDev**, um painel as-a-service e gerenciador de utilites local. O objetivo é garantir um código idiomático no Go, separação modular e uma resposta instantânea do lado da interface através de Alpine.js e HTMX.

## Stack e Arquitetura Base
- **Backend**: Go (Golang) 1.24
- **Frontend / UI**: HTML templates gerados do lado do servidor (SSR), Alpine.js, Tailwind CSS (se aplicável), e HTMX (sem frameworks pesados).
- **Roteamento**: `go-chi/chi` (leve e idiomático junto a `net/http` do standard library).
- **Persistência**: Banco em arquivo local estruturado (ex: `state.json`) modificado ativamente em tempo de execução para garantir dependência zero da infra de cliente.
- **Distribuição**: `go:embed` (garante que tudo compile num único executável limpo).
- **Interações do Sistema**: Integrações com SDK de Docker, chamadas de sistema (subprocessos command / exec), e extração de SO via `gopsutil`.

## Comandos Principais
O ciclo de vida do desenvolvimento no Go é gerenciado através da ferramenta Make (`Makefile`), mas essencialmente utiliza standard Go:
- `make dev` — Iniciar o servidor local (roda via `go run ./cmd/factorydev/...`).
- `make build` — Compilar o executável para uso da própria máquina (`bin/factorydev`).
- `make lint` — Executar análise e formatação do código, em conjunto com o `golangci-lint` ou `go vet`.
- `make clean` — Remover os binários compilados das pastas `bin/` ou `dist/`.

## Estrutura de Diretórios e Pacotes

A estrutura obedece ao Standard Go Layout.

```
/
├── cmd/
│   └── factorydev/   # O entry point do software (main.go). Deve ser o menor possível e apenas iniciar as configurações.
├── internal/         # Código privado da nossa aplicação ("business logic"). Outros projetos não podem importar isso.
│   ├── app/          # Core da aplicação.
│   ├── config/       # Processamento e parse da configuração local/state.
│   ├── handler/      # Controladores HTTP. Cada arquivo gerencia uma feature via endpoints pros fragmentos HTMX.
│   ├── ssh/          # Integrações com ssh/config e chaves locais.
│   ├── git/          # Interações com binários ou repositórios Git.
│   ├── docker/       # Clientes e wrappers em cima do docker daemon.
│   └── system/       # Leitura de métricas e hardware local.     
├── web/
│   ├── static/       # Arquivos que bypassam renderização, carregados via chi.FileServer (CSS/JS puros).
│   └── templates/    # Fragmentos e esqueletos base HTML invocados nos handlers.
└── docs/             # Arquitetura, Onboarding de código, etc.
```

## Convenções de Nomenclatura

- **Pacotes Go:** Devem ser em letras minúsculas (ex: `handler`, não `Handler`, não `HttpHandlers`).
- **Arquivos Go:** Em `snake_case` curtos e relativos a responsabilidade (ex: `ssh_handler.go`, `gitconfig.go`).
- **Variáveis / Funções / Structs (Go):** 
  - Usar `PascalCase` **APENAS SE** for ser exportado para um pacote fora do seu.
  - Usar `camelCase` se for privado ao pacote onde foi escrito.
  - Ex: tipo utilitário não precisando ser visível no handler -> `type dockerConfig struct {...}`. Função que um Handler de fora vai chamar -> `func FetchSystemStats() (...)`.
- **Arquivos HTML:** Geralmente `snake_case.html` e formatados em pedaços (fragments) quando servirem a uma requisição do HTMX.

## Regras Críticas de Desenvolvimento

### Separação de Lógica e Renderização
A renderização jamais deve conter lógica de regras de negócio.
- O Handlers no `internal/handler/` devem processar a lógica, construir struct anônimas com os dados requeridos, e finalmente passar pra func `template.Execute()`.
- Lógicas complexas com Docker ou SSH não ficam dentro dos Handlers, e sim em serviços em pacotes como `internal/docker` (Handler atua apenas enviando um Status 400 ou renderizando o swap do template HTMX).

### Lógica HTMX (Eventos do Frontend)
Sempre que possível resolva ações locais do usuário com o Alpine (`x-data`, `x-show`, modais temporários) a menos que o dado necessite da confiabilidade ou alteração real do OS.
- Enviar form requests usando `hx-post` a um endpoint da API em Go, recebendo um novo bloco HTML com atualizações da view, que vai substituir via `hx-target` ou `hx-swap`.

### Tratamento de Erros e Boas Práticas (Go)
- Nunca usar `panic()` a menos que seja imprevisivelmente fatal durante startup do router.
- Em Go "Errors are values". Sempre valide o retorno `err != nil`. Se ocorrer num Handler HTTP e precisar notificar no Frontend, devolva um bloco HTML contendo o componente de Toasting/Erro.
- Ao repassar um erro para o log/terminal de debug na web, use "wrapping" com `fmt.Errorf("falha listando repositórios: %w", err)`.

### Persistência do Estado (Sync Seguro)
A alteração do `state.json` (que salva tags de containers, preferências visuais ou repositórios) deve ser evitada de race conditions. 
- Use os padrões do pacote de config local e `sync.RWMutex` do Go em caso de manipulação in-memory com flushes atômicos para o JSON em disco.

### Segurança e Execução
Na operação direta na máquina (exec de binários shell locais para ssh, git clone ou docker) tenha máxima validação de form inputs nos Go Handlers impedindo command injection. 
- Use funções como `exec.Command("git", "clone", repoUrl)` passando os argumentos da array de forma isolada, em vez de interpolar com Strings shell-like sem tratamento.
