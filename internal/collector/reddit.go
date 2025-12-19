package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RedditCollector Reddit 전용 수집기
type RedditCollector struct {
	cfg       *CollectorConfig
	ytdlpPath string
	client    *http.Client
}

// RedditResponse Reddit API 응답
type RedditResponse struct {
	Data struct {
		Children []struct {
			Data struct {
				Title        string  `json:"title"`
				Subreddit    string  `json:"subreddit"`
				URL          string  `json:"url"`
				IsVideo      bool    `json:"is_video"`
				Score        int64   `json:"score"`
				NumComments  int64   `json:"num_comments"`
				Permalink    string  `json:"permalink"`
				Author       string  `json:"author"`
				CreatedUTC   float64 `json:"created_utc"`
				Media        struct {
					RedditVideo struct {
						FallbackURL string `json:"fallback_url"`
						Duration    int    `json:"duration"`
					} `json:"reddit_video"`
				} `json:"media"`
				SecureMedia struct {
					RedditVideo struct {
						FallbackURL string `json:"fallback_url"`
						Duration    int    `json:"duration"`
					} `json:"reddit_video"`
				} `json:"secure_media"`
				Preview struct {
					RedditVideoPreview struct {
						FallbackURL string `json:"fallback_url"`
						Duration    int    `json:"duration"`
					} `json:"reddit_video_preview"`
				} `json:"preview"`
			} `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

// NewRedditCollector 새 Reddit 수집기 생성
func NewRedditCollector(cfg *CollectorConfig) *RedditCollector {
	ytdlpPath := cfg.YtDlpPath
	if ytdlpPath == "" {
		ytdlpPath = "yt-dlp"
	}

	return &RedditCollector{
		cfg:       cfg,
		ytdlpPath: ytdlpPath,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CollectFromSubreddit 서브레딧에서 영상 수집
func (r *RedditCollector) CollectFromSubreddit(ctx context.Context, subreddit string, sortBy string, maxResults int) ([]*CollectedVideo, error) {
	// sortBy: hot, new, top, rising
	if sortBy == "" {
		sortBy = "hot"
	}

	// API URL 구성
	apiURL := fmt.Sprintf("https://www.reddit.com/r/%s/%s.json?limit=%d", 
		subreddit, sortBy, maxResults*3) // 영상만 필터링하므로 더 많이 요청

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ClipBot/1.0")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Reddit API 요청 실패: %w", err)
	}
	defer resp.Body.Close()

	var redditResp RedditResponse
	if err := json.NewDecoder(resp.Body).Decode(&redditResp); err != nil {
		return nil, fmt.Errorf("응답 파싱 실패: %w", err)
	}

	var videos []*CollectedVideo
	for _, child := range redditResp.Data.Children {
		post := child.Data

		// 영상 게시물만 필터링
		if !r.hasVideo(post) {
			continue
		}

		// 영상 길이 필터
		duration := r.getVideoDuration(post)
		if r.cfg.MaxDuration > 0 && duration > r.cfg.MaxDuration {
			continue
		}

		// 점수(업보트) 필터
		if r.cfg.Filters.MinViews > 0 && post.Score < r.cfg.Filters.MinViews {
			continue
		}

		video := &CollectedVideo{
			ID:          strings.TrimPrefix(post.Permalink, "/r/"),
			Source:      SourceReddit,
			URL:         r.getVideoURL(post),
			Title:       post.Title,
			Author:      post.Author,
			Duration:    duration,
			Views:       post.Score, // Reddit은 점수를 views로 사용
			Likes:       post.Score,
			Comments:    post.NumComments,
			CollectedAt: time.Now(),
			Tags:        []string{post.Subreddit},
		}

		videos = append(videos, video)

		if len(videos) >= maxResults {
			break
		}
	}

	return videos, nil
}

// DownloadFromSubreddit 서브레딧에서 영상 다운로드
func (r *RedditCollector) DownloadFromSubreddit(ctx context.Context, subreddit string, sortBy string, maxResults int) ([]*CollectedVideo, error) {
	// 먼저 영상 목록 수집
	videos, err := r.CollectFromSubreddit(ctx, subreddit, sortBy, maxResults)
	if err != nil {
		return nil, err
	}

	outputDir := r.cfg.OutputDir
	if outputDir == "" {
		outputDir = "./input"
	}
	os.MkdirAll(outputDir, 0755)

	var downloaded []*CollectedVideo

	for _, video := range videos {
		fmt.Printf("  📥 다운로드: %s\n", truncateString(video.Title, 50))

		filePath, err := r.downloadVideo(ctx, video.URL, outputDir, video.Title)
		if err != nil {
			fmt.Printf("  ⚠️ 실패: %v\n", err)
			continue
		}

		video.FilePath = filePath
		downloaded = append(downloaded, video)
	}

	return downloaded, nil
}

// downloadVideo 영상 다운로드
func (r *RedditCollector) downloadVideo(ctx context.Context, url, outputDir, title string) (string, error) {
	// 파일명 정리
	safeTitle := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, title)

	if len(safeTitle) > 50 {
		safeTitle = safeTitle[:50]
	}

	outputPath := filepath.Join(outputDir, safeTitle+".mp4")

	args := []string{
		url,
		"-o", outputPath,
		"-f", "mp4/best[ext=mp4]/best",
		"--merge-output-format", "mp4",
		"--no-warnings",
	}

	cmd := exec.CommandContext(ctx, r.ytdlpPath, args...)
	if err := cmd.Run(); err != nil {
		return "", err
	}

	return outputPath, nil
}

// hasVideo 영상이 있는 게시물인지 확인
func (r *RedditCollector) hasVideo(post struct {
	Title        string  `json:"title"`
	Subreddit    string  `json:"subreddit"`
	URL          string  `json:"url"`
	IsVideo      bool    `json:"is_video"`
	Score        int64   `json:"score"`
	NumComments  int64   `json:"num_comments"`
	Permalink    string  `json:"permalink"`
	Author       string  `json:"author"`
	CreatedUTC   float64 `json:"created_utc"`
	Media        struct {
		RedditVideo struct {
			FallbackURL string `json:"fallback_url"`
			Duration    int    `json:"duration"`
		} `json:"reddit_video"`
	} `json:"media"`
	SecureMedia struct {
		RedditVideo struct {
			FallbackURL string `json:"fallback_url"`
			Duration    int    `json:"duration"`
		} `json:"reddit_video"`
	} `json:"secure_media"`
	Preview struct {
		RedditVideoPreview struct {
			FallbackURL string `json:"fallback_url"`
			Duration    int    `json:"duration"`
		} `json:"reddit_video_preview"`
	} `json:"preview"`
}) bool {
	if post.IsVideo {
		return true
	}

	url := strings.ToLower(post.URL)
	videoHosts := []string{
		"v.redd.it", "youtube.com", "youtu.be", "streamable.com",
		"gfycat.com", "imgur.com/a/", "clips.twitch.tv",
		"tiktok.com", "twitter.com", "x.com",
	}

	for _, host := range videoHosts {
		if strings.Contains(url, host) {
			return true
		}
	}

	return false
}

// getVideoURL 영상 URL 추출
func (r *RedditCollector) getVideoURL(post struct {
	Title        string  `json:"title"`
	Subreddit    string  `json:"subreddit"`
	URL          string  `json:"url"`
	IsVideo      bool    `json:"is_video"`
	Score        int64   `json:"score"`
	NumComments  int64   `json:"num_comments"`
	Permalink    string  `json:"permalink"`
	Author       string  `json:"author"`
	CreatedUTC   float64 `json:"created_utc"`
	Media        struct {
		RedditVideo struct {
			FallbackURL string `json:"fallback_url"`
			Duration    int    `json:"duration"`
		} `json:"reddit_video"`
	} `json:"media"`
	SecureMedia struct {
		RedditVideo struct {
			FallbackURL string `json:"fallback_url"`
			Duration    int    `json:"duration"`
		} `json:"reddit_video"`
	} `json:"secure_media"`
	Preview struct {
		RedditVideoPreview struct {
			FallbackURL string `json:"fallback_url"`
			Duration    int    `json:"duration"`
		} `json:"reddit_video_preview"`
	} `json:"preview"`
}) string {
	// Reddit 자체 영상
	if post.SecureMedia.RedditVideo.FallbackURL != "" {
		return strings.Replace(post.SecureMedia.RedditVideo.FallbackURL, "?source=fallback", "", 1)
	}
	if post.Media.RedditVideo.FallbackURL != "" {
		return strings.Replace(post.Media.RedditVideo.FallbackURL, "?source=fallback", "", 1)
	}
	if post.Preview.RedditVideoPreview.FallbackURL != "" {
		return post.Preview.RedditVideoPreview.FallbackURL
	}

	// 외부 URL
	return post.URL
}

// getVideoDuration 영상 길이 추출
func (r *RedditCollector) getVideoDuration(post struct {
	Title        string  `json:"title"`
	Subreddit    string  `json:"subreddit"`
	URL          string  `json:"url"`
	IsVideo      bool    `json:"is_video"`
	Score        int64   `json:"score"`
	NumComments  int64   `json:"num_comments"`
	Permalink    string  `json:"permalink"`
	Author       string  `json:"author"`
	CreatedUTC   float64 `json:"created_utc"`
	Media        struct {
		RedditVideo struct {
			FallbackURL string `json:"fallback_url"`
			Duration    int    `json:"duration"`
		} `json:"reddit_video"`
	} `json:"media"`
	SecureMedia struct {
		RedditVideo struct {
			FallbackURL string `json:"fallback_url"`
			Duration    int    `json:"duration"`
		} `json:"reddit_video"`
	} `json:"secure_media"`
	Preview struct {
		RedditVideoPreview struct {
			FallbackURL string `json:"fallback_url"`
			Duration    int    `json:"duration"`
		} `json:"reddit_video_preview"`
	} `json:"preview"`
}) int {
	if post.SecureMedia.RedditVideo.Duration > 0 {
		return post.SecureMedia.RedditVideo.Duration
	}
	if post.Media.RedditVideo.Duration > 0 {
		return post.Media.RedditVideo.Duration
	}
	if post.Preview.RedditVideoPreview.Duration > 0 {
		return post.Preview.RedditVideoPreview.Duration
	}
	return 0
}

// GetPopularSubreddits 인기 영상 서브레딧 목록
func (r *RedditCollector) GetPopularSubreddits() []string {
	return []string{
		"videos",
		"TikTokCringe",
		"Unexpected",
		"interestingasfuck",
		"oddlysatisfying",
		"nextfuckinglevel",
		"PublicFreakout",
		"aww",
		"funny",
		"gaming",
		"sports",
		"MadeMeSmile",
		"Damnthatsinteresting",
		"BeAmazed",
		"woahdude",
	}
}

// CollectViralVideos 바이럴 영상 수집 (여러 서브레딧에서)
func (r *RedditCollector) CollectViralVideos(ctx context.Context, maxPerSubreddit int) ([]*CollectedVideo, error) {
	subreddits := r.GetPopularSubreddits()
	var allVideos []*CollectedVideo

	for _, sub := range subreddits {
		fmt.Printf("  🔍 r/%s 검색 중...\n", sub)
		
		videos, err := r.DownloadFromSubreddit(ctx, sub, "hot", maxPerSubreddit)
		if err != nil {
			fmt.Printf("  ⚠️ r/%s 실패: %v\n", sub, err)
			continue
		}
		
		allVideos = append(allVideos, videos...)
	}

	return allVideos, nil
}

