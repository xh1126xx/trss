import { EventsOn } from '../wailsjs/runtime/runtime.js';

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

  // 接收字幕事件（现在包含 source 和 target）
  EventsOn('subtitle', (data) => {
    show(data.source, data.target);
  });

  EventsOn('error', (msg) => {
    console.error('[TRSS]', msg);
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
    // bilingual
    targetEl.classList.remove('hidden');
    sourceEl.classList.remove('hidden');
  }
}

function show(sourceText, targetText) {
  clearTimeout(timeoutId);

  // 原文（仅双语或纯原文模式显示）
  if (mode === 'source' || mode === 'bilingual') {
    if (sourceText) {
      sourceEl.textContent = sourceText;
      sourceEl.classList.remove('hidden');
    } else {
      sourceEl.classList.add('hidden');
    }
  }

  // 译文（仅翻译或双语模式显示）
  if (mode === 'target' || mode === 'bilingual') {
    if (targetText) {
      targetEl.textContent = targetText;
      targetEl.classList.remove('hidden');
    } else {
      targetEl.classList.add('hidden');
    }
  }

  // 5 秒后淡出
  timeoutId = setTimeout(() => {
    targetEl.style.opacity = '0.2';
    sourceEl.style.opacity = '0.2';
  }, 5000);

  targetEl.style.opacity = '1';
  sourceEl.style.opacity = '1';
}
