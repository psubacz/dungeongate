package menu

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/dungeongate/internal/session/banner"
	authv1 "github.com/dungeongate/pkg/api/auth/v1"
	gamev2 "github.com/dungeongate/pkg/api/games/v2"
)

// MockSSHChannel implements a minimal ssh.Channel for testing
type MockSSHChannel struct {
	mock.Mock
	readBuffer  []byte
	writeBuffer []byte
	readIndex   int
}

func (m *MockSSHChannel) Read(data []byte) (int, error) {
	// If we have preset read data, use it
	if m.readBuffer != nil && m.readIndex < len(m.readBuffer) {
		n := copy(data, m.readBuffer[m.readIndex:])
		m.readIndex += n
		return n, nil
	}

	args := m.Called(data)
	return args.Int(0), args.Error(1)
}

func (m *MockSSHChannel) Write(data []byte) (int, error) {
	if m.writeBuffer != nil {
		m.writeBuffer = append(m.writeBuffer, data...)
	}
	args := m.Called(data)
	return args.Int(0), args.Error(1)
}

func (m *MockSSHChannel) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockSSHChannel) CloseWrite() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockSSHChannel) SendRequest(name string, wantReply bool, payload []byte) (bool, error) {
	args := m.Called(name, wantReply, payload)
	return args.Bool(0), args.Error(1)
}

func (m *MockSSHChannel) Stderr() io.ReadWriter {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(io.ReadWriter)
}

// SetReadData sets the data that will be returned by Read() calls
func (m *MockSSHChannel) SetReadData(data string) {
	m.readBuffer = []byte(data)
	m.readIndex = 0
}

// GetWrittenData returns all data written to the channel
func (m *MockSSHChannel) GetWrittenData() string {
	if m.writeBuffer == nil {
		return ""
	}
	return string(m.writeBuffer)
}

// ClearWriteBuffer clears the write buffer
func (m *MockSSHChannel) ClearWriteBuffer() {
	m.writeBuffer = nil
}

// EnableWriteTracking initializes the write buffer to track written data
func (m *MockSSHChannel) EnableWriteTracking() {
	m.writeBuffer = make([]byte, 0)
}

func TestNewMenuHandler(t *testing.T) {
	// Create a real banner manager for testing
	bannerConfig := &banner.BannerConfig{}
	bannerManager := banner.NewBannerManager(bannerConfig, "test-version")

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Test with nil clients to verify constructor
	handler := NewMenuHandler(bannerManager, nil, nil, logger)

	assert.NotNil(t, handler)
	assert.Equal(t, bannerManager, handler.bannerManager)
	assert.Equal(t, logger, handler.logger)
}

func TestShowAnonymousMenu_Success(t *testing.T) {
	// Create a temporary banner file for testing
	tempFile, err := os.CreateTemp("", "test_banner_*.txt")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	bannerContent := `Welcome to DungeonGate
=====================

Choose an option:
[L]ogin
[R]egister  
[W]atch games
[G]uest access
[Q]uit
`
	_, err = tempFile.WriteString(bannerContent)
	require.NoError(t, err)
	tempFile.Close()

	// Use real banner manager with test banner
	bannerConfig := &banner.BannerConfig{
		MainAnon: tempFile.Name(),
	}
	bannerManager := banner.NewBannerManager(bannerConfig, "test-version")
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := NewMenuHandler(bannerManager, nil, nil, logger)

	// Setup mock channel
	channel := &MockSSHChannel{}
	channel.EnableWriteTracking()
	channel.SetReadData("l") // User selects login
	channel.On("Write", mock.AnythingOfType("[]uint8")).Return(len("test"), nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	choice, err := handler.ShowAnonymousMenu(ctx, channel, "testuser")

	assert.NoError(t, err)
	assert.NotNil(t, choice)
	assert.Equal(t, "login", choice.Action)
	assert.Equal(t, "", choice.Value)

	// Verify banner was written
	writtenData := channel.GetWrittenData()
	assert.Contains(t, writtenData, "DungeonGate")
	assert.Contains(t, writtenData, "[L]ogin")

	channel.AssertExpectations(t)
}

func TestShowAnonymousMenu_AllChoices(t *testing.T) {
	testCases := []struct {
		input          string
		expectedAction string
	}{
		{"l", "login"},
		{"L", "login"},
		{"r", "register"},
		{"R", "register"},
		{"w", "watch"},
		{"W", "watch"},
		{"q", "quit"},
		{"Q", "quit"},
	}

	for _, tc := range testCases {
		t.Run("input_"+tc.input, func(t *testing.T) {
			// Create a temporary banner file for testing
			tempFile, err := os.CreateTemp("", "test_banner_*.txt")
			require.NoError(t, err)
			defer os.Remove(tempFile.Name())

			bannerContent := `Welcome to DungeonGate
=====================

Choose an option:
[L]ogin
[R]egister  
[W]atch games
[G]uest access
[Q]uit
`
			_, err = tempFile.WriteString(bannerContent)
			require.NoError(t, err)
			tempFile.Close()

			bannerConfig := &banner.BannerConfig{
				MainAnon: tempFile.Name(),
			}
			bannerManager := banner.NewBannerManager(bannerConfig, "test-version")
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

			handler := NewMenuHandler(bannerManager, nil, nil, logger)

			channel := &MockSSHChannel{}
			channel.EnableWriteTracking()
			channel.SetReadData(tc.input)
			channel.On("Write", mock.AnythingOfType("[]uint8")).Return(len("test"), nil)

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			choice, err := handler.ShowAnonymousMenu(ctx, channel, "testuser")

			assert.NoError(t, err)
			assert.NotNil(t, choice)
			assert.Equal(t, tc.expectedAction, choice.Action)
		})
	}
}

func TestShowAnonymousMenu_InvalidChoice(t *testing.T) {
	// Create a temporary banner file for testing
	tempFile, err := os.CreateTemp("", "test_banner_*.txt")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	bannerContent := `Welcome to DungeonGate
=====================

Choose an option:
[L]ogin
[R]egister  
[W]atch games
[G]uest access
[Q]uit
`
	_, err = tempFile.WriteString(bannerContent)
	require.NoError(t, err)
	tempFile.Close()

	bannerConfig := &banner.BannerConfig{
		MainAnon: tempFile.Name(),
	}
	bannerManager := banner.NewBannerManager(bannerConfig, "test-version")
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := NewMenuHandler(bannerManager, nil, nil, logger)

	channel := &MockSSHChannel{}
	// Set up multiple reads: first invalid, then continue reading until timeout
	channel.SetReadData("x") // Invalid choice
	channel.On("Write", mock.AnythingOfType("[]uint8")).Return(len("test"), nil)
	// Allow multiple Read calls for re-prompting after invalid choice
	channel.On("Read", mock.AnythingOfType("[]uint8")).Return(0, io.EOF).Maybe()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// This should timeout because invalid choice causes redisplay
	choice, err := handler.ShowAnonymousMenu(ctx, channel, "testuser")

	// Should timeout or return context error
	assert.Error(t, err)
	assert.Nil(t, choice)
}

func TestShowUserMenu_Success(t *testing.T) {
	bannerConfig := &banner.BannerConfig{
		MainUser: "/Users/caboose/dungeongate/assets/banners/main_user.txt",
	}
	bannerManager := banner.NewBannerManager(bannerConfig, "test-version")
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := NewMenuHandler(bannerManager, nil, nil, logger)

	channel := &MockSSHChannel{}
	channel.EnableWriteTracking()
	channel.SetReadData("p") // User selects play
	channel.On("Write", mock.AnythingOfType("[]uint8")).Return(len("test"), nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	choice, err := handler.ShowUserMenu(ctx, channel, "testuser")

	assert.NoError(t, err)
	assert.NotNil(t, choice)
	assert.Equal(t, "play", choice.Action)
	assert.Equal(t, "", choice.Value)

	// Verify user banner was written
	writtenData := channel.GetWrittenData()
	assert.Contains(t, writtenData, "testuser")
	assert.Contains(t, writtenData, "Play")

	channel.AssertExpectations(t)
}

func TestShowUserMenu_AllChoices(t *testing.T) {
	testCases := []struct {
		input          string
		expectedAction string
	}{
		{"p", "play"},
		{"P", "play"},
		{"w", "watch"},
		{"W", "watch"},
		{"e", "edit_profile"},
		{"E", "edit_profile"},
		{"r", "view_recordings"},
		{"R", "view_recordings"},
		{"s", "statistics"},
		{"S", "statistics"},
		{"q", "quit"},
		{"Q", "quit"},
	}

	for _, tc := range testCases {
		t.Run("input_"+tc.input, func(t *testing.T) {
			bannerConfig := &banner.BannerConfig{
				MainUser: "/Users/caboose/dungeongate/assets/banners/main_user.txt",
			}
			bannerManager := banner.NewBannerManager(bannerConfig, "test-version")
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

			handler := NewMenuHandler(bannerManager, nil, nil, logger)

			channel := &MockSSHChannel{}
			channel.EnableWriteTracking()
			channel.SetReadData(tc.input)
			channel.On("Write", mock.AnythingOfType("[]uint8")).Return(len("test"), nil)

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			choice, err := handler.ShowUserMenu(ctx, channel, "testuser")

			assert.NoError(t, err)
			assert.NotNil(t, choice)
			assert.Equal(t, tc.expectedAction, choice.Action)
		})
	}
}

func TestParseGameChoice(t *testing.T) {
	testCases := []struct {
		input     string
		maxGames  int
		expected  int
		shouldErr bool
	}{
		{"1", 3, 0, false},   // Valid first choice
		{"2", 3, 1, false},   // Valid second choice
		{"3", 3, 2, false},   // Valid third choice
		{"0", 3, -1, true},   // Invalid - too low
		{"4", 3, -1, true},   // Invalid - too high
		{"abc", 3, -1, true}, // Invalid - not a number
		{"", 3, -1, true},    // Invalid - empty
		{"1.5", 3, -1, true}, // Invalid - decimal
		{"-1", 3, -1, true},  // Invalid - negative
	}

	for _, tc := range testCases {
		t.Run("input_"+tc.input, func(t *testing.T) {
			result, err := parseGameChoice(tc.input, tc.maxGames)

			if tc.shouldErr {
				assert.Error(t, err)
				assert.Equal(t, -1, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestBuildGameSelectionBanner(t *testing.T) {
	bannerConfig := &banner.BannerConfig{
		MainUser: "/Users/caboose/dungeongate/assets/banners/main_user.txt",
	}
	bannerManager := banner.NewBannerManager(bannerConfig, "test-version")
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := NewMenuHandler(bannerManager, nil, nil, logger)

	games := []*gamev2.Game{
		{
			Id:          "nethack",
			Name:        "NetHack",
			Description: "The classic roguelike adventure",
			Version:     "3.7.0",
			Status:      gamev2.GameStatus_GAME_STATUS_ENABLED,
		},
		{
			Id:          "dcss",
			Name:        "Dungeon Crawl Stone Soup",
			Description: "", // Test empty description
			Version:     "", // Test empty version
			Status:      gamev2.GameStatus_GAME_STATUS_UNSPECIFIED,
		},
	}

	banner := handler.buildGameSelectionBanner(games, "testuser")

	// Verify banner contains expected elements
	assert.Contains(t, banner, "Game Selection")
	assert.Contains(t, banner, "testuser")
	assert.Contains(t, banner, "[1] NetHack")
	assert.Contains(t, banner, "[2] Dungeon Crawl Stone Soup")
	assert.Contains(t, banner, "The classic roguelike adventure")
	assert.Contains(t, banner, "Version: 3.7.0")
	assert.Contains(t, banner, "[q] Return to main menu")
	assert.Contains(t, banner, "Enter your choice:")

	// Make sure empty fields don't break the banner
	assert.NotContains(t, banner, "Description: \r\n")
	assert.NotContains(t, banner, "Version: \r\n")
}

// Benchmark tests
func BenchmarkShowAnonymousMenu(b *testing.B) {
	bannerConfig := &banner.BannerConfig{}
	bannerManager := banner.NewBannerManager(bannerConfig, "test-version")
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewMenuHandler(bannerManager, nil, nil, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		channel := &MockSSHChannel{}
		channel.SetReadData("q")
		channel.On("Write", mock.AnythingOfType("[]uint8")).Return(len("test"), nil)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		handler.ShowAnonymousMenu(ctx, channel, "user")
		cancel()
	}
}

func TestBuildSpectateMenuBanner(t *testing.T) {
	bannerConfig := &banner.BannerConfig{}
	bannerManager := banner.NewBannerManager(bannerConfig, "test-version")
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := NewMenuHandler(bannerManager, nil, nil, logger)

	// Create mock sessions for testing
	now := time.Now()
	sessions := []*gamev2.GameSession{
		{
			Id:           "session1",
			Username:     "dorkfish",
			GameId:       "nethack",
			TerminalSize: &gamev2.TerminalSize{Width: 120, Height: 30},
			StartTime:    timestamppb.New(now.Add(-10 * time.Minute)),
			LastActivity: timestamppb.New(now.Add(-5 * time.Second)),
			Spectators:   []*gamev2.SpectatorInfo{},
		},
		{
			Id:           "session2",
			Username:     "GhostTown",
			GameId:       "nethack",
			TerminalSize: &gamev2.TerminalSize{Width: 209, Height: 46},
			StartTime:    timestamppb.New(now.Add(-40 * time.Minute)),
			LastActivity: timestamppb.New(now.Add(-4*time.Minute - 16*time.Second)),
			Spectators:   []*gamev2.SpectatorInfo{},
		},
	}

	banner := handler.buildSpectateMenuBanner(sessions)

	// Verify banner contains expected elements
	assert.Contains(t, banner, "The following games are in progress:")
	assert.Contains(t, banner, "Username         Game    Size    Start date & time    Idle time   Watchers")
	assert.Contains(t, banner, "a) dorkfish")
	assert.Contains(t, banner, "NH370")
	assert.Contains(t, banner, "120x30")
	assert.Contains(t, banner, "b) GhostTown")
	assert.Contains(t, banner, "209x46")
	assert.Contains(t, banner, "4m 16s") // Idle time for second session
	assert.Contains(t, banner, "(1-2 of 2)")
	assert.Contains(t, banner, "Spectate which game? ('?' for help) =>")
}

func TestFormatGameDisplay(t *testing.T) {
	bannerConfig := &banner.BannerConfig{}
	bannerManager := banner.NewBannerManager(bannerConfig, "test-version")
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := NewMenuHandler(bannerManager, nil, nil, logger)

	testCases := []struct {
		gameID   string
		expected string
	}{
		{"nethack", "NH370"},
		{"NETHACK", "NH370"},
		{"dcss", "DCSS"},
		{"crawl", "DCSS"},
		{"angband", "ANG"},
		{"tome", "TOME"},
		{"unknown", "UNKNO"},
		{"long_game_name", "LONG_"},
		{"short", "SHORT"},
	}

	for _, tc := range testCases {
		t.Run("gameID_"+tc.gameID, func(t *testing.T) {
			result := handler.formatGameDisplay(tc.gameID)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestCalculateIdleTime(t *testing.T) {
	bannerConfig := &banner.BannerConfig{}
	bannerManager := banner.NewBannerManager(bannerConfig, "test-version")
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := NewMenuHandler(bannerManager, nil, nil, logger)

	now := time.Now()
	testCases := []struct {
		name         string
		lastActivity *timestamppb.Timestamp
		expected     string
	}{
		{"no_activity", nil, ""},
		{"recent_activity", timestamppb.New(now.Add(-10 * time.Second)), ""},
		{"one_minute", timestamppb.New(now.Add(-1 * time.Minute)), "1m"},
		{"one_minute_30_seconds", timestamppb.New(now.Add(-90 * time.Second)), "1m 30s"},
		{"five_minutes", timestamppb.New(now.Add(-5 * time.Minute)), "5m"},
		{"one_hour", timestamppb.New(now.Add(-1 * time.Hour)), "1h"},
		{"one_hour_30_minutes", timestamppb.New(now.Add(-90 * time.Minute)), "1h 30m"},
		{"45_seconds", timestamppb.New(now.Add(-45 * time.Second)), "45s"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			session := &gamev2.GameSession{
				LastActivity: tc.lastActivity,
			}
			result := handler.calculateIdleTime(session)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestHasIdleTimeUpdates(t *testing.T) {
	bannerConfig := &banner.BannerConfig{}
	bannerManager := banner.NewBannerManager(bannerConfig, "test-version")
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := NewMenuHandler(bannerManager, nil, nil, logger)

	now := time.Now()
	testCases := []struct {
		name     string
		sessions []*gamev2.GameSession
		expected bool
	}{
		{
			name:     "no_sessions",
			sessions: []*gamev2.GameSession{},
			expected: false,
		},
		{
			name: "recent_activity_only",
			sessions: []*gamev2.GameSession{
				{LastActivity: timestamppb.New(now.Add(-10 * time.Second))},
			},
			expected: false,
		},
		{
			name: "has_idle_time",
			sessions: []*gamev2.GameSession{
				{LastActivity: timestamppb.New(now.Add(-2 * time.Minute))},
			},
			expected: true,
		},
		{
			name: "mixed_activity",
			sessions: []*gamev2.GameSession{
				{LastActivity: timestamppb.New(now.Add(-10 * time.Second))},
				{LastActivity: timestamppb.New(now.Add(-3 * time.Minute))},
			},
			expected: true,
		},
		{
			name: "nil_activity",
			sessions: []*gamev2.GameSession{
				{LastActivity: nil},
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := handler.hasIdleTimeUpdates(tc.sessions)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFilterUserSessions(t *testing.T) {
	bannerConfig := &banner.BannerConfig{}
	bannerManager := banner.NewBannerManager(bannerConfig, "test-version")
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := NewMenuHandler(bannerManager, nil, nil, logger)

	sessions := []*gamev2.GameSession{
		{Id: "session1", UserId: 1, Username: "user1"},
		{Id: "session2", UserId: 2, Username: "user2"},
		{Id: "session3", UserId: 1, Username: "user1"},
		{Id: "session4", UserId: 3, Username: "user3"},
	}

	// Test with anonymous user (should return all sessions)
	result := handler.filterUserSessions(sessions, nil)
	assert.Len(t, result, 4)

	// Test with authenticated user (should filter out their own sessions)
	user := &authv1.User{Id: "1", Username: "user1"}
	result = handler.filterUserSessions(sessions, user)
	assert.Len(t, result, 2)
	assert.Equal(t, "session2", result[0].Id)
	assert.Equal(t, "session4", result[1].Id)

	// Test with user who has no sessions
	user = &authv1.User{Id: "99", Username: "user99"}
	result = handler.filterUserSessions(sessions, user)
	assert.Len(t, result, 4)
}

func TestBuildSpectateMenuBannerWithManySessions(t *testing.T) {
	bannerConfig := &banner.BannerConfig{}
	bannerManager := banner.NewBannerManager(bannerConfig, "test-version")
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := NewMenuHandler(bannerManager, nil, nil, logger)

	// Create 30 sessions to test a-z and A-Z lettering
	sessions := make([]*gamev2.GameSession, 30)
	now := time.Now()
	for i := 0; i < 30; i++ {
		sessions[i] = &gamev2.GameSession{
			Id:           fmt.Sprintf("session%d", i+1),
			Username:     fmt.Sprintf("user%d", i+1),
			GameId:       "nethack",
			TerminalSize: &gamev2.TerminalSize{Width: 80, Height: 24},
			StartTime:    timestamppb.New(now.Add(-10 * time.Minute)),
			LastActivity: timestamppb.New(now.Add(-5 * time.Second)),
			Spectators:   []*gamev2.SpectatorInfo{},
		}
	}

	banner := handler.buildSpectateMenuBanner(sessions)

	// Verify it uses a-z for first 26 sessions
	assert.Contains(t, banner, "a) user1")
	assert.Contains(t, banner, "z) user26")

	// Verify it uses A-Z for sessions 27+
	assert.Contains(t, banner, "A) user27")
	assert.Contains(t, banner, "D) user30")

	// Verify pagination info
	assert.Contains(t, banner, "(1-30 of 30)")
}

func BenchmarkBuildGameSelectionBanner(b *testing.B) {
	bannerConfig := &banner.BannerConfig{}
	bannerManager := banner.NewBannerManager(bannerConfig, "test-version")
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewMenuHandler(bannerManager, nil, nil, logger)

	games := []*gamev2.Game{
		{Id: "nethack", Name: "NetHack", Description: "Classic roguelike", Version: "3.7.0"},
		{Id: "dcss", Name: "DCSS", Description: "Modern roguelike", Version: "0.31"},
		{Id: "angband", Name: "Angband", Description: "Original roguelike", Version: "4.2"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.buildGameSelectionBanner(games, "testuser")
	}
}
