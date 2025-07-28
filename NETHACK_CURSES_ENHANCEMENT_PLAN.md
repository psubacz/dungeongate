# NetHack Curses Interface Enhancement Plan for DungeonGate

## Executive Summary

This document outlines a comprehensive plan to enhance the DungeonGate game service to fully support NetHack curses interface configuration options. The enhancement will build upon the existing solid foundation while adding advanced configuration management, user interface improvements, and runtime customization capabilities.

## Current State Analysis

### ✅ Existing Strengths
- Solid NetHack adapter with automatic directory creation
- Basic `.nethackrc` configuration with sensible defaults
- Auto-detection of NetHack system paths via `--showpaths`
- Per-user configuration directories and environment setup
- Comprehensive path management for saves, bones, levels, etc.
- Planned profile management system (PROFILE_MANAGEMENT_PLAN.md)

### ❌ Current Limitations
- Limited to TTY interface (`windowtype:tty`) only
- No support for curses interface features and customization
- No user interface for configuration editing
- Missing advanced display options (colors, graphics, layout)
- No runtime configuration modification capabilities
- Basic configuration options only (color, DECgraphics, basic gameplay)

## Enhancement Objectives

### Primary Goals
1. **Full Curses Interface Support**: Enable `windowtype:curses` with all advanced features
2. **Comprehensive Configuration Management**: Support all NetHack curses-specific options
3. **User-Friendly Interface**: Provide web UI and CLI tools for configuration editing
4. **Runtime Customization**: Allow dynamic configuration changes during gameplay
5. **Advanced Visual Options**: Support graphics modes, color schemes, and layout customization

### Secondary Goals
1. **Configuration Validation**: Ensure configuration correctness and compatibility
2. **Preset Management**: Provide curated configuration templates
3. **Migration Tools**: Easy upgrade from existing TTY configurations
4. **Documentation**: Comprehensive guides and examples
5. **Testing Framework**: Automated testing for configuration scenarios

## Technical Architecture Enhancement

### 1. Configuration Schema Extension

#### Enhanced Configuration Types (pkg/config/game_config.go)

```go
// NetHackCursesConfig extends existing NetHack configuration
type NetHackCursesConfig struct {
    // Interface Options
    WindowType      string            `yaml:"window_type" json:"window_type"`           // tty, curses, qt, x11
    CursesGraphics  bool              `yaml:"curses_graphics" json:"curses_graphics"`   // cursesgraphics option
    IBMGraphics     bool              `yaml:"ibm_graphics" json:"ibm_graphics"`         // IBMgraphics option
    
    // Visual Layout
    AlignMessage    string            `yaml:"align_message" json:"align_message"`       // top, bottom, left, right
    AlignStatus     string            `yaml:"align_status" json:"align_status"`         // top, bottom, left, right
    WindowBorders   int               `yaml:"window_borders" json:"window_borders"`     // 1=on, 2=off, 3=auto
    
    // Display Features
    PopupDialog     bool              `yaml:"popup_dialog" json:"popup_dialog"`         // popup_dialog option
    SplashScreen    bool              `yaml:"splash_screen" json:"splash_screen"`       // splash_screen option
    GuiColor        bool              `yaml:"gui_color" json:"gui_color"`               // guicolor option
    MouseSupport    bool              `yaml:"mouse_support" json:"mouse_support"`       // mouse_support option
    
    // Terminal Settings
    TerminalCols    int               `yaml:"terminal_cols" json:"terminal_cols"`       // term_cols option
    TerminalRows    int               `yaml:"terminal_rows" json:"terminal_rows"`       // term_rows option
    
    // Color Customization
    Colors          NetHackColorConfig `yaml:"colors" json:"colors"`
    MenuColors      []MenuColorRule    `yaml:"menu_colors" json:"menu_colors"`
    StatusHilites   []StatusHilite     `yaml:"status_hilites" json:"status_hilites"`
}

type NetHackColorConfig struct {
    Enabled         bool              `yaml:"enabled" json:"enabled"`
    BlackAndWhite   bool              `yaml:"black_and_white" json:"black_and_white"`
    CustomPalette   map[string]string `yaml:"custom_palette" json:"custom_palette"`
}

type MenuColorRule struct {
    Pattern         string            `yaml:"pattern" json:"pattern"`
    Color           string            `yaml:"color" json:"color"`
    Attribute       string            `yaml:"attribute" json:"attribute"`
}

type StatusHilite struct {
    Field           string            `yaml:"field" json:"field"`
    Threshold       interface{}       `yaml:"threshold" json:"threshold"`
    Color           string            `yaml:"color" json:"color"`
    Attribute       string            `yaml:"attribute" json:"attribute"`
}
```

#### Configuration Validation Framework

```go
// Enhanced validation for curses-specific options
type CursesConfigValidator struct {
    terminalCapabilities *TerminalCapabilities
    nethackVersion      string
}

func (v *CursesConfigValidator) ValidateConfig(config *NetHackCursesConfig) error {
    // Validate mutually exclusive options
    // Validate terminal capabilities
    // Validate color support
    // Validate layout constraints
}
```

### 2. Enhanced NetHack Adapter (internal/games/adapters/nethack_adapter.go)

#### Curses Interface Support

```go
// Enhanced adapter with curses interface support
func (a *NetHackAdapter) PrepareCursesEnvironment(ctx context.Context, req *PrepareEnvironmentRequest) error {
    // Set curses-specific environment variables
    env := map[string]string{
        "TERM":              a.detectOptimalTermType(req.Config.CursesConfig),
        "COLORTERM":         "truecolor", // Enable 24-bit color support
        "NETHACK_INTERFACE": "curses",
    }
    
    // Configure terminal capabilities
    if req.Config.CursesConfig.MouseSupport {
        env["NETHACK_MOUSE"] = "1"
    }
    
    return a.setEnvironmentVariables(env)
}

func (a *NetHackAdapter) GenerateAdvancedNethackrc(config *NetHackCursesConfig) (string, error) {
    // Generate comprehensive .nethackrc with curses options
    // Include validation and compatibility checks
    // Support configuration templates and presets
}
```

#### Terminal Detection and Optimization

```go
func (a *NetHackAdapter) DetectTerminalCapabilities() *TerminalCapabilities {
    // Detect color support (8-bit, 24-bit)
    // Detect mouse support
    // Detect terminal size constraints
    // Detect graphics character support
}
```

### 3. Enhanced gRPC API (api/proto/games/game_service_v2.proto)

#### New Configuration Management Methods

```protobuf
service GameService {
    // Existing methods...
    
    // Configuration management
    rpc GetGameConfiguration(GetGameConfigurationRequest) returns (GetGameConfigurationResponse);
    rpc UpdateGameConfiguration(UpdateGameConfigurationRequest) returns (UpdateGameConfigurationResponse);
    rpc ValidateGameConfiguration(ValidateGameConfigurationRequest) returns (ValidateGameConfigurationResponse);
    rpc ListConfigurationPresets(ListConfigurationPresetsRequest) returns (ListConfigurationPresetsResponse);
    
    // Runtime configuration
    rpc UpdateRuntimeConfiguration(UpdateRuntimeConfigurationRequest) returns (UpdateRuntimeConfigurationResponse);
    rpc GetConfigurationSchema(GetConfigurationSchemaRequest) returns (GetConfigurationSchemaResponse);
}

message NetHackCursesConfiguration {
    string window_type = 1;
    bool curses_graphics = 2;
    bool ibm_graphics = 3;
    string align_message = 4;
    string align_status = 5;
    int32 window_borders = 6;
    bool popup_dialog = 7;
    bool splash_screen = 8;
    bool gui_color = 9;
    bool mouse_support = 10;
    int32 terminal_cols = 11;
    int32 terminal_rows = 12;
    NetHackColorConfiguration colors = 13;
    repeated MenuColorRule menu_colors = 14;
    repeated StatusHilite status_hilites = 15;
}
```

### 4. Configuration Management Service

#### New Service Component (internal/games/application/config_service.go)

```go
type ConfigurationService struct {
    configRepo     repository.ConfigurationRepository
    validator      *CursesConfigValidator
    presetManager  *PresetManager
    logger         *slog.Logger
}

func (s *ConfigurationService) UpdateConfiguration(ctx context.Context, userID string, config *NetHackCursesConfig) error {
    // Validate configuration
    // Save to database
    // Update filesystem .nethackrc
    // Broadcast configuration change to active sessions
}

func (s *ConfigurationService) GetConfigurationPresets() ([]*ConfigurationPreset, error) {
    // Return curated configuration templates
    // Categories: Beginner, Advanced, Accessibility, Tournament
}
```

### 5. Web UI for Configuration Management

#### React Configuration Editor (web/src/components/games/NetHackConfigEditor.tsx)

```tsx
interface NetHackConfigEditorProps {
    userId: string;
    currentConfig: NetHackCursesConfig;
    onConfigUpdate: (config: NetHackCursesConfig) => void;
}

const NetHackConfigEditor: React.FC<NetHackConfigEditorProps> = ({
    userId,
    currentConfig,
    onConfigUpdate
}) => {
    // Interface configuration panel
    // Visual preview of layout options
    // Color picker for custom schemes
    // Real-time validation feedback
    // Preset selection and management
};
```

#### Configuration Categories

1. **Interface Settings**: Window type, graphics mode, layout
2. **Visual Appearance**: Colors, fonts, borders, graphics
3. **Interaction**: Mouse support, key bindings, popup behavior
4. **Advanced**: Terminal settings, custom options, debugging

## Implementation Roadmap

### Phase 1: Foundation (2-3 weeks)
**Goals**: Extend configuration infrastructure and basic curses support

#### Week 1: Configuration Schema
- [ ] Extend `NetHackConfig` types in `pkg/config/game_config.go`
- [ ] Add curses-specific configuration fields
- [ ] Implement configuration validation framework
- [ ] Update `configs/game-service.yaml` with curses options
- [ ] Create configuration migration utilities

#### Week 2: Enhanced NetHack Adapter
- [ ] Modify `internal/games/adapters/nethack_adapter.go` for curses support
- [ ] Implement terminal capability detection
- [ ] Add `.nethackrc` generation with curses options
- [ ] Create curses-specific environment setup
- [ ] Add configuration validation and error handling

#### Week 3: gRPC API Extension
- [ ] Extend `api/proto/games/game_service_v2.proto` with configuration methods
- [ ] Implement configuration management gRPC handlers
- [ ] Add configuration validation API endpoints
- [ ] Create preset management API
- [ ] Update generated Go code and documentation

### Phase 2: Core Features (3-4 weeks)
**Goals**: Implement configuration management and basic UI

#### Week 4: Configuration Service
- [ ] Create `internal/games/application/config_service.go`
- [ ] Implement database repository for configurations
- [ ] Add configuration caching and performance optimization
- [ ] Create preset management system
- [ ] Implement configuration change broadcasting

#### Week 5: Database Integration
- [ ] Design configuration database schema
- [ ] Implement migration scripts for configuration storage
- [ ] Add configuration repository with CRUD operations
- [ ] Implement configuration versioning and history
- [ ] Add backup and restore capabilities

#### Week 6: Web UI Foundation
- [ ] Create React configuration editor components
- [ ] Implement configuration form with validation
- [ ] Add real-time preview capabilities
- [ ] Create preset selection interface
- [ ] Implement configuration import/export

#### Week 7: Integration Testing
- [ ] Create comprehensive test suite for configuration system
- [ ] Test curses interface functionality with various configurations
- [ ] Validate configuration persistence and retrieval
- [ ] Test API endpoints and error handling
- [ ] Performance testing for configuration operations

### Phase 3: Advanced Features (3-4 weeks)
**Goals**: Runtime configuration, advanced UI, and optimization

#### Week 8: Runtime Configuration
- [ ] Implement live configuration updates during gameplay
- [ ] Add configuration change notification system
- [ ] Create configuration rollback capabilities
- [ ] Implement session-specific configuration overrides
- [ ] Add configuration conflict resolution

#### Week 9: Advanced UI Features
- [ ] Implement visual configuration preview
- [ ] Add color scheme editor with live preview
- [ ] Create layout designer for window positioning
- [ ] Implement configuration comparison tools
- [ ] Add accessibility features and keyboard navigation

#### Week 10: Color and Graphics Enhancement
- [ ] Implement comprehensive color customization
- [ ] Add support for custom color palettes
- [ ] Create menu color rule editor
- [ ] Implement status highlight configuration
- [ ] Add graphics mode selection and preview

#### Week 11: Polish and Optimization
- [ ] Optimize configuration loading and caching
- [ ] Implement configuration validation improvements
- [ ] Add comprehensive error messages and help text
- [ ] Create configuration migration and upgrade tools
- [ ] Performance optimization and memory usage improvements

### Phase 4: Production Readiness (2-3 weeks)
**Goals**: Documentation, testing, and deployment preparation

#### Week 12: Documentation and Guides
- [ ] Create comprehensive configuration documentation
- [ ] Write user guides for curses interface features
- [ ] Document API endpoints and integration examples
- [ ] Create troubleshooting guides
- [ ] Add inline help and tooltips

#### Week 13: Testing and Quality Assurance
- [ ] Comprehensive integration testing
- [ ] User acceptance testing with different configurations
- [ ] Performance testing under load
- [ ] Security testing for configuration management
- [ ] Browser compatibility testing for web UI

#### Week 14: Deployment and Monitoring
- [ ] Prepare production deployment configuration
- [ ] Implement monitoring and logging for configuration changes
- [ ] Create configuration backup and disaster recovery procedures
- [ ] Implement feature flags for gradual rollout
- [ ] Prepare rollback procedures

## Configuration Presets

### Beginner Preset
```yaml
window_type: "curses"
curses_graphics: true
align_message: "bottom"
align_status: "right"
window_borders: 3
popup_dialog: true
splash_screen: true
gui_color: true
mouse_support: true
terminal_cols: 100
terminal_rows: 30
colors:
  enabled: true
  black_and_white: false
```

### Advanced Player Preset
```yaml
window_type: "curses"
curses_graphics: true
align_message: "top"
align_status: "bottom"
window_borders: 1
popup_dialog: false
splash_screen: false
gui_color: true
mouse_support: false
terminal_cols: 120
terminal_rows: 40
colors:
  enabled: true
  custom_palette:
    white: "#FFFFFF"
    black: "#000000"
    red: "#FF6B6B"
    green: "#4ECDC4"
```

### Tournament/Competition Preset
```yaml
window_type: "curses"
curses_graphics: false
align_message: "bottom"
align_status: "bottom"
window_borders: 2
popup_dialog: false
splash_screen: false
gui_color: false
mouse_support: false
terminal_cols: 80
terminal_rows: 24
colors:
  enabled: false
  black_and_white: true
```

### Accessibility Preset
```yaml
window_type: "curses"
curses_graphics: false
align_message: "bottom"
align_status: "right"
window_borders: 1
popup_dialog: true
splash_screen: false
gui_color: true
mouse_support: true
terminal_cols: 100
terminal_rows: 30
colors:
  enabled: true
  custom_palette:
    # High contrast colors for better visibility
    white: "#FFFFFF"
    black: "#000000"
    red: "#FF0000"
    green: "#00FF00"
    blue: "#0000FF"
    yellow: "#FFFF00"
```

## Technical Considerations

### 1. Backward Compatibility
- Maintain support for existing TTY interface configurations
- Provide automatic migration from TTY to curses configurations
- Ensure existing sessions continue to work during upgrade
- Support graceful fallback when curses interface is unavailable

### 2. Performance Optimization
- Cache configuration objects to minimize database queries
- Implement efficient configuration change propagation
- Optimize `.nethackrc` generation and file I/O operations
- Use connection pooling for configuration API calls

### 3. Security Considerations
- Validate all configuration inputs to prevent injection attacks
- Implement proper authorization for configuration changes
- Sanitize user-provided configuration values
- Audit configuration changes for compliance

### 4. Monitoring and Observability
- Add metrics for configuration usage patterns
- Monitor configuration change frequency and errors
- Track curses interface adoption rates
- Log configuration validation failures for debugging

### 5. Testing Strategy
- Unit tests for all configuration validation logic
- Integration tests for configuration persistence
- End-to-end tests for curses interface functionality
- Performance tests for configuration operations under load
- Browser automation tests for web UI components

## Success Metrics

### Functional Metrics
- [ ] 100% of NetHack curses interface options supported
- [ ] Configuration validation catches 95%+ invalid configurations
- [ ] Configuration changes take effect within 1 second
- [ ] Web UI loads and responds within 500ms
- [ ] Zero data loss during configuration updates

### User Experience Metrics
- [ ] 90%+ user satisfaction with configuration interface
- [ ] Average configuration setup time under 2 minutes
- [ ] 50%+ adoption rate of curses interface over TTY
- [ ] Reduced support tickets related to configuration issues

### Technical Metrics
- [ ] Configuration API response time under 100ms
- [ ] 99.9% uptime for configuration services
- [ ] Memory usage increase under 5% for enhanced features
- [ ] Test coverage above 90% for all configuration code

## Risk Mitigation

### Technical Risks
- **Terminal Compatibility**: Extensive testing across different terminal types
- **Performance Impact**: Careful optimization and caching strategies
- **Configuration Corruption**: Robust validation and backup systems
- **Integration Complexity**: Phased rollout with feature flags

### User Experience Risks
- **Learning Curve**: Comprehensive documentation and guided setup
- **Migration Issues**: Automated migration tools with manual fallback
- **Interface Confusion**: Clear labeling and help documentation
- **Configuration Overload**: Curated presets and simplified modes

### Operational Risks
- **Deployment Complexity**: Thorough testing and rollback procedures
- **Data Migration**: Careful planning and testing of database migrations
- **Service Dependencies**: Graceful degradation when services are unavailable
- **Scalability Concerns**: Load testing and performance monitoring

## Conclusion

This enhancement plan provides a comprehensive roadmap for transforming DungeonGate's NetHack support from basic TTY interface to a full-featured curses interface with advanced configuration management. The phased approach ensures manageable development cycles while building upon the existing solid foundation.

The implementation will significantly improve user experience, provide extensive customization options, and position DungeonGate as a premier platform for NetHack gaming with modern interface capabilities.

**Total Estimated Timeline**: 12-14 weeks
**Resource Requirements**: 2-3 developers (backend, frontend, DevOps)
**Risk Level**: Medium (well-defined scope, existing foundation)
**Business Impact**: High (significant user experience improvement, competitive advantage)