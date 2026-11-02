package ws.tilda.sentryprotocol

import android.app.NotificationManager
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody

class SafeReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val tripId = intent.getStringExtra("TRIP_ID") ?: return
        val pendingResult = goAsync()

        CoroutineScope(Dispatchers.IO).launch {
            try {
                val client = OkHttpClient.Builder()
                    .addInterceptor { chain ->
                        val request = chain.request().newBuilder()
                            .header("User-Agent", "Mozilla/5.0 (Android) Xochi/1.0")
                            .build()
                        chain.proceed(request)
                    }
                    .build()
                val req = Request.Builder()
                    .url("https://xochi.deplexo.com/v1/interval/$tripId/safe")
                    .post("{}".toRequestBody("application/json".toMediaType()))
                    .build()
                client.newCall(req).execute()
                
                val manager = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
                manager.cancel(1)
                
                val prefs = context.getSharedPreferences("xochi_prefs", Context.MODE_PRIVATE)
                prefs.edit().remove("active_trip_id").remove("active_trip_expires_at").remove("active_trip_grace").apply()
            } catch (e: Exception) {
            } finally {
                pendingResult.finish()
            }
        }
    }
}
