# Changelog

## [1.0.1] - 2026-11-02
### Added
- **TextBee SMS Integration:** Emergency contacts can now be phone numbers, and alerts will be sent via SMS.
- **Volume Down Panic Trigger:** Press Volume Down 5 times rapidly to trigger an immediate silent distress signal.
- **F-Droid Metadata:** Added full Fastlane setup for F-Droid submission.

### Fixed
- Web PWA splash screen delay (reduced from 25s to 1.5s).
- Web PWA manifest icons for better installation on Android.
- Email acknowledgment links (changed from POST to GET).
- Android UI not showing success/failure popups before returning to the home screen.
- Android 14 Foreground Service crash.

### Security
- Rotated the Postgres database password.
