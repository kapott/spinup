// Package ui provides TUI components for spinup.
package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestAlertLevel_String(t *testing.T) {
	tests := []struct {
		level    AlertLevel
		expected string
	}{
		{AlertLevelInfo, "INFO"},
		{AlertLevelWarning, "WARNING"},
		{AlertLevelError, "ERROR"},
		{AlertLevelCritical, "CRITICAL"},
		{AlertLevel(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("AlertLevel.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAlertLevel_Icon(t *testing.T) {
	tests := []struct {
		level    AlertLevel
		expected string
	}{
		{AlertLevelInfo, IconInfo},
		{AlertLevelWarning, IconWarning},
		{AlertLevelError, IconError},
		{AlertLevelCritical, "🔴"},
	}

	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			if got := tt.level.Icon(); got != tt.expected {
				t.Errorf("AlertLevel.Icon() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewInfoAlert(t *testing.T) {
	alert := NewInfoAlert("Test Title", "Test message")

	if alert.Level != AlertLevelInfo {
		t.Errorf("Level = %v, want %v", alert.Level, AlertLevelInfo)
	}
	if alert.Title != "Test Title" {
		t.Errorf("Title = %v, want %v", alert.Title, "Test Title")
	}
	if alert.Message != "Test message" {
		t.Errorf("Message = %v, want %v", alert.Message, "Test message")
	}
	if !alert.Dismissible {
		t.Error("Expected Dismissible to be true")
	}
	if alert.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}
}

func TestNewWarningAlert(t *testing.T) {
	alert := NewWarningAlert("Warning Title", "Warning message")

	if alert.Level != AlertLevelWarning {
		t.Errorf("Level = %v, want %v", alert.Level, AlertLevelWarning)
	}
	if alert.Title != "Warning Title" {
		t.Errorf("Title = %v, want %v", alert.Title, "Warning Title")
	}
	if !alert.Dismissible {
		t.Error("Expected Dismissible to be true")
	}
}

func TestNewErrorAlert(t *testing.T) {
	alert := NewErrorAlert("Error Title", "Error message")

	if alert.Level != AlertLevelError {
		t.Errorf("Level = %v, want %v", alert.Level, AlertLevelError)
	}
	if !alert.Dismissible {
		t.Error("Expected Dismissible to be true")
	}
}

func TestNewCriticalAlert(t *testing.T) {
	alert := NewCriticalAlert("Critical Title", "Critical message")

	if alert.Level != AlertLevelCritical {
		t.Errorf("Level = %v, want %v", alert.Level, AlertLevelCritical)
	}
	if alert.Dismissible {
		t.Error("Expected Dismissible to be false for critical alerts")
	}
}

func TestNewBillingNotVerifiedAlert(t *testing.T) {
	alert := NewBillingNotVerifiedAlert("12345678", "vast.ai", "https://console.vast.ai/", "spinup.log")

	if alert.Level != AlertLevelCritical {
		t.Errorf("Level = %v, want %v", alert.Level, AlertLevelCritical)
	}
	if alert.InstanceID != "12345678" {
		t.Errorf("InstanceID = %v, want %v", alert.InstanceID, "12345678")
	}
	if alert.Provider != "vast.ai" {
		t.Errorf("Provider = %v, want %v", alert.Provider, "vast.ai")
	}
	if alert.ConsoleURL != "https://console.vast.ai/" {
		t.Errorf("ConsoleURL = %v, want %v", alert.ConsoleURL, "https://console.vast.ai/")
	}
	if alert.LogFile != "spinup.log" {
		t.Errorf("LogFile = %v, want %v", alert.LogFile, "spinup.log")
	}
	if len(alert.Actions) != 4 {
		t.Errorf("Expected 4 actions, got %d", len(alert.Actions))
	}
	if alert.Dismissible {
		t.Error("Expected Dismissible to be false for billing not verified alert")
	}
}

func TestAlert_WithDetails(t *testing.T) {
	alert := NewInfoAlert("Title", "Message").WithDetails("Detail 1", "Detail 2")

	if len(alert.Details) != 2 {
		t.Errorf("Expected 2 details, got %d", len(alert.Details))
	}
	if alert.Details[0] != "Detail 1" || alert.Details[1] != "Detail 2" {
		t.Errorf("Details not set correctly: %v", alert.Details)
	}
}

func TestAlert_WithActions(t *testing.T) {
	alert := NewWarningAlert("Title", "Message").WithActions("Action 1", "Action 2")

	if len(alert.Actions) != 2 {
		t.Errorf("Expected 2 actions, got %d", len(alert.Actions))
	}
	if alert.Actions[0] != "Action 1" || alert.Actions[1] != "Action 2" {
		t.Errorf("Actions not set correctly: %v", alert.Actions)
	}
}

func TestAlertModel_SetAlert(t *testing.T) {
	m := NewAlertModel()

	alert := NewInfoAlert("Test", "Test message")
	m.SetAlert(&alert)

	if m.alert == nil {
		t.Error("Expected alert to be set")
	}
	if m.alert.Title != "Test" {
		t.Errorf("Alert title = %v, want %v", m.alert.Title, "Test")
	}
	if m.dismissed {
		t.Error("Expected dismissed to be false")
	}
}

func TestAlertModel_IsDismissed(t *testing.T) {
	m := NewAlertModel()
	alert := NewInfoAlert("Test", "Test message")
	m.SetAlert(&alert)

	if m.IsDismissed() {
		t.Error("Expected IsDismissed to return false initially")
	}

	m.dismissed = true
	if !m.IsDismissed() {
		t.Error("Expected IsDismissed to return true after dismissal")
	}
}

func TestAlertModel_View_NoAlert(t *testing.T) {
	m := NewAlertModel()
	view := m.View()

	if view != "" {
		t.Errorf("Expected empty view when no alert, got: %v", view)
	}
}

func TestAlertModel_View_Dismissed(t *testing.T) {
	m := NewAlertModel()
	alert := NewInfoAlert("Test", "Test message")
	m.SetAlert(&alert)
	m.dismissed = true

	view := m.View()
	if view != "" {
		t.Errorf("Expected empty view when dismissed, got: %v", view)
	}
}

func TestAlertModel_View_InfoAlert(t *testing.T) {
	m := NewAlertModelWithAlert(NewInfoAlert("Test Info", "This is an info message"))
	m.width = 80
	m.height = 24

	view := m.View()

	if !strings.Contains(view, "Test Info") {
		t.Error("Expected view to contain alert title")
	}
	if !strings.Contains(view, "This is an info message") {
		t.Error("Expected view to contain alert message")
	}
	if !strings.Contains(view, "Press Enter or Space to dismiss") {
		t.Error("Expected view to contain dismiss hint for dismissible alert")
	}
}

func TestAlertModel_View_WarningAlert(t *testing.T) {
	m := NewAlertModelWithAlert(NewWarningAlert("Test Warning", "This is a warning message"))
	m.width = 80
	m.height = 24

	view := m.View()

	if !strings.Contains(view, "WARNING") {
		t.Error("Expected view to contain WARNING")
	}
	if !strings.Contains(view, "Test Warning") {
		t.Error("Expected view to contain alert title")
	}
}

func TestAlertModel_View_ErrorAlert(t *testing.T) {
	m := NewAlertModelWithAlert(NewErrorAlert("Test Error", "This is an error message"))
	m.width = 80
	m.height = 24

	view := m.View()

	if !strings.Contains(view, "ERROR") {
		t.Error("Expected view to contain ERROR")
	}
	if !strings.Contains(view, "Test Error") {
		t.Error("Expected view to contain alert title")
	}
}

func TestAlertModel_View_CriticalAlert(t *testing.T) {
	alert := NewBillingNotVerifiedAlert("12345678", "vast.ai", "https://console.vast.ai/", "spinup.log")
	m := NewAlertModelWithAlert(alert)
	m.width = 80
	m.height = 24

	view := m.View()

	if !strings.Contains(view, "CRITICAL") {
		t.Error("Expected view to contain CRITICAL")
	}
	if !strings.Contains(view, "12345678") {
		t.Error("Expected view to contain instance ID")
	}
	if !strings.Contains(view, "IMMEDIATE ACTION REQUIRED") {
		t.Error("Expected view to contain action header")
	}
	if !strings.Contains(view, "https://console.vast.ai/") {
		t.Error("Expected view to contain console URL")
	}
	if !strings.Contains(view, "spinup.log") {
		t.Error("Expected view to contain log file")
	}
}

func TestAlertModel_Update_FlashMsg(t *testing.T) {
	alert := NewCriticalAlert("Critical", "Message")
	m := NewAlertModelWithAlert(alert)

	// Initial state
	if m.flashState != 0 {
		t.Errorf("Expected initial flashState to be 0, got %d", m.flashState)
	}

	// Send flash message
	m, _ = m.Update(alertFlashMsg{})

	// Flash state should toggle for critical alerts
	if m.flashState != 1 {
		t.Errorf("Expected flashState to be 1 after flash msg, got %d", m.flashState)
	}

	// Toggle again
	m, _ = m.Update(alertFlashMsg{})
	if m.flashState != 0 {
		t.Errorf("Expected flashState to be 0 after second flash msg, got %d", m.flashState)
	}
}

func TestAlertModel_Update_FlashMsg_NonCritical(t *testing.T) {
	alert := NewInfoAlert("Info", "Message")
	m := NewAlertModelWithAlert(alert)

	// Initial state
	if m.flashState != 0 {
		t.Errorf("Expected initial flashState to be 0, got %d", m.flashState)
	}

	// Send flash message
	m, _ = m.Update(alertFlashMsg{})

	// Flash state should NOT toggle for non-critical alerts
	if m.flashState != 0 {
		t.Errorf("Expected flashState to remain 0 for info alert, got %d", m.flashState)
	}
}

func TestAlertModel_Update_ShowMsg(t *testing.T) {
	m := NewAlertModel()

	alert := NewWarningAlert("New Warning", "New message")
	m, _ = m.Update(AlertShowMsg{Alert: alert})

	if m.alert == nil {
		t.Error("Expected alert to be set after AlertShowMsg")
	}
	if m.alert.Title != "New Warning" {
		t.Errorf("Expected alert title to be 'New Warning', got %v", m.alert.Title)
	}
}

func TestAlertModel_Update_DismissMsg(t *testing.T) {
	alert := NewInfoAlert("Info", "Message")
	m := NewAlertModelWithAlert(alert)

	m, _ = m.Update(AlertDismissMsg{})

	if !m.dismissed {
		t.Error("Expected dismissed to be true after AlertDismissMsg")
	}
	if m.alert != nil {
		t.Error("Expected alert to be nil after dismiss")
	}
}

func TestAlertModel_Update_DismissMsg_NonDismissible(t *testing.T) {
	alert := NewCriticalAlert("Critical", "Message")
	m := NewAlertModelWithAlert(alert)

	m, _ = m.Update(AlertDismissMsg{})

	// Critical alerts are not dismissible
	if m.dismissed {
		t.Error("Expected dismissed to remain false for non-dismissible alert")
	}
	if m.alert == nil {
		t.Error("Expected alert to remain set for non-dismissible alert")
	}
}

func TestAlertModel_Update_KeyPress_Enter_Dismissible(t *testing.T) {
	alert := NewInfoAlert("Info", "Message")
	m := NewAlertModelWithAlert(alert)

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !m.dismissed {
		t.Error("Expected dismissed to be true after Enter key on dismissible alert")
	}
	if cmd == nil {
		t.Error("Expected command to be returned after dismiss")
	}
}

func TestAlertModel_Update_KeyPress_Enter_NonDismissible(t *testing.T) {
	alert := NewCriticalAlert("Critical", "Message")
	m := NewAlertModelWithAlert(alert)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.dismissed {
		t.Error("Expected dismissed to remain false for non-dismissible alert")
	}
}

func TestAlertModel_Update_WindowSize(t *testing.T) {
	m := NewAlertModel()

	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	if m.width != 100 {
		t.Errorf("Expected width to be 100, got %d", m.width)
	}
	if m.height != 50 {
		t.Errorf("Expected height to be 50, got %d", m.height)
	}
}

func TestShowAlert(t *testing.T) {
	alert := NewInfoAlert("Test", "Message")
	cmd := ShowAlert(alert)

	msg := cmd()
	showMsg, ok := msg.(AlertShowMsg)
	if !ok {
		t.Errorf("Expected AlertShowMsg, got %T", msg)
	}
	if showMsg.Alert.Title != "Test" {
		t.Errorf("Expected alert title 'Test', got %v", showMsg.Alert.Title)
	}
}

func TestDismissAlert(t *testing.T) {
	cmd := DismissAlert()

	msg := cmd()
	_, ok := msg.(AlertDismissMsg)
	if !ok {
		t.Errorf("Expected AlertDismissMsg, got %T", msg)
	}
}

func TestRenderAlertInline(t *testing.T) {
	alert := NewInfoAlert("Inline Test", "Inline message")
	rendered := RenderAlertInline(alert)

	if !strings.Contains(rendered, "Inline Test") {
		t.Error("Expected rendered output to contain alert title")
	}
	if !strings.Contains(rendered, "Inline message") {
		t.Error("Expected rendered output to contain alert message")
	}
}

func TestRenderCriticalAlertRaw(t *testing.T) {
	alert := NewBillingNotVerifiedAlert("12345678", "vast.ai", "https://console.vast.ai/", "spinup.log")
	rendered := RenderCriticalAlertRaw(alert)

	// Check for ANSI escape codes
	if !strings.Contains(rendered, "\033[") {
		t.Error("Expected rendered output to contain ANSI escape codes")
	}
	// Check for box drawing characters
	if !strings.Contains(rendered, "╔") {
		t.Error("Expected rendered output to contain box drawing character")
	}
	if !strings.Contains(rendered, "12345678") {
		t.Error("Expected rendered output to contain instance ID")
	}
}

func TestRenderCriticalAlertRaw_NonCritical(t *testing.T) {
	alert := NewInfoAlert("Info", "Message")
	rendered := RenderCriticalAlertRaw(alert)

	// For non-critical alerts, it should fall back to RenderAlertInline
	if strings.Contains(rendered, "\033[5m") { // blink code
		t.Error("Non-critical alerts should not have blink codes")
	}
}

func TestAlertModel_SetDimensions(t *testing.T) {
	m := NewAlertModel()
	m.SetDimensions(120, 40)

	if m.width != 120 {
		t.Errorf("Expected width to be 120, got %d", m.width)
	}
	if m.height != 40 {
		t.Errorf("Expected height to be 40, got %d", m.height)
	}
}

func TestAlert_CreatedAt(t *testing.T) {
	before := time.Now()
	alert := NewInfoAlert("Test", "Message")
	after := time.Now()

	if alert.CreatedAt.Before(before) || alert.CreatedAt.After(after) {
		t.Errorf("CreatedAt should be between %v and %v, got %v", before, after, alert.CreatedAt)
	}
}

// Tests for Spot Interruption Alert (F038)

func TestNewSpotInterruptionAlert(t *testing.T) {
	alert := NewSpotInterruptionAlert(2.15, 3*time.Hour+18*time.Minute, "vast.ai", "12345678")

	if alert.SessionCost != 2.15 {
		t.Errorf("SessionCost = %v, want %v", alert.SessionCost, 2.15)
	}
	if alert.Duration != 3*time.Hour+18*time.Minute {
		t.Errorf("Duration = %v, want %v", alert.Duration, 3*time.Hour+18*time.Minute)
	}
	if alert.Provider != "vast.ai" {
		t.Errorf("Provider = %v, want %v", alert.Provider, "vast.ai")
	}
	if alert.InstanceID != "12345678" {
		t.Errorf("InstanceID = %v, want %v", alert.InstanceID, "12345678")
	}
}

func TestSpotInterruptionModel_SetAlert(t *testing.T) {
	m := NewSpotInterruptionModel()

	alert := NewSpotInterruptionAlert(5.50, 2*time.Hour, "lambda", "instance-123")
	m.SetAlert(&alert)

	if m.alert == nil {
		t.Error("Expected alert to be set")
	}
	if m.alert.SessionCost != 5.50 {
		t.Errorf("SessionCost = %v, want %v", m.alert.SessionCost, 5.50)
	}
	if m.action != SpotInterruptionActionNone {
		t.Errorf("Expected action to be None, got %v", m.action)
	}
}

func TestSpotInterruptionModel_HasAlert(t *testing.T) {
	m := NewSpotInterruptionModel()

	if m.HasAlert() {
		t.Error("Expected HasAlert to return false initially")
	}

	alert := NewSpotInterruptionAlert(1.0, time.Hour, "runpod", "test")
	m.SetAlert(&alert)

	if !m.HasAlert() {
		t.Error("Expected HasAlert to return true after SetAlert")
	}
}

func TestSpotInterruptionModel_View(t *testing.T) {
	alert := NewSpotInterruptionAlert(2.15, 3*time.Hour+18*time.Minute, "vast.ai", "12345678")
	m := NewSpotInterruptionModelWithAlert(alert)
	m.width = 80
	m.height = 24

	view := m.View()

	// Check for PRD Section 5.3 layout elements
	if !strings.Contains(view, "SPOT INSTANCE INTERRUPTED") {
		t.Error("Expected view to contain 'SPOT INSTANCE INTERRUPTED' title")
	}
	if !strings.Contains(view, "Your spot instance was reclaimed by the provider") {
		t.Error("Expected view to contain explanation message")
	}
	if !strings.Contains(view, "This is normal for spot instances") {
		t.Error("Expected view to contain reassurance message")
	}
	if !strings.Contains(view, "Session cost:") {
		t.Error("Expected view to contain session cost")
	}
	if !strings.Contains(view, "€2.15") {
		t.Error("Expected view to contain formatted cost €2.15")
	}
	if !strings.Contains(view, "Duration:") {
		t.Error("Expected view to contain duration")
	}
	if !strings.Contains(view, "3h 18m") {
		t.Error("Expected view to contain formatted duration '3h 18m'")
	}
	if !strings.Contains(view, "[r]") {
		t.Error("Expected view to contain [r] restart option")
	}
	if !strings.Contains(view, "Restart with new instance") {
		t.Error("Expected view to contain restart description")
	}
	if !strings.Contains(view, "[q]") {
		t.Error("Expected view to contain [q] quit option")
	}
	if !strings.Contains(view, "Quit") {
		t.Error("Expected view to contain quit description")
	}
}

func TestSpotInterruptionModel_View_NoAlert(t *testing.T) {
	m := NewSpotInterruptionModel()
	view := m.View()

	if view != "" {
		t.Errorf("Expected empty view when no alert, got: %v", view)
	}
}

func TestSpotInterruptionModel_Update_KeyPress_R(t *testing.T) {
	alert := NewSpotInterruptionAlert(1.0, time.Hour, "provider", "id")
	m := NewSpotInterruptionModelWithAlert(alert)

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	if m.action != SpotInterruptionActionRestart {
		t.Errorf("Expected action to be Restart, got %v", m.action)
	}
	if cmd == nil {
		t.Error("Expected command to be returned")
	}

	// Verify the message
	msg := cmd()
	actionMsg, ok := msg.(SpotInterruptionActionMsg)
	if !ok {
		t.Errorf("Expected SpotInterruptionActionMsg, got %T", msg)
	}
	if actionMsg.Action != SpotInterruptionActionRestart {
		t.Errorf("Expected Restart action in message, got %v", actionMsg.Action)
	}
}

func TestSpotInterruptionModel_Update_KeyPress_UpperR(t *testing.T) {
	alert := NewSpotInterruptionAlert(1.0, time.Hour, "provider", "id")
	m := NewSpotInterruptionModelWithAlert(alert)

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})

	if m.action != SpotInterruptionActionRestart {
		t.Errorf("Expected action to be Restart for uppercase R, got %v", m.action)
	}
	if cmd == nil {
		t.Error("Expected command to be returned")
	}
}

func TestSpotInterruptionModel_Update_KeyPress_Q(t *testing.T) {
	alert := NewSpotInterruptionAlert(1.0, time.Hour, "provider", "id")
	m := NewSpotInterruptionModelWithAlert(alert)

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	if m.action != SpotInterruptionActionQuit {
		t.Errorf("Expected action to be Quit, got %v", m.action)
	}
	if cmd == nil {
		t.Error("Expected command to be returned")
	}

	// Verify the message
	msg := cmd()
	actionMsg, ok := msg.(SpotInterruptionActionMsg)
	if !ok {
		t.Errorf("Expected SpotInterruptionActionMsg, got %T", msg)
	}
	if actionMsg.Action != SpotInterruptionActionQuit {
		t.Errorf("Expected Quit action in message, got %v", actionMsg.Action)
	}
}

func TestSpotInterruptionModel_Update_KeyPress_UpperQ(t *testing.T) {
	alert := NewSpotInterruptionAlert(1.0, time.Hour, "provider", "id")
	m := NewSpotInterruptionModelWithAlert(alert)

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Q'}})

	if m.action != SpotInterruptionActionQuit {
		t.Errorf("Expected action to be Quit for uppercase Q, got %v", m.action)
	}
	if cmd == nil {
		t.Error("Expected command to be returned")
	}
}

func TestSpotInterruptionModel_Update_KeyPress_CtrlC(t *testing.T) {
	alert := NewSpotInterruptionAlert(1.0, time.Hour, "provider", "id")
	m := NewSpotInterruptionModelWithAlert(alert)

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if m.action != SpotInterruptionActionQuit {
		t.Errorf("Expected action to be Quit for Ctrl+C, got %v", m.action)
	}
	if cmd == nil {
		t.Error("Expected command to be returned")
	}
}

func TestSpotInterruptionModel_Update_KeyPress_NoAlert(t *testing.T) {
	m := NewSpotInterruptionModel()

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	if m.action != SpotInterruptionActionNone {
		t.Errorf("Expected action to remain None when no alert, got %v", m.action)
	}
	if cmd != nil {
		t.Error("Expected no command when no alert")
	}
}

func TestSpotInterruptionModel_Update_KeyPress_OtherKey(t *testing.T) {
	alert := NewSpotInterruptionAlert(1.0, time.Hour, "provider", "id")
	m := NewSpotInterruptionModelWithAlert(alert)

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	if m.action != SpotInterruptionActionNone {
		t.Errorf("Expected action to remain None for unrecognized key, got %v", m.action)
	}
	if cmd != nil {
		t.Error("Expected no command for unrecognized key")
	}
}

func TestSpotInterruptionModel_Update_WindowSize(t *testing.T) {
	m := NewSpotInterruptionModel()

	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	if m.width != 100 {
		t.Errorf("Expected width to be 100, got %d", m.width)
	}
	if m.height != 50 {
		t.Errorf("Expected height to be 50, got %d", m.height)
	}
}

func TestSpotInterruptionModel_SetDimensions(t *testing.T) {
	m := NewSpotInterruptionModel()
	m.SetDimensions(120, 40)

	if m.width != 120 {
		t.Errorf("Expected width to be 120, got %d", m.width)
	}
	if m.height != 40 {
		t.Errorf("Expected height to be 40, got %d", m.height)
	}
}

func TestRenderSpotInterruptionInline(t *testing.T) {
	alert := NewSpotInterruptionAlert(2.15, 3*time.Hour+18*time.Minute, "vast.ai", "12345678")
	rendered := RenderSpotInterruptionInline(alert)

	if !strings.Contains(rendered, "SPOT INSTANCE INTERRUPTED") {
		t.Error("Expected rendered output to contain title")
	}
	if !strings.Contains(rendered, "€2.15") {
		t.Error("Expected rendered output to contain cost")
	}
	if !strings.Contains(rendered, "3h 18m") {
		t.Error("Expected rendered output to contain duration")
	}
}

func TestSpotInterruptionAction_Constants(t *testing.T) {
	// Ensure action constants have expected values
	if SpotInterruptionActionNone != 0 {
		t.Errorf("SpotInterruptionActionNone should be 0, got %d", SpotInterruptionActionNone)
	}
	if SpotInterruptionActionRestart != 1 {
		t.Errorf("SpotInterruptionActionRestart should be 1, got %d", SpotInterruptionActionRestart)
	}
	if SpotInterruptionActionQuit != 2 {
		t.Errorf("SpotInterruptionActionQuit should be 2, got %d", SpotInterruptionActionQuit)
	}
}

func TestSpotInterruptionModel_Action(t *testing.T) {
	alert := NewSpotInterruptionAlert(1.0, time.Hour, "provider", "id")
	m := NewSpotInterruptionModelWithAlert(alert)

	if m.Action() != SpotInterruptionActionNone {
		t.Errorf("Initial action should be None, got %v", m.Action())
	}

	m.action = SpotInterruptionActionRestart
	if m.Action() != SpotInterruptionActionRestart {
		t.Errorf("Action should be Restart, got %v", m.Action())
	}
}

func TestSpotInterruptionModel_Init(t *testing.T) {
	m := NewSpotInterruptionModel()
	cmd := m.Init()

	if cmd != nil {
		t.Error("Init should return nil command")
	}
}

func TestSpotInterruptionModel_View_ShortDuration(t *testing.T) {
	// Test with a duration less than an hour
	alert := NewSpotInterruptionAlert(0.50, 45*time.Minute, "runpod", "test")
	m := NewSpotInterruptionModelWithAlert(alert)
	m.width = 80
	m.height = 24

	view := m.View()

	if !strings.Contains(view, "45m") {
		t.Error("Expected view to contain '45m' for sub-hour duration")
	}
}
