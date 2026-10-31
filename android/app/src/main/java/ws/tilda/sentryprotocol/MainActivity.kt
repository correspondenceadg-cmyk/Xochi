package ws.tilda.sentryprotocol

import android.Manifest
import android.content.Context
import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.core.content.ContextCompat
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject

class MainActivity : ComponentActivity() {
    private var tripId by mutableStateOf("")
    private val client = OkHttpClient()
    private val API_BASE = "https://xochi.deplexo.com"

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        requestPermissions()

        setContent {
            MaterialTheme(colorScheme = darkColorScheme()) {
                AppRoot(
                    onStartTrip = { minutes -> startTrip(minutes) },
                    onPanic = { triggerPanic() }
                )
            }
        }
    }

    private fun startTrip(minutes: Int) {
        val prefs = getSharedPreferences("xochi_prefs", Context.MODE_PRIVATE)
        val email = prefs.getString("contact_email", "emergency@example.com") ?: "emergency@example.com"

        CoroutineScope(Dispatchers.IO).launch {
            try {
                val json = """{"window_minutes":$minutes,"contacts":["$email"],"message":"Android Walk"}"""
                val body = json.toRequestBody("application/json".toMediaType())
                val req = Request.Builder().url("$API_BASE/v1/interval").post(body).build()
                val res = client.newCall(req).execute()
                val data = res.body?.string() ?: ""
                val obj = JSONObject(data)
                
                tripId = obj.getString("id")
                val intent = Intent(this@MainActivity, TrackingService::class.java)
                intent.putExtra("TRIP_ID", tripId)
                ContextCompat.startForegroundService(this@MainActivity, intent)
            } catch (e: Exception) { }
        }
    }

    private fun triggerPanic() {
        CoroutineScope(Dispatchers.IO).launch {
            if (tripId.isNotEmpty()) {
                try {
                    val body = "{}".toRequestBody("application/json".toMediaType())
                    val req = Request.Builder().url("$API_BASE/v1/interval/$tripId/distress").post(body).build()
                    client.newCall(req).execute()
                } catch (e: Exception) { }
            }
            runOnUiThread {
                finishAffinity()
                System.exit(0)
            }
        }
    }

    private fun requestPermissions() {
        val launcher = registerForActivityResult(ActivityResultContracts.RequestMultiplePermissions()) {}
        launcher.launch(arrayOf(
            Manifest.permission.ACCESS_FINE_LOCATION,
            Manifest.permission.POST_NOTIFICATIONS
        ))
    }
}

// AppRoot

@Composable
fun AppRoot(onStartTrip: (Int) -> Unit, onPanic: () -> Unit) {
    val context = LocalContext.current
    val prefs = context.getSharedPreferences("xochi_prefs", Context.MODE_PRIVATE)

    var showSplash by remember { mutableStateOf(true) }
    var isCoverMode by remember { mutableStateOf(prefs.getBoolean("stealth_mode", false)) }
    var showSettings by remember { mutableStateOf(false) }

    if (showSplash) {
        SplashScreen {
            delay(2000)
            showSplash = false
        }
    } else if (showSettings) {
        SettingsScreen(
            prefs = prefs,
            onClose = { showSettings = false; isCoverMode = prefs.getBoolean("stealth_mode", false) }
        )
    } else if (isCoverMode) {
        TimerCoverScreen(
            prefs = prefs,
            onTripleTap = { onPanic() },
            onExitCover = { isCoverMode = false }
        )
    } else {
        XochiHomeScreen(
            onStartTrip = { minutes -> onStartTrip(minutes) },
            onOpenSettings = { showSettings = true }
        )
    }
}

// SplashScreen

@Composable
fun SplashScreen(onTimeout: suspend () -> Unit) {
    LaunchedEffect(Unit) { onTimeout() }
    Column(
        modifier = Modifier.fillMaxSize().background(Color(0xFF0F172A)),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center
    ) {
        Image(
            painter = painterResource(id = R.drawable.logo),
            contentDescription = "Logo",
            modifier = Modifier.size(200.dp)
        )
        Spacer(modifier = Modifier.height(24.dp))
        Text(
            "Xochi",
            color = Color.White,
            fontSize = 56.sp,
            fontFamily = FontFamily(androidx.compose.ui.text.font.Font(R.font.el_abogado_loco))
        )
    }
}

// XochiHomeScreen

@Composable
fun XochiHomeScreen(onStartTrip: (Int) -> Unit, onOpenSettings: () -> Unit) {
    var selectedMinutes by remember { mutableStateOf(10) }
    val times = listOf(5, 10, 15, 30)
    val customFont = FontFamily(androidx.compose.ui.text.font.Font(R.font.el_abogado_loco))

    Box(modifier = Modifier.fillMaxSize().background(Color(0xFF0F172A))) {
        IconButton(
            onClick = onOpenSettings,
            modifier = Modifier.align(Alignment.TopEnd).padding(16.dp)
        ) {
            Icon(
                imageVector = Icons.Default.Settings,
                contentDescription = "Settings",
                tint = Color(0xFF94A3B8)
            )
        }

        Column(
            modifier = Modifier.fillMaxSize().padding(32.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            Image(
                painter = painterResource(id = R.drawable.logo),
                contentDescription = "Logo",
                modifier = Modifier.size(160.dp)
            )
            Spacer(modifier = Modifier.height(24.dp))
            Text("Xochi", color = Color.White, fontSize = 48.sp, fontFamily = customFont)
            Spacer(modifier = Modifier.height(16.dp))
            Text(
                "A trip-scoped safety check-in. Set a timer for your walk. If you don't tap \"Safe\" before time runs out, we'll alert your emergency contact.",
                color = Color(0xFF94A3B8),
                fontSize = 16.sp,
                textAlign = TextAlign.Center,
                fontFamily = customFont
            )
            Spacer(modifier = Modifier.height(32.dp))

            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                times.forEach { time ->
                    val isSelected = selectedMinutes == time
                    Button(
                        onClick = { selectedMinutes = time },
                        colors = ButtonDefaults.buttonColors(
                            containerColor = if (isSelected) Color(0xFF38BDF8) else Color(0xFF1E293B),
                            contentColor = if (isSelected) Color.Black else Color.White
                        ),
                        modifier = Modifier.weight(1f),
                        contentPadding = PaddingValues(0.dp)
                    ) {
                        Text("${time}m", maxLines = 1)
                    }
                }
            }

            Spacer(modifier = Modifier.height(32.dp))
            Button(
                onClick = { onStartTrip(selectedMinutes) },
                colors = ButtonDefaults.buttonColors(containerColor = Color(0xFF38BDF8)),
                modifier = Modifier.fillMaxWidth().height(56.dp),
                shape = RoundedCornerShape(8.dp)
            ) {
                Text("Start Walk", fontSize = 18.sp, color = Color.Black, maxLines = 1, softWrap = false)
            }
        }
    }
}

// SettingsScreen

@Composable
fun SettingsScreen(prefs: android.content.SharedPreferences, onClose: () -> Unit) {
    var stealthMode by remember { mutableStateOf(prefs.getBoolean("stealth_mode", false)) }
    var tapWindowStr by remember { mutableStateOf(prefs.getInt("tap_window", 1500).toString()) }
    var contactEmail by remember { mutableStateOf(prefs.getString("contact_email", "emergency@example.com") ?: "") }

    Column(
        modifier = Modifier.fillMaxSize().background(Color(0xFF0F172A)).padding(32.dp),
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Text("Settings", color = Color.White, fontSize = 24.sp)
            TextButton(onClick = onClose) { Text("Done", color = Color(0xFF38BDF8)) }
        }
        Spacer(modifier = Modifier.height(32.dp))

        Text("Emergency Contact Email", color = Color.White, modifier = Modifier.fillMaxWidth())
        Spacer(modifier = Modifier.height(8.dp))
        OutlinedTextField(
            value = contactEmail,
            onValueChange = { 
                contactEmail = it
                prefs.edit().putString("contact_email", it).apply() 
            },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true,
            colors = OutlinedTextFieldDefaults.colors(
                focusedBorderColor = Color(0xFF38BDF8),
                unfocusedBorderColor = Color(0xFF1E293B),
                focusedTextColor = Color.White,
                unfocusedTextColor = Color.White
            )
        )
        
        Spacer(modifier = Modifier.height(24.dp))

        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Text("Enable Stealth Mode (Timer)", color = Color.White)
            Switch(
                checked = stealthMode,
                onCheckedChange = { 
                    stealthMode = it
                    prefs.edit().putBoolean("stealth_mode", it).apply()
                }
            )
        }
        
        Spacer(modifier = Modifier.height(24.dp))
        Text("Max time between taps for panic (ms)", color = Color.White, modifier = Modifier.fillMaxWidth())
        Spacer(modifier = Modifier.height(8.dp))
        OutlinedTextField(
            value = tapWindowStr,
            onValueChange = { 
                tapWindowStr = it
                it.toIntOrNull()?.let { ms -> prefs.edit().putInt("tap_window", ms).apply() }
            },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true,
            colors = OutlinedTextFieldDefaults.colors(
                focusedBorderColor = Color(0xFF38BDF8),
                unfocusedBorderColor = Color(0xFF1E293B),
                focusedTextColor = Color.White,
                unfocusedTextColor = Color.White
            )
        )
    }
}

// TimerCoverScreen

@Composable
fun TimerCoverScreen(
    prefs: android.content.SharedPreferences,
    onTripleTap: () -> Unit,
    onExitCover: () -> Unit
) {
    val tapWindow = prefs.getInt("tap_window", 1500).toLong()

    var isRunning by remember { mutableStateOf(false) }
    var isTimerMode by remember { mutableStateOf(false) }
    var timeMs by remember { mutableStateOf(0L) }
    var targetTimeMs by remember { mutableStateOf(10 * 60 * 1000L) }
    var tapCount by remember { mutableStateOf(0) }
    var lastTapTime by remember { mutableStateOf(0L) }

    LaunchedEffect(isRunning) {
        while (isRunning) {
            delay(10)
            if (isTimerMode) {
                if (timeMs > 0) timeMs -= 10
                else isRunning = false
            } else {
                timeMs += 10
            }
        }
    }

    Column(
        modifier = Modifier.fillMaxSize().background(Color.Black).padding(32.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.SpaceBetween
    ) {
        Box(
            modifier = Modifier.fillMaxWidth().height(60.dp).clickable { onExitCover() },
            contentAlignment = Alignment.Center
        ) {
            Row(horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                TextButton(onClick = { isTimerMode = false; isRunning = false }) {
                    Text("Stopwatch", color = if (!isTimerMode) Color.White else Color.Gray)
                }
                TextButton(onClick = { isTimerMode = true; isRunning = false; timeMs = targetTimeMs }) {
                    Text("Timer", color = if (isTimerMode) Color.White else Color.Gray)
                }
            }
        }

        val displayMs = if (isTimerMode) timeMs else timeMs
        val mins = (displayMs / 60000).toString().padStart(2, '0')
        val secs = ((displayMs / 1000) % 60).toString().padStart(2, '0')
        val ms = ((displayMs % 1000) / 10).toString().padStart(2, '0')

        Row(verticalAlignment = Alignment.Bottom) {
            Text(mins, color = Color.White, fontSize = 80.sp, fontWeight = FontWeight.Light)
            Text(":", color = Color.White, fontSize = 80.sp, fontWeight = FontWeight.Light)
            Text(secs, color = Color.White, fontSize = 80.sp, fontWeight = FontWeight.Light)
            Text(".$ms", color = Color.Gray, fontSize = 30.sp, modifier = Modifier.padding(bottom = 16.dp))
        }

        Row(horizontalArrangement = Arrangement.spacedBy(32.dp)) {
            Button(
                onClick = { 
                    val now = System.currentTimeMillis()
                    if (now - lastTapTime > tapWindow) tapCount = 0
                    tapCount++
                    lastTapTime = now
                    if (tapCount >= 3) {
                        tapCount = 0
                        onTripleTap()
                    }
                },
                shape = CircleShape,
                modifier = Modifier.size(80.dp),
                contentPadding = PaddingValues(0.dp),
                colors = ButtonDefaults.buttonColors(containerColor = Color(0xFF333333))
            ) {
                Text(if (isTimerMode) "Reset" else "Lap", color = Color.White, maxLines = 1, softWrap = false)
            }
            Button(
                onClick = { 
                    if (isTimerMode && timeMs == 0L) timeMs = targetTimeMs
                    isRunning = !isRunning 
                },
                shape = CircleShape,
                modifier = Modifier.size(80.dp),
                contentPadding = PaddingValues(0.dp),
                colors = ButtonDefaults.buttonColors(containerColor = if (isRunning) Color(0xFF5C0A0A) else Color(0xFF0A5C36))
            ) {
                Text(if (isRunning) "Stop" else "Start", color = if (isRunning) Color(0xFFFF3B30) else Color(0xFF4CD964), maxLines = 1, softWrap = false)
            }
        }
    }
}
