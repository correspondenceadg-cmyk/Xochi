# Threat Model

**Protected:**
- Data in transit (HTTPS enforced).
- Location data deleted 24h after closure.
- Third-party tracking (no analytics, no external fonts).
- **Cover Mode:** Disguises the app as a timer. If triggered, it fires a silent distress signal and blanks the screen. No network requests leak the user's intent visually.

**NOT Protected:**
- An attacker with access to your unlocked phone can press "Safe".
- Network logs on the same Wi-Fi (use a VPN if concerned).
- Notification history on the device.
- **Cover Mode:** A determined attacker who watches your screen can see you tap the Lap button, or see the app's package name in Android App Info.
- The app does not replace emergency services (911).
