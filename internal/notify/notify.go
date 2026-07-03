// Package notify provides cross-platform desktop notifications.
// It uses platform-specific commands to send OS-level notifications.
package notify

import (
	"os/exec"
	"runtime"
	"strings"
)

// Send displays a desktop notification with the given title and body.
// Returns an error if the notification could not be sent.
func Send(title, body string) error {
	switch runtime.GOOS {
	case "darwin":
		return sendMacOS(title, body)
	case "linux":
		return sendLinux(title, body)
	case "windows":
		return sendWindows(title, body)
	default:
		return ErrUnsupportedPlatform
	}
}

// SendIfSupported sends a notification only if the platform supports it.
// Returns nil silently on unsupported platforms.
func SendIfSupported(title, body string) error {
	if !Supported() {
		return nil
	}
	return Send(title, body)
}

// Supported reports whether the current platform supports desktop notifications.
func Supported() bool {
	switch runtime.GOOS {
	case "darwin", "linux", "windows":
		return true
	default:
		return false
	}
}

func sendMacOS(title, body string) error {
	script := `display notification "` + escapeAppleScript(body) + `" with title "` + escapeAppleScript(title) + `"`
	cmd := exec.Command("osascript", "-e", script)
	return cmd.Run()
}

func sendLinux(title, body string) error {
	// Try notify-send first
	if err := exec.Command("notify-send", title, body).Run(); err == nil {
		return nil
	}
	// Fallback to zenity
	cmd := exec.Command("zenity", "--notification", "--text="+title+"\n"+body)
	return cmd.Run()
}

func sendWindows(title, body string) error {
	// Use PowerShell to show a toast notification
	script := `
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom, ContentType = WindowsRuntime] | Out-Null

$template = @"
<toast>
    <visual>
        <binding template="ToastGeneric">
            <text>` + escapeXML(title) + `</text>
            <text>` + escapeXML(body) + `</text>
        </binding>
    </visual>
</toast>
"@

$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml($template)
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("cli_mate").Show($toast)
`
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	return cmd.Run()
}

func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// ErrUnsupportedPlatform is returned when notifications are not supported.
type ErrUnsupportedPlatformError struct{}

func (e ErrUnsupportedPlatformError) Error() string {
	return "desktop notifications are not supported on this platform"
}

var ErrUnsupportedPlatform = &ErrUnsupportedPlatformError{}
