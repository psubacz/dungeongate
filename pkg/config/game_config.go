package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// GameServiceConfig represents the game service configuration
type GameServiceConfig struct {
	Version     string              `yaml:"version"`
	InheritFrom string              `yaml:"inherit_from,omitempty"`
	Server      *ServerConfig       `yaml:"server"`
	Database    *DatabaseConfig     `yaml:"database"`
	GameEngine  *GameEngineConfig   `yaml:"game_engine"`
	Games       []*GameConfig       `yaml:"games"`
	Kubernetes  *KubernetesConfig   `yaml:"kubernetes"`
	Storage     *GameStorageConfig  `yaml:"storage"`
	Logging     *LoggingConfig      `yaml:"logging"`
	Metrics     *MetricsConfig      `yaml:"metrics"`
	Health      *HealthConfig       `yaml:"health"`
	Security    *GameSecurityConfig `yaml:"security"`
}

// GameEngineConfig represents game engine configuration
type GameEngineConfig struct {
	Mode             string                  `yaml:"mode"` // "container", "process", "hybrid"
	ProcessPool      *ProcessPoolConfig      `yaml:"process_pool"`
	ContainerRuntime *ContainerRuntimeConfig `yaml:"container_runtime"`
	Isolation        *IsolationConfig        `yaml:"isolation"`
	Monitoring       *GameMonitoringConfig   `yaml:"monitoring"`
	Chroot           *ChrootConfig           `yaml:"chroot"`
	Resources        *ResourcesConfig        `yaml:"resources"`
}

// GameConfig represents a specific game configuration
type GameConfig struct {
	ID          string              `yaml:"id"`
	Name        string              `yaml:"name"`
	ShortName   string              `yaml:"short_name"`
	Version     string              `yaml:"version"`
	Enabled     bool                `yaml:"enabled"`
	Binary      *BinaryConfig       `yaml:"binary"`
	Files       *FilesConfig        `yaml:"files"`
	Paths       *GamePathsConfig    `yaml:"paths"`
	Setup       *GameSetupOptions   `yaml:"setup"`
	Cleanup     *GameCleanupOptions `yaml:"cleanup"`
	Settings    *GameSettings       `yaml:"settings"`
	Environment map[string]string   `yaml:"environment"`
	Resources   *ResourcesConfig    `yaml:"resources"`
	Container   *ContainerConfig    `yaml:"container"`
	Networking  *NetworkingConfig   `yaml:"networking"`
}

// BinaryConfig represents binary configuration
type BinaryConfig struct {
	Path             string   `yaml:"path"`
	Args             []string `yaml:"args"`
	WorkingDirectory string   `yaml:"working_directory"`
	User             string   `yaml:"user"`
	Group            string   `yaml:"group"`
	Permissions      string   `yaml:"permissions"`
}

// FilesConfig represents files configuration
type FilesConfig struct {
	DataDirectory   string             `yaml:"data_directory"`
	SaveDirectory   string             `yaml:"save_directory"`
	ConfigDirectory string             `yaml:"config_directory"`
	LogDirectory    string             `yaml:"log_directory"`
	TempDirectory   string             `yaml:"temp_directory"`
	SharedFiles     []string           `yaml:"shared_files"`
	UserFiles       []string           `yaml:"user_files"`
	Permissions     *PermissionsConfig `yaml:"permissions"`
}

// PermissionsConfig represents file permissions
type PermissionsConfig struct {
	DataDirectory string `yaml:"data_directory"`
	SaveDirectory string `yaml:"save_directory"`
	UserFiles     string `yaml:"user_files"`
	LogFiles      string `yaml:"log_files"`
}

// GamePathsConfig represents game-specific path configuration
type GamePathsConfig struct {
	AutoDetect bool               `yaml:"auto_detect"`
	System     *SystemPathsConfig `yaml:"system"`
	User       *UserPathsConfig   `yaml:"user"`
}

// SystemPathsConfig represents system-level paths (from nethack --showpaths)
type SystemPathsConfig struct {
	ScoreDir    string `yaml:"score_dir"`
	SysConfFile string `yaml:"sysconf_file"`
	SymbolsFile string `yaml:"symbols_file"`
	DataFile    string `yaml:"data_file"`
}

// UserPathsConfig represents user-specific paths (relative to user directory)
type UserPathsConfig struct {
	BaseDir    string `yaml:"base_dir"`
	SaveDir    string `yaml:"save_dir"`
	ConfigDir  string `yaml:"config_dir"`
	BonesDir   string `yaml:"bones_dir"`
	LevelDir   string `yaml:"level_dir"`
	LockDir    string `yaml:"lock_dir"`
	TroubleDir string `yaml:"trouble_dir"`
}

// GameSetupOptions represents game setup configuration
type GameSetupOptions struct {
	CreateUserDirs    bool `yaml:"create_user_dirs"`
	CopyDefaultConfig bool `yaml:"copy_default_config"`
	InitializeShared  bool `yaml:"initialize_shared"`
	ValidatePaths     bool `yaml:"validate_paths"`
	SetPermissions    bool `yaml:"set_permissions"`
	DetectSystemPaths bool `yaml:"detect_system_paths"`
	CreateSaveLinks   bool `yaml:"create_save_links"`
}

// GameCleanupOptions represents game cleanup configuration
type GameCleanupOptions struct {
	RemoveUserDirs     bool `yaml:"remove_user_dirs"`
	ClearTempFiles     bool `yaml:"clear_temp_files"`
	RemoveLockFiles    bool `yaml:"remove_lock_files"`
	ClearPersonalBones bool `yaml:"clear_personal_bones"`
	PreserveConfig     bool `yaml:"preserve_config"`
	BackupSaves        bool `yaml:"backup_saves"`
	CleanupSaveLinks   bool `yaml:"cleanup_save_links"`
	ValidateCleanup    bool `yaml:"validate_cleanup"`
}

// GameSettings represents game-specific settings
type GameSettings struct {
	MaxPlayers         int               `yaml:"max_players"`
	MaxSessionDuration string            `yaml:"max_session_duration"`
	IdleTimeout        string            `yaml:"idle_timeout"`
	SaveInterval       string            `yaml:"save_interval"`
	AutoSave           bool              `yaml:"auto_save"`
	Spectating         *SpectatingConfig `yaml:"spectating"`
	Recording          *RecordingConfig  `yaml:"recording"`
	Options            map[string]string `yaml:"options"`
}

// RecordingConfig represents recording configuration
type RecordingConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Format        string `yaml:"format"`
	Compression   string `yaml:"compression"`
	MaxFileSize   string `yaml:"max_file_size"`
	RetentionDays int    `yaml:"retention_days"`
	AutoCleanup   bool   `yaml:"auto_cleanup"`
}

// ContainerConfig represents container configuration
type ContainerConfig struct {
	Image           string                 `yaml:"image"`
	Tag             string                 `yaml:"tag"`
	Registry        string                 `yaml:"registry"`
	PullPolicy      string                 `yaml:"pull_policy"`
	Resources       *ResourcesConfig       `yaml:"resources"`
	Volumes         []*VolumeConfig        `yaml:"volumes"`
	Environment     map[string]string      `yaml:"environment"`
	SecurityContext *SecurityContextConfig `yaml:"security_context"`
	NetworkMode     string                 `yaml:"network_mode"`
}

// VolumeConfig represents volume configuration
type VolumeConfig struct {
	Name       string `yaml:"name"`
	HostPath   string `yaml:"host_path"`
	MountPath  string `yaml:"mount_path"`
	ReadOnly   bool   `yaml:"read_only"`
	VolumeType string `yaml:"volume_type"`
}

// SecurityContextConfig represents security context
type SecurityContextConfig struct {
	RunAsUser              int  `yaml:"run_as_user"`
	RunAsGroup             int  `yaml:"run_as_group"`
	FSGroup                int  `yaml:"fs_group"`
	Privileged             bool `yaml:"privileged"`
	ReadOnlyRootFilesystem bool `yaml:"read_only_root_filesystem"`
}

// NetworkingConfig represents networking configuration
type NetworkingConfig struct {
	Mode           string        `yaml:"mode"`
	Ports          []*PortConfig `yaml:"ports"`
	ExposedPorts   []string      `yaml:"exposed_ports"`
	NetworkAliases []string      `yaml:"network_aliases"`
	DNSConfig      *DNSConfig    `yaml:"dns_config"`
}

// PortConfig represents port configuration
type PortConfig struct {
	ContainerPort int    `yaml:"container_port"`
	HostPort      int    `yaml:"host_port"`
	Protocol      string `yaml:"protocol"`
}

// DNSConfig represents DNS configuration
type DNSConfig struct {
	Nameservers []string `yaml:"nameservers"`
	Search      []string `yaml:"search"`
	Options     []string `yaml:"options"`
}

// KubernetesConfig represents Kubernetes configuration
type KubernetesConfig struct {
	Enabled          bool               `yaml:"enabled"`
	Namespace        string             `yaml:"namespace"`
	ServiceAccount   string             `yaml:"service_account"`
	ConfigMapName    string             `yaml:"config_map_name"`
	PodTemplate      *PodTemplateConfig `yaml:"pod_template"`
	Service          *ServiceConfig     `yaml:"service"`
	Ingress          *IngressConfig     `yaml:"ingress"`
	StorageClass     string             `yaml:"storage_class"`
	PersistentVolume *PVConfig          `yaml:"persistent_volume"`
}

// PodTemplateConfig represents pod template configuration
type PodTemplateConfig struct {
	Labels       map[string]string   `yaml:"labels"`
	Annotations  map[string]string   `yaml:"annotations"`
	NodeSelector map[string]string   `yaml:"node_selector"`
	Tolerations  []*TolerationConfig `yaml:"tolerations"`
	Affinity     *AffinityConfig     `yaml:"affinity"`
}

// TolerationConfig represents toleration configuration
type TolerationConfig struct {
	Key      string `yaml:"key"`
	Operator string `yaml:"operator"`
	Value    string `yaml:"value"`
	Effect   string `yaml:"effect"`
}

// AffinityConfig represents affinity configuration
type AffinityConfig struct {
	NodeAffinity *NodeAffinityConfig `yaml:"node_affinity"`
	PodAffinity  *PodAffinityConfig  `yaml:"pod_affinity"`
}

// NodeAffinityConfig represents node affinity configuration
type NodeAffinityConfig struct {
	RequiredDuringSchedulingIgnoredDuringExecution  *NodeSelectorConfig    `yaml:"required_during_scheduling_ignored_during_execution"`
	PreferredDuringSchedulingIgnoredDuringExecution []*PreferredScheduling `yaml:"preferred_during_scheduling_ignored_during_execution"`
}

// NodeSelectorConfig represents node selector configuration
type NodeSelectorConfig struct {
	NodeSelectorTerms []*NodeSelectorTerm `yaml:"node_selector_terms"`
}

// NodeSelectorTerm represents node selector term
type NodeSelectorTerm struct {
	MatchExpressions []*MatchExpression `yaml:"match_expressions"`
	MatchFields      []*MatchExpression `yaml:"match_fields"`
}

// MatchExpression represents match expression
type MatchExpression struct {
	Key      string   `yaml:"key"`
	Operator string   `yaml:"operator"`
	Values   []string `yaml:"values"`
}

// PreferredScheduling represents preferred scheduling
type PreferredScheduling struct {
	Weight     int               `yaml:"weight"`
	Preference *NodeSelectorTerm `yaml:"preference"`
}

// PodAffinityConfig represents pod affinity configuration
type PodAffinityConfig struct {
	RequiredDuringSchedulingIgnoredDuringExecution  []*PodAffinityTerm         `yaml:"required_during_scheduling_ignored_during_execution"`
	PreferredDuringSchedulingIgnoredDuringExecution []*WeightedPodAffinityTerm `yaml:"preferred_during_scheduling_ignored_during_execution"`
}

// PodAffinityTerm represents pod affinity term
type PodAffinityTerm struct {
	LabelSelector *LabelSelector `yaml:"label_selector"`
	TopologyKey   string         `yaml:"topology_key"`
}

// WeightedPodAffinityTerm represents weighted pod affinity term
type WeightedPodAffinityTerm struct {
	Weight          int              `yaml:"weight"`
	PodAffinityTerm *PodAffinityTerm `yaml:"pod_affinity_term"`
}

// LabelSelector represents label selector
type LabelSelector struct {
	MatchLabels      map[string]string  `yaml:"match_labels"`
	MatchExpressions []*MatchExpression `yaml:"match_expressions"`
}

// ServiceConfig represents service configuration
type ServiceConfig struct {
	Type      string            `yaml:"type"`
	Ports     []*ServicePort    `yaml:"ports"`
	Selector  map[string]string `yaml:"selector"`
	ClusterIP string            `yaml:"cluster_ip"`
}

// ServicePort represents service port
type ServicePort struct {
	Name       string `yaml:"name"`
	Port       int    `yaml:"port"`
	TargetPort int    `yaml:"target_port"`
	Protocol   string `yaml:"protocol"`
}

// IngressConfig represents ingress configuration
type IngressConfig struct {
	Enabled     bool              `yaml:"enabled"`
	Annotations map[string]string `yaml:"annotations"`
	Rules       []*IngressRule    `yaml:"rules"`
	TLS         []*IngressTLS     `yaml:"tls"`
}

// IngressRule represents ingress rule
type IngressRule struct {
	Host  string         `yaml:"host"`
	Paths []*IngressPath `yaml:"paths"`
}

// IngressPath represents ingress path
type IngressPath struct {
	Path     string          `yaml:"path"`
	PathType string          `yaml:"path_type"`
	Backend  *IngressBackend `yaml:"backend"`
}

// IngressBackend represents ingress backend
type IngressBackend struct {
	Service *IngressServiceBackend `yaml:"service"`
}

// IngressServiceBackend represents ingress service backend
type IngressServiceBackend struct {
	Name string             `yaml:"name"`
	Port *ServicePortConfig `yaml:"port"`
}

// ServicePortConfig represents service port configuration
type ServicePortConfig struct {
	Number int    `yaml:"number"`
	Name   string `yaml:"name"`
}

// IngressTLS represents ingress TLS
type IngressTLS struct {
	Hosts      []string `yaml:"hosts"`
	SecretName string   `yaml:"secret_name"`
}

// PVConfig represents persistent volume configuration
type PVConfig struct {
	Size         string   `yaml:"size"`
	StorageClass string   `yaml:"storage_class"`
	AccessModes  []string `yaml:"access_modes"`
}

// ProcessPoolConfig represents process pool configuration
type ProcessPoolConfig struct {
	Enabled             bool   `yaml:"enabled"`
	MinProcesses        int    `yaml:"min_processes"`
	MaxProcesses        int    `yaml:"max_processes"`
	IdleTimeout         string `yaml:"idle_timeout"`
	RespawnInterval     string `yaml:"respawn_interval"`
	HealthCheckInterval string `yaml:"health_check_interval"`
}

// ContainerRuntimeConfig represents container runtime configuration
type ContainerRuntimeConfig struct {
	Runtime      string          `yaml:"runtime"`
	RuntimePath  string          `yaml:"runtime_path"`
	RuntimeArgs  []string        `yaml:"runtime_args"`
	NetworkMode  string          `yaml:"network_mode"`
	CgroupParent string          `yaml:"cgroup_parent"`
	ShmSize      string          `yaml:"shm_size"`
	Ulimits      []*UlimitConfig `yaml:"ulimits"`
}

// UlimitConfig represents ulimit configuration
type UlimitConfig struct {
	Name string `yaml:"name"`
	Soft int64  `yaml:"soft"`
	Hard int64  `yaml:"hard"`
}

// IsolationConfig represents isolation configuration
type IsolationConfig struct {
	Namespaces   *NamespaceConfig  `yaml:"namespaces"`
	Cgroups      *CgroupConfig     `yaml:"cgroups"`
	Capabilities *CapabilityConfig `yaml:"capabilities"`
	Seccomp      *SeccompConfig    `yaml:"seccomp"`
	AppArmor     *AppArmorConfig   `yaml:"apparmor"`
}

// NamespaceConfig represents namespace configuration
type NamespaceConfig struct {
	PID     bool `yaml:"pid"`
	Network bool `yaml:"network"`
	Mount   bool `yaml:"mount"`
	UTS     bool `yaml:"uts"`
	IPC     bool `yaml:"ipc"`
	User    bool `yaml:"user"`
}

// CgroupConfig represents cgroup configuration
type CgroupConfig struct {
	Enabled     bool   `yaml:"enabled"`
	CgroupPath  string `yaml:"cgroup_path"`
	CPULimit    string `yaml:"cpu_limit"`
	MemoryLimit string `yaml:"memory_limit"`
	PidsLimit   int    `yaml:"pids_limit"`
}

// CapabilityConfig represents capability configuration
type CapabilityConfig struct {
	Drop []string `yaml:"drop"`
	Add  []string `yaml:"add"`
}

// SeccompConfig represents seccomp configuration
type SeccompConfig struct {
	Enabled bool   `yaml:"enabled"`
	Profile string `yaml:"profile"`
}

// AppArmorConfig represents AppArmor configuration
type AppArmorConfig struct {
	Enabled bool   `yaml:"enabled"`
	Profile string `yaml:"profile"`
}

// ChrootConfig represents chroot configuration
type ChrootConfig struct {
	Enabled  bool   `yaml:"enabled"`
	RootPath string `yaml:"root_path"`
	UserID   int    `yaml:"user_id"`
	GroupID  int    `yaml:"group_id"`
}

// ResourcesConfig represents resource configuration
type ResourcesConfig struct {
	CPULimit      string `yaml:"cpu_limit"`
	MemoryLimit   string `yaml:"memory_limit"`
	CPURequest    string `yaml:"cpu_request"`
	MemoryRequest string `yaml:"memory_request"`
	DiskLimit     string `yaml:"disk_limit"`
	NetworkLimit  string `yaml:"network_limit"`
	PidsLimit     int    `yaml:"pids_limit"`
}

// GameMonitoringConfig represents game monitoring configuration
type GameMonitoringConfig struct {
	Enabled             bool                   `yaml:"enabled"`
	HealthCheckInterval string                 `yaml:"health_check_interval"`
	MetricsInterval     string                 `yaml:"metrics_interval"`
	LogLevel            string                 `yaml:"log_level"`
	AlertThresholds     *AlertThresholdsConfig `yaml:"alert_thresholds"`
}

// AlertThresholdsConfig represents alert thresholds
type AlertThresholdsConfig struct {
	CPUUsage    float64 `yaml:"cpu_usage"`
	MemoryUsage float64 `yaml:"memory_usage"`
	DiskUsage   float64 `yaml:"disk_usage"`
	LoadAverage float64 `yaml:"load_average"`
}

// GameStorageConfig represents game storage configuration
type GameStorageConfig struct {
	GameDataPath string          `yaml:"game_data_path"`
	UserDataPath string          `yaml:"user_data_path"`
	LogPath      string          `yaml:"log_path"`
	TempPath     string          `yaml:"temp_path"`
	BackupPath   string          `yaml:"backup_path"`
	Volumes      []*VolumeConfig `yaml:"volumes"`
	Backup       *BackupConfig   `yaml:"backup"`
	Cleanup      *CleanupConfig  `yaml:"cleanup"`
}

// BackupConfig represents backup configuration
type BackupConfig struct {
	Enabled         bool   `yaml:"enabled"`
	Interval        string `yaml:"interval"`
	RetentionDays   int    `yaml:"retention_days"`
	CompressBackups bool   `yaml:"compress_backups"`
	BackupLocation  string `yaml:"backup_location"`
}

// CleanupConfig represents cleanup configuration
type CleanupConfig struct {
	Enabled            bool   `yaml:"enabled"`
	Interval           string `yaml:"interval"`
	MaxAge             string `yaml:"max_age"`
	DeleteEmptyDirs    bool   `yaml:"delete_empty_dirs"`
	PreserveRecordings bool   `yaml:"preserve_recordings"`
}

// GameSecurityConfig represents game security configuration
type GameSecurityConfig struct {
	Sandboxing    *SandboxingConfig         `yaml:"sandboxing"`
	AccessControl *AccessControlConfig      `yaml:"access_control"`
	RateLimiting  *RateLimitingConfig       `yaml:"rate_limiting"`
	Monitoring    *SecurityMonitoringConfig `yaml:"monitoring"`
}

// SandboxingConfig represents sandboxing configuration
type SandboxingConfig struct {
	Enabled         bool     `yaml:"enabled"`
	AllowedSyscalls []string `yaml:"allowed_syscalls"`
	BlockedSyscalls []string `yaml:"blocked_syscalls"`
	AllowedPaths    []string `yaml:"allowed_paths"`
	BlockedPaths    []string `yaml:"blocked_paths"`
}

// AccessControlConfig represents access control configuration
type AccessControlConfig struct {
	Enabled               bool     `yaml:"enabled"`
	AllowedUsers          []string `yaml:"allowed_users"`
	AllowedGroups         []string `yaml:"allowed_groups"`
	RequireAuthentication bool     `yaml:"require_authentication"`
	MaxConcurrentSessions int      `yaml:"max_concurrent_sessions"`
}

// SecurityMonitoringConfig represents security monitoring configuration
type SecurityMonitoringConfig struct {
	Enabled                   bool `yaml:"enabled"`
	LogSecurityEvents         bool `yaml:"log_security_events"`
	AlertOnSuspiciousActivity bool `yaml:"alert_on_suspicious_activity"`
	MonitorFileAccess         bool `yaml:"monitor_file_access"`
	MonitorNetworkAccess      bool `yaml:"monitor_network_access"`
}

// LoadGameServiceConfig loads game service configuration with inheritance support
func LoadGameServiceConfig(configPath string) (*GameServiceConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	expanded := os.ExpandEnv(string(data))

	var config GameServiceConfig
	if err := yaml.Unmarshal([]byte(expanded), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Load and merge common configuration if specified
	if config.InheritFrom != "" {
		commonConfigPath := FindCommonConfig(configPath)
		if config.InheritFrom == "common.yaml" {
			// Use the common.yaml in the same directory
			commonConfig, err := LoadCommonConfig(commonConfigPath)
			if err != nil {
				return nil, fmt.Errorf("failed to load common config: %w", err)
			}
			MergeWithCommon(&config, commonConfig)
		}
	}

	applyGameDefaults(&config)

	return &config, nil
}

// applyGameDefaults applies default values to game configuration
func applyGameDefaults(cfg *GameServiceConfig) {
	if cfg.Version == "" {
		cfg.Version = "0.0.2"
	}

	if cfg.Server == nil {
		cfg.Server = &ServerConfig{
			Port:           8084,
			GRPCPort:       9094,
			Host:           "0.0.0.0",
			Timeout:        "60s",
			MaxConnections: 1000,
		}
	}

	if cfg.GameEngine == nil {
		cfg.GameEngine = &GameEngineConfig{
			Mode: "container",
			ContainerRuntime: &ContainerRuntimeConfig{
				Runtime:     "docker",
				NetworkMode: "bridge",
				ShmSize:     "64m",
			},
			Isolation: &IsolationConfig{
				Namespaces: &NamespaceConfig{
					PID:     true,
					Network: true,
					Mount:   true,
					UTS:     true,
					IPC:     true,
					User:    false,
				},
				Capabilities: &CapabilityConfig{
					Drop: []string{"ALL"},
					Add:  []string{"CHOWN", "SETUID", "SETGID"},
				},
			},
			Resources: &ResourcesConfig{
				CPULimit:      "1000m",
				MemoryLimit:   "512Mi",
				CPURequest:    "100m",
				MemoryRequest: "128Mi",
				PidsLimit:     100,
			},
			Monitoring: &GameMonitoringConfig{
				Enabled:             true,
				HealthCheckInterval: "30s",
				MetricsInterval:     "15s",
				LogLevel:            "info",
				AlertThresholds: &AlertThresholdsConfig{
					CPUUsage:    80.0,
					MemoryUsage: 85.0,
					DiskUsage:   90.0,
					LoadAverage: 2.0,
				},
			},
		}
	}

	if cfg.Kubernetes == nil {
		cfg.Kubernetes = &KubernetesConfig{
			Enabled:        false,
			Namespace:      "dungeongate",
			ServiceAccount: "dungeongate-game-service",
			ConfigMapName:  "dungeongate-game-config",
		}
	}

	if cfg.Storage == nil {
		cfg.Storage = &GameStorageConfig{
			GameDataPath: "/var/lib/dungeongate/games",
			UserDataPath: "/var/lib/dungeongate/users",
			LogPath:      "/var/log/dungeongate/games",
			TempPath:     "/tmp/dungeongate/games",
			BackupPath:   "/var/backups/dungeongate",
			Backup: &BackupConfig{
				Enabled:         true,
				Interval:        "24h",
				RetentionDays:   30,
				CompressBackups: true,
			},
			Cleanup: &CleanupConfig{
				Enabled:            true,
				Interval:           "1h",
				MaxAge:             "7d",
				DeleteEmptyDirs:    true,
				PreserveRecordings: true,
			},
		}
	}

	if cfg.Security == nil {
		cfg.Security = &GameSecurityConfig{
			Sandboxing: &SandboxingConfig{
				Enabled: true,
				AllowedSyscalls: []string{
					"read", "write", "open", "close", "stat", "fstat",
					"lstat", "poll", "lseek", "mmap", "mprotect", "munmap",
					"brk", "rt_sigaction", "rt_sigprocmask", "rt_sigreturn",
					"ioctl", "access", "pipe", "select", "sched_yield",
					"mremap", "msync", "mincore", "madvise", "shmget",
					"shmat", "shmctl", "dup", "dup2", "pause", "nanosleep",
					"getitimer", "alarm", "setitimer", "getpid", "sendfile",
					"socket", "connect", "accept", "sendto", "recvfrom",
					"sendmsg", "recvmsg", "shutdown", "bind", "listen",
					"getsockname", "getpeername", "socketpair", "setsockopt",
					"getsockopt", "clone", "fork", "vfork", "execve", "exit",
					"wait4", "kill", "uname", "semget", "semop", "semctl",
					"shmdt", "msgget", "msgsnd", "msgrcv", "msgctl", "fcntl",
					"flock", "fsync", "fdatasync", "truncate", "ftruncate",
					"getdents", "getcwd", "chdir", "fchdir", "rename", "mkdir",
					"rmdir", "creat", "link", "unlink", "symlink", "readlink",
					"chmod", "fchmod", "chown", "fchown", "lchown", "umask",
					"gettimeofday", "getrlimit", "getrusage", "sysinfo",
					"times", "ptrace", "getuid", "syslog", "getgid", "setuid",
					"setgid", "geteuid", "getegid", "setpgid", "getppid",
					"getpgrp", "setsid", "setreuid", "setregid", "getgroups",
					"setgroups", "setresuid", "getresuid", "setresgid",
					"getresgid", "getpgid", "setfsuid", "setfsgid", "getsid",
					"capget", "capset", "rt_sigpending", "rt_sigtimedwait",
					"rt_sigqueueinfo", "rt_sigsuspend", "sigaltstack",
					"utime", "mknod", "uselib", "personality", "ustat",
					"statfs", "fstatfs", "sysfs", "getpriority", "setpriority",
					"sched_setparam", "sched_getparam", "sched_setscheduler",
					"sched_getscheduler", "sched_get_priority_max",
					"sched_get_priority_min", "sched_rr_get_interval",
					"mlock", "munlock", "mlockall", "munlockall", "vhangup",
					"modify_ldt", "pivot_root", "prctl", "arch_prctl",
					"adjtimex", "setrlimit", "chroot", "sync", "acct",
					"settimeofday", "mount", "umount2", "swapon", "swapoff",
					"reboot", "sethostname", "setdomainname", "iopl", "ioperm",
					"create_module", "init_module", "delete_module",
					"get_kernel_syms", "query_module", "quotactl", "nfsservctl",
					"getpmsg", "putpmsg", "afs_syscall", "tuxcall", "security",
					"gettid", "readahead", "setxattr", "lsetxattr", "fsetxattr",
					"getxattr", "lgetxattr", "fgetxattr", "listxattr",
					"llistxattr", "flistxattr", "removexattr", "lremovexattr",
					"fremovexattr", "tkill", "time", "futex", "sched_setaffinity",
					"sched_getaffinity", "set_thread_area", "io_setup",
					"io_destroy", "io_getevents", "io_submit", "io_cancel",
					"get_thread_area", "lookup_dcookie", "epoll_create",
					"epoll_ctl_old", "epoll_wait_old", "remap_file_pages",
					"getdents64", "set_tid_address", "restart_syscall",
					"semtimedop", "fadvise64", "timer_create", "timer_settime",
					"timer_gettime", "timer_getoverrun", "timer_delete",
					"clock_settime", "clock_gettime", "clock_getres",
					"clock_nanosleep", "exit_group", "epoll_wait", "epoll_ctl",
					"tgkill", "utimes", "vserver", "mbind", "set_mempolicy",
					"get_mempolicy", "mq_open", "mq_unlink", "mq_timedsend",
					"mq_timedreceive", "mq_notify", "mq_getsetattr", "kexec_load",
					"waitid", "add_key", "request_key", "keyctl", "ioprio_set",
					"ioprio_get", "inotify_init", "inotify_add_watch",
					"inotify_rm_watch", "migrate_pages", "openat", "mkdirat",
					"mknodat", "fchownat", "futimesat", "newfstatat", "unlinkat",
					"renameat", "linkat", "symlinkat", "readlinkat", "fchmodat",
					"faccessat", "pselect6", "ppoll", "unshare", "set_robust_list",
					"get_robust_list", "splice", "tee", "sync_file_range",
					"vmsplice", "move_pages", "utimensat", "epoll_pwait",
					"signalfd", "timerfd_create", "eventfd", "fallocate",
					"timerfd_settime", "timerfd_gettime", "accept4", "signalfd4",
					"eventfd2", "epoll_create1", "dup3", "pipe2", "inotify_init1",
					"preadv", "pwritev", "rt_tgsigqueueinfo", "perf_event_open",
					"recvmmsg", "fanotify_init", "fanotify_mark", "prlimit64",
					"name_to_handle_at", "open_by_handle_at", "clock_adjtime",
					"syncfs", "sendmmsg", "setns", "getcpu", "process_vm_readv",
					"process_vm_writev", "kcmp", "finit_module", "sched_setattr",
					"sched_getattr", "renameat2", "seccomp", "getrandom",
					"memfd_create", "kexec_file_load", "bpf", "execveat",
					"userfaultfd", "membarrier", "mlock2", "copy_file_range",
					"preadv2", "pwritev2", "pkey_mprotect", "pkey_alloc",
					"pkey_free", "statx", "io_pgetevents", "rseq",
				},
				AllowedPaths: []string{
					"/usr/games",
					"/var/games",
					"/tmp",
					"/dev/null",
					"/dev/zero",
					"/dev/random",
					"/dev/urandom",
					"/proc/self",
					"/proc/thread-self",
					"/proc/version",
					"/proc/cpuinfo",
					"/proc/meminfo",
					"/proc/stat",
					"/proc/uptime",
					"/proc/loadavg",
					"/etc/passwd",
					"/etc/group",
					"/etc/hosts",
					"/etc/resolv.conf",
					"/etc/nsswitch.conf",
					"/etc/ld.so.cache",
					"/etc/ld.so.conf",
					"/etc/ld.so.conf.d",
					"/lib",
					"/lib64",
					"/usr/lib",
					"/usr/lib64",
					"/usr/share/terminfo",
					"/usr/share/locale",
				},
				BlockedPaths: []string{
					"/etc/shadow",
					"/etc/sudoers",
					"/etc/ssh",
					"/root",
					"/home",
					"/var/log",
					"/var/run",
					"/var/lib/dpkg",
					"/var/lib/apt",
					"/boot",
					"/sys",
					"/proc/sys",
					"/proc/*/mem",
					"/proc/*/maps",
					"/proc/*/environ",
					"/proc/*/cmdline",
					"/proc/kcore",
					"/proc/kmem",
					"/proc/kallsyms",
					"/proc/modules",
					"/dev/mem",
					"/dev/kmem",
					"/dev/port",
				},
			},
			AccessControl: &AccessControlConfig{
				Enabled:               true,
				RequireAuthentication: true,
				MaxConcurrentSessions: 10,
			},
			RateLimiting: &RateLimitingConfig{
				Enabled:             true,
				MaxConnectionsPerIP: 5,
				ConnectionWindow:    "1m",
			},
			Monitoring: &SecurityMonitoringConfig{
				Enabled:                   true,
				LogSecurityEvents:         true,
				AlertOnSuspiciousActivity: true,
				MonitorFileAccess:         true,
				MonitorNetworkAccess:      true,
			},
		}
	}
}

// Validate validates the game service configuration
func (cfg *GameServiceConfig) Validate() error {
	if cfg.Server == nil {
		return fmt.Errorf("server configuration is required")
	}
	if cfg.GameEngine == nil {
		return fmt.Errorf("game engine configuration is required")
	}
	if cfg.Storage == nil {
		return fmt.Errorf("storage configuration is required")
	}

	// Validate game engine mode
	switch cfg.GameEngine.Mode {
	case "container", "process", "hybrid":
		// Valid modes
	default:
		return fmt.Errorf("invalid game engine mode: %s", cfg.GameEngine.Mode)
	}

	// Validate individual game configurations
	for _, game := range cfg.Games {
		if err := game.Validate(); err != nil {
			return fmt.Errorf("game %s validation failed: %w", game.ID, err)
		}
	}

	return nil
}

// Validate validates a game configuration
func (game *GameConfig) Validate() error {
	if game.ID == "" {
		return fmt.Errorf("game ID is required")
	}
	if game.Name == "" {
		return fmt.Errorf("game name is required")
	}
	if game.Binary == nil {
		return fmt.Errorf("binary configuration is required")
	}
	if game.Binary.Path == "" {
		return fmt.Errorf("binary path is required")
	}

	// Validate path configuration
	if err := game.validatePaths(); err != nil {
		return fmt.Errorf("path validation failed: %w", err)
	}

	// Validate setup options
	if err := game.validateSetupOptions(); err != nil {
		return fmt.Errorf("setup options validation failed: %w", err)
	}

	// Validate cleanup options
	if err := game.validateCleanupOptions(); err != nil {
		return fmt.Errorf("cleanup options validation failed: %w", err)
	}

	// Validate resource limits
	if game.Resources != nil {
		if game.Resources.CPULimit != "" {
			if _, err := ParseResourceQuantity(game.Resources.CPULimit); err != nil {
				return fmt.Errorf("invalid CPU limit: %w", err)
			}
		}
		if game.Resources.MemoryLimit != "" {
			if _, err := ParseResourceQuantity(game.Resources.MemoryLimit); err != nil {
				return fmt.Errorf("invalid memory limit: %w", err)
			}
		}
	}

	return nil
}

// ParseResourceQuantity parses a resource quantity string
func ParseResourceQuantity(s string) (int64, error) {
	// Simple implementation - in production, use k8s.io/apimachinery/pkg/api/resource
	// For now, just validate it's not empty
	if s == "" {
		return 0, fmt.Errorf("resource quantity cannot be empty")
	}
	return 1, nil
}

// GetGameTimeoutDuration returns game timeout as duration
func (game *GameConfig) GetGameTimeoutDuration() time.Duration {
	if game.Settings != nil && game.Settings.MaxSessionDuration != "" {
		if duration, err := time.ParseDuration(game.Settings.MaxSessionDuration); err == nil {
			return duration
		}
	}
	return 4 * time.Hour // Default fallback
}

// GetIdleTimeoutDuration returns idle timeout as duration
func (game *GameConfig) GetIdleTimeoutDuration() time.Duration {
	if game.Settings != nil && game.Settings.IdleTimeout != "" {
		if duration, err := time.ParseDuration(game.Settings.IdleTimeout); err == nil {
			return duration
		}
	}
	return 30 * time.Minute // Default fallback
}

// GetDefaultNetHackConfig returns a default NetHack game configuration
func GetDefaultNetHackConfig() *GameConfig {
	return &GameConfig{
		ID:        "nethack",
		Name:      "NetHack",
		ShortName: "nh",
		Version:   "3.7.0",
		Enabled:   true,
		Binary: &BinaryConfig{
			Path:             "/opt/homebrew/bin/nethack",
			Args:             []string{"-u", "${USERNAME}"},
			WorkingDirectory: "/var/games/nethack",
			User:             "games",
			Group:            "games",
			Permissions:      "0755",
		},
		Paths: &GamePathsConfig{
			AutoDetect: true,
			System: &SystemPathsConfig{
				ScoreDir:    "/opt/homebrew/share/nethack/",
				SysConfFile: "/opt/homebrew/Cellar/nethack/3.6.7/libexec/sysconf",
				SymbolsFile: "/opt/homebrew/Cellar/nethack/3.6.7/libexec/symbols",
				DataFile:    "nhdat",
			},
			User: &UserPathsConfig{
				BaseDir:    "games/nethack",
				SaveDir:    "games/nethack/saves",
				ConfigDir:  "games/nethack/config",
				BonesDir:   "games/nethack/bones",
				LevelDir:   "games/nethack/levels",
				LockDir:    "games/nethack/locks",
				TroubleDir: "games/nethack/trouble",
			},
		},
		Setup: &GameSetupOptions{
			CreateUserDirs:    true,
			CopyDefaultConfig: true,
			InitializeShared:  true,
			ValidatePaths:     true,
			SetPermissions:    true,
			DetectSystemPaths: true,
			CreateSaveLinks:   true,
		},
		Cleanup: &GameCleanupOptions{
			RemoveUserDirs:     false,
			ClearTempFiles:     true,
			RemoveLockFiles:    true,
			ClearPersonalBones: false,
			PreserveConfig:     true,
			BackupSaves:        true,
			CleanupSaveLinks:   true,
			ValidateCleanup:    true,
		},
		Files: &FilesConfig{
			DataDirectory:   "/var/games/nethack",
			SaveDirectory:   "/var/games/nethack/save",
			ConfigDirectory: "/var/games/nethack/config",
			LogDirectory:    "/var/log/nethack",
			TempDirectory:   "/tmp/nethack",
			SharedFiles:     []string{"nhdat", "license", "recover"},
			UserFiles:       []string{"${USERNAME}.nh", "${USERNAME}.0", "${USERNAME}.bak"},
			Permissions: &PermissionsConfig{
				DataDirectory: "0755",
				SaveDirectory: "0755",
				UserFiles:     "0644",
				LogFiles:      "0644",
			},
		},
		Settings: &GameSettings{
			MaxPlayers:         50,
			MaxSessionDuration: "4h",
			IdleTimeout:        "30m",
			SaveInterval:       "5m",
			AutoSave:           true,
			Spectating: &SpectatingConfig{
				Enabled:                 true,
				MaxSpectatorsPerSession: 5,
				SpectatorTimeout:        "2h",
			},
			Recording: &RecordingConfig{
				Enabled:       true,
				Format:        "ttyrec",
				Compression:   "gzip",
				MaxFileSize:   "100MB",
				RetentionDays: 30,
				AutoCleanup:   true,
			},
			Options: map[string]string{
				"MAXNROFPLAYERS": "50",
				"SEDLEVEL":       "5",
				"DUMPLOG":        "1",
				"LIVELOG":        "1",
				"XLOGFILE":       "/var/games/nethack/xlogfile",
				"LIVELOGFILE":    "/var/games/nethack/livelog",
			},
		},
		Environment: map[string]string{
			"NETHACKOPTIONS": "@/var/games/nethack/config/${USERNAME}.nethackrc",
			"HACKDIR":        "/var/games/nethack",
			"TERM":           "xterm-256color",
			"USER":           "${USERNAME}",
			"HOME":           "/var/games/nethack/users/${USERNAME}",
			"SHELL":          "/bin/sh",
		},
		Resources: &ResourcesConfig{
			CPULimit:      "500m",
			MemoryLimit:   "256Mi",
			CPURequest:    "100m",
			MemoryRequest: "64Mi",
			DiskLimit:     "1Gi",
			PidsLimit:     50,
		},
		Container: &ContainerConfig{
			Image:      "dungeongate/nethack",
			Tag:        "3.7.0",
			Registry:   "ghcr.io",
			PullPolicy: "IfNotPresent",
			Volumes: []*VolumeConfig{
				{
					Name:      "nethack-data",
					HostPath:  "/var/games/nethack",
					MountPath: "/var/games/nethack",
					ReadOnly:  false,
				},
				{
					Name:      "nethack-saves",
					HostPath:  "/var/games/nethack/save",
					MountPath: "/var/games/nethack/save",
					ReadOnly:  false,
				},
			},
			Environment: map[string]string{
				"GAME":     "nethack",
				"USERNAME": "${USERNAME}",
				"TERM":     "xterm-256color",
			},
			SecurityContext: &SecurityContextConfig{
				RunAsUser:              1000,
				RunAsGroup:             1000,
				ReadOnlyRootFilesystem: true,
				Privileged:             false,
			},
			NetworkMode: "none",
		},
		Networking: &NetworkingConfig{
			Mode: "isolated",
		},
	}
}

// GetNetHackSystemPaths returns system paths for NetHack
func (game *GameConfig) GetNetHackSystemPaths() *SystemPathsConfig {
	if game.Paths != nil && game.Paths.System != nil {
		return game.Paths.System
	}
	// Return default system paths for NetHack on macOS/Homebrew
	return &SystemPathsConfig{
		ScoreDir:    "/opt/homebrew/share/nethack/",
		SysConfFile: "/opt/homebrew/Cellar/nethack/3.6.7/libexec/sysconf",
		SymbolsFile: "/opt/homebrew/Cellar/nethack/3.6.7/libexec/symbols",
		DataFile:    "nhdat",
	}
}

// GetUserPaths returns user-specific paths for the game
func (game *GameConfig) GetUserPaths() *UserPathsConfig {
	if game.Paths != nil && game.Paths.User != nil {
		return game.Paths.User
	}
	// Return default user paths
	return &UserPathsConfig{
		BaseDir:    "games/" + game.ID,
		SaveDir:    "games/" + game.ID + "/saves",
		ConfigDir:  "games/" + game.ID + "/config",
		BonesDir:   "games/" + game.ID + "/bones",
		LevelDir:   "games/" + game.ID + "/levels",
		LockDir:    "games/" + game.ID + "/locks",
		TroubleDir: "games/" + game.ID + "/trouble",
	}
}

// ShouldAutoDetectPaths returns whether to auto-detect system paths
func (game *GameConfig) ShouldAutoDetectPaths() bool {
	return game.Paths != nil && game.Paths.AutoDetect
}

// GetSetupOptions returns setup options with defaults
func (game *GameConfig) GetSetupOptions() *GameSetupOptions {
	if game.Setup != nil {
		return game.Setup
	}
	// Return default setup options
	return &GameSetupOptions{
		CreateUserDirs:    true,
		CopyDefaultConfig: true,
		InitializeShared:  true,
		ValidatePaths:     true,
		SetPermissions:    true,
		DetectSystemPaths: true,
		CreateSaveLinks:   true,
	}
}

// GetCleanupOptions returns cleanup options with defaults
func (game *GameConfig) GetCleanupOptions() *GameCleanupOptions {
	if game.Cleanup != nil {
		return game.Cleanup
	}
	// Return default cleanup options
	return &GameCleanupOptions{
		RemoveUserDirs:     false,
		ClearTempFiles:     true,
		RemoveLockFiles:    true,
		ClearPersonalBones: false,
		PreserveConfig:     true,
		BackupSaves:        true,
		CleanupSaveLinks:   true,
		ValidateCleanup:    true,
	}
}

// validatePaths validates game path configuration
func (game *GameConfig) validatePaths() error {
	if game.Paths == nil {
		return fmt.Errorf("paths configuration is required")
	}

	// Validate system paths
	if game.Paths.System != nil {
		if err := game.validateSystemPaths(game.Paths.System); err != nil {
			return fmt.Errorf("system paths validation failed: %w", err)
		}
	}

	// Validate user paths
	if game.Paths.User != nil {
		if err := game.validateUserPaths(game.Paths.User); err != nil {
			return fmt.Errorf("user paths validation failed: %w", err)
		}
	}

	return nil
}

// validateSystemPaths validates system path configuration
func (game *GameConfig) validateSystemPaths(paths *SystemPathsConfig) error {
	if paths.ScoreDir == "" {
		return fmt.Errorf("score_dir is required")
	}
	if paths.SysConfFile == "" {
		return fmt.Errorf("sysconf_file is required")
	}
	if paths.SymbolsFile == "" {
		return fmt.Errorf("symbols_file is required")
	}
	if paths.DataFile == "" {
		return fmt.Errorf("data_file is required")
	}

	// Validate paths exist and are accessible (if validation is enabled)
	if game.GetSetupOptions().ValidatePaths {
		if err := validatePathExists(paths.ScoreDir, true); err != nil {
			return fmt.Errorf("score_dir validation failed: %w", err)
		}
		if err := validatePathExists(paths.SysConfFile, false); err != nil {
			return fmt.Errorf("sysconf_file validation failed: %w", err)
		}
		if err := validatePathExists(paths.SymbolsFile, false); err != nil {
			return fmt.Errorf("symbols_file validation failed: %w", err)
		}
	}

	return nil
}

// validateUserPaths validates user path configuration
func (game *GameConfig) validateUserPaths(paths *UserPathsConfig) error {
	if paths.BaseDir == "" {
		return fmt.Errorf("base_dir is required")
	}
	if paths.SaveDir == "" {
		return fmt.Errorf("save_dir is required")
	}
	if paths.ConfigDir == "" {
		return fmt.Errorf("config_dir is required")
	}
	if paths.BonesDir == "" {
		return fmt.Errorf("bones_dir is required")
	}
	if paths.LevelDir == "" {
		return fmt.Errorf("level_dir is required")
	}
	if paths.LockDir == "" {
		return fmt.Errorf("lock_dir is required")
	}
	if paths.TroubleDir == "" {
		return fmt.Errorf("trouble_dir is required")
	}

	// Validate path names don't contain invalid characters
	pathNames := map[string]string{
		"base_dir":    paths.BaseDir,
		"save_dir":    paths.SaveDir,
		"config_dir":  paths.ConfigDir,
		"bones_dir":   paths.BonesDir,
		"level_dir":   paths.LevelDir,
		"lock_dir":    paths.LockDir,
		"trouble_dir": paths.TroubleDir,
	}

	for name, path := range pathNames {
		if err := validatePathName(path); err != nil {
			return fmt.Errorf("%s validation failed: %w", name, err)
		}
	}

	return nil
}

// validateSetupOptions validates setup options
func (game *GameConfig) validateSetupOptions() error {
	// Setup options are optional, but if auto-detect is enabled, ensure it's valid for the game type
	if game.Setup != nil && game.Setup.DetectSystemPaths {
		// Only certain games support auto-detection
		supportedGames := []string{"nethack", "dcss", "angband"}
		supported := false
		for _, supportedGame := range supportedGames {
			if game.ID == supportedGame {
				supported = true
				break
			}
		}
		if !supported {
			return fmt.Errorf("auto-detection of system paths is not supported for game type '%s'", game.ID)
		}
	}

	return nil
}

// validateCleanupOptions validates cleanup options
func (game *GameConfig) validateCleanupOptions() error {
	if game.Cleanup != nil {
		// Validate that if backup_saves is enabled, we're not removing user dirs
		if game.Cleanup.BackupSaves && game.Cleanup.RemoveUserDirs {
			return fmt.Errorf("cannot backup saves and remove user directories simultaneously")
		}

		// Validate that if preserve_config is false, we're not copying default config
		if !game.Cleanup.PreserveConfig && game.Setup != nil && game.Setup.CopyDefaultConfig {
			return fmt.Errorf("conflicting configuration: preserve_config is false but copy_default_config is true")
		}
	}

	return nil
}

// validatePathExists validates that a path exists and is accessible
func validatePathExists(path string, isDirectory bool) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("path does not exist: %s", path)
		}
		return fmt.Errorf("cannot access path %s: %w", path, err)
	}

	if isDirectory && !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}
	if !isDirectory && info.IsDir() {
		return fmt.Errorf("path is a directory but expected a file: %s", path)
	}

	return nil
}

// validatePathName validates that a path name doesn't contain invalid characters
func validatePathName(path string) error {
	// Check for invalid characters
	invalidChars := []string{"\x00", "\n", "\r", "\t"}
	for _, char := range invalidChars {
		if strings.Contains(path, char) {
			return fmt.Errorf("path contains invalid character: %q", char)
		}
	}

	// Check for relative path traversal attempts
	if strings.Contains(path, "..") {
		return fmt.Errorf("path contains parent directory references: %s", path)
	}

	// Check for absolute paths in user paths (they should be relative)
	if filepath.IsAbs(path) {
		return fmt.Errorf("user path should be relative, not absolute: %s", path)
	}

	return nil
}

// ValidateAtStartup performs comprehensive validation suitable for service startup
func (cfg *GameServiceConfig) ValidateAtStartup() error {
	// Perform basic validation first
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("basic validation failed: %w", err)
	}

	// Perform path existence validation for games that have validate_paths enabled
	for _, game := range cfg.Games {
		if game.GetSetupOptions().ValidatePaths {
			if err := game.validatePathExistence(); err != nil {
				return fmt.Errorf("path existence validation failed for game %s: %w", game.ID, err)
			}
		}
	}

	// Validate binary paths exist
	for _, game := range cfg.Games {
		if game.Enabled {
			if err := validateBinaryExists(game.Binary.Path); err != nil {
				return fmt.Errorf("binary validation failed for game %s: %w", game.ID, err)
			}
		}
	}

	return nil
}

// validatePathExistence validates that configured paths actually exist
func (game *GameConfig) validatePathExistence() error {
	if game.Paths == nil {
		return nil
	}

	// Validate system paths exist
	if game.Paths.System != nil {
		systemPaths := game.Paths.System
		if err := validatePathExists(systemPaths.ScoreDir, true); err != nil {
			return fmt.Errorf("score directory validation failed: %w", err)
		}
		if err := validatePathExists(systemPaths.SysConfFile, false); err != nil {
			return fmt.Errorf("sysconf file validation failed: %w", err)
		}
		if err := validatePathExists(systemPaths.SymbolsFile, false); err != nil {
			return fmt.Errorf("symbols file validation failed: %w", err)
		}
	}

	return nil
}

// validateBinaryExists validates that a binary exists and is executable
func validateBinaryExists(binaryPath string) error {
	info, err := os.Stat(binaryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("binary does not exist: %s", binaryPath)
		}
		return fmt.Errorf("cannot access binary %s: %w", binaryPath, err)
	}

	if info.IsDir() {
		return fmt.Errorf("binary path is a directory: %s", binaryPath)
	}

	// Check if file is executable (basic check)
	mode := info.Mode()
	if mode&0111 == 0 {
		return fmt.Errorf("binary is not executable: %s", binaryPath)
	}

	return nil
}

// ValidateGameConfiguration validates a specific game configuration with detailed reporting
func (game *GameConfig) ValidateGameConfiguration() *ValidationReport {
	report := &ValidationReport{
		GameID:   game.ID,
		GameName: game.Name,
		Errors:   []string{},
		Warnings: []string{},
	}

	// Basic validation
	if err := game.Validate(); err != nil {
		report.Errors = append(report.Errors, err.Error())
		return report
	}

	// Check binary exists if validation is enabled
	if game.GetSetupOptions().ValidatePaths {
		if err := validateBinaryExists(game.Binary.Path); err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("Binary validation: %v", err))
		}
	}

	// Check if auto-detection can be performed
	if game.ShouldAutoDetectPaths() {
		if game.ID == "nethack" {
			// Test if nethack --showpaths works
			if err := testNetHackShowPaths(); err != nil {
				report.Warnings = append(report.Warnings, fmt.Sprintf("NetHack --showpaths failed: %v", err))
			}
		}
	}

	// Validate path permissions
	if game.Paths != nil && game.Paths.System != nil && game.GetSetupOptions().ValidatePaths {
		if err := game.validatePathExistence(); err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("Path existence check: %v", err))
		}
	}

	return report
}

// ValidationReport contains validation results for a game configuration
type ValidationReport struct {
	GameID   string   `json:"game_id"`
	GameName string   `json:"game_name"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

// IsValid returns true if there are no errors
func (r *ValidationReport) IsValid() bool {
	return len(r.Errors) == 0
}

// HasWarnings returns true if there are warnings
func (r *ValidationReport) HasWarnings() bool {
	return len(r.Warnings) > 0
}

// testNetHackShowPaths tests if nethack --showpaths command works
func testNetHackShowPaths() error {
	// This would be implemented to actually test the command
	// For now, just return nil to indicate it would work
	return nil
}
