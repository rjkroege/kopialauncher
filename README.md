`kopialauncher` is a wrapper for the [Kopia](https://kopia.io) backup program on MacOS. It
adds backing up from an APFS snapshot so that snapshots never need to 
be in an filesystem inconsistent state. Tested only on Big Sur x86.

Use it from `launchagent` per [this stackoverflow](https://stackoverflow.com/questions/49289890/error-code-9216-when-attempting-to-access-keychain-password-in-launchagent).

# Building
`go install github.com/rjkroege/kopialauncher` should be sufficient.

# launchagent
Stick this in `~/Library/LaunchAgents/com.zerowidth.launched.gounodkopia.plist` after replacing
`YOUR_GCS_CRED_JSON_FILE` and `YOUR_HOME_DIR` with appropriate personalized values.

```
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple Computer//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.zerowidth.launched.gounodkopia</string>
	<key>Program</key>
	<string>/usr/local/bin/kopialauncher</string>
	<key>StartInterval</key>
	<integer>3600</integer>
	<key>EnvironmentVariables</key>
	<dict>
		<key>GOOGLE_APPLICATION_CREDENTIALS</key>
		<string>YOUR_GCS_CRED_JSON_FILE</string>
		<key>HOME</key>
		<string>YOUR_HOME_DIR</string>
	</dict>	
</dict>
</plist>
```

# Commands
Turn on like so:

```
# Start
launchctl enable user/`{id -u}/com.zerowidth.launched.gounodkopia
launchctl bootstrap gui/`{id -u} $_h/Library/LaunchAgents/com.zerowidth.launched.gounodkopia.plist

# Stop
launchctl bootout gui/`{id -u}/com.zerowidth.launched.gounodkopia
launchctl disable user/`{id -u}/com.zerowidth.launched.gounodkopia

# Kick
launchctl kickstart gui/`{id -u}/com.zerowidth.launched.gounodkopia
```

# Debugging
Use the MacOS Console app to go hunting for `kopialauncher-*` in the "Log Reports" section.
