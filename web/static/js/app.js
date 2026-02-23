// ── Utilitário: detecta a página atual pela URL ─────────────────
function inferPage() {
  const p = window.location.pathname
  if (p.startsWith('/tools/keys')) return 'keys'
  if (p === '/' || p.startsWith('/tools/ssh')) return 'ssh'
  if (p.startsWith('/tools/repos')) return 'repos'
  if (p.startsWith('/doctor')) return 'doctor'
  return ''
}

// ── Toast ──────────────────────────────────────────────────────
function showToastMsg(msg, type) {
  const toast = document.createElement('div')
  toast.className = 'fdev-toast fdev-toast--' + (type || 'success')
  toast.textContent = msg
  document.getElementById('toast-container')?.appendChild(toast)
  setTimeout(() => toast.remove(), 3500)
}

document.body.addEventListener('showToast', function (e) {
  showToastMsg(e.detail.msg, e.detail.type)
})

// ── Drawer ─────────────────────────────────────────────────────
function openDrawer() {
  document.getElementById('drawer')?.classList.remove('fdev-drawer--closed')
  document.getElementById('drawer-overlay')?.classList.remove('hidden')
}

function closeDrawer() {
  document.getElementById('drawer')?.classList.add('fdev-drawer--closed')
  document.getElementById('drawer-overlay')?.classList.add('hidden')
  const content = document.getElementById('drawer-content')
  if (content) content.innerHTML = ''
}

document.body.addEventListener('openDrawer', openDrawer)
document.body.addEventListener('closeDrawer', closeDrawer)

// ── Refresh da lista de repositórios ───────────────────────────
document.body.addEventListener('refreshRepos', function () {
  if (!window.htmx) return
  // Só atualiza se o usuário ainda estiver na página de repos
  if (window.location.pathname.startsWith('/tools/repos')) {
    htmx.ajax('GET', '/tools/repos', { target: '#main-content', swap: 'innerHTML' })
  }
})

// ── HTMX 2.x: permitir swap em 422 (erros de validação) ───────
document.body.addEventListener('htmx:beforeSwap', function (e) {
  if (e.detail.xhr.status === 422) {
    e.detail.shouldSwap = true
    e.detail.isError = false
  }
})

// ── HTMX 2.x: processar HX-Trigger em respostas de erro ───────
// O HTMX 2.x não dispara triggers em respostas não-2xx por padrão.
document.body.addEventListener('htmx:responseError', function (e) {
  const trigger = e.detail.xhr.getResponseHeader('HX-Trigger')
  if (!trigger) return
  const t = trigger.trim()
  if (t.startsWith('{')) {
    try {
      const events = JSON.parse(t)
      if (events.showToast) showToastMsg(events.showToast.msg, events.showToast.type || 'error')
      if (events.openDrawer)  openDrawer()
      if (events.closeDrawer) closeDrawer()
    } catch (_) {}
  } else {
    // Valor simples (ex: "openDrawer")
    if (t === 'openDrawer')  openDrawer()
    if (t === 'closeDrawer') closeDrawer()
  }
})

// ── Expandir/recolher detalhes de conta ───────────────────────
function toggleAccountDetails(id) {
  const el = document.getElementById('account-details-' + id)
  if (!el) return
  el.classList.toggle('hidden')
  const chevron = document.getElementById('chevron-' + id)
  if (chevron) chevron.classList.toggle('open')
}

// ── Mostrar/ocultar chave secreta ──────────────────────────────
function toggleSecret(id, btn) {
  const el = document.getElementById(id)
  if (!el) return
  const hidden = el.classList.toggle('hidden')
  if (btn) btn.textContent = hidden ? 'Mostrar' : 'Ocultar'
}

// ── Copiar conteúdo de elemento para a área de transferência ───
function copyElContent(btn, elId) {
  const el = document.getElementById(elId)
  if (!el || !navigator.clipboard) return
  navigator.clipboard.writeText(el.textContent.trim()).then(function () {
    const original = btn.textContent
    btn.textContent = 'Copiado!'
    setTimeout(() => { btn.textContent = original }, 2000)
  }).catch(function () {
    showToastMsg('Não foi possível copiar', 'error')
  })
}
