// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"net/smtp"
	"text/template"
	"time"

	"github.com/zyvorai/transiva/logger"
)

// NotificationManager handles email notifications for export events
type NotificationManager struct {
	config *EmailConfig
	log    logger.Logger
}

// EmailConfig contains SMTP configuration
type EmailConfig struct {
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	FromAddress  string
	ToAddresses  []string
	TLSEnabled   bool
	AuthMethod   string // "plain", "login", "crammd5"
}

// ExportNotification contains export information for notifications
type ExportNotification struct {
	VMName           string
	Status           string // "success", "failed", "started"
	StartTime        time.Time
	EndTime          time.Time
	Duration         time.Duration
	TotalSize        int64
	FilesCount       int
	OutputDir        string
	ErrorMessage     string
	Provider         string
	Format           string
	Compressed       bool
	Verified         bool
	CloudDestination string
}

// NewNotificationManager creates a new notification manager
func NewNotificationManager(config *EmailConfig, log logger.Logger) *NotificationManager {
	return &NotificationManager{
		config: config,
		log:    log,
	}
}

// SendStartNotification sends a notification when export starts
func (nm *NotificationManager) SendStartNotification(notification *ExportNotification) error {
	if nm.config == nil || len(nm.config.ToAddresses) == 0 {
		nm.log.Info("email notifications not configured, skipping start notification")
		return nil
	}

	notification.Status = "started"
	subject := fmt.Sprintf("[HyperExport] Export Started: %s", notification.VMName)
	body := nm.renderStartTemplate(notification)

	return nm.sendEmail(subject, body)
}

// SendSuccessNotification sends a notification when export succeeds
func (nm *NotificationManager) SendSuccessNotification(notification *ExportNotification) error {
	if nm.config == nil || len(nm.config.ToAddresses) == 0 {
		nm.log.Info("email notifications not configured, skipping success notification")
		return nil
	}

	notification.Status = "success"
	subject := fmt.Sprintf("[HyperExport] Export Completed: %s", notification.VMName)
	body := nm.renderSuccessTemplate(notification)

	return nm.sendEmail(subject, body)
}

// SendFailureNotification sends a notification when export fails
func (nm *NotificationManager) SendFailureNotification(notification *ExportNotification) error {
	if nm.config == nil || len(nm.config.ToAddresses) == 0 {
		nm.log.Info("email notifications not configured, skipping failure notification")
		return nil
	}

	notification.Status = "failed"
	subject := fmt.Sprintf("[HyperExport] Export Failed: %s", notification.VMName)
	body := nm.renderFailureTemplate(notification)

	return nm.sendEmail(subject, body)
}

// sendEmail sends an email using SMTP
func (nm *NotificationManager) sendEmail(subject, body string) error {
	nm.log.Info("sending email notification",
		"to", nm.config.ToAddresses,
		"subject", subject)

	// Build message
	message := nm.buildEmailMessage(subject, body)

	// SMTP authentication
	var auth smtp.Auth
	switch nm.config.AuthMethod {
	case "plain":
		auth = smtp.PlainAuth("", nm.config.SMTPUsername, nm.config.SMTPPassword, nm.config.SMTPHost)
	case "login":
		auth = &loginAuth{nm.config.SMTPUsername, nm.config.SMTPPassword}
	case "crammd5":
		auth = smtp.CRAMMD5Auth(nm.config.SMTPUsername, nm.config.SMTPPassword)
	default:
		// Default to plain auth
		auth = smtp.PlainAuth("", nm.config.SMTPUsername, nm.config.SMTPPassword, nm.config.SMTPHost)
	}

	// Send email
	addr := fmt.Sprintf("%s:%d", nm.config.SMTPHost, nm.config.SMTPPort)
	err := smtp.SendMail(
		addr,
		auth,
		nm.config.FromAddress,
		nm.config.ToAddresses,
		[]byte(message),
	)

	if err != nil {
		nm.log.Error("failed to send email", "error", err)
		return fmt.Errorf("send email: %w", err)
	}

	nm.log.Info("email sent successfully")
	return nil
}

// buildEmailMessage constructs the email message
func (nm *NotificationManager) buildEmailMessage(subject, body string) string {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "From: %s\r\n", nm.config.FromAddress)
	fmt.Fprintf(&buf, "To: %s\r\n", nm.config.ToAddresses[0]) // Primary recipient
	fmt.Fprintf(&buf, "Subject: %s\r\n", subject)
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(body)

	return buf.String()
}

// renderStartTemplate renders the start notification template
func (nm *NotificationManager) renderStartTemplate(notification *ExportNotification) string {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #4CAF50; color: white; padding: 20px; text-align: center; }
        .content { background-color: #f9f9f9; padding: 20px; border: 1px solid #ddd; }
        .info { margin: 10px 0; }
        .label { font-weight: bold; color: #555; }
        .value { color: #333; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h2>🚀 Export Started</h2>
        </div>
        <div class="content">
            <h3>VM Export Started</h3>
            <div class="info">
                <span class="label">VM Name:</span>
                <span class="value">{{.VMName}}</span>
            </div>
            <div class="info">
                <span class="label">Provider:</span>
                <span class="value">{{.Provider}}</span>
            </div>
            <div class="info">
                <span class="label">Format:</span>
                <span class="value">{{.Format}}</span>
            </div>
            <div class="info">
                <span class="label">Started At:</span>
                <span class="value">{{.StartTime.Format "2006-01-02 15:04:05"}}</span>
            </div>
            <p>The export process has started. You will receive another notification when it completes.</p>
        </div>
    </div>
</body>
</html>
`
	return nm.renderTemplate(tmpl, notification)
}

// renderSuccessTemplate renders the success notification template
func (nm *NotificationManager) renderSuccessTemplate(notification *ExportNotification) string {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #4CAF50; color: white; padding: 20px; text-align: center; }
        .content { background-color: #f9f9f9; padding: 20px; border: 1px solid #ddd; }
        .info { margin: 10px 0; }
        .label { font-weight: bold; color: #555; }
        .value { color: #333; }
        .success { color: #4CAF50; font-weight: bold; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h2>✅ Export Completed Successfully</h2>
        </div>
        <div class="content">
            <h3>VM Export Completed</h3>
            <div class="info">
                <span class="label">VM Name:</span>
                <span class="value">{{.VMName}}</span>
            </div>
            <div class="info">
                <span class="label">Provider:</span>
                <span class="value">{{.Provider}}</span>
            </div>
            <div class="info">
                <span class="label">Format:</span>
                <span class="value">{{.Format}}{{if .Compressed}} (compressed){{end}}</span>
            </div>
            <div class="info">
                <span class="label">Duration:</span>
                <span class="value">{{.Duration}}</span>
            </div>
            <div class="info">
                <span class="label">Total Size:</span>
                <span class="value">{{FormatBytes .TotalSize}}</span>
            </div>
            <div class="info">
                <span class="label">Files:</span>
                <span class="value">{{.FilesCount}}</span>
            </div>
            <div class="info">
                <span class="label">Output Directory:</span>
                <span class="value">{{.OutputDir}}</span>
            </div>
            {{if .Verified}}
            <div class="info">
                <span class="success">✓ Export verified with checksums</span>
            </div>
            {{end}}
            {{if .CloudDestination}}
            <div class="info">
                <span class="label">Uploaded to:</span>
                <span class="value">{{.CloudDestination}}</span>
            </div>
            {{end}}
            <p>The export completed successfully!</p>
        </div>
    </div>
</body>
</html>
`
	return nm.renderTemplate(tmpl, notification)
}

// renderFailureTemplate renders the failure notification template
func (nm *NotificationManager) renderFailureTemplate(notification *ExportNotification) string {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #f44336; color: white; padding: 20px; text-align: center; }
        .content { background-color: #f9f9f9; padding: 20px; border: 1px solid #ddd; }
        .info { margin: 10px 0; }
        .label { font-weight: bold; color: #555; }
        .value { color: #333; }
        .error { color: #f44336; background-color: #ffebee; padding: 10px; border-left: 4px solid #f44336; margin: 15px 0; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h2>❌ Export Failed</h2>
        </div>
        <div class="content">
            <h3>VM Export Failed</h3>
            <div class="info">
                <span class="label">VM Name:</span>
                <span class="value">{{.VMName}}</span>
            </div>
            <div class="info">
                <span class="label">Provider:</span>
                <span class="value">{{.Provider}}</span>
            </div>
            <div class="info">
                <span class="label">Started At:</span>
                <span class="value">{{.StartTime.Format "2006-01-02 15:04:05"}}</span>
            </div>
            <div class="info">
                <span class="label">Failed At:</span>
                <span class="value">{{.EndTime.Format "2006-01-02 15:04:05"}}</span>
            </div>
            <div class="error">
                <strong>Error:</strong><br>
                {{.ErrorMessage}}
            </div>
            <p>The export process failed. Please check the logs for more details.</p>
        </div>
    </div>
</body>
</html>
`
	return nm.renderTemplate(tmpl, notification)
}

// renderTemplate renders a template with notification data
func (nm *NotificationManager) renderTemplate(tmplStr string, notification *ExportNotification) string {
	funcMap := template.FuncMap{
		"FormatBytes": formatBytesForTemplate,
	}

	tmpl, err := template.New("email").Funcs(funcMap).Parse(tmplStr)
	if err != nil {
		nm.log.Error("failed to parse email template", "error", err)
		return "Failed to render email template"
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, notification); err != nil {
		nm.log.Error("failed to execute email template", "error", err)
		return "Failed to render email template"
	}

	return buf.String()
}

func formatBytesForTemplate(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// loginAuth implements the LOGIN authentication mechanism
type loginAuth struct {
	username string
	password string
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte{}, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		switch string(fromServer) {
		case "Username:":
			return []byte(a.username), nil
		case "Password:":
			return []byte(a.password), nil
		default:
			return nil, fmt.Errorf("unknown server challenge")
		}
	}
	return nil, nil
}
