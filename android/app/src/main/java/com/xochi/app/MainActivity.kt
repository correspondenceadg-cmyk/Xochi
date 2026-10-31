package com.xochi.app

import android.Manifest
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
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
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
    private var isCoverMode by mutableStateOf(true)
    private var tapCount by mutableStateOf(0)
    private var lastTapTime by mutableStateOf(0L)
    private var tripId by mutableStateOf("")
    private val client = OkHttpClient()
    private val API_BASE = "https://xochi.deplexo.com"

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        requestPermissions()

        setContent {
            MaterialTheme(colorScheme = darkColorScheme()) {
                if (isCoverMode) {
                    TimerCoverScreen(
                        onTripleTap = {
                            val now = System.currentTimeMillis()
                            if (now - lastTapTime > 1500) tapCount = 0
                            tapCount++
                            lastTapTime = now
                            if (tapCount >= 3) {
                                tapCount = 0
                                triggerPanic()
                            }
                        },
                        onExitCover = { isCoverMode = false }
                    )
                } else {
                    XochiHomeScreen(
                        onStartTrip = { minutes -> startTrip(minutes) }
                    )
                }
            }
        }
    }

    private fun startTrip(minutes: Int) {
        CoroutineScope(Dispatchers.IO).launch {
            try {
                val json = """{"window_minutes":$minutes,"contacts":["emergency@example.com"],"message":"Android Walk"}"""
                val body = json.toRequestBody("application/json".toMediaType())
                val req = Request.Builder().url("$API_BASE/v1/interval").post(body).build()
                val res = client.newCall(req).execute()
                val data = res.body?.string() ?: ""
                val obj = JSONObject(data)
                
                tripId = obj.getString("id")
                val intent = Intent(this@MainActivity, TrackingService::class.java)
                intent.putExtra("TRIP_ID", tripId)
                ContextCompat.startForegroundService(this@MainActivity, intent)
                
                runOnUiThread { isCoverMode = true }
            } catch (e: Exception) { }
        }
    }

    private fun triggerPanic() {
        CoroutineScope(Dispatchers.IO).launch {
            if (tripId.isNotEmpty()) {
                val req = Request.Builder().url("$API_BASE/v1/interval/$tripId/distress").post("".toRequestBody()).build()
                client.newCall(req).execute()
            }
            runOnUiThread { finishAffinity() }
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

// XochiHomeScreen

@Composable
fun XochiHomeScreen(onStartTrip: (Int) -> Unit) {
    var selectedMinutes by remember { mutableStateOf(10) }
    val times = listOf(5, 10, 15, 30)

    Column(
        modifier = Modifier.fillMaxSize().background(Color(0xFF0F172A)).padding(32.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center
    ) {
        Image(
            painter = painterResource(id = R.drawable.logo),
            contentDescription = "Logo",
            modifier = Modifier.size(160.dp)
        )
        Spacer(modifier = Modifier.height(24.dp))
        Text(
            "Xochi",
            color = Color.White,
            fontSize = 48.sp,
            fontFamily = FontFamily.Serif // FontFamily(Font(R.font.el_abogado_loco))
        )
        Spacer(modifier = Modifier.height(16.dp))
        Text(
            "A trip-scoped safety check-in. Set a timer for your walk. If you don't tap \"Safe\" before time runs out, we'll alert your emergency contact.",
            color = Color(0xFF94A3B8),
            fontSize = 16.sp,
            textAlign = TextAlign.Center,
            fontFamily = FontFamily.Serif // FontFamily(Font(R.font.el_abogado_loco))
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
            Text("Start Walk", fontSize = 18.sp, color = Color.Black, maxLines = 1)
        }
    }
}

// TimerCoverScreen

@Composable
fun TimerCoverScreen(onTripleTap: () -> Unit, onExitCover: () -> Unit) {
    var isRunning by remember { mutableStateOf(false) }
    var timeMs by remember { mutableStateOf(0L) }

    LaunchedEffect(isRunning) {
        while (isRunning) {
            delay(10)
            timeMs += 10
        }
    }

    Column(
        modifier = Modifier.fillMaxSize().background(Color.Black).padding(32.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.SpaceBetween
    ) {
        Box(modifier = Modifier.fillMaxWidth().height(40.dp).clickable { onExitCover() })

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
                onClick = { onTripleTap() },
                shape = CircleShape,
                modifier = Modifier.size(80.dp),
                contentPadding = PaddingValues(0.dp),
                colors = ButtonDefaults.buttonColors(containerColor = Color(0xFF333333))
            ) {
                Text("Lap", color = Color.White, maxLines = 1, softWrap = false)
            }
            Button(
                onClick = { isRunning = !isRunning },
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
