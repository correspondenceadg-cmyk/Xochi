package ws.tilda.sentryprotocol

import android.Manifest
import android.content.Context
import android.content.Intent
import android.os.Build
import android.os.Bundle
import android.os.Vibrator
import android.os.VibratorManager
import android.widget.Toast
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.combinedClickable
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
    private var expiresAt by mutableStateOf(0L)
    private val client = OkHttpClient()
    private val API_BASE = "https://xochi.deplexo.com"

    private val permissionLauncher = registerForActivityResult(ActivityResultContracts.RequestMultiplePermissions()) { grants ->
        val fine = grants[Manifest.permission.ACCESS_FINE_LOCATION] == true
        if (fine && Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            bgPermissionLauncher.launch(Manifest.permission.ACCESS_BACKGROUND_LOCATION)
        }
    }

    private val bgPermissionLauncher = registerForActivityResult(ActivityResultContracts.RequestPermission()) { }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val prefs = getSharedPreferences("xochi_prefs", Context.MODE_PRIVATE)
        val savedId = prefs.getString("active_trip_id", "") ?: ""
        val savedExp = prefs.getLong("active_trip_expires_at", 0L)
        val now = System.currentTimeMillis() / 1000
        if (savedId.isNotEmpty() && now < savedExp + 300) {
            tripId = savedId
            expiresAt = savedExp
        } else if (savedId.isNotEmpty()) {
            prefs.edit().remove("active_trip_id").remove("active_trip_expires_at").apply()
        }

        permissionLauncher.launch(arrayOf(
            Manifest.permission.ACCESS_FINE_LOCATION,
            Manifest.permission.POST_NOTIFICATIONS
        ))

        setContent {
            MaterialTheme(colorScheme = darkColorScheme()) {
                AppRoot(
                    tripId = tripId,
                    expiresAt = expiresAt,
                    onStartTrip = { minutes -> startTrip(minutes) },
                    onSafe = { closeTrip("safe") },
                    onExtend = { extendTrip(10) },
                    onDistress = { triggerDistress() },
                    onPanic = { triggerPanic() }
                )
            }
        }
    }

    private fun vibrate() {
        val vibrator = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            val vibratorManager = getSystemService(Context.VIBRATOR_MANAGER_SERVICE) as VibratorManager
            vibratorManager.defaultVibrator
        } else {
            @Suppress("DEPRECATION")
            getSystemService(Context.VIBRATOR_SERVICE) as Vibrator
        }
        vibrator.vibrate(android.os.VibrationEffect.createOneShot(100, android.os.VibrationEffect.DEFAULT_AMPLITUDE))
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
                val obj = JSONObject(res.body?.string() ?: "")
                val newId = obj.getString("id")
                val newExp = obj.getLong("expires_at")

                prefs.edit().putString("active_trip_id", newId).putLong("active_trip_expires_at", newExp).apply()

                val intent = Intent(this@MainActivity, TrackingService::class.java)
                intent.putExtra("TRIP_ID", newId)
                ContextCompat.startForegroundService(this@MainActivity, intent)

                runOnUiThread {
                    tripId = newId
                    expiresAt = newExp
                    vibrate()
                }
            } catch (e: Exception) { }
        }
    }

    private fun extendTrip(minutes: Int) {
        val prefs = getSharedPreferences("xochi_prefs", Context.MODE_PRIVATE)
        CoroutineScope(Dispatchers.IO).launch {
            try {
                val json = """{"minutes":$minutes}"""
                val body = json.toRequestBody("application/json".toMediaType())
                val req = Request.Builder().url("$API_BASE/v1/interval/$tripId/extend").post(body).build()
                val res = client.newCall(req).execute()
                val obj = JSONObject(res.body?.string() ?: "")
                val newExp = obj.getLong("expires_at")
                prefs.edit().putLong("active_trip_expires_at", newExp).apply()
                runOnUiThread { expiresAt = newExp; vibrate() }
            } catch (e: Exception) { }
        }
    }

    private fun closeTrip(action: String) {
        val prefs = getSharedPreferences("xochi_prefs", Context.MODE_PRIVATE)
        CoroutineScope(Dispatchers.IO).launch {
            try {
                val req = Request.Builder()
                    .url("$API_BASE/v1/interval/$tripId/$action")
                    .post("{}".toRequestBody("application/json".toMediaType()))
                    .build()
                client.newCall(req).execute()
            } catch (e: Exception) { }
            prefs.edit().remove("active_trip_id").remove("active_trip_expires_at").apply()
            runOnUiThread {
                tripId = ""
                expiresAt = 0L
            }
        }
    }

    private fun triggerDistress() {
        val prefs = getSharedPreferences("xochi_prefs", Context.MODE_PRIVATE)
        val currentId = tripId
        CoroutineScope(Dispatchers.IO).launch {
            if (currentId.isNotEmpty()) {
                try {
                    val req = Request.Builder()
                        .url("$API_BASE/v1/interval/$currentId/distress")
                        .post("{}".toRequestBody("application/json".toMediaType()))
                        .build()
                    client.newCall(req).execute()
                } catch (e: Exception) { }
            }
            prefs.edit().remove("active_trip_id").remove("active_trip_expires_at").apply()
            runOnUiThread {
                tripId = ""
                expiresAt = 0L
            }
        }
    }

    private fun triggerPanic() {
        val prefs = getSharedPreferences("xochi_prefs", Context.MODE_PRIVATE)
        val currentId = tripId
        val email = prefs.getString("contact_email", "emergency@example.com") ?: "emergency@example.com"

        CoroutineScope(Dispatchers.IO).launch {
            try {
                val targetId = if (currentId.isNotEmpty()) {
                    currentId
                } else {
                    val json = """{"window_minutes":1,"contacts":["$email"],"message":"Panic trigger"}"""
                    val body = json.toRequestBody("application/json".toMediaType())
                    val req = Request.Builder().url("$API_BASE/v1/interval").post(body).build()
                    val res = client.newCall(req).execute()
                    JSONObject(res.body?.string() ?: "").getString("id")
                }
                val distressReq = Request.Builder()
                    .url("$API_BASE/v1/interval/$targetId/distress")
                    .post("{}".toRequestBody("application/json".toMediaType()))
                    .build()
                client.newCall(distressReq).execute()
            } catch (e: Exception) { }
            prefs.edit().remove("active_trip_id").remove("active_trip_expires_at").apply()
            runOnUiThread {
                finishAffinity()
                System.exit(0)
            }
        }
    }
}

// AppRoot

@Composable
fun AppRoot(
    tripId: String,
    expiresAt: Long,
    onStartTrip: (Int) -> Unit,
    onSafe: () -> Unit,
    onExtend: () -> Unit,
    onDistress: () -> Unit,
    onPanic: () -> Unit
) {
    val prefs = LocalContext.current.getSharedPreferences("xochi_prefs", Context.MODE_PRIVATE)
    var showSplash by remember { mutableStateOf(true) }
    var isCoverMode by remember { mutableStateOf(false) }
    var showSettings by remember { mutableStateOf(false) }

    LaunchedEffect(Unit) {
        delay(2000)
        showSplash = false
    }

    when {
        showSplash -> SplashScreen()
        showSettings -> SettingsScreen(prefs, onClose = { showSettings = false })
        isCoverMode -> TimerCoverScreen(prefs, onTripleTap = { onPanic() }, onExitCover = { isCoverMode = false })
        tripId.isNotEmpty() -> ActiveTripScreen(expiresAt, onSafe, onExtend, onDistress)
        else -> HomeScreen(
            onStartTrip = onStartTrip,
            onOpenSettings = { showSettings = true },
            onOpenCover = { isCoverMode = true }
        )
    }
}

// SplashScreen

@Composable
fun SplashScreen() {
val customFont = try {
    customFont
} catch (e: Exception) {
    FontFamily.Serif
}

    Column(
        modifier = Modifier.fillMaxSize().background(Color(0xFF0F172A)),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center
    ) {
        Image(painter = painterResource(id = R.drawable.logo), contentDescription = "Logo", modifier = Modifier.size(250.dp))
        Spacer(modifier = Modifier.height(24.dp))
        Text("Xochi", color = Color.White, fontSize = 56.sp, fontFamily = customFont)
    }
}

// HomeScreen

@Composable
fun HomeScreen(onStartTrip: (Int) -> Unit, onOpenSettings: () -> Unit, onOpenCover: () -> Unit) {
val customFont = try {
    customFont
} catch (e: Exception) {
    FontFamily.Serif
}

    var selectedMinutes by remember { mutableStateOf(10) }
    val times = listOf(5, 10, 15, 30)

    Box(modifier = Modifier.fillMaxSize().background(Color(0xFF0F172A))) {
        IconButton(onClick = onOpenSettings, modifier = Modifier.align(Alignment.TopEnd).padding(16.dp)) {
            Icon(Icons.Default.Settings, contentDescription = "Settings", tint = Color(0xFF94A3B8))
        }

        Column(
            modifier = Modifier.fillMaxSize().padding(32.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            Image(painter = painterResource(id = R.drawable.logo), contentDescription = "Logo", modifier = Modifier.size(200.dp))
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

            Spacer(modifier = Modifier.height(16.dp))
            TextButton(onClick = onOpenCover) {
                Text("Enter Cover Mode", color = Color(0xFF94A3B8))
            }
        }
    }
}

// ActiveTripScreen

@Composable
fun ActiveTripScreen(expiresAt: Long, onSafe: () -> Unit, onExtend: () -> Unit, onDistress: () -> Unit) {
    var now by remember { mutableStateOf(System.currentTimeMillis() / 1000) }
    LaunchedEffect(Unit) {
        while (true) {
            now = System.currentTimeMillis() / 1000
            delay(1000)
        }
    }

    val remaining = expiresAt - now
    val inGrace = remaining in -300L until 0L
    val alerted = remaining <= -300L

    val displayMins = (Math.max(0L, remaining) / 60).toInt()
    val displaySecs = (Math.max(0L, remaining) % 60).toInt()

    Column(
        modifier = Modifier.fillMaxSize().background(Color(0xFF0F172A)).padding(32.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center
    ) {
        when {
            alerted -> {
                Text("ALERT SENT", color = Color(0xFFEF4444), fontSize = 28.sp, fontWeight = FontWeight.Bold)
                Spacer(modifier = Modifier.height(16.dp))
                Text("Your contacts have been notified with your last location.", color = Color(0xFF94A3B8), textAlign = TextAlign.Center)
                Spacer(modifier = Modifier.height(32.dp))
                Button(onClick = onSafe, modifier = Modifier.fillMaxWidth().height(56.dp), shape = RoundedCornerShape(8.dp), colors = ButtonDefaults.buttonColors(containerColor = Color(0xFF1E293B))) {
                    Text("Return Home", color = Color.White, fontSize = 18.sp)
                }
            }
            else -> {
                Surface(
                    color = if (inGrace) Color(0x33EF4444) else Color(0x3338BDF8),
                    shape = RoundedCornerShape(50)
                ) {
                    Text(
                        if (inGrace) "GRACE PERIOD" else "TIMER ACTIVE",
                        color = if (inGrace) Color(0xFFEF4444) else Color(0xFF38BDF8),
                        fontWeight = FontWeight.Bold,
                        modifier = Modifier.padding(horizontal = 16.dp, vertical = 6.dp),
                        fontSize = 12.sp
                    )
                }
                Spacer(modifier = Modifier.height(24.dp))
                Text(
                    "${displayMins.toString().padStart(2, '0')}:${displaySecs.toString().padStart(2, '0')}",
                    color = if (inGrace) Color(0xFFEF4444) else Color.White,
                    fontSize = 80.sp,
                    fontWeight = FontWeight.Light,
                    fontFamily = FontFamily.Monospace
                )
                Spacer(modifier = Modifier.height(32.dp))
                Button(onClick = onSafe, modifier = Modifier.fillMaxWidth().height(64.dp), shape = RoundedCornerShape(8.dp), colors = ButtonDefaults.buttonColors(containerColor = Color(0xFF10B981))) {
                    Text("I'm Safe", color = Color.White, fontSize = 20.sp, fontWeight = FontWeight.Bold)
                }
                Spacer(modifier = Modifier.height(12.dp))
                Button(onClick = onExtend, modifier = Modifier.fillMaxWidth().height(56.dp), shape = RoundedCornerShape(8.dp), colors = ButtonDefaults.buttonColors(containerColor = Color(0xFF1E293B))) {
                    Text("Extend +10m", color = Color.White, fontSize = 16.sp)
                }
                Spacer(modifier = Modifier.height(12.dp))
                Button(onClick = onDistress, modifier = Modifier.fillMaxWidth().height(56.dp), shape = RoundedCornerShape(8.dp), colors = ButtonDefaults.buttonColors(containerColor = Color(0xFF5C0A0A))) {
                    Text("DISTRESS", color = Color(0xFFEF4444), fontSize = 16.sp, fontWeight = FontWeight.Bold)
                }
            }
        }
    }
}

// SettingsScreen

@Composable
fun SettingsScreen(prefs: android.content.SharedPreferences, onClose: () -> Unit) {
    val context = LocalContext.current
    var tapWindowStr by remember { mutableStateOf(prefs.getInt("tap_window", 1500).toString()) }
    var contactEmail by remember { mutableStateOf(prefs.getString("contact_email", "emergency@example.com") ?: "") }

    Column(
        modifier = Modifier.fillMaxSize().background(Color(0xFF0F172A)).padding(32.dp),
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween, verticalAlignment = Alignment.CenterVertically) {
            Text("Settings", color = Color.White, fontSize = 24.sp)
            TextButton(onClick = onClose) { Text("Done", color = Color(0xFF38BDF8)) }
        }
        Spacer(modifier = Modifier.height(32.dp))

        Text("Emergency Contact Email", color = Color.White, modifier = Modifier.fillMaxWidth())
        Spacer(modifier = Modifier.height(8.dp))
        OutlinedTextField(
            value = contactEmail,
            onValueChange = { contactEmail = it; prefs.edit().putString("contact_email", it).apply() },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true,
            colors = OutlinedTextFieldDefaults.colors(focusedBorderColor = Color(0xFF38BDF8), unfocusedBorderColor = Color(0xFF1E293B), focusedTextColor = Color.White, unfocusedTextColor = Color.White)
        )

        Spacer(modifier = Modifier.height(12.dp))
        OutlinedButton(
            onClick = {
                CoroutineScope(Dispatchers.IO).launch {
                    try {
                        val json = """{"email":"$contactEmail"}"""
                        val body = json.toRequestBody("application/json".toMediaType())
                        val req = Request.Builder().url("https://xochi.deplexo.com/v1/test-alert").post(body).build()
                        OkHttpClient().newCall(req).execute()
                        CoroutineScope(Dispatchers.Main).launch {
                            Toast.makeText(context, "Test alert sent to $contactEmail", Toast.LENGTH_LONG).show()
                        }
                    } catch (e: Exception) {
                        CoroutineScope(Dispatchers.Main).launch {
                            Toast.makeText(context, "Failed to send", Toast.LENGTH_SHORT).show()
                        }
                    }
                }
            },
            modifier = Modifier.fillMaxWidth()
        ) { Text("Send Test Alert", color = Color(0xFF38BDF8)) }

        Spacer(modifier = Modifier.height(24.dp))
        Text("Max time between taps for panic (ms)", color = Color.White, modifier = Modifier.fillMaxWidth())
        Spacer(modifier = Modifier.height(8.dp))
        OutlinedTextField(
            value = tapWindowStr,
            onValueChange = { tapWindowStr = it; it.toIntOrNull()?.let { ms -> prefs.edit().putInt("tap_window", ms).apply() } },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true,
            colors = OutlinedTextFieldDefaults.colors(focusedBorderColor = Color(0xFF38BDF8), unfocusedBorderColor = Color(0xFF1E293B), focusedTextColor = Color.White, unfocusedTextColor = Color.White)
        )
    }
}

// TimerCoverScreen

@OptIn(ExperimentalFoundationApi::class)
@Composable
fun TimerCoverScreen(prefs: android.content.SharedPreferences, onTripleTap: () -> Unit, onExitCover: () -> Unit) {
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
                if (timeMs > 0) timeMs -= 10 else isRunning = false
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
            modifier = Modifier.fillMaxWidth().height(60.dp).combinedClickable(onClick = {}, onLongClick = { onExitCover() }),
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

        val mins = (timeMs / 60000).toString().padStart(2, '0')
        val secs = ((timeMs / 1000) % 60).toString().padStart(2, '0')
        val ms = ((timeMs % 1000) / 10).toString().padStart(2, '0')

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
