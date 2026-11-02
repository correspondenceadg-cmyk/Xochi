# Xochi

Trip-scoped safety check-in. A monorepo containing a Go backend, a TypeScript PWA, and a native Kotlin Android app.

## Quickstart

### Backend
```bash
export DATABASE_URL=some_postgres_url
export TICK_SECRET=dev
export BREVO_API_KEY=some_brevo_key
go run ./cmd/guardian
go test ./...
```

### Web
```bash
cd web
npm install
npm run dev
```

### Android
This project is built entirely via GitHub Actions and the terminal.

To build the APK manually, ensure the Android SDK and Gradle 8.3+ are installed, and then run:

```bash
cd android
java -version # Must be 17+
unset GRAdle_OPT
./gradlew assembleRelease
```

The APK will be located at `android/app/build/outputs/apk/release/app-release.apk`. Alternatively, push a tag (e.g., `v0.11.1`) to trigger the automated release pipeline.
