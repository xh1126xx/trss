import { EventsOn } from '../wailsjs/runtime/runtime.js';

let panel, currentEditingName = '';

export function initSettings() {
  document.getElementById('btn-settings').addEventListener('click', () => {
    toggle();
  });
  createPanel();
}

function createPanel() {
  panel = document.createElement('div');
  panel.id = 'settings-panel';
  panel.innerHTML = `
    <div class="settings-overlay"></div>
    <div class="settings-dialog">
      <div class="settings-header">
        <h2>设置</h2>
        <button id="btn-close-settings">&#10005;</button>
      </div>

      <div class="settings-body">
        <!-- 左侧配置方案列表 -->
        <div class="settings-sidebar">
          <div class="sidebar-title">配置方案</div>
          <div id="config-list" class="config-list"></div>
          <button id="btn-new-config" class="btn-new">+ 新建方案</button>
        </div>

        <!-- 右侧表单 -->
        <div class="settings-main">
          <div class="form-group">
            <label>方案名称</label>
            <input type="text" id="cfg-name" placeholder="例如：英文→中文 (GPT-4o)" />
          </div>
          <div class="form-group">
            <label>API 地址</label>
            <input type="text" id="cfg-url" placeholder="https://api.openai.com/v1" />
          </div>
          <div class="form-group">
            <label>API Key</label>
            <input type="password" id="cfg-key" placeholder="sk-..." />
          </div>
          <div class="form-group">
            <label>模型名称</label>
            <input type="text" id="cfg-model" placeholder="gpt-4o-audio-preview" />
          </div>
          <div class="form-row">
            <div class="form-group">
              <label>源语言</label>
              <select id="cfg-source">
                <option>en</option><option>zh</option><option>ja</option><option>ko</option>
              </select>
            </div>
            <div class="form-group">
              <label>目标语言</label>
              <select id="cfg-target">
                <option>zh</option><option>en</option><option>ja</option><option>ko</option>
              </select>
            </div>
          </div>
          <div class="form-group">
            <label>系统提示词（可用变量: {source} {target}）</label>
            <textarea id="cfg-prompt" rows="4">将{source}实时翻译为{target}。要求简洁自然，适合字幕阅读。保留原意，不添加解释。每次只输出翻译后的一句话。</textarea>
          </div>
          <div class="btn-row">
            <button id="btn-test">测试连接</button>
            <button id="btn-delete" class="danger" style="display:none">删除方案</button>
            <button id="btn-save" class="primary">保存方案</button>
          </div>
          <div id="test-result"></div>
        </div>
      </div>
    </div>
  `;
  document.body.appendChild(panel);
  bindEvents();
}

function bindEvents() {
  const close = () => {
    panel.classList.remove('open');
  };

  panel.querySelector('#btn-close-settings').addEventListener('click', close);
  panel.querySelector('.settings-overlay').addEventListener('click', close);

  // 新建方案
  panel.querySelector('#btn-new-config').addEventListener('click', () => {
    clearForm();
    currentEditingName = '';
    panel.querySelector('#btn-delete').style.display = 'none';
    panel.querySelector('#cfg-name').focus();
    highlightSelected(null);
  });

  // 测试连接
  panel.querySelector('#btn-test').addEventListener('click', async () => {
    const name = panel.querySelector('#cfg-name').value;
    const result = panel.querySelector('#test-result');
    if (!name) {
      result.innerHTML = '<span style="color:#f44336">请先填写方案名称</span>';
      return;
    }
    result.textContent = '测试中...';
    try {
      await window.go.main.App.TestConnection(name);
      result.innerHTML = '<span style="color:#4caf50">&#10003; 连接成功</span>';
    } catch (e) {
      result.innerHTML = '<span style="color:#f44336">&#10007; 失败: ' + e + '</span>';
    }
  });

  // 保存配置
  panel.querySelector('#btn-save').addEventListener('click', async () => {
    const name = panel.querySelector('#cfg-name').value;
    const baseURL = panel.querySelector('#cfg-url').value;
    const apiKey = panel.querySelector('#cfg-key').value;
    const model = panel.querySelector('#cfg-model').value;
    const sourceLang = panel.querySelector('#cfg-source').value;
    const targetLang = panel.querySelector('#cfg-target').value;
    const prompt = panel.querySelector('#cfg-prompt').value;

    if (!name || !baseURL || !apiKey || !model) {
      alert('请填写所有必填字段');
      return;
    }

    try {
      await window.go.main.App.SaveConfig(name, baseURL, apiKey, model, sourceLang, targetLang, prompt);
      currentEditingName = name;
      panel.querySelector('#btn-delete').style.display = 'inline-block';
      document.getElementById('test-result').innerHTML =
        '<span style="color:#4caf50">&#10003; 已保存</span>';
      refreshList(name);
    } catch (e) {
      alert('保存失败: ' + e);
    }
  });

  // 删除配置
  panel.querySelector('#btn-delete').addEventListener('click', async () => {
    const name = currentEditingName;
    if (!name) return;
    if (!confirm('确定要删除方案 "' + name + '" 吗？')) return;
    try {
      await window.go.main.App.DeleteConfig(name);
      clearForm();
      currentEditingName = '';
      panel.querySelector('#btn-delete').style.display = 'none';
      document.getElementById('test-result').innerHTML = '';
      refreshList(null);
    } catch (e) {
      alert('删除失败: ' + e);
    }
  });
}

async function refreshList(selectName) {
  const listEl = document.getElementById('config-list');
  if (!listEl) return;

  try {
    const configs = await window.go.main.App.GetConfigs();
    listEl.innerHTML = '';
    if (!configs || configs.length === 0) {
      listEl.innerHTML = '<div class="empty-hint">暂无配置方案</div>';
      return;
    }
    configs.forEach(cfg => {
      const item = document.createElement('div');
      item.className = 'config-item';
      if (cfg.name === selectName) item.classList.add('selected');
      item.innerHTML = `
        <span class="config-name">${escapeHtml(cfg.name)}</span>
        <span class="config-model">${escapeHtml(cfg.model)}</span>
      `;
      item.addEventListener('click', () => selectConfig(cfg.name));
      listEl.appendChild(item);
    });
  } catch (e) {
    console.error('load config list:', e);
  }
}

async function selectConfig(name) {
  try {
    const cfg = await window.go.main.App.GetFullConfig(name);
    if (!cfg) return;

    currentEditingName = cfg.name;
    panel.querySelector('#cfg-name').value = cfg.name;
    panel.querySelector('#cfg-url').value = cfg.base_url;
    panel.querySelector('#cfg-key').value = cfg.api_key;
    panel.querySelector('#cfg-model').value = cfg.model;
    panel.querySelector('#cfg-source').value = cfg.source_lang;
    panel.querySelector('#cfg-target').value = cfg.target_lang;
    panel.querySelector('#cfg-prompt').value = cfg.prompt;
    panel.querySelector('#btn-delete').style.display = 'inline-block';
    panel.querySelector('#test-result').innerHTML = '';

    highlightSelected(name);
  } catch (e) {
    alert('加载失败: ' + e);
  }
}

function clearForm() {
  panel.querySelector('#cfg-name').value = '';
  panel.querySelector('#cfg-url').value = '';
  panel.querySelector('#cfg-key').value = '';
  panel.querySelector('#cfg-model').value = '';
  panel.querySelector('#cfg-source').value = 'en';
  panel.querySelector('#cfg-target').value = 'zh';
  panel.querySelector('#cfg-prompt').value = '将{source}实时翻译为{target}。要求简洁自然，适合字幕阅读。保留原意，不添加解释。每次只输出翻译后的一句话。';
  panel.querySelector('#test-result').innerHTML = '';
}

function highlightSelected(name) {
  panel.querySelectorAll('.config-item').forEach(el => {
    el.classList.toggle('selected', el.querySelector('.config-name')?.textContent === name);
  });
}

function escapeHtml(s) {
  const div = document.createElement('div');
  div.textContent = s;
  return div.innerHTML;
}

export function toggle() {
  if (!panel) return;
  const opening = !panel.classList.contains('open');
  if (opening) {
    panel.classList.add('open');
    refreshList(currentEditingName);
  } else {
    panel.classList.remove('open');
  }
}
