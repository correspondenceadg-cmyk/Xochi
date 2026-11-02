package ws.tilda.sentryprotocol

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.Context
import android.content.Intent
import android.location.Location
import android.location.LocationListener
import android.location.LocationManager
import android.os.Build
import android.os.IBinder
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody

class TrackingService : Service(), LocationListener {
    private val client = OkHttpClient.Builder()
        .addInterceptor { chain ->
            val request = chain.request().newBuilder()
                .header("User-Agent", "Mozilla/5.0 (Android) Xochi/1.0")
                .build()
            chain.proceed(request)
        }
        .build()
    private var tripId: String = ""
    private val API_BASE = "https://xochiquetzal.de.deplexo.com"

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        tripId = intent?.getStringExtra("TRIP_ID") ?: return START_NOT_STICKY
        createNotificationChannel()
        
        val notification = Notification.Builder(this, "xochi_channel")
            .setContentTitle("Timer Active")
            .setContentText("Tracking safety window...")
            .setSmallIcon(android.R.drawable.ic_menu_compass)
            .build()
            
        try {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                startForeground(1, notification, android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_LOCATION)
            } else {
                startForeground(1, notification)
            }
        } catch (e: Exception) {
            stopSelf()
            return START_NOT_STICKY
        }
        startLocationUpdates()
        return START_STICKY
    }

    private fun startLocationUpdates() {
        val lm = getSystemService(Context.LOCATION_SERVICE) as LocationManager
        try {
            lm.requestLocationUpdates(LocationManager.GPS_PROVIDER, 60000L, 10f, this)
        } catch (e: SecurityException) { }
    }

    override fun onLocationChanged(location: Location) {
        if (tripId.isEmpty()) return
        val json = """{"lat":${location.latitude},"lon":${location.longitude}}"""
        val body = json.toRequestBody("application/json".toMediaType())
        
        val req = Request.Builder()
            .url("$API_BASE/v1/interval/$tripId/ping")
            .post(body)
            .build()
            
        client.newCall(req).enqueue(object : okhttp3.Callback {
            override fun onFailure(call: okhttp3.Call, e: java.io.IOException) {}
            override fun onResponse(call: okhttp3.Call, response: okhttp3.Response) { response.close() }
        })
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel("xochi_channel", "Xochi Service", NotificationManager.IMPORTANCE_LOW)
            val manager = getSystemService(NotificationManager::class.java)
            manager.createNotificationChannel(channel)
        }
    }

    override fun onBind(intent: Intent?): IBinder? = null
}
