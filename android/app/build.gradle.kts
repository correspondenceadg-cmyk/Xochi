import java.util.Properties
import java.io.FileInputStream

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

val keystoreProperties = Properties()
val keystorePropertiesFile = rootProject.file("keystore.properties")
if (keystorePropertiesFile.exists()) {
    keystoreProperties.load(FileInputStream(keystorePropertiesFile))
}

android {
    namespace = "ws.tilda.sentryprotocol"
    compileSdk = 34

    defaultConfig {
        applicationId = "ws.tilda.sentryprotocol"
        minSdk = 26
        targetSdk = 34
        versionCode = 800
        versionName = "0.8.0"
    }

    signingConfigs {
        create("release") {
            val kPath = project.findProperty("KEYSTORE_PATH") as String? ?: System.getenv("KEYSTORE_PATH")
            if (kPath != null) {
                storeFile = rootProject.file(kPath)
                storePassword = project.findProperty("KEYSTORE_PASSWORD") as String? ?: System.getenv("KEYSTORE_PASSWORD")
                keyAlias = project.findProperty("KEY_ALIAS") as String? ?: System.getenv("KEY_ALIAS")
                keyPassword = project.findProperty("KEY_PASSWORD") as String? ?: System.getenv("KEY_PASSWORD")
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            val releaseSigning = signingConfigs.getByName("release")
            if (releaseSigning.storeFile != null) {
                signingConfig = releaseSigning
            }
        }
    }

    buildFeatures {
        compose = true
    }
    composeOptions {
        kotlinCompilerExtensionVersion = "1.5.4"
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions {
        jvmTarget = "17"
    }
}

dependencies {
    implementation("androidx.core:core-ktx:1.12.0")
    implementation("androidx.lifecycle:lifecycle-runtime-ktx:2.7.0")
    implementation("androidx.activity:activity-compose:1.8.2")
    implementation(platform("androidx.compose:compose-bom:2023.10.01"))
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.ui:ui-graphics")
    implementation("androidx.compose.ui:ui-tooling-preview")
    implementation("androidx.compose.material3:material3")
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
}
