package handler

import "errors"

type MockMailer struct {
	setupCalled bool
	resetCalled bool
}

func (m *MockMailer) SendPasswordSetupEmail(to, link string) error {
	m.setupCalled = true
	return nil
}

func (m *MockMailer) SendVerificationCodeEmail(to, code string) error {
	return nil
}

func (m *MockMailer) SendPasswordResetEmail(to, link string) error {
	m.resetCalled = true
	return nil
}

func (m *MockMailer) WasSendCalled() bool {
	return m.setupCalled
}

func (m *MockMailer) WasResetCalled() bool {
	return m.resetCalled
}

type FailingMockMailer struct{}

func (m *FailingMockMailer) SendPasswordSetupEmail(to, link string) error {
	return errMailerFailed
}

func (m *FailingMockMailer) SendVerificationCodeEmail(to, code string) error {
	return errMailerFailed
}

func (m *FailingMockMailer) SendPasswordResetEmail(to, link string) error {
	return errMailerFailed
}

var errMailerFailed = errors.New("mock mailer: send failed")
