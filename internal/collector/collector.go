package collector

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/clipbot/internal/config"
)

// Source 수집 소스 타입
type Source string

const (
	SourceYouTube Source = "youtube"
	SourceReddit  Source = "reddit"
	SourceTikTok  Source = "tiktok"
	SourceTwitter Source = "twitter"
	SourceRSS     Source = "rss"
	SourceCustom  Source = "custom"
)

// CollectorConfig는 config.CollectorConfig 타입의 별칭
type CollectorConfig = config.CollectorConfig
type SourceConfig = config.SourceConfig
type FilterConfig = config.FilterConfig

// CollectedVideo 수집된 영상 정보
type CollectedVideo struct {
	ID          string    `json:"id"`
	Source      Source    `json:"source"`
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Duration    int       `json:"duration"`
	Views       int64     `json:"views"`
	Likes       int64     `json:"likes"`
	Comments    int64     `json:"comments"`
	Shares      int64     `json:"shares"`
	Author      string    `json:"author"`
	FilePath    string    `json:"file_path"`
	Thumbnail   string    `json:"thumbnail"`
	CollectedAt time.Time `json:"collected_at"`
	Tags        []string  `json:"tags"`
}

// Collector 영상 수집기
type Collector struct {
	cfg       *CollectorConfig
	ytdlpPath string
	collected map[string]bool // 이미 수집된 영상 ID 추적
	mu        sync.RWMutex
}

// NewCollector 새 수집기 생성
func NewCollector(cfg *CollectorConfig) *Collector {
	ytdlpPath := cfg.YtDlpPath
	if ytdlpPath == "" {
		ytdlpPath = "yt-dlp"
	}

	return &Collector{
		cfg:       cfg,
		ytdlpPath: ytdlpPath,
		collected: make(map[string]bool),
	}
}

// Start 자동 수집 시작
func (c *Collector) Start(ctx context.Context) error {
	if !c.cfg.Enabled {
		return fmt.Errorf("수집기가 비활성화되어 있습니다")
	}

	fmt.Println("🔍 영상 수집기 시작!")

	interval := time.Duration(c.cfg.CheckInterval) * time.Minute
	if interval == 0 {
		interval = 30 * time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// 즉시 첫 수집 실행
	c.collectAll(ctx)

	for {
		select {
		case <-ctx.Done():
			fmt.Println("⏹ 수집기 중지됨")
			return nil
		case <-ticker.C:
			c.collectAll(ctx)
		}
	}
}

// collectAll 모든 소스에서 수집
func (c *Collector) collectAll(ctx context.Context) {
	fmt.Printf("\n📥 영상 수집 중... (%s)\n", time.Now().Format("15:04:05"))

	for _, source := range c.cfg.Sources {
		if !source.Enabled {
			continue
		}

		videos, err := c.CollectFromSource(ctx, source)
		if err != nil {
			fmt.Printf("❌ %s 수집 실패: %v\n", source.Type, err)
			continue
		}

		fmt.Printf("✅ %s: %d개 영상 수집됨\n", source.Type, len(videos))
	}
}

// CollectFromSource 특정 소스에서 수집
func (c *Collector) CollectFromSource(ctx context.Context, source SourceConfig) ([]*CollectedVideo, error) {
	switch Source(source.Type) {
	case SourceYouTube:
		return c.collectFromYouTube(ctx, source)
	case SourceReddit:
		return c.collectFromReddit(ctx, source)
	case SourceTikTok:
		return c.collectFromTikTok(ctx, source)
	case SourceTwitter:
		return c.collectFromTwitter(ctx, source)
	case SourceRSS:
		return c.collectFromRSS(ctx, source)
	default:
		return nil, fmt.Errorf("지원하지 않는 소스: %s", source.Type)
	}
}

// collectFromYouTube YouTube에서 영상 수집
func (c *Collector) collectFromYouTube(ctx context.Context, source SourceConfig) ([]*CollectedVideo, error) {
	var videos []*CollectedVideo

	for _, url := range source.URLs {
		collected, err := c.downloadWithYtDlp(ctx, url, source.MaxItems, "youtube")
		if err != nil {
			fmt.Printf("⚠️ YouTube 수집 오류 (%s): %v\n", url, err)
			continue
		}
		videos = append(videos, collected...)
	}

	// 키워드 검색
	for _, keyword := range source.Keywords {
		searchURL := fmt.Sprintf("ytsearch%d:%s", source.MaxItems, keyword)
		collected, err := c.downloadWithYtDlp(ctx, searchURL, source.MaxItems, "youtube")
		if err != nil {
			fmt.Printf("⚠️ YouTube 검색 오류 (%s): %v\n", keyword, err)
			continue
		}
		videos = append(videos, collected...)
	}

	return videos, nil
}

// collectFromReddit Reddit에서 영상 수집
func (c *Collector) collectFromReddit(ctx context.Context, source SourceConfig) ([]*CollectedVideo, error) {
	var videos []*CollectedVideo

	// 인기 영상 서브레딧들
	defaultSubreddits := []string{
		"https://www.reddit.com/r/videos/top/.json?t=day",
		"https://www.reddit.com/r/TikTokCringe/top/.json?t=day",
		"https://www.reddit.com/r/Unexpected/top/.json?t=day",
		"https://www.reddit.com/r/interestingasfuck/top/.json?t=day",
	}

	urls := source.URLs
	if len(urls) == 0 {
		urls = defaultSubreddits
	}

	for _, url := range urls {
		// Reddit URL을 yt-dlp로 처리
		collected, err := c.downloadWithYtDlp(ctx, url, source.MaxItems, "reddit")
		if err != nil {
			continue
		}
		videos = append(videos, collected...)
	}

	return videos, nil
}

// collectFromTikTok TikTok에서 영상 수집
func (c *Collector) collectFromTikTok(ctx context.Context, source SourceConfig) ([]*CollectedVideo, error) {
	var videos []*CollectedVideo

	for _, url := range source.URLs {
		collected, err := c.downloadWithYtDlp(ctx, url, source.MaxItems, "tiktok")
		if err != nil {
			continue
		}
		videos = append(videos, collected...)
	}

	// 해시태그 검색
	for _, hashtag := range source.Hashtags {
		hashtag = strings.TrimPrefix(hashtag, "#")
		url := fmt.Sprintf("https://www.tiktok.com/tag/%s", hashtag)
		collected, err := c.downloadWithYtDlp(ctx, url, source.MaxItems, "tiktok")
		if err != nil {
			continue
		}
		videos = append(videos, collected...)
	}

	return videos, nil
}

// collectFromTwitter Twitter에서 영상 수집
func (c *Collector) collectFromTwitter(ctx context.Context, source SourceConfig) ([]*CollectedVideo, error) {
	var videos []*CollectedVideo

	for _, url := range source.URLs {
		collected, err := c.downloadWithYtDlp(ctx, url, source.MaxItems, "twitter")
		if err != nil {
			continue
		}
		videos = append(videos, collected...)
	}

	return videos, nil
}

// collectFromRSS RSS 피드에서 영상 수집
func (c *Collector) collectFromRSS(ctx context.Context, source SourceConfig) ([]*CollectedVideo, error) {
	var videos []*CollectedVideo

	for _, url := range source.URLs {
		collected, err := c.downloadWithYtDlp(ctx, url, source.MaxItems, "rss")
		if err != nil {
			continue
		}
		videos = append(videos, collected...)
	}

	return videos, nil
}

// downloadWithYtDlp yt-dlp를 사용하여 영상 다운로드
func (c *Collector) downloadWithYtDlp(ctx context.Context, url string, maxItems int, source string) ([]*CollectedVideo, error) {
	// 출력 디렉토리 생성
	outputDir := c.cfg.OutputDir
	if outputDir == "" {
		outputDir = "./input"
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, err
	}

	// 출력 템플릿
	outputTemplate := filepath.Join(outputDir, "%(title).50s_%(id)s.%(ext)s")

	args := []string{
		url,
		"-o", outputTemplate,
		"--no-playlist",
		"--write-info-json", // 메타데이터 저장
		"--write-thumbnail", // 썸네일 저장
		// 비디오+오디오 포함된 단일 스트림 우선, 없으면 병합
		"-f", "best[ext=mp4][vcodec!=none][acodec!=none]/bestvideo[ext=mp4]+bestaudio[ext=m4a]/best",
		"--merge-output-format", "mp4",
		"--no-warnings",
		"--ignore-errors",
		"--no-overwrites", // 이미 있는 파일 건너뛰기
	}

	// 최대 다운로드 수
	if maxItems > 0 {
		args = append(args, "--max-downloads", fmt.Sprintf("%d", maxItems))
	}

	// 영상 길이 필터
	if c.cfg.MaxDuration > 0 {
		args = append(args, "--match-filter", fmt.Sprintf("duration <= %d", c.cfg.MaxDuration))
	}
	if c.cfg.MinDuration > 0 {
		args = append(args, "--match-filter", fmt.Sprintf("duration >= %d", c.cfg.MinDuration))
	}

	// 파일 크기 필터
	if c.cfg.MaxFileSize > 0 {
		args = append(args, "--max-filesize", fmt.Sprintf("%dM", c.cfg.MaxFileSize))
	}

	// 조회수 필터
	if c.cfg.Filters.MinViews > 0 {
		args = append(args, "--match-filter", fmt.Sprintf("view_count >= %d", c.cfg.Filters.MinViews))
	}

	// 쿠키 파일 (로그인 필요한 경우)
	cookiesFile := filepath.Join(outputDir, "../cookies.txt")
	if _, err := os.Stat(cookiesFile); err == nil {
		args = append(args, "--cookies", cookiesFile)
	}

	fmt.Printf("  📥 다운로드: %s\n", truncateString(url, 60))

	cmd := exec.CommandContext(ctx, c.ytdlpPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// 일부 영상 실패는 무시
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	// 다운로드된 파일 목록 반환
	return c.scanDownloadedVideos(outputDir, source)
}

// scanDownloadedVideos 다운로드된 영상 스캔
func (c *Collector) scanDownloadedVideos(dir, source string) ([]*CollectedVideo, error) {
	var videos []*CollectedVideo

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	videoExts := map[string]bool{".mp4": true, ".webm": true, ".mkv": true}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if !videoExts[ext] {
			continue
		}

		// 이미 처리된 영상인지 확인
		c.mu.RLock()
		if c.collected[entry.Name()] {
			c.mu.RUnlock()
			continue
		}
		c.mu.RUnlock()

		video := &CollectedVideo{
			ID:          strings.TrimSuffix(entry.Name(), ext),
			Source:      Source(source),
			Title:       strings.TrimSuffix(entry.Name(), ext),
			FilePath:    filepath.Join(dir, entry.Name()),
			CollectedAt: time.Now(),
		}

		// 수집된 영상으로 표시
		c.mu.Lock()
		c.collected[entry.Name()] = true
		c.mu.Unlock()

		videos = append(videos, video)
	}

	return videos, nil
}

// DownloadSingle 단일 URL 다운로드
func (c *Collector) DownloadSingle(ctx context.Context, url string) (*CollectedVideo, error) {
	videos, err := c.downloadWithYtDlp(ctx, url, 1, "custom")
	if err != nil {
		return nil, err
	}
	if len(videos) == 0 {
		return nil, fmt.Errorf("다운로드된 영상이 없습니다")
	}
	return videos[0], nil
}

// GetCollectedCount 수집된 영상 수 반환
func (c *Collector) GetCollectedCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.collected)
}

// truncateString 문자열 자르기
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ValidateCopyright 저작권 검증 (기본 필터링)
func (c *Collector) ValidateCopyright(video *CollectedVideo) bool {
	// 저작권 관련 키워드 체크
	copyrightKeywords := []string{
		"copyright", "©", "all rights reserved",
		"licensed", "official video", "music video",
	}

	title := strings.ToLower(video.Title)
	desc := strings.ToLower(video.Description)

	for _, keyword := range copyrightKeywords {
		if strings.Contains(title, keyword) || strings.Contains(desc, keyword) {
			return false
		}
	}

	return true
}

// GetTrendingHashtags 트렌딩 해시태그 가져오기
func (c *Collector) GetTrendingHashtags(platform string) []string {
	// 플랫폼별 트렌딩 해시태그 (예시)
	trendingTags := map[string][]string{
		"tiktok": {
			"#fyp", "#viral", "#trending", "#foryou", "#tiktok",
			"#funny", "#comedy", "#meme", "#dance", "#challenge",
		},
		"youtube": {
			"#shorts", "#viral", "#trending", "#youtube",
			"#subscribe", "#funny", "#satisfying",
		},
		"instagram": {
			"#reels", "#viral", "#explore", "#trending",
			"#instagood", "#reelsinstagram",
		},
	}

	if tags, exists := trendingTags[platform]; exists {
		return tags
	}
	return []string{"#viral", "#trending"}
}
