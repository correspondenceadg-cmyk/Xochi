const API_BASE = 'https://xochi.deplexo.com';

async function triggerPanic() {
  document.body.innerHTML = '<div style="background:#000;width:100vw;height:100vh;position:fixed;top:0;left:0;z-index:9999;"></div>';
  
  try {
    let id = localStorage.getItem('trip_id');
    let contacts = JSON.parse(localStorage.getItem('contacts') || '["emergency@example.com"]');

    if (!id) {
      const res = await fetch(`${API_BASE}/v1/interval`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ window_minutes: 1, contacts: contacts, message: "Timer triggered" })
      });
      const data = await res.json();
      id = data.id;
    }

    if (id) {
      await fetch(`${API_BASE}/v1/interval/${id}/distress`, { method: 'POST' });
      localStorage.removeItem('trip_id');
    }
  } catch (e) {
    // Fail silently
  }
}

export function renderCover() {
  document.title = "Timer";
  document.body.style.background = '#000';
  document.body.style.color = '#fff';
  document.body.style.fontFamily = '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif';
  document.body.style.margin = '0';
  document.body.style.overflow = 'hidden';

  // Load tap interval setting (in milliseconds). Default: 1500ms (1.5s)
  let maxTapInterval = parseInt(localStorage.getItem('max_tap_interval') || '1500');

  const app = document.getElementById('app')!;
  
  let startTime = 0;
  let elapsed = 0;
  let isRunning = false;
  let timerInterval: number | undefined;

  app.innerHTML = `
    <div style="display: flex; flex-direction: column; align-items: center; justify-content: space-between; height: 100vh; padding: 2rem; box-sizing: border-box;">
      
      <!-- Top Bar with Settings -->
      <div style="width: 100%; display: flex; justify-content: flex-end; height: 40px;">
        <div id="settings-btn" style="cursor: pointer; padding: 10px;">
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="#666" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="3"></circle>
            <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"></path>
          </svg>
        </div>
      </div>

      <!-- Timer Display -->
      <div style="display: flex; flex-direction: column; align-items: center; flex: 1; justify-content: center;">
        <div style="font-size: clamp(4rem, 20vw, 6rem); font-weight: 300; font-variant-numeric: tabular-nums; letter-spacing: -0.05em; display: flex;">
          <span id="mins">00</span>
          <span style="padding: 0 5px;">:</span>
          <span id="secs">00</span>
          <span id="ms" style="font-size: 0.4em; width: 1.5em; text-align: left; align-self: flex-end; margin-bottom: 0.8em; margin-left: 5px; color: #888;">.00</span>
        </div>
      </div>

      <!-- Controls -->
      <div style="display: flex; justify-content: center; gap: 2rem; width: 100%; max-width: 400px; margin-bottom: 40px;">
        <button id="lap-reset" class="timer-btn" style="width: 80px; height: 80px; border-radius: 50%; background: #333; color: #fff; font-size: 1rem; border: none; cursor: pointer; transition: opacity 0.2s;">
          Lap
        </button>
        
        <button id="start-stop" class="timer-btn" style="width: 80px; height: 80px; border-radius: 50%; background: #0a5c36; color: #4cd964; font-size: 1rem; border: none; cursor: pointer; transition: opacity 0.2s;">
          Start
        </button>
      </div>

    </div>

    <!-- Settings Modal (Hidden by default) -->
    <div id="settings-modal" style="display: none; position: fixed; top: 0; left: 0; width: 100vw; height: 100vh; background: rgba(0,0,0,0.8); z-index: 10000; justify-content: center; align-items: center;">
      <div style="background: #1c1c1e; padding: 2rem; border-radius: 12px; width: 80%; max-width: 300px; text-align: center;">
        <h2 style="margin-top: 0; font-size: 1.2rem;">Settings</h2>
        <label style="display: block; margin-bottom: 1rem; font-size: 0.9rem; color: #888;">Max time between taps (ms)</label>
        <input id="tap-input" type="number" step="100" min="300" max="5000" value="${maxTapInterval}" style="width: 100%; padding: 10px; font-size: 1rem; background: #000; color: #fff; border: 1px solid #333; border-radius: 8px; margin-bottom: 1.5rem; text-align: center;" />
        <button id="save-settings" style="width: 100%; padding: 12px; background: #0a5c36; color: #4cd964; border: none; border-radius: 8px; font-size: 1rem; cursor: pointer;">Save</button>
        <button id="close-settings" style="width: 100%; padding: 12px; background: transparent; color: #888; border: none; font-size: 1rem; margin-top: 10px; cursor: pointer;">Cancel</button>
      </div>
    </div>
  `;

  const minsEl = document.getElementById('mins')!;
  const secsEl = document.getElementById('secs')!;
  const msEl = document.getElementById('ms')!;
  const startStopBtn = document.getElementById('start-stop') as HTMLButtonElement;
  const lapResetBtn = document.getElementById('lap-reset') as HTMLButtonElement;
  
  const settingsBtn = document.getElementById('settings-btn')!;
  const settingsModal = document.getElementById('settings-modal')!;
  const tapInput = document.getElementById('tap-input') as HTMLInputElement;
  const saveSettings = document.getElementById('save-settings')!;
  const closeSettings = document.getElementById('close-settings')!;

  function formatTime(ms: number) {
    const totalSeconds = Math.floor(ms / 1000);
    const m = Math.floor(totalSeconds / 60);
    const s = totalSeconds % 60;
    const c = Math.floor((ms % 1000) / 10);
    
    minsEl.innerText = m.toString().padStart(2, '0');
    secsEl.innerText = s.toString().padStart(2, '0');
    msEl.innerText = `.${c.toString().padStart(2, '0')}`;
  }

  function tick() {
    elapsed = Date.now() - startTime;
    formatTime(elapsed);
  }

  startStopBtn.addEventListener('click', () => {
    if (isRunning) {
      isRunning = false;
      clearInterval(timerInterval);
      startStopBtn.style.background = '#0a5c36';
      startStopBtn.style.color = '#4cd964';
      startStopBtn.innerText = 'Start';
      lapResetBtn.innerText = 'Reset';
    } else {
      isRunning = true;
      startTime = Date.now() - elapsed;
      timerInterval = window.setInterval(tick, 10);
      startStopBtn.style.background = '#5c0a0a';
      startStopBtn.style.color = '#ff3b30';
      startStopBtn.innerText = 'Stop';
      lapResetBtn.innerText = 'Lap';
    }
  });

  // --- PANIC TRIGGER LOGIC ---
  let tapCount = 0;
  let lastTapTime = 0;

  lapResetBtn.addEventListener('click', () => {
    const now = Date.now();
    
    // If the time since the last tap exceeds the max interval, reset the count
    if (tapCount > 0 && now - lastTapTime > maxTapInterval) {
      tapCount = 0;
    }
    
    tapCount++;
    lastTapTime = now;

    if (tapCount === 3) {
      tapCount = 0;
      // Immediate visual feedback
      lapResetBtn.style.background = '#ff3b30';
      setTimeout(triggerPanic, 100);
      return;
    }

    // Regular Lap/Reset behavior
    if (!isRunning) {
      elapsed = 0;
      formatTime(0);
    }
  });

  // Settings Modal Logic
  settingsBtn.addEventListener('click', () => {
    settingsModal.style.display = 'flex';
  });

  closeSettings.addEventListener('click', () => {
    settingsModal.style.display = 'none';
    tapInput.value = maxTapInterval.toString(); // Revert to saved value
  });

  saveSettings.addEventListener('click', () => {
    const val = parseInt(tapInput.value);
    if (val >= 300 && val <= 5000) {
      maxTapInterval = val;
      localStorage.setItem('max_tap_interval', val.toString());
    }
    settingsModal.style.display = 'none';
  });
}
