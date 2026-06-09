import { EventsOn } from '../wailsjs/runtime/runtime.js';

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

  // 直接使用 Wails runtime 监听事件（不在 App 上）
  EventsOn('subtitle', (result) => {
    console.log('[TRSS] subtitle:', result);
    show(result.text, result.isFinal);
  });

  EventsOn('error', (msg) => {
    console.error('[TRSS]', msg);
    // 在前端也显示错误
    show('⚠ ' + msg, true);
  });
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
