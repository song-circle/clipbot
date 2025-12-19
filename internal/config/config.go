package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config 전체 설정 구조체
type Config struct {
	App       AppConfig       `mapstructure:"app"`
	Video     VideoConfig     `mapstructure:"video"`
	AI        AIConfig        `mapstructure:"ai"`
	Platforms PlatformsConfig `mapstructure:"platforms"`
	Schedule  ScheduleConfig  `mapstructure:"schedule"`
	Storage   StorageConfig   `mapstructure:"storage"`
	Collector CollectorConfig `mapstructure:"collector"`
}

// CollectorConfig 영상 수집 설정
type CollectorConfig struct {
	Enabled       bool             `mapstructure:"enabled"`
	OutputDir     string           `mapstructure:"output_dir"`
	MaxDuration   int              `mapstructure:"max_duration"`
	MinDuration   int              `mapstructure:"min_duration"`
	MaxFileSize   int64            `mapstructure:"max_file_size"`
	CheckInterval int              `mapstructure:"check_interval"`
	Sources       []SourceConfig   `mapstructure:"sources"`
	Filters       FilterConfig     `mapstructure:"filters"`
	YtDlpPath     string           `mapstructure:"ytdlp_path"`
	StockVideo    StockVideoConfig `mapstructure:"stock_video"`
}

// StockVideoConfig 스톡 영상 API 설정
type StockVideoConfig struct {
	PexelsAPIKey  string `mapstructure:"pexels_api_key"`
	PixabayAPIKey string `mapstructure:"pixabay_api_key"`
}

// SourceConfig 수집 소스 설정
type SourceConfig struct {
	Type     string   `mapstructure:"type"`
	Enabled  bool     `mapstructure:"enabled"`
	URLs     []string `mapstructure:"urls"`
	Keywords []string `mapstructure:"keywords"`
	Hashtags []string `mapstructure:"hashtags"`
	MaxItems int      `mapstructure:"max_items"`
	SortBy   string   `mapstructure:"sort_by"`
}

// FilterConfig 필터 설정
type FilterConfig struct {
	MinViews     int64    `mapstructure:"min_views"`
	MinLikes     int64    `mapstructure:"min_likes"`
	Languages    []string `mapstructure:"languages"`
	ExcludeWords []string `mapstructure:"exclude_words"`
	IncludeWords []string `mapstructure:"include_words"`
	RequireAudio bool     `mapstructure:"require_audio"`
}

// AppConfig 앱 기본 설정
type AppConfig struct {
	Name     string `mapstructure:"name"`
	LogLevel string `mapstructure:"log_level"`
	Debug    bool   `mapstructure:"debug"`
}

// VideoConfig 영상 편집 설정
type VideoConfig struct {
	FFmpegPath    string         `mapstructure:"ffmpeg_path"`
	DefaultWidth  int            `mapstructure:"default_width"`
	DefaultHeight int            `mapstructure:"default_height"`
	DefaultFPS    int            `mapstructure:"default_fps"`
	MaxDuration   int            `mapstructure:"max_duration"` // 초 단위
	OutputFormat  string         `mapstructure:"output_format"`
	Quality       string         `mapstructure:"quality"` // low, medium, high, ultra
	Effects       EffectsConfig  `mapstructure:"effects"`
	Subtitle      SubtitleConfig `mapstructure:"subtitle"`
}

// EffectsConfig 효과 설정
type EffectsConfig struct {
	AutoCrop        bool    `mapstructure:"auto_crop"`
	FaceTracking    bool    `mapstructure:"face_tracking"`
	ZoomEffect      bool    `mapstructure:"zoom_effect"`
	TransitionType  string  `mapstructure:"transition_type"` // fade, slide, zoom
	SpeedMultiplier float64 `mapstructure:"speed_multiplier"`
}

// SubtitleConfig 자막 설정
type SubtitleConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	Language   string `mapstructure:"language"`
	Style      string `mapstructure:"style"` // modern, classic, minimal
	FontSize   int    `mapstructure:"font_size"`
	FontColor  string `mapstructure:"font_color"`
	BGColor    string `mapstructure:"bg_color"`
	Position   string `mapstructure:"position"`     // top, center, bottom
	WordByWord bool   `mapstructure:"word_by_word"` // 단어별 하이라이트
}

// AIConfig AI 관련 설정
type AIConfig struct {
	Provider     string `mapstructure:"provider"` // openai, local
	OpenAIKey    string `mapstructure:"openai_key"`
	Model        string `mapstructure:"model"`
	WhisperModel string `mapstructure:"whisper_model"` // tiny, base, small, medium, large
	AutoCaption  bool   `mapstructure:"auto_caption"`
	AutoHashtags bool   `mapstructure:"auto_hashtags"`
	ContentIdeas bool   `mapstructure:"content_ideas"`
}

// PlatformsConfig 각 플랫폼 설정
type PlatformsConfig struct {
	TikTok    TikTokConfig    `mapstructure:"tiktok"`
	YouTube   YouTubeConfig   `mapstructure:"youtube"`
	Instagram InstagramConfig `mapstructure:"instagram"`
	Facebook  FacebookConfig  `mapstructure:"facebook"`
	Twitter   TwitterConfig   `mapstructure:"twitter"`
}

// TikTokConfig 틱톡 설정
type TikTokConfig struct {
	Enabled     bool     `mapstructure:"enabled"`
	SessionID   string   `mapstructure:"session_id"`
	Username    string   `mapstructure:"username"`
	DefaultTags []string `mapstructure:"default_tags"`
	AutoPost    bool     `mapstructure:"auto_post"`
}

// YouTubeConfig 유튜브 설정
type YouTubeConfig struct {
	Enabled       bool     `mapstructure:"enabled"`
	ClientID      string   `mapstructure:"client_id"`
	ClientSecret  string   `mapstructure:"client_secret"`
	RefreshToken  string   `mapstructure:"refresh_token"`
	ChannelID     string   `mapstructure:"channel_id"`
	DefaultTags   []string `mapstructure:"default_tags"`
	CategoryID    string   `mapstructure:"category_id"`
	PrivacyStatus string   `mapstructure:"privacy_status"` // public, private, unlisted
	ShortsHashtag bool     `mapstructure:"shorts_hashtag"` // #Shorts 자동 추가
}

// InstagramConfig 인스타그램 설정
type InstagramConfig struct {
	Enabled     bool     `mapstructure:"enabled"`
	Username    string   `mapstructure:"username"`
	Password    string   `mapstructure:"password"`
	SessionData string   `mapstructure:"session_data"`
	DefaultTags []string `mapstructure:"default_tags"`
}

// FacebookConfig 페이스북 설정
type FacebookConfig struct {
	Enabled     bool     `mapstructure:"enabled"`
	AccessToken string   `mapstructure:"access_token"`
	PageID      string   `mapstructure:"page_id"`
	DefaultTags []string `mapstructure:"default_tags"`
}

// TwitterConfig 트위터/X 설정
type TwitterConfig struct {
	Enabled      bool     `mapstructure:"enabled"`
	APIKey       string   `mapstructure:"api_key"`
	APISecret    string   `mapstructure:"api_secret"`
	AccessToken  string   `mapstructure:"access_token"`
	AccessSecret string   `mapstructure:"access_secret"`
	DefaultTags  []string `mapstructure:"default_tags"`
}

// ScheduleConfig 스케줄 설정
type ScheduleConfig struct {
	Enabled       bool     `mapstructure:"enabled"`
	Timezone      string   `mapstructure:"timezone"`
	PostTimes     []string `mapstructure:"post_times"` // "09:00", "15:00", "21:00"
	PostsPerDay   int      `mapstructure:"posts_per_day"`
	RandomDelay   int      `mapstructure:"random_delay"` // 분 단위 랜덤 딜레이
	WeekendPause  bool     `mapstructure:"weekend_pause"`
	OptimalTiming bool     `mapstructure:"optimal_timing"` // AI 기반 최적 시간 분석
}

// StorageConfig 저장소 설정
type StorageConfig struct {
	InputDir     string `mapstructure:"input_dir"`
	OutputDir    string `mapstructure:"output_dir"`
	TempDir      string `mapstructure:"temp_dir"`
	ArchiveDir   string `mapstructure:"archive_dir"`
	DatabasePath string `mapstructure:"database_path"`
}

// Load 설정 파일 로드
func Load(configPath string) (*Config, error) {
	v := viper.New()

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// 기본 경로에서 설정 파일 찾기
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
		v.AddConfigPath("$HOME/.clipbot")
	}

	// 환경 변수 바인딩
	v.AutomaticEnv()
	v.SetEnvPrefix("CLIPBOT")

	// 기본값 설정
	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// 설정 파일이 없으면 기본 설정 생성
			return createDefaultConfig()
		}
		return nil, fmt.Errorf("설정 파일 읽기 실패: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("설정 파싱 실패: %w", err)
	}

	// 환경 변수에서 민감한 정보 오버라이드
	overrideFromEnv(&cfg)

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	// App 기본값
	v.SetDefault("app.name", "ClipBot")
	v.SetDefault("app.log_level", "info")
	v.SetDefault("app.debug", false)

	// Video 기본값
	v.SetDefault("video.default_width", 1080)
	v.SetDefault("video.default_height", 1920)
	v.SetDefault("video.default_fps", 30)
	v.SetDefault("video.max_duration", 60)
	v.SetDefault("video.output_format", "mp4")
	v.SetDefault("video.quality", "high")

	// Effects 기본값
	v.SetDefault("video.effects.auto_crop", true)
	v.SetDefault("video.effects.transition_type", "fade")
	v.SetDefault("video.effects.speed_multiplier", 1.0)

	// Subtitle 기본값
	v.SetDefault("video.subtitle.enabled", true)
	v.SetDefault("video.subtitle.language", "ko")
	v.SetDefault("video.subtitle.style", "modern")
	v.SetDefault("video.subtitle.font_size", 48)
	v.SetDefault("video.subtitle.font_color", "#FFFFFF")
	v.SetDefault("video.subtitle.position", "bottom")
	v.SetDefault("video.subtitle.word_by_word", true)

	// AI 기본값
	v.SetDefault("ai.provider", "openai")
	v.SetDefault("ai.model", "gpt-4")
	v.SetDefault("ai.whisper_model", "base")
	v.SetDefault("ai.auto_caption", true)
	v.SetDefault("ai.auto_hashtags", true)

	// Schedule 기본값
	v.SetDefault("schedule.timezone", "Asia/Seoul")
	v.SetDefault("schedule.posts_per_day", 3)
	v.SetDefault("schedule.random_delay", 30)

	// Storage 기본값
	v.SetDefault("storage.input_dir", "./input")
	v.SetDefault("storage.output_dir", "./output")
	v.SetDefault("storage.temp_dir", "./temp")
	v.SetDefault("storage.archive_dir", "./archive")
	v.SetDefault("storage.database_path", "./data/clipbot.db")

	// Collector 기본값
	v.SetDefault("collector.enabled", true)
	v.SetDefault("collector.output_dir", "./input")
	v.SetDefault("collector.max_duration", 60)
	v.SetDefault("collector.min_duration", 5)
	v.SetDefault("collector.max_file_size", 100)
	v.SetDefault("collector.check_interval", 60)
}

func createDefaultConfig() (*Config, error) {
	cfg := &Config{
		App: AppConfig{
			Name:     "ClipBot",
			LogLevel: "info",
			Debug:    false,
		},
		Video: VideoConfig{
			DefaultWidth:  1080,
			DefaultHeight: 1920,
			DefaultFPS:    30,
			MaxDuration:   60,
			OutputFormat:  "mp4",
			Quality:       "high",
			Effects: EffectsConfig{
				AutoCrop:        true,
				TransitionType:  "fade",
				SpeedMultiplier: 1.0,
			},
			Subtitle: SubtitleConfig{
				Enabled:    true,
				Language:   "ko",
				Style:      "modern",
				FontSize:   48,
				FontColor:  "#FFFFFF",
				Position:   "bottom",
				WordByWord: true,
			},
		},
		AI: AIConfig{
			Provider:     "openai",
			Model:        "gpt-4",
			WhisperModel: "base",
			AutoCaption:  true,
			AutoHashtags: true,
		},
		Schedule: ScheduleConfig{
			Timezone:    "Asia/Seoul",
			PostsPerDay: 3,
			RandomDelay: 30,
			PostTimes:   []string{"09:00", "15:00", "21:00"},
		},
		Storage: StorageConfig{
			InputDir:     "./input",
			OutputDir:    "./output",
			TempDir:      "./temp",
			ArchiveDir:   "./archive",
			DatabasePath: "./data/clipbot.db",
		},
		Collector: CollectorConfig{
			Enabled:       true,
			OutputDir:     "./input",
			MaxDuration:   60,
			MinDuration:   5,
			MaxFileSize:   100,
			CheckInterval: 60,
			Sources: []SourceConfig{
				{Type: "youtube", Enabled: true, Keywords: []string{"shorts viral"}, MaxItems: 5},
				{Type: "reddit", Enabled: true, URLs: []string{"videos", "TikTokCringe"}, MaxItems: 5},
			},
			Filters: FilterConfig{
				MinViews:     1000,
				RequireAudio: true,
			},
		},
	}

	return cfg, nil
}

func overrideFromEnv(cfg *Config) {
	// 민감한 정보는 환경 변수에서 우선적으로 가져옴
	if key := os.Getenv("CLIPBOT_OPENAI_KEY"); key != "" {
		cfg.AI.OpenAIKey = key
	}
	if id := os.Getenv("CLIPBOT_YOUTUBE_CLIENT_ID"); id != "" {
		cfg.Platforms.YouTube.ClientID = id
	}
	if secret := os.Getenv("CLIPBOT_YOUTUBE_CLIENT_SECRET"); secret != "" {
		cfg.Platforms.YouTube.ClientSecret = secret
	}

	// youtube_credentials.json 파일에서 OAuth 정보 로드
	loadYouTubeCredentials(cfg)
	if token := os.Getenv("CLIPBOT_TIKTOK_SESSION"); token != "" {
		cfg.Platforms.TikTok.SessionID = token
	}
	if token := os.Getenv("CLIPBOT_FACEBOOK_TOKEN"); token != "" {
		cfg.Platforms.Facebook.AccessToken = token
	}
	if key := os.Getenv("CLIPBOT_TWITTER_API_KEY"); key != "" {
		cfg.Platforms.Twitter.APIKey = key
	}
}

// EnsureDirectories 필요한 디렉토리 생성
func (c *Config) EnsureDirectories() error {
	dirs := []string{
		c.Storage.InputDir,
		c.Storage.OutputDir,
		c.Storage.TempDir,
		c.Storage.ArchiveDir,
		filepath.Dir(c.Storage.DatabasePath),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("디렉토리 생성 실패 %s: %w", dir, err)
		}
	}

	return nil
}

// loadYouTubeCredentials youtube_credentials.json 파일에서 OAuth 정보 로드
func loadYouTubeCredentials(cfg *Config) {
	// 여러 경로에서 credentials 파일 찾기
	possiblePaths := []string{
		"./youtube_credentials.json",
		"youtube_credentials.json",
		"./credentials/youtube_credentials.json",
	}

	var credPath string
	for _, p := range possiblePaths {
		if _, err := os.Stat(p); err == nil {
			credPath = p
			break
		}
	}

	if credPath == "" {
		return // credentials 파일이 없으면 그냥 반환
	}

	data, err := os.ReadFile(credPath)
	if err != nil {
		return
	}

	var creds struct {
		Installed struct {
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
		} `json:"installed"`
		Web struct {
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
		} `json:"web"`
	}

	if err := json.Unmarshal(data, &creds); err != nil {
		return
	}

	// installed 또는 web 타입의 credentials 로드
	if creds.Installed.ClientID != "" {
		cfg.Platforms.YouTube.ClientID = creds.Installed.ClientID
		cfg.Platforms.YouTube.ClientSecret = creds.Installed.ClientSecret
		cfg.Platforms.YouTube.Enabled = true
	} else if creds.Web.ClientID != "" {
		cfg.Platforms.YouTube.ClientID = creds.Web.ClientID
		cfg.Platforms.YouTube.ClientSecret = creds.Web.ClientSecret
		cfg.Platforms.YouTube.Enabled = true
	}
}

// GetEnabledPlatforms 활성화된 플랫폼 목록 반환
func (c *Config) GetEnabledPlatforms() []string {
	var platforms []string
	if c.Platforms.TikTok.Enabled {
		platforms = append(platforms, "tiktok")
	}
	if c.Platforms.YouTube.Enabled {
		platforms = append(platforms, "youtube")
	}
	if c.Platforms.Instagram.Enabled {
		platforms = append(platforms, "instagram")
	}
	if c.Platforms.Facebook.Enabled {
		platforms = append(platforms, "facebook")
	}
	if c.Platforms.Twitter.Enabled {
		platforms = append(platforms, "twitter")
	}
	return platforms
}
