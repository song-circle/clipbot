package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// YouTubeCollector YouTube 전용 수집기
type YouTubeCollector struct {
	cfg       *CollectorConfig
	apiKey    string
	ytdlpPath string
	client    *http.Client
}

// YouTubeSearchResult YouTube 검색 결과
type YouTubeSearchResult struct {
	Items []struct {
		ID struct {
			VideoID string `json:"videoId"`
		} `json:"id"`
		Snippet struct {
			Title        string `json:"title"`
			Description  string `json:"description"`
			ChannelTitle string `json:"channelTitle"`
			PublishedAt  string `json:"publishedAt"`
			Thumbnails   struct {
				High struct {
					URL string `json:"url"`
				} `json:"high"`
			} `json:"thumbnails"`
		} `json:"snippet"`
	} `json:"items"`
}

// YouTubeVideoDetails YouTube 영상 상세 정보
type YouTubeVideoDetails struct {
	Items []struct {
		ID         string `json:"id"`
		Statistics struct {
			ViewCount    string `json:"viewCount"`
			LikeCount    string `json:"likeCount"`
			CommentCount string `json:"commentCount"`
		} `json:"statistics"`
		ContentDetails struct {
			Duration string `json:"duration"`
		} `json:"contentDetails"`
	} `json:"items"`
}

// NewYouTubeCollector 새 YouTube 수집기 생성
func NewYouTubeCollector(cfg *CollectorConfig, apiKey string) *YouTubeCollector {
	ytdlpPath := cfg.YtDlpPath
	if ytdlpPath == "" {
		ytdlpPath = "yt-dlp"
	}

	return &YouTubeCollector{
		cfg:       cfg,
		apiKey:    apiKey,
		ytdlpPath: ytdlpPath,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

// SearchVideos 키워드로 영상 검색
func (y *YouTubeCollector) SearchVideos(ctx context.Context, query string, maxResults int) ([]*CollectedVideo, error) {
	if y.apiKey != "" {
		return y.searchWithAPI(ctx, query, maxResults)
	}
	return y.searchWithYtDlp(ctx, query, maxResults)
}

// searchWithAPI YouTube Data API로 검색
func (y *YouTubeCollector) searchWithAPI(ctx context.Context, query string, maxResults int) ([]*CollectedVideo, error) {
	searchURL := fmt.Sprintf(
		"https://www.googleapis.com/youtube/v3/search?part=snippet&type=video&videoDuration=short&q=%s&maxResults=%d&key=%s",
		url.QueryEscape(query),
		maxResults,
		y.apiKey,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := y.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result YouTubeSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var videos []*CollectedVideo
	for _, item := range result.Items {
		video := &CollectedVideo{
			ID:          item.ID.VideoID,
			Source:      SourceYouTube,
			URL:         fmt.Sprintf("https://www.youtube.com/watch?v=%s", item.ID.VideoID),
			Title:       item.Snippet.Title,
			Description: item.Snippet.Description,
			Author:      item.Snippet.ChannelTitle,
			Thumbnail:   item.Snippet.Thumbnails.High.URL,
			CollectedAt: time.Now(),
		}
		videos = append(videos, video)
	}

	return videos, nil
}

// searchWithYtDlp yt-dlp로 검색 (API 키 없이)
func (y *YouTubeCollector) searchWithYtDlp(ctx context.Context, query string, maxResults int) ([]*CollectedVideo, error) {
	searchURL := fmt.Sprintf("ytsearch%d:%s", maxResults, query)
	
	outputDir := y.cfg.OutputDir
	if outputDir == "" {
		outputDir = "./input"
	}
	os.MkdirAll(outputDir, 0755)

	args := []string{
		searchURL,
		"-o", filepath.Join(outputDir, "%(title).50s_%(id)s.%(ext)s"),
		"-f", "mp4/best[ext=mp4]/best",
		"--merge-output-format", "mp4",
		"--write-info-json",
		"--no-warnings",
		"--ignore-errors",
	}

	// 쇼츠만 필터 (60초 이하)
	if y.cfg.MaxDuration > 0 {
		args = append(args, "--match-filter", fmt.Sprintf("duration <= %d", y.cfg.MaxDuration))
	}

	cmd := exec.CommandContext(ctx, y.ytdlpPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}

	return y.scanDownloaded(outputDir)
}

// CollectFromChannel 채널에서 최신 영상 수집
func (y *YouTubeCollector) CollectFromChannel(ctx context.Context, channelURL string, maxVideos int) ([]*CollectedVideo, error) {
	outputDir := y.cfg.OutputDir
	if outputDir == "" {
		outputDir = "./input"
	}
	os.MkdirAll(outputDir, 0755)

	args := []string{
		channelURL,
		"-o", filepath.Join(outputDir, "%(title).50s_%(id)s.%(ext)s"),
		"-f", "mp4/best[ext=mp4]/best",
		"--merge-output-format", "mp4",
		"--playlist-end", fmt.Sprintf("%d", maxVideos),
		"--write-info-json",
		"--no-warnings",
		"--ignore-errors",
		"--no-overwrites",
	}

	// 쇼츠만 필터
	if y.cfg.MaxDuration > 0 {
		args = append(args, "--match-filter", fmt.Sprintf("duration <= %d", y.cfg.MaxDuration))
	}

	fmt.Printf("📥 채널에서 수집 중: %s\n", channelURL)

	cmd := exec.CommandContext(ctx, y.ytdlpPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}

	return y.scanDownloaded(outputDir)
}

// CollectShorts YouTube Shorts만 수집
func (y *YouTubeCollector) CollectShorts(ctx context.Context, query string, maxResults int) ([]*CollectedVideo, error) {
	// Shorts 검색 쿼리 수정
	shortsQuery := query + " #shorts"
	
	outputDir := y.cfg.OutputDir
	if outputDir == "" {
		outputDir = "./input"
	}
	os.MkdirAll(outputDir, 0755)

	args := []string{
		fmt.Sprintf("ytsearch%d:%s", maxResults*2, shortsQuery), // 필터링 위해 더 많이 검색
		"-o", filepath.Join(outputDir, "%(title).50s_%(id)s.%(ext)s"),
		"-f", "mp4/best[ext=mp4]/best",
		"--merge-output-format", "mp4",
		"--match-filter", "duration <= 60", // 60초 이하만
		"--max-downloads", fmt.Sprintf("%d", maxResults),
		"--write-info-json",
		"--no-warnings",
		"--ignore-errors",
		"--no-overwrites",
	}

	fmt.Printf("📥 Shorts 수집 중: %s\n", query)

	cmd := exec.CommandContext(ctx, y.ytdlpPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}

	return y.scanDownloaded(outputDir)
}

// CollectTrending 트렌딩 영상 수집
func (y *YouTubeCollector) CollectTrending(ctx context.Context, region string, maxResults int) ([]*CollectedVideo, error) {
	// 트렌딩 페이지 URL (지역별)
	trendingURL := fmt.Sprintf("https://www.youtube.com/feed/trending?gl=%s", region)

	outputDir := y.cfg.OutputDir
	if outputDir == "" {
		outputDir = "./input"
	}
	os.MkdirAll(outputDir, 0755)

	args := []string{
		trendingURL,
		"-o", filepath.Join(outputDir, "%(title).50s_%(id)s.%(ext)s"),
		"-f", "mp4/best[ext=mp4]/best",
		"--merge-output-format", "mp4",
		"--playlist-end", fmt.Sprintf("%d", maxResults),
		"--match-filter", fmt.Sprintf("duration <= %d", y.cfg.MaxDuration),
		"--write-info-json",
		"--no-warnings",
		"--ignore-errors",
		"--no-overwrites",
	}

	fmt.Printf("📥 트렌딩 수집 중 (%s)...\n", region)

	cmd := exec.CommandContext(ctx, y.ytdlpPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Run() // 에러 무시 (일부 영상 실패 가능)

	return y.scanDownloaded(outputDir)
}

// scanDownloaded 다운로드된 파일 스캔
func (y *YouTubeCollector) scanDownloaded(dir string) ([]*CollectedVideo, error) {
	var videos []*CollectedVideo

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".mp4" && ext != ".webm" && ext != ".mkv" {
			continue
		}

		video := &CollectedVideo{
			ID:          strings.TrimSuffix(entry.Name(), ext),
			Source:      SourceYouTube,
			Title:       strings.TrimSuffix(entry.Name(), ext),
			FilePath:    filepath.Join(dir, entry.Name()),
			CollectedAt: time.Now(),
		}

		// info.json에서 추가 정보 읽기
		infoPath := filepath.Join(dir, strings.TrimSuffix(entry.Name(), ext)+".info.json")
		if infoData, err := os.ReadFile(infoPath); err == nil {
			var info map[string]interface{}
			if json.Unmarshal(infoData, &info) == nil {
				if title, ok := info["title"].(string); ok {
					video.Title = title
				}
				if desc, ok := info["description"].(string); ok {
					video.Description = desc
				}
				if views, ok := info["view_count"].(float64); ok {
					video.Views = int64(views)
				}
				if likes, ok := info["like_count"].(float64); ok {
					video.Likes = int64(likes)
				}
				if duration, ok := info["duration"].(float64); ok {
					video.Duration = int(duration)
				}
				if uploader, ok := info["uploader"].(string); ok {
					video.Author = uploader
				}
				if webpage, ok := info["webpage_url"].(string); ok {
					video.URL = webpage
				}
			}
		}

		videos = append(videos, video)
	}

	return videos, nil
}

// DownloadByID 영상 ID로 다운로드
func (y *YouTubeCollector) DownloadByID(ctx context.Context, videoID string) (*CollectedVideo, error) {
	url := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	
	outputDir := y.cfg.OutputDir
	if outputDir == "" {
		outputDir = "./input"
	}
	os.MkdirAll(outputDir, 0755)

	outputPath := filepath.Join(outputDir, "%(title).50s_%(id)s.%(ext)s")

	args := []string{
		url,
		"-o", outputPath,
		"-f", "mp4/best[ext=mp4]/best",
		"--merge-output-format", "mp4",
		"--write-info-json",
		"--write-thumbnail",
	}

	cmd := exec.CommandContext(ctx, y.ytdlpPath, args...)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("다운로드 실패: %w", err)
	}

	videos, err := y.scanDownloaded(outputDir)
	if err != nil {
		return nil, err
	}

	for _, v := range videos {
		if strings.Contains(v.ID, videoID) {
			return v, nil
		}
	}

	return nil, fmt.Errorf("다운로드된 영상을 찾을 수 없습니다")
}

