import './style.css';
import { initSubtitle } from './subtitle.js';
import { initSettings } from './settings.js';
import { EventsOn } from '../wailsjs/runtime/runtime.js';

// 同声传译 - 前端入口
document.addEventListener('DOMContentLoaded', () => {
  initSubtitle();
  initSettings();

  const btnToggle = document.getElementById('btn-toggle');
  const profileSelect = document.getElementById('profile-select');
  const statusText = document.getElementById('status-text');

  let isRunning = false;

  // 监听运行状态变化
  EventsOn('status', (data) => {
    if (data.listening) {
      isRunning = true;
      btnToggle.textContent = '⏸ 暂停';
      btnToggle.classList.add('active');
      statusText.textContent = '● 运行中';
      statusText.classList.add('active');
    } else {
      isRunning = false;
      btnToggle.textContent = '▶ 开始';
      btnToggle.classList.remove('active');
      statusText.textContent = '⏸ 已暂停';
      statusText.classList.remove('active');
    }
  });

  // 加载配置方案列表
  async function loadProfiles() {
    if (!window.go || !window.go.main || !window.go.main.App) return;
    try {
      const configs = await window.go.main.App.GetConfigs();
      const currentVal = profileSelect.value;
      profileSelect.innerHTML = '<option value="">选择配置方案...</option>';
      (configs || []).forEach((cfg) => {
        const opt = document.createElement('option');
        opt.value = cfg.name;
        opt.textContent = cfg.name;
        profileSelect.appendChild(opt);
      });
      if (currentVal) profileSelect.value = currentVal;
    } catch (e) {
      console.error('Failed to load profiles:', e);
    }
  }

  // 开始/暂停按钮
  btnToggle.addEventListener('click', async () => {
    const profile = profileSelect.value;
    if (!profile) {
      alert('请先选择配置方案');
      return;
    }

    if (!isRunning) {
      try {
        await window.go.main.App.StartListening(profile);
      } catch (e) {
        alert('启动失败: ' + e);
      }
    } else {
      window.go.main.App.StopListening();
    }
  });

  // 初始加载 + 定期刷新
  loadProfiles();
  setInterval(loadProfiles, 3000);
});
