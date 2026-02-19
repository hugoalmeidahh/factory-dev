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

document.body.addEventListener('showToast', function (e) {
  const { msg, type } = e.detail
  const toast = document.createElement('div')
  toast.className = 'fdev-toast fdev-toast--' + (type || 'success')
  toast.textContent = msg
  document.getElementById('toast-container')?.appendChild(toast)
  setTimeout(() => toast.remove(), 3000)
})

document.body.addEventListener('openDrawer', openDrawer)
document.body.addEventListener('closeDrawer', closeDrawer)
document.body.addEventListener('refreshAccounts', function () {
  if (window.htmx) {
    window.htmx.ajax('GET', '/tools/ssh/accounts', {
      target: '#main-content',
      swap: 'innerHTML'
    })
  }
})

function toggleAccountDetails(id) {
  const el = document.getElementById('account-details-' + id)
  if (!el) return
  el.classList.toggle('hidden')
}

function toggleSecret(id, btn) {
  const el = document.getElementById(id)
  if (!el) return
  const hidden = el.classList.toggle('hidden')
  if (btn) btn.textContent = hidden ? 'Mostrar' : 'Ocultar'
}
