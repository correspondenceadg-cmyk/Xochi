import { renderCover } from './cover';

const API_BASE = 'https://xochiquetzal.de.deplexo.com';

let state = { id: '', expiresAt: 0, graceSecs: 0, timerInterval: 0, notified: false };

const app = document.getElementById('app')!;

const LOGO_SVG = `<img src="/logo.svg" class="logo" alt="Xochi Logo" />`;
const LOADING_SVG = `<img src="/logo.svg" class="logo loading" alt="Loading" />`;

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
    
    state.id = savedId;
    state.expiresAt = data.expires_at;
    state.graceSecs = parseInt(localStorage.getItem('grace_secs') || '0');
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
        <label>Emergency Contacts (comma separated)</label>
        <input id="email" type="text" placeholder="contact@example.com, +15550100" />
      </div>

      <div class="input-group">
        <label>Walk Duration (Minutes)</label>
        <input id="window" type="number" value="10" min="1" placeholder="e.g. 10" />
      </div>

      <div class="input-group">
        <label>Grace Period (Seconds)</label>
        <input id="grace" type="number" value="0" min="0" placeholder="e.g. 300" />
      </div>

      <button id="start" class="btn btn-primary">Start Walk</button>
      <button id="cover-btn" class="btn btn-secondary" style="margin-top: 1rem; font-size: 0.85rem;">Enter Cover Mode</button>
    </div>
  `;

  document.getElementById('start')!.onclick = async () => {
    const btn = document.getElementById('start') as HTMLButtonElement;
    const emailInput = (document.getElementById('email') as HTMLInputElement).value;
    const contacts = emailInput.split(',').map(c => c.trim()).filter(c => c.length > 0);
    const windowMins = parseInt((document.getElementById('window') as HTMLInputElement).value);
    const grace = parseInt((document.getElementById('grace') as HTMLInputElement).value);
    
    if (contacts.length === 0 || !windowMins) return alert("Please fill in required fields");
    if (contacts.length > 5) return alert("Maximum 5 contacts allowed");

    btn.disabled = true;
    btn.innerHTML = LOADING_SVG;

    try {
      const res = await fetch(`${API_BASE}/v1/interval`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ window_minutes: windowMins, grace_secs: grace, contacts: contacts, message: "Walking" })
      });
      const data = await res.json();
      if (data.id) {
        state.id = data.id;
        state.expiresAt = data.expires_at;
        state.graceSecs = data.grace_secs;
        localStorage.setItem('trip_id', state.id);
        localStorage.setItem('expires_at', state.expiresAt.toString());
        localStorage.setItem('grace_secs', state.graceSecs.toString());
        renderActive();
      }
    } catch (e) { 
      alert('Failed to connect to server'); 
      btn.disabled = false;
      btn.innerHTML = "Start Walk";
    }
  };

  document.getElementById('cover-btn')!.onclick = () => {
    renderCover();
  };
}

function renderActive() {
  if ("Notification" in window && Notification.permission === "default") {
    Notification.requestPermission();
  }

  app.innerHTML = `
    <div class="container">
      <div id="status-badge" class="status-badge">Timer Active</div>
      <h1 id="header-text">Stay Safe</h1>
      <p id="sub-text" class="subtitle">Tap "I'm Safe" as soon as you arrive.</p>
      
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
  const badge = document.getElementById('status-badge')!;
  const header = document.getElementById('header-text')!;
  const subtext = document.getElementById('sub-text')!;

  const updateTimer = () => {
    const remaining = state.expiresAt - Math.floor(Date.now() / 1000);
    const inGrace = remaining <= 0;
    const alerted = remaining <= -state.graceSecs;

    if (alerted) {
      document.body.classList.remove('alarm');
      badge.className = 'status-badge danger';
      badge.innerText = 'Alert Sent';
      header.innerText = 'Contacts Notified';
      subtext.innerText = 'Your emergency contacts have been sent your last location.';
      timerEl.innerText = "00:00";
      timerEl.classList.remove('warning');
      (document.getElementById('extend') as HTMLButtonElement).style.display = 'none';
      (document.getElementById('distress') as HTMLButtonElement).style.display = 'none';
      (document.getElementById('safe') as HTMLButtonElement).innerText = 'Return Home';
      (document.getElementById('safe') as HTMLButtonElement).onclick = () => {
        localStorage.clear();
        window.location.reload();
      };
    } else if (inGrace) {
      document.body.classList.add('alarm');
      badge.className = 'status-badge danger';
      badge.innerText = 'Time Lapsed';
      header.innerText = 'Grace Period';
      subtext.innerText = 'Tap "I\'m Safe" immediately or a distress signal will be sent.';
      
      const graceRemaining = state.graceSecs + remaining;
      const mins = Math.floor(Math.max(0, graceRemaining) / 60);
      const secs = Math.max(0, graceRemaining) % 60;
      timerEl.innerText = `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
      timerEl.classList.add('warning');
      (document.getElementById('extend') as HTMLButtonElement).style.display = 'none';

      if (!state.notified && "Notification" in window && Notification.permission === "granted") {
        state.notified = true;
        new Notification("Xochi Alert", { body: "Time lapsed! Tap I'm Safe immediately." });
      }
    } else {
      document.body.classList.remove('alarm');
      const mins = Math.floor(Math.max(0, remaining) / 60);
      const secs = Math.max(0, remaining) % 60;
      timerEl.innerText = `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
      timerEl.classList.remove('warning');
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
  document.body.classList.remove('alarm');
  await fetch(`${API_BASE}/v1/interval/${state.id}/${action}`, { method: 'POST' });
  localStorage.clear();
  
  app.innerHTML = `
    <div class="container">
      ${LOGO_SVG}
      <h1>${action === 'safe' ? 'Safe & Sound' : 'Help is on the way'}</h1>
      <p class="subtitle">
        ${action === 'safe' 
          ? 'Your trip was marked safe. No contacts were alerted.' 
          : 'Your distress signal was sent to your contacts immediately.'}
      </p>
      <button class="btn btn-secondary" onclick="location.reload()">Return to Home</button>
    </div>
  `;
}

// Init

const savedId = localStorage.getItem('trip_id');
const savedExpires = localStorage.getItem('expires_at');
if (savedId && savedExpires) {
  state.id = savedId;
  state.expiresAt = parseInt(savedExpires);
  state.graceSecs = parseInt(localStorage.getItem('grace_secs') || '0');
  renderActive();
} else {
  renderBind();
}
