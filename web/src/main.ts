const API_BASE = 'https://xochi.deplexo.com';

let state = { id: '', expiresAt: 0, timerInterval: 0 };

const app = document.getElementById('app')!;

const LOGO_SVG = `<img src="/logo.svg" class="logo" alt="Xochi Logo" />`;

// Pull to Refresh Setup

const indicator = document.createElement('div');
indicator.className = 'ptr-indicator';
document.body.appendChild(indicator);

let touchStartY = 0;
let isPulling = false;
let pullDist = 0;

document.body.addEventListener('touchstart', (e) => {
  if (window.scrollY === 0) {
    touchStartY = e.touches[0].clientY;
    isPulling = true;
  }
}, { passive: true });

document.body.addEventListener('touchmove', (e) => {
  if (!isPulling) return;
  pullDist = e.touches[0].clientY - touchStartY;
  
  if (pullDist > 0) {
    indicator.classList.add('pulling');
    const pullPercent = Math.min(pullDist / 100, 1);
    indicator.style.transform = `translateX(-50%) rotate(${pullDist * 2}deg)`;
    indicator.style.top = `${-60 + (pullDist * 1.5)}px`;
  }
}, { passive: true });

document.body.addEventListener('touchend', () => {
  if (!isPulling) return;
  isPulling = false;
  
  if (pullDist > 60) {
    indicator.className = 'ptr-indicator refreshing';
    indicator.style.top = '20px';
    indicator.style.transform = 'translateX(-50%)';
    refreshState();
  } else {
    indicator.className = 'ptr-indicator';
    indicator.style.top = '-60px';
  }
  pullDist = 0;
});

async function refreshState() {
  const savedId = localStorage.getItem('trip_id');
  if (!savedId) {
    window.location.reload();
    return;
  }
  
  try {
    const res = await fetch(`${API_BASE}/v1/interval/${savedId}/status`);
    if (res.status === 404) {
      localStorage.clear();
      window.location.reload();
      return;
    }
    
    const data = await res.json();
    if (data.state === 'SAFE' || data.state === 'DISTRESS' || data.state === 'UNKNOWN') {
      localStorage.clear();
      window.location.reload();
      return;
    }
    
    // Active or Grace: sync the timer
    state.id = savedId;
    state.expiresAt = data.expires_at;
    localStorage.setItem('expires_at', state.expiresAt.toString());
    
    if (!document.getElementById('timer-display')) {
      renderActive();
    }
  } catch (e) {
    console.error('Refresh failed', e);
  } finally {
    setTimeout(() => {
      indicator.className = 'ptr-indicator';
      indicator.style.top = '-60px';
    }, 500);
  }
}

// Rendering

function renderBind() {
  app.innerHTML = `
    <div class="container">
      ${LOGO_SVG}
      <h1>Xochi</h1>
      <p class="subtitle">
        A trip-scoped safety check-in. Set a timer for your walk. 
        If you don't tap "Safe" before time runs out, we'll alert your emergency contact.
      </p>

      <div class="input-group">
        <label>Emergency Contact Email</label>
        <input id="email" type="email" placeholder="contact@example.com" />
      </div>

      <div class="input-group">
        <label>Walk Duration (Minutes)</label>
        <input id="window" type="number" value="10" min="1" placeholder="e.g. 10" />
      </div>

      <button id="start" class="btn btn-primary">Start Walk</button>
    </div>
  `;

  document.getElementById('start')!.onclick = async () => {
    const email = (document.getElementById('email') as HTMLInputElement).value;
    const windowMins = parseInt((document.getElementById('window') as HTMLInputElement).value);
    
    if (!email || !windowMins) return alert("Please fill in both fields");

    try {
      const res = await fetch(`${API_BASE}/v1/interval`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ window_minutes: windowMins, contacts: [email], message: "Walking" })
      });
      const data = await res.json();
      if (data.id) {
        state.id = data.id;
        state.expiresAt = data.expires_at;
        localStorage.setItem('trip_id', state.id);
        localStorage.setItem('expires_at', state.expiresAt.toString());
        renderActive();
      }
    } catch (e) { alert('Failed to connect to server'); }
  };
}

function renderActive() {
  app.innerHTML = `
    <div class="container">
      <div class="status-badge">Timer Active</div>
      <h1>Stay Safe</h1>
      <p class="subtitle">Tap "I'm Safe" as soon as you arrive. If the timer hits zero, a 5-minute grace period starts.</p>
      
      <div id="timer-display" class="timer"></div>
      
      <button id="safe" class="btn btn-safe">I'm Safe</button>
      <button id="extend" class="btn btn-secondary">Extend +10m</button>
      <button id="distress" class="btn btn-danger">DISTRESS</button>
    </div>
  `;

  document.getElementById('safe')!.onclick = () => closeTrip('safe');
  document.getElementById('distress')!.onclick = () => closeTrip('distress');
  document.getElementById('extend')!.onclick = async () => {
    const res = await fetch(`${API_BASE}/v1/interval/${state.id}/extend`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ minutes: 10 })
    });
    const data = await res.json();
    if (data.expires_at) {
      state.expiresAt = data.expires_at;
      localStorage.setItem('expires_at', state.expiresAt.toString());
    }
  };

  const timerEl = document.getElementById('timer-display')!;
  const updateTimer = () => {
    const remaining = state.expiresAt - Math.floor(Date.now() / 1000);
    const mins = Math.floor(Math.max(0, remaining) / 60);
    const secs = Math.max(0, remaining) % 60;
    timerEl.innerText = `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
    
    if (remaining <= 0) {
      timerEl.classList.add('warning');
      timerEl.innerText = "GRACE";
    }
  };

  updateTimer();
  if (state.timerInterval) clearInterval(state.timerInterval);
  state.timerInterval = window.setInterval(updateTimer, 1000);

  if (navigator.geolocation) {
    navigator.geolocation.watchPosition(async (pos) => {
      await fetch(`${API_BASE}/v1/interval/${state.id}/ping`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ lat: pos.coords.latitude, lon: pos.coords.longitude })
      });
    }, (err) => console.log('Geo error:', err), { enableHighAccuracy: false, maximumAge: 30000 });
  }
}

async function closeTrip(action: 'safe' | 'distress') {
  clearInterval(state.timerInterval);
  await fetch(`${API_BASE}/v1/interval/${state.id}/${action}`, { method: 'POST' });
  localStorage.removeItem('trip_id');
  localStorage.removeItem('expires_at');
  
  app.innerHTML = `
    <div class="container">
      ${LOGO_SVG}
      <h1>${action === 'safe' ? 'Safe & Sound' : 'Help is on the way'}</h1>
      <p class="subtitle">
        ${action === 'safe' 
          ? 'Your trip was marked safe. No contacts were alerted.' 
          : 'Your distress signal was sent to your contacts immediately.'}
      </p>
      <button class="btn btn-secondary" onclick="localStorage.clear(); location.reload()">Return to Home</button>
    </div>
  `;
}

const savedId = localStorage.getItem('trip_id');
if (savedId) {
  refreshState();
} else {
  renderBind();
}
