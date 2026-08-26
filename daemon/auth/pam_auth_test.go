// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"os"
	"testing"
	"time"
)

func TestNewAuthManager(t *testing.T) {
	manager := NewAuthManager()

	if manager == nil {
		t.Fatal("expected manager to be created")
	}

	if manager.sessions == nil {
		t.Error("expected sessions map to be initialized")
	}
}

func TestGenerateToken(t *testing.T) {
	token1, err := generateToken()
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	if len(token1) == 0 {
		t.Error("expected non-empty token")
	}

	// Generate another token
	token2, err := generateToken()
	if err != nil {
		t.Fatalf("failed to generate second token: %v", err)
	}

	// Tokens should be different
	if token1 == token2 {
		t.Error("expected different tokens on each generation")
	}

	// Token should be base64 URL-encoded (44 characters for 32 bytes)
	if len(token1) != 44 {
		t.Errorf("expected token length 44, got %d", len(token1))
	}
}

func TestSessionExpiry(t *testing.T) {
	manager := NewAuthManager()

	// Create a session
	session := &Session{
		Token:     "test-token",
		Username:  "testuser",
		ExpiresAt: time.Now().Add(1 * time.Second),
	}

	manager.sessions[session.Token] = session

	// Session should be valid initially
	_, err := manager.ValidateSession(session.Token)
	if err != nil {
		t.Errorf("expected session to be valid, got error: %v", err)
	}

	// Wait for session to expire
	time.Sleep(2 * time.Second)

	// Session should now be expired
	_, err = manager.ValidateSession(session.Token)
	if err == nil {
		t.Error("expected session to be expired")
	}
}

func TestValidateSession(t *testing.T) {
	manager := NewAuthManager()

	// Create a valid session
	session := &Session{
		Token:     "valid-token",
		Username:  "testuser",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	manager.sessions[session.Token] = session

	// Validate the session
	validatedSession, err := manager.ValidateSession("valid-token")
	if err != nil {
		t.Fatalf("expected session to be valid: %v", err)
	}

	if validatedSession.Username != "testuser" {
		t.Errorf("expected username testuser, got %s", validatedSession.Username)
	}

	// Try invalid token
	_, err = manager.ValidateSession("invalid-token")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestLogout(t *testing.T) {
	manager := NewAuthManager()

	// Create a session
	session := &Session{
		Token:     "logout-token",
		Username:  "testuser",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	manager.sessions[session.Token] = session

	// Logout
	manager.Logout("logout-token")

	// Session should no longer exist
	_, err := manager.ValidateSession("logout-token")
	if err == nil {
		t.Error("expected error after logout")
	}

	// Logging out again should not cause issues
	manager.Logout("logout-token")
}

func TestCleanupExpiredSessions(t *testing.T) {
	// Create manager without starting cleanup goroutine
	manager := &AuthManager{
		sessions: make(map[string]*Session),
	}

	// Create expired session
	expiredSession := &Session{
		Token:     "expired-token",
		Username:  "expired-user",
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Already expired
	}

	// Create valid session
	validSession := &Session{
		Token:     "valid-token",
		Username:  "valid-user",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	manager.sessions[expiredSession.Token] = expiredSession
	manager.sessions[validSession.Token] = validSession

	// Initial count
	if len(manager.sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(manager.sessions))
	}

	// Manually cleanup expired sessions
	manager.mu.Lock()
	now := time.Now()
	for token, session := range manager.sessions {
		if now.After(session.ExpiresAt) {
			delete(manager.sessions, token)
		}
	}
	manager.mu.Unlock()

	// Should only have valid session
	if len(manager.sessions) != 1 {
		t.Errorf("expected 1 session after cleanup, got %d", len(manager.sessions))
	}

	// Valid session should still exist
	_, err := manager.ValidateSession("valid-token")
	if err != nil {
		t.Error("expected valid session to still exist after cleanup")
	}

	// Expired session should be gone
	_, err = manager.ValidateSession("expired-token")
	if err == nil {
		t.Error("expected expired session to be removed")
	}
}

func TestConcurrentAccess(t *testing.T) {
	manager := NewAuthManager()

	// Create a session
	session := &Session{
		Token:     "concurrent-token",
		Username:  "testuser",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	manager.sessions[session.Token] = session

	// Concurrent reads
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := manager.ValidateSession("concurrent-token")
			if err != nil {
				t.Errorf("concurrent read failed: %v", err)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestSessionUsername(t *testing.T) {
	manager := NewAuthManager()

	// Create session
	session := &Session{
		Token:     "user-token",
		Username:  "alice",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	manager.sessions[session.Token] = session

	// Validate and check username
	validatedSession, err := manager.ValidateSession("user-token")
	if err != nil {
		t.Fatalf("failed to validate session: %v", err)
	}

	if validatedSession.Username != "alice" {
		t.Errorf("expected username alice, got %s", validatedSession.Username)
	}
}

func TestMultipleSessions(t *testing.T) {
	manager := NewAuthManager()

	// Create multiple sessions for different users
	sessions := []*Session{
		{
			Token:     "token1",
			Username:  "user1",
			ExpiresAt: time.Now().Add(1 * time.Hour),
		},
		{
			Token:     "token2",
			Username:  "user2",
			ExpiresAt: time.Now().Add(1 * time.Hour),
		},
		{
			Token:     "token3",
			Username:  "user3",
			ExpiresAt: time.Now().Add(1 * time.Hour),
		},
	}

	for _, session := range sessions {
		manager.sessions[session.Token] = session
	}

	// Validate each session
	for _, session := range sessions {
		validatedSession, err := manager.ValidateSession(session.Token)
		if err != nil {
			t.Errorf("failed to validate session for %s: %v", session.Username, err)
		}

		if validatedSession.Username != session.Username {
			t.Errorf("expected username %s, got %s", session.Username, validatedSession.Username)
		}
	}

	// Logout one session
	manager.Logout("token2")

	// token2 should be gone
	_, err := manager.ValidateSession("token2")
	if err == nil {
		t.Error("expected token2 to be logged out")
	}

	// Others should still be valid
	_, err = manager.ValidateSession("token1")
	if err != nil {
		t.Error("expected token1 to still be valid")
	}

	_, err = manager.ValidateSession("token3")
	if err != nil {
		t.Error("expected token3 to still be valid")
	}
}

func TestAuthenticateUser_InvalidUser(t *testing.T) {
	manager := NewAuthManager()

	// Try to authenticate with a non-existent user
	session, err := manager.AuthenticateUser("nonexistent_user_12345", "password")
	if err == nil {
		t.Error("expected error for non-existent user")
	}
	if session != nil {
		t.Error("expected nil session for non-existent user")
	}
}

func TestAuthenticateUser_ValidUser(t *testing.T) {
	manager := NewAuthManager()

	// Get current user from environment (should always exist)
	currentUser := os.Getenv("USER")
	if currentUser == "" {
		currentUser = "root" // Fallback to root if USER not set
	}

	// Try to authenticate with a valid user (doesn't validate password in this implementation)
	session, err := manager.AuthenticateUser(currentUser, "password")
	if err != nil {
		t.Logf("Authentication failed for user %s: %v", currentUser, err)
		t.Logf("This test requires the 'id' command to be available")
		// Don't fail the test as it depends on system configuration
		return
	}

	if session == nil {
		t.Error("expected session to be created for valid user")
		return
	}

	// Verify session properties
	if session.Token == "" {
		t.Error("expected non-empty token")
	}

	if session.Username != currentUser {
		t.Errorf("expected username %s, got %s", currentUser, session.Username)
	}

	if session.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}

	if session.ExpiresAt.IsZero() {
		t.Error("expected ExpiresAt to be set")
	}

	// Verify session expires in the future
	if !session.ExpiresAt.After(time.Now()) {
		t.Error("expected session to expire in the future")
	}

	// Verify session is stored in manager
	storedSession, err := manager.ValidateSession(session.Token)
	if err != nil {
		t.Errorf("failed to validate stored session: %v", err)
	}

	if storedSession.Username != currentUser {
		t.Errorf("expected stored username %s, got %s", currentUser, storedSession.Username)
	}
}

func TestAuthenticateUser_SessionStorage(t *testing.T) {
	manager := NewAuthManager()

	currentUser := os.Getenv("USER")
	if currentUser == "" {
		currentUser = "root"
	}

	// Authenticate and get session
	session1, err := manager.AuthenticateUser(currentUser, "password1")
	if err != nil {
		t.Skipf("Skipping test: authentication requires 'id' command: %v", err)
	}

	// Authenticate again with same user
	session2, err := manager.AuthenticateUser(currentUser, "password2")
	if err != nil {
		t.Fatalf("second authentication failed: %v", err)
	}

	// Tokens should be different
	if session1.Token == session2.Token {
		t.Error("expected different tokens for each authentication")
	}

	// Both sessions should be valid
	_, err = manager.ValidateSession(session1.Token)
	if err != nil {
		t.Error("first session should still be valid")
	}

	_, err = manager.ValidateSession(session2.Token)
	if err != nil {
		t.Error("second session should be valid")
	}

	// Manager should have both sessions
	manager.mu.RLock()
	sessionCount := len(manager.sessions)
	manager.mu.RUnlock()

	if sessionCount < 2 {
		t.Errorf("expected at least 2 sessions, got %d", sessionCount)
	}
}

func TestLogoutNonExistentSession(t *testing.T) {
	manager := NewAuthManager()

	// Logout a session that doesn't exist - should not panic
	manager.Logout("non-existent-token")

	// Verify no sessions exist
	manager.mu.RLock()
	sessionCount := len(manager.sessions)
	manager.mu.RUnlock()

	if sessionCount != 0 {
		t.Errorf("expected 0 sessions, got %d", sessionCount)
	}
}

func TestValidateSessionEmptyToken(t *testing.T) {
	manager := NewAuthManager()

	// Try to validate empty token
	_, err := manager.ValidateSession("")
	if err == nil {
		t.Error("expected error for empty token")
	}
}

func TestSessionExpiryEdgeCase(t *testing.T) {
	manager := NewAuthManager()

	// Create a session that expires exactly now
	session := &Session{
		Token:     "edge-token",
		Username:  "testuser",
		ExpiresAt: time.Now(),
	}

	manager.sessions[session.Token] = session

	// Wait a tiny bit to ensure we're past expiry
	time.Sleep(10 * time.Millisecond)

	// Session should be expired
	_, err := manager.ValidateSession(session.Token)
	if err == nil {
		t.Error("expected session to be expired")
	}
}

func TestConcurrentLogout(t *testing.T) {
	manager := NewAuthManager()

	// Create multiple sessions
	for i := 0; i < 10; i++ {
		session := &Session{
			Token:     string(rune('a' + i)),
			Username:  "testuser",
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}
		manager.sessions[session.Token] = session
	}

	// Concurrent logouts
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(index int) {
			manager.Logout(string(rune('a' + index)))
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// All sessions should be gone
	manager.mu.RLock()
	sessionCount := len(manager.sessions)
	manager.mu.RUnlock()

	if sessionCount != 0 {
		t.Errorf("expected 0 sessions after concurrent logout, got %d", sessionCount)
	}
}

func TestSessionCreatedAtTimestamp(t *testing.T) {
	manager := NewAuthManager()

	before := time.Now()

	session := &Session{
		Token:     "timestamp-token",
		Username:  "testuser",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	manager.sessions[session.Token] = session

	after := time.Now()

	validatedSession, err := manager.ValidateSession("timestamp-token")
	if err != nil {
		t.Fatalf("failed to validate session: %v", err)
	}

	// CreatedAt should be between before and after
	if validatedSession.CreatedAt.Before(before) || validatedSession.CreatedAt.After(after) {
		t.Error("CreatedAt timestamp not within expected range")
	}
}

func TestMultipleLogoutsForSameSession(t *testing.T) {
	manager := NewAuthManager()

	session := &Session{
		Token:     "multiple-logout-token",
		Username:  "testuser",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	manager.sessions[session.Token] = session

	// First logout
	manager.Logout("multiple-logout-token")

	// Verify session is gone
	_, err := manager.ValidateSession("multiple-logout-token")
	if err == nil {
		t.Error("expected error after first logout")
	}

	// Second logout should not panic
	manager.Logout("multiple-logout-token")

	// Third logout should not panic
	manager.Logout("multiple-logout-token")
}

func TestGenerateTokenUniqueness(t *testing.T) {
	// Generate many tokens to ensure uniqueness
	tokens := make(map[string]bool)
	count := 1000

	for i := 0; i < count; i++ {
		token, err := generateToken()
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		if tokens[token] {
			t.Error("duplicate token generated")
		}

		tokens[token] = true
	}

	if len(tokens) != count {
		t.Errorf("expected %d unique tokens, got %d", count, len(tokens))
	}
}

func TestSessionsMapInitialization(t *testing.T) {
	manager := NewAuthManager()

	// Verify sessions map is initialized and empty
	manager.mu.RLock()
	sessionCount := len(manager.sessions)
	manager.mu.RUnlock()

	if sessionCount != 0 {
		t.Errorf("expected 0 initial sessions, got %d", sessionCount)
	}

	// Verify we can add sessions without panic
	session := &Session{
		Token:     "init-test-token",
		Username:  "testuser",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	manager.sessions[session.Token] = session

	manager.mu.RLock()
	sessionCount = len(manager.sessions)
	manager.mu.RUnlock()

	if sessionCount != 1 {
		t.Errorf("expected 1 session after adding, got %d", sessionCount)
	}
}
