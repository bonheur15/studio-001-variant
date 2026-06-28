(function() {
  var meta = document.querySelector('meta[name="session-id"]');
  var sessionId = meta ? meta.getAttribute('content') : null;
  if (sessionId) {
    sessionStorage.setItem('session-id', sessionId);
  }

  document.addEventListener('htmx:configRequest', function(e) {
    var sid = sessionStorage.getItem('session-id');
    if (sid) {
      e.detail.headers['X-Session-Id'] = sid;
    }
  });
})();

function switchMode(mode) {
  var fields = document.getElementById('fields-mode');
  var stringMode = document.getElementById('string-mode');
  var fieldsBtn = document.getElementById('mode-fields');
  var stringBtn = document.getElementById('mode-string');

  if (mode === 'fields') {
    fields.classList.remove('hidden');
    stringMode.classList.add('hidden');
    fieldsBtn.className = 'flex-1 px-3 py-1.5 text-sm font-medium rounded-md bg-white shadow-sm text-gray-900';
    stringBtn.className = 'flex-1 px-3 py-1.5 text-sm font-medium rounded-md text-gray-500 hover:text-gray-700';
  } else {
    fields.classList.add('hidden');
    stringMode.classList.remove('hidden');
    stringBtn.className = 'flex-1 px-3 py-1.5 text-sm font-medium rounded-md bg-white shadow-sm text-gray-900';
    fieldsBtn.className = 'flex-1 px-3 py-1.5 text-sm font-medium rounded-md text-gray-500 hover:text-gray-700';
  }
}

document.addEventListener('DOMContentLoaded', function() {
  var driver = document.getElementById('driver');
  var port = document.getElementById('port');

  if (driver && port) {
    var ports = {
      mysql: '3306',
      postgres: '5432',
      mongodb: '27017'
    };

    driver.addEventListener('change', function() {
      if (!port.value || port.value === '3306' || port.value === '5432' || port.value === '27017') {
        port.value = ports[this.value] || '';
      }
    });
  }
});

function toggleSidebar() {
  var sidebar = document.getElementById('sidebar');
  var overlay = document.getElementById('sidebar-overlay');
  if (sidebar && overlay) {
    var isOpen = sidebar.classList.toggle('open');
    overlay.classList.toggle('open', isOpen);
  }
}

document.addEventListener('htmx:beforeSwap', function(e) {
  if (e.detail.target && e.detail.target.id === 'main-area' && window.innerWidth < 1024) {
    var sidebar = document.getElementById('sidebar');
    var overlay = document.getElementById('sidebar-overlay');
    if (sidebar && overlay) {
      sidebar.classList.remove('open');
      overlay.classList.remove('open');
    }
  }
});
