// 设置面板管理
let panel;

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
      <div class="settings-tabs">
        <button class="tab active" data-tab="translation">翻译配置</button>
        <button class="tab" data-tab="display">显示</button>
      </div>

      <div class="tab-content" id="tab-translation">
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
          <button id="btn-save" class="primary">保存方案</button>
        </div>
        <div id="test-result"></div>
      </div>

      <div class="tab-content hidden" id="tab-display">
        <div class="form-group">
          <label>字体大小 (<span id="dsp-font-label">22</span>px)</label>
          <input type="range" id="dsp-font-size" min="14" max="40" value="22" />
        </div>
        <div class="form-group">
          <label>背景透明度 (<span id="dsp-opacity-label">85</span>%)</label>
          <input type="range" id="dsp-bg-opacity" min="30" max="100" value="85" />
        </div>
      </div>
    </div>
  `;
  document.body.appendChild(panel);
  bindEvents();
}

function bindEvents() {
  const close = () => {
    panel.classList.toggle('open');
  };

  panel.querySelector('#btn-close-settings').addEventListener('click', close);
  panel.querySelector('.settings-overlay').addEventListener('click', close);

  // 标签切换
  panel.querySelectorAll('.tab').forEach(tab => {
    tab.addEventListener('click', () => {
      panel.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
      tab.classList.add('active');
      panel.querySelectorAll('.tab-content').forEach(c => c.classList.add('hidden'));
      const target = panel.querySelector('#tab-' + tab.dataset.tab);
      if (target) target.classList.remove('hidden');
    });
  });

  // 显示设置实时预览
  const fontSizeSlider = panel.querySelector('#dsp-font-size');
  const opacitySlider = panel.querySelector('#dsp-bg-opacity');
  if (fontSizeSlider) {
    fontSizeSlider.addEventListener('input', () => {
      panel.querySelector('#dsp-font-label').textContent = fontSizeSlider.value;
      document.querySelectorAll('.subtitle-line.tgt').forEach(el => {
        el.style.fontSize = fontSizeSlider.value + 'px';
      });
    });
  }
  if (opacitySlider) {
    opacitySlider.addEventListener('input', () => {
      const pct = opacitySlider.value;
      panel.querySelector('#dsp-opacity-label').textContent = pct;
      const alpha = pct / 100;
      document.querySelectorAll('.subtitle-line').forEach(el => {
        el.style.background = 'rgba(0, 0, 0, ' + alpha.toFixed(2) + ')';
      });
    });
  }

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
      close(); // 关闭面板
    } catch (e) {
      alert('保存失败: ' + e);
    }
  });
}

export function toggle() {
  if (panel) panel.classList.toggle('open');
}
