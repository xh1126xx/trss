// 字幕显示管理
let mode = 'target';
let timeoutId = null;
let sourceEl, targetEl, modeSel;

export function initSubtitle() {
  sourceEl = document.getElementById('subtitle-source');
  targetEl = document.getElementById('subtitle-target');
  modeSel = document.getElementById('display-mode');

  modeSel.addEventListener('change', () => {
    setMode(modeSel.value);
  });

  // 等待 Wails 运行时就绪后绑定事件
  waitForWails(() => {
    window.go.main.App.EventsOn('subtitle', (result) => {
      show(result.text, result.isFinal);
    });

    window.go.main.App.EventsOn('error', (msg) => {
      console.error('[TRSS]', msg);
    });

    window.go.main.App.EventsOn('status', (data) => {
      console.log('[TRSS] status:', data);
    });
  });
}

function waitForWails(fn) {
  let attempts = 0;
  const check = setInterval(() => {
    if (window.go && window.go.main && window.go.main.App && window.go.main.App.EventsOn) {
      clearInterval(check);
      fn();
    } else if (++attempts > 100) {
      clearInterval(check);
      console.error('[TRSS] Wails runtime not ready after 10s');
    }
  }, 100);
}

function setMode(m) {
  mode = m;
  if (mode === 'source') {
    targetEl.classList.add('hidden');
    sourceEl.classList.remove('hidden');
  } else if (mode === 'target') {
    targetEl.classList.remove('hidden');
    sourceEl.classList.add('hidden');
  } else {
    targetEl.classList.remove('hidden');
    sourceEl.classList.remove('hidden');
  }
}

function show(text, isFinal) {
  clearTimeout(timeoutId);

  if (mode !== 'source') {
    targetEl.textContent = text;
    targetEl.classList.toggle('final', isFinal);
  }
  if (mode === 'bilingual' || mode === 'source') {
    sourceEl.textContent = text;
  }

  if (isFinal) {
    timeoutId = setTimeout(() => {
      targetEl.style.opacity = '0.3';
    }, 5000);
  } else {
    targetEl.style.opacity = '1';
  }
}
