# Tarefas do Projeto FactoryDev (Novas Funcionalidades)

## Epic 1: Ambiente de Desenvolvimento e Configurações
- [ ] **Story 1: Centralização de Variáveis de Ambiente (.env)**
  - [ ] Criar estruturação no `state.json` para mapear paths de projetos
  - [ ] Construir interface web (tabela dinâmica Key/Value) para edição visual
  - [ ] Implementar integração nos Handlers Go para leitura/escrita e injeção
- [ ] **Story 2: Gerenciador de Aliases do Sistema**
  - [ ] Criar rotina Go para injetar source `.fdev/aliases.sh` no `.zshrc`/`.bashrc`
  - [ ] Criar CRUD web para gerenciar aliases
  - [ ] Endpoint backend para gravar as alterações no arquivo isolado
- [ ] **Story 3: Instalador Base de Ferramentas**
  - [ ] Estruturar matriz JSON de apps via `internal/installer` e specs por OS
  - [ ] Implementar verificação de binários instalados via `os/exec.LookPath`
  - [ ] Desenvolver Componente UI (cards/tabela) com botões e modal logging

## Epic 2: Segurança, Processos e Redes
- [ ] **Story 4: Gerenciador de Processos Ativos (Task Manager)**
  - [ ] Construir API Controller para listar top PIDs `gopsutil`
  - [ ] Criar Template HTMX com auto-refresh exibindo CPU/RAM
  - [ ] Endpoint para Matar (Kill/SIGKILL) processos baseados no request UI
- [ ] **Story 5: Monitor de Tráfego de Rede**
  - [ ] Implementar rotina coletora (cacher) de TX/RX usando NetStat+PIDs
  - [ ] Desenvolver UI apresentando Host Remoto (IP/Ports) por Processo
  - [ ] Implementar Handler de Bloqueio Rápido (Local null route / iptables drop)
- [ ] **Story 6: Configuração Simplificada de Firewall**
  - [ ] Wrapper em `internal/system` para detectar CLI do OS Firewall (`ufw`/`socketfilterfw`)
  - [ ] Construir Widget UI On/Off de Global Status com request HTMX
  - [ ] Tratar injeção de senha SUDO e toggle do estado

## Epic 3: Automação CI/CD
- [x] **Story 7: Configuração de GitHub Actions (Workflow)**
  - [x] Criação e teste do `.github/workflows/ci.yml` (Lint e Test em PRs)
  - [x] Implementação de `.github/workflows/release.yml` para tags (`v*.*.*`)
  - [x] Setup da matriz de Build (Mac/Linux AMD/ARM) e Upload de Releases GHA
