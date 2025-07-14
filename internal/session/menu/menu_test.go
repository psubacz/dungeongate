package menu

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dungeongate/internal/session/banner"
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
	bannerManager := banner.NewBannerManager(bannerConfig)

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
	bannerManager := banner.NewBannerManager(bannerConfig)
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
			bannerManager := banner.NewBannerManager(bannerConfig)
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
	bannerManager := banner.NewBannerManager(bannerConfig)
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
	bannerManager := banner.NewBannerManager(bannerConfig)
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
			bannerManager := banner.NewBannerManager(bannerConfig)
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
	bannerManager := banner.NewBannerManager(bannerConfig)
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
	bannerManager := banner.NewBannerManager(bannerConfig)
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

func BenchmarkBuildGameSelectionBanner(b *testing.B) {
	bannerConfig := &banner.BannerConfig{}
	bannerManager := banner.NewBannerManager(bannerConfig)
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
