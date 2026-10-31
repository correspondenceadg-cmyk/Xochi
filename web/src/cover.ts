const API_BASE = 'https://xochi.deplexo.com';

async function triggerPanic() {
  // 1. Instantly replace the UI with a black screen to look like a crash
  document.body.innerHTML = '<div style="background:#000;width:100vw;height:100vh;position:fixed;top:0;left:0;z-index:9999;"></div>';
  
  try {
    let id = localStorage.getItem('trip_id');
    let contacts = JSON.parse(localStorage.getItem('contacts') || '["emergency@example.com"]');

    // 2. If no active trip exists, create a dummy one and immediately distress it
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
  
  // Standard iOS-style dark timer colors
  document.body.style.background = '#000';
  document.body.style.color = '#fff';
  document.body.style.fontFamily = '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif';
  document.body.style.margin = '0';
  document.body.style.overflow = 'hidden';

  const app = document.getElementById('app')!;
  
  let startTime = 0;
  let elapsed = 0;
  let isRunning = false;
  let timerInterval: number | undefined;

  app.innerHTML = `
    <div style="display: flex; flex-direction: column; align-items: center; justify-content: space-between; height: 100vh; padding: 2rem; box-sizing: border-box;">
      
      <!-- Top spacer -->
      <div style="height: 60px;"></div>

      <!-- Timer Display -->
      <div style="display: flex; flex-direction: column; align-items: center;">
        <div style="font-size: clamp(4rem, 20vw, 6rem); font-weight: 300; font-variant-numeric: tabular-nums; letter-spacing: -0.05em; display: flex;">
          <span id="mins">00</span>
          <span id="colon" style="cursor: pointer; padding: 0 5px; user-select: none; -webkit-user-select: none;">:</span>
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
  `;

  const minsEl = document.getElementById('mins')!;
  const secsEl = document.getElementById('secs')!;
  const msEl = document.getElementById('ms')!;
  const startStopBtn = document.getElementById('start-stop') as HTMLButtonElement;
  const lapResetBtn = document.getElementById('lap-reset') as HTMLButtonElement;
  const colonEl = document.getElementById('colon')!;

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
      // Stop
      isRunning = false;
      clearInterval(timerInterval);
      startStopBtn.style.background = '#0a5c36';
      startStopBtn.style.color = '#4cd964';
      startStopBtn.innerText = 'Start';
      lapResetBtn.innerText = 'Reset';
    } else {
      // Start
      isRunning = true;
      startTime = Date.now() - elapsed;
      timerInterval = window.setInterval(tick, 10);
      startStopBtn.style.background = '#5c0a0a';
      startStopBtn.style.color = '#ff3b30';
      startStopBtn.innerText = 'Stop';
      lapResetBtn.innerText = 'Lap';
    }
  });

  lapResetBtn.addEventListener('click', () => {
    if (isRunning) {
      // Lap logic would go here (can ignore for cover)
    } else {
      elapsed = 0;
      formatTime(0);
    }
  });

  // --- PANIC TRIGGER ---
  // Triple tap the colon
  let tapCount = 0;
  let tapTimer: number;
  
  colonEl.addEventListener('click', (e) => {
    e.stopPropagation();
    tapCount++;
    clearTimeout(tapTimer);
    
    if (tapCount === 3) {
      tapCount = 0;
      // Immediate visual feedback before the crash
      colonEl.style.color = '#ff3b30';
      setTimeout(triggerPanic, 100);
    } else {
      tapTimer = window.setTimeout(() => { tapCount = 0; }, 600);
    }
  });
  
  // Prevent accidental text selection on rapid tapping
  colonEl.addEventListener('mousedown', (e) => e.preventDefault());
  colonEl.addEventListener('touchstart', (e) => e.preventDefault(), { passive: false });
}
