package com.xochi.app

import android.Manifest
import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.core.content.ContextCompat
import kotlinx.coroutines.delay

class MainActivity : ComponentActivity() {
    private var isCoverMode by mutableStateOf(true)
    private var tapCount by mutableStateOf(0)
    private var lastTapTime by mutableStateOf(0L)

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        requestPermissions()

        setContent {
            if (isCoverMode) {
                TimerCoverUI(
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
                XochiMainUI(onStartTrip = { id ->
                    val intent = Intent(this, TrackingService::class.java)
                    intent.putExtra("TRIP_ID", id)
                    ContextCompat.startForegroundService(this, intent)
                })
            }
        }
    }

    private fun triggerPanic() {
        // Send DISTRESS
    }

    private fun requestPermissions() {
        val launcher = registerForActivityResult(ActivityResultContracts.RequestMultiplePermissions()) {}
        launcher.launch(arrayOf(
            Manifest.permission.ACCESS_FINE_LOCATION,
            Manifest.permission.POST_NOTIFICATIONS
        ))
    }
}

@Composable
fun TimerCoverUI(onTripleTap: () -> Unit, onExitCover: () -> Unit) {
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
                colors = ButtonDefaults.buttonColors(containerColor = Color(0xFF333333))
            ) {
                Text("Lap", color = Color.White)
            }
            Button(
                onClick = { isRunning = !isRunning },
                shape = CircleShape,
                modifier = Modifier.size(80.dp),
                colors = ButtonDefaults.buttonColors(containerColor = if (isRunning) Color(0xFF5C0A0A) else Color(0xFF0A5C36))
            ) {
                Text(if (isRunning) "Stop" else "Start", color = if (isRunning) Color(0xFFFF3B30) else Color(0xFF4CD964))
            }
        }
    }
}

@Composable
fun XochiMainUI(onStartTrip: (String) -> Unit) {
    Column(
        modifier = Modifier.fillMaxSize().background(Color(0xFF0F172A)).padding(32.dp),
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Text("Xochi", color = Color.White, fontSize = 32.sp)
        Text("Safety Check-in", color = Color.Gray, fontSize = 16.sp)
        Spacer(modifier = Modifier.height(32.dp))
        Button(onClick = { onStartTrip("mock-trip-id") }) {
            Text("Start Walk")
        }
    }
}
