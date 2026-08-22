plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.compose")
}

android {
    namespace = "com.sunpanel.app"
    compileSdk = 35

    defaultConfig {
        applicationId = "com.sunpanel.app"
        // Android 7.0: đủ mới để WebView có sẵn WebSocket và ES2017 mà giao diện
        // panel cần, đủ cũ để chạy được trên điện thoại đời thấp.
        minSdk = 24
        targetSdk = 35
        versionCode = 1
        versionName = "1.0"
    }

    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")

            // Bản phát hành không ký sẵn: khóa ký là của người dựng bản, không
            // phải thứ nằm trong kho mã.
            signingConfig = null
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = "17"
    }

    buildFeatures {
        compose = true
    }
}

dependencies {
    implementation(platform("androidx.compose:compose-bom:2024.10.01"))
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.material:material-icons-extended")
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.ui:ui-tooling-preview")
    implementation("androidx.activity:activity-compose:1.9.3")
    implementation("androidx.core:core-ktx:1.13.1")
    implementation("androidx.lifecycle:lifecycle-runtime-ktx:2.8.7")
    implementation("androidx.webkit:webkit:1.12.1")

    // Nhánh JSch còn được bảo trì: bản gốc dừng từ 2018 và không có các thuật
    // toán mà sshd đời mới yêu cầu, nên nó không nối được vào một VPS mới cài.
    implementation("com.github.mwiede:jsch:2.28.7")

    debugImplementation("androidx.compose.ui:ui-tooling")

    // org.json trong bản Android chỉ là khung rỗng khi chạy test trên JVM, nên
    // bài kiểm thử cần bản thật.
    testImplementation("org.json:json:20240303")
    testImplementation("junit:junit:4.13.2")
}
