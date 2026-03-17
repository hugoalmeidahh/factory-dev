# Planejamento de Novas Funcionalidades - FactoryDev

Este documento detalha as histórias de usuário (formato Jira), sub-tarefas e refinamento técnico para as novas funcionalidades solicitadas no FactoryDev.

## Epic 1: Ambiente de Desenvolvimento e Configurações

### Story 1: Centralização de Variáveis de Ambiente (.env)
**Como** desenvolvedor, **quero** gerenciar todos os meus arquivos `.env` globais e por projeto dentro da pasta `.fdev` **para** facilitar o reuso das chaves API e a segurança destas configurações.

**Refinamento Técnico:**
- Criar a sub-pasta `~/.fdev/envs/`.
- Permitir edição de variáveis por projeto através de interface Key-Value com Alpine.js na web.
- Salvar dados usando o pacote genérico `internal/storage` em formato encriptado com chave local, ou em puro texto atrelado aos projetos.
- Permitir injeção destas envs ao clonar os repositórios, ao abrir terminais nos diretórios-alvos e na injeção para o executor do Docker Manager.

**Sub-tarefas:**
- [ ] Criar estruturação no `state.json` para mapear paths de projetos aos respectivos arquivos de `.env`.
- [ ] Construir interface web (tabela dinâmica Key/Value) para edição visual das envs.
- [ ] Implementar integração nos Handlers Go para leitura/escrita destes arquivos e carga nos perfis dos projetos.

### Story 2: Gerenciador de Aliases do Sistema
**Como** desenvolvedor, **quero** gerenciar atalhos de terminal (aliases) do meu sistema visualmente **para** criar atalhos rápido sem precisar editar arquivos de rc do Unix manualmente.

**Refinamento Técnico:**
- O projeto FactoryDev deve gravar um arquivo concentrador `~/.fdev/aliases.sh`.
- O FactoryDev irá injetar automaticamente (se não existir) um `source ~/.fdev/aliases.sh` no `~/.zshrc` e `~/.bashrc` do usuário (compatibilidade macOS e Linux).
- O CRUD de aliases atualizará atômicamente este arquivo isolado, evitando quebrar o terminal do utilizador.

**Sub-tarefas:**
- [ ] Rotina em Go para checar presença em configs ZSH e Bash e injetar o arquivo `.fdev/aliases.sh`.
- [ ] CRUD web completo para Adicionar, Modificar e Apagar `alias atalho="comando longo"`.
- [ ] Endpoint backend em Go para gravar o arquivo com lock local de SO para segurança de threads.

### Story 3: Instalador Base de Ferramentas (Package Manager Interativo)
**Como** colaborador recebendo novo equipamento, **quero** poder instalar rapidamente dependências como N/Node, Golang, Android, HTTPie, Docker, Git e Homebrew **para** acelerar o setup do OS.

**Refinamento Técnico:**
- Mão-na-roda essencial usando uma abstração `internal/installer`.
- **macOS:** Instalar primeiramente Homebrew se não existir. Em seguida executar `brew install` ou `brew install --cask`.
- **Linux:** Validar package mangers (`apt-get`, `dnf`) ou usar binários puros de source/CURL via Terminal.
- Requerer elevação de privilégio com prompts (sudo) interativamente para que a instalação não falhe.

**Sub-tarefas:**
- [ ] Mapear matriz/manifesto JSON no backend com as ferramentas disponíveis, pacotes por sistema, e o comando condizente de check status.
- [ ] Função para detectar "Instalado/Falta" usando varredura via `os/exec.LookPath()`.
- [ ] Componente visual de cards ou tabela (Tool + Status + Botões Ação Instal/Uninstall) com um modal Web-Terminal atrelado (Xterm.js proxy para capturar os logs nativos de compilações em andamento).

---

## Epic 2: Segurança, Processos e Redes

### Story 4: Gerenciador de Processos Ativos (Task Manager)
**Como** usuário técnico, **quero** visualizar quais processos estão pesando na máquina e encerrá-los facilmente com 1 botão, **para** liberar RAM e CPU diretamente.

**Refinamento Técnico:**
- Usar o pacote `github.com/shirou/gopsutil/v3/process` da dependência atual.
- Exibir os top N processos em uma tabela UI ao estilo *Activity Monitor* / *HTop*, atualizada de forma branda via HTMX (Polling 5s) e controlada por flag active window para não onerar o navegador.
- Ao clicar em "Kill", emitir ordem interna chamando `os.Process.Kill()` ou envio direto de signal SIGKILL para o PID nativo correspondente.

**Sub-tarefas:**
- [ ] API Controller para devolver payload com a lista de top Memory/CPU PIDs ativos.
- [ ] Template UI e formatação da tabela de uso com ordenação e filtro client-side (Alpine.js).
- [ ] Endpoint especial POST para encerrar processo nativo e lidar com processos de sistema/root perante erro "Permission denied" ou prompt Sudo.

### Story 5: Monitor de Tráfego de Rede e Bloqueio
**Como** engenheiro de sistema, **quero** monitorizar o tráfego gerado pela máquina, analisar transferências em conexões ativas e ter opção de bloquear conexões específicas **para** auditar consumo ou parar vazamento de pacotes indesejados.

**Refinamento Técnico:**
- Usar subpacote `gopsutil/net` atrelado aos pacotes Pcap no Go ou `lsof`/`netstat` nativos para cruzar as conexões com os processos geradores.
- Identificar host remoto (Remote IP, Port, Status LISTEN/ESTABLISHED).
- Operação de boqueio se dará alterando temporariamente tabela de rede nativa local (ex: Linux blackhole network route `sudo ip route add blackhole <ip_address>`, iptables drop, ou macOS via `pf` - Packet Filter).

**Sub-tarefas:**
- [ ] Construir loop coletor em background in-memory das métricas para exibir e mensurar RX/TX em tempo real sem crashar com excessos de chamadas sistêmicas.
- [ ] Exibir endpoints remotos ativos interligados a Processos em uma tabela.
- [ ] Implementar mecânica de Go `exec.Command` com privilégios de Root (sudo) para inserir regras de drop ou null route na interface de rede.

### Story 6: Configuração Simplificada de Firewall
**Como** desenvolvedor trafegando em redes de internet comuns, **quero** analisar configurações e habilitar/desativar o firewall do OS através do UI **para** garantir segurança rápida.

**Refinamento Técnico:**
- Mapeamento direto do binário subjacente: `ufw` ou `firewalld` no Linux; `/usr/libexec/ApplicationFirewall/socketfilterfw` no macOS.
- O factorydev apenas mudará ativamente o `--setglobalstate on | off` ou habilitará o `service ufw start`.

**Sub-tarefas:**
- [ ] Wrapper em `internal/system` para detectar e lidar com o Engine de Firewall.
- [ ] Widget visual interligado ao painel do "System Info" ou "Network" e tela base para Allow Port/Deny Port.
- [ ] Go Handlers de toggle com requisição de Sudo (prompt local/auth root).

---

## Epic 3: Automação CI/CD

### Story 7: Configuração de GitHub Actions (Workflow de Pipeline e Build)
**Como** mantenedor, **quero** que todo push passe em steps de Lint e Testes automáticos, compilando os executáveis pra AMD/ARM Linux e Mac em Tags **para** automatizar a distribuição deste projeto.

**Refinamento Técnico:**
- Usar os comandos existentes no `Makefile` atual (como `make lint` e `make release`), adaptando-os no workflow (`.github/workflows/main.yml`).
- Pipeline de pull requests apenas valida linter/tests e build. Push tag gera a página de *Releases* nativa do GitHub com artefatos injetados usando `softprops/action-gh-release`.

**Sub-tarefas:**
- [ ] Criação do `.github/workflows/ci.yml` rodando `golangci-lint` em PRs.
- [ ] Arquivo `.github/workflows/release.yml` instigado apenas por tags Git (`v*.*.*`).
- [ ] Pipeline multi-os, gerando artefatos pelo `go build` cruzando `GOOS`/`GOARCH` (darwin-amd64/arm64 e linux-amd64/arm64) e anexando nas releases do GHA.

---

## Sugestões Futuras Extras e Oportunidades (Backlog)

1. **Local SSL/TLS (Integração mkcert):** Um clique para abstrair o comando `mkcert`, gerando um certificado HTTPS confiável pro `localhost`, vital se for testar Webhooks HTTPS do lado do Front.
2. **Gerenciador de Port Forwarding / Ngrok Local:** Interface para embutir/ligar Cloudflared/Ngrok apontando para os containers locais na interface "Repositórios" ou "Docker", retornando aos dev a URL pública com clique de botão "Copiar".
3. **Database Runner Rápido:** Uma tela auxiliar que se auto-conecta nos containers SQL/Redis do Docker Manager e permite rodar query rápida de debug e ver saídas sem carregar ferramentas desktop pesadas como DBeaver se for algo pontual.
4. **Editor Visual do Arquivo `hosts`:** Uma forma rápida de o utilizador modificar os nomes de serviço apontando pro Docker (ex: escrever `127.0.0.1 proj1.local`) direto da página de configuração de repositório, sem precisar do terminal, limpando automaticamente o DNS cache de sistema `mDNSResponder`.
5. **Integração de Notificações Nativas:** Uma opção onde os workers assíncronos (como Git Clone) possam terminar e notificar usando o utilitário Notification Center ou DBus avisando visualmente o término.
