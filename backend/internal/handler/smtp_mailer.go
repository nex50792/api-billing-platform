package handler

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"

	mail "github.com/wneessen/go-mail"
)

type SMTPMailer struct {
	SMTPHost string
	SMTPPort int
	Username string
	Password string
	From     string
	DB       *sql.DB
}

func NewSMTPMailer(db *sql.DB) *SMTPMailer {
	return &SMTPMailer{
		SMTPHost: os.Getenv("SMTP_HOST"),
		SMTPPort: getEnvInt("SMTP_PORT", 587),
		Username: os.Getenv("SMTP_USERNAME"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     os.Getenv("SMTP_FROM"),
		DB:       db,
	}
}

func getEnvInt(key string, def int) int {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	var i int
	_, err := fmt.Sscanf(val, "%d", &i)
	if err != nil {
		return def
	}
	return i
}

func (m *SMTPMailer) send(to, subject, body string) error {
	msg := mail.NewMsg()
	if err := msg.From(m.From); err != nil {
		return fmt.Errorf("invalid from: %w", err)
	}
	if err := msg.To(to); err != nil {
		return fmt.Errorf("invalid to: %w", err)
	}
	msg.Subject(subject)
	msg.SetBodyString(mail.TypeTextHTML, body)

	client, err := mail.NewClient(m.SMTPHost,
		mail.WithPort(m.SMTPPort),
		mail.WithUsername(m.Username),
		mail.WithPassword(m.Password),
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithTimeout(15*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to create mail client: %w", err)
	}

	sendErr := client.DialAndSend(msg)
	m.logSend(to, subject, body, sendErr)
	return sendErr
}

// DB=nil の場合はスキップ。
func (m *SMTPMailer) logSend(to, subject, body string, sendErr error) {
	status := "success"
	errorMsg := ""
	if sendErr != nil {
		status = "error"
		errorMsg = sendErr.Error()
		slog.Error("mail send failed", "to", to, "subject", subject, "error", sendErr)
	} else {
		slog.Info("mail send success", "to", to, "subject", subject)
	}
	if m.DB == nil {
		return
	}
	if _, dberr := m.DB.Exec(
		`INSERT INTO email_logs (to_email, subject, body, sent_at, status, error_message) VALUES ($1, $2, $3, $4, $5, $6)`,
		to, subject, body, time.Now(), status, errorMsg,
	); dberr != nil {
		slog.Error("mail email_logs save failed", "error", dberr)
	}
}

func (m *SMTPMailer) SendPasswordSetupEmail(to, link string) error {
	return m.send(to,
		"【API Billing Platform】パスワード設定のご案内",
		"<p>以下のリンクからパスワードを設定してください。<br><a href='"+link+"'>パスワード設定</a></p>",
	)
}

func (m *SMTPMailer) SendPasswordResetEmail(to, link string) error {
	return m.send(to,
		"【API Billing Platform】パスワードリセットのご案内",
		"<p>以下のリンクからパスワードをリセットしてください。<br><a href='"+link+"'>パスワードリセット</a></p><p>このリンクは1時間有効です。</p>",
	)
}

func (m *SMTPMailer) SendVerificationCodeEmail(to, code string) error {
	return m.send(to,
		"【API Billing Platform】認証コードのご案内",
		fmt.Sprintf("<p>以下の認証コードを入力してください。<br><b>%s</b></p>", code),
	)
}
