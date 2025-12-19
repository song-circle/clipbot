package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// StockVideoCollector 저작권 프리 스톡 영상 수집기
type StockVideoCollector struct {
	cfg        *CollectorConfig
	pexelsKey  string // Pexels API 키 (무료)
	pixabayKey string // Pixabay API 키 (무료)
	client     *http.Client
}

// StockSource 스톡 영상 소스
type StockSource string

const (
	StockPexels  StockSource = "pexels"
	StockPixabay StockSource = "pixabay"
	StockCoverr  StockSource = "coverr"
	StockMixkit  StockSource = "mixkit"
)

// PexelsResponse Pexels API 응답
type PexelsResponse struct {
	Videos []struct {
		ID       int    `json:"id"`
		URL      string `json:"url"`
		Duration int    `json:"duration"`
		User     struct {
			Name string `json:"name"`
		} `json:"user"`
		VideoFiles []struct {
			ID       int    `json:"id"`
			Quality  string `json:"quality"`
			FileType string `json:"file_type"`
			Link     string `json:"link"`
			Width    int    `json:"width"`
			Height   int    `json:"height"`
		} `json:"video_files"`
		VideoPictures []struct {
			Picture string `json:"picture"`
		} `json:"video_pictures"`
	} `json:"videos"`
	TotalResults int `json:"total_results"`
}

// PixabayResponse Pixabay API 응답
type PixabayResponse struct {
	Hits []struct {
		ID       int    `json:"id"`
		PageURL  string `json:"pageURL"`
		Duration int    `json:"duration"`
		User     string `json:"user"`
		Videos   struct {
			Large struct {
				URL    string `json:"url"`
				Width  int    `json:"width"`
				Height int    `json:"height"`
			} `json:"large"`
			Medium struct {
				URL    string `json:"url"`
				Width  int    `json:"width"`
				Height int    `json:"height"`
			} `json:"medium"`
		} `json:"videos"`
	} `json:"hits"`
	TotalHits int `json:"totalHits"`
}

// NewStockVideoCollector 새 스톡 영상 수집기 생성
func NewStockVideoCollector(cfg *CollectorConfig, pexelsKey, pixabayKey string) *StockVideoCollector {
	return &StockVideoCollector{
		cfg:        cfg,
		pexelsKey:  pexelsKey,
		pixabayKey: pixabayKey,
		client:     &http.Client{Timeout: 60 * time.Second},
	}
}

// SearchPexels Pexels에서 영상 검색 (무료, 상업용 가능, 크레딧 권장)
func (s *StockVideoCollector) SearchPexels(ctx context.Context, query string, count int) ([]*CollectedVideo, error) {
	if s.pexelsKey == "" {
		return nil, fmt.Errorf("Pexels API 키가 필요합니다. https://www.pexels.com/api/ 에서 무료로 발급받으세요")
	}

	apiURL := fmt.Sprintf(
		"https://api.pexels.com/videos/search?query=%s&per_page=%d&orientation=portrait",
		url.QueryEscape(query),
		count,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", s.pexelsKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Pexels API 요청 실패: %w", err)
	}
	defer resp.Body.Close()

	var pexelsResp PexelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&pexelsResp); err != nil {
		return nil, err
	}

	var videos []*CollectedVideo
	for _, v := range pexelsResp.Videos {
		// 세로 영상 우선
		var bestFile struct {
			Link   string
			Width  int
			Height int
		}
		for _, f := range v.VideoFiles {
			if f.Quality == "hd" && f.Height > f.Width { // 세로 영상
				bestFile.Link = f.Link
				bestFile.Width = f.Width
				bestFile.Height = f.Height
				break
			}
			if f.Quality == "hd" {
				bestFile.Link = f.Link
				bestFile.Width = f.Width
				bestFile.Height = f.Height
			}
		}

		if bestFile.Link == "" && len(v.VideoFiles) > 0 {
			bestFile.Link = v.VideoFiles[0].Link
		}

		video := &CollectedVideo{
			ID:          fmt.Sprintf("pexels_%d", v.ID),
			Source:      "pexels",
			URL:         bestFile.Link,
			Title:       query,
			Description: fmt.Sprintf("Video by %s on Pexels (Free to use, no attribution required)", v.User.Name),
			Author:      v.User.Name,
			Duration:    v.Duration,
			CollectedAt: time.Now(),
			Tags:        []string{"pexels", "royalty-free", "stock", query},
		}

		if len(v.VideoPictures) > 0 {
			video.Thumbnail = v.VideoPictures[0].Picture
		}

		videos = append(videos, video)
	}

	return videos, nil
}

// SearchPixabay Pixabay에서 영상 검색 (무료, 상업용 가능, 크레딧 불필요)
func (s *StockVideoCollector) SearchPixabay(ctx context.Context, query string, count int) ([]*CollectedVideo, error) {
	if s.pixabayKey == "" {
		return nil, fmt.Errorf("Pixabay API 키가 필요합니다. https://pixabay.com/api/docs/ 에서 무료로 발급받으세요")
	}

	// Pixabay API는 per_page 최소값이 3
	perPage := count
	if perPage < 3 {
		perPage = 3
	}

	apiURL := fmt.Sprintf(
		"https://pixabay.com/api/videos/?key=%s&q=%s&per_page=%d",
		s.pixabayKey,
		url.QueryEscape(query),
		perPage,
	)

	resp, err := s.client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("Pixabay API 요청 실패: %w", err)
	}
	defer resp.Body.Close()

	var pixabayResp PixabayResponse
	if err := json.NewDecoder(resp.Body).Decode(&pixabayResp); err != nil {
		return nil, err
	}

	var videos []*CollectedVideo
	for i, v := range pixabayResp.Hits {
		// 요청한 개수만큼만 반환
		if i >= count {
			break
		}

		videoURL := v.Videos.Large.URL
		if videoURL == "" {
			videoURL = v.Videos.Medium.URL
		}

		video := &CollectedVideo{
			ID:          fmt.Sprintf("pixabay_%d", v.ID),
			Source:      "pixabay",
			URL:         videoURL,
			Title:       query,
			Description: fmt.Sprintf("Video by %s on Pixabay (Free for commercial use, no attribution required)", v.User),
			Author:      v.User,
			Duration:    v.Duration,
			CollectedAt: time.Now(),
			Tags:        []string{"pixabay", "royalty-free", "stock", "cc0", query},
		}

		videos = append(videos, video)
	}

	return videos, nil
}

// DownloadStockVideo 스톡 영상 다운로드
func (s *StockVideoCollector) DownloadStockVideo(ctx context.Context, video *CollectedVideo) error {
	outputDir := s.cfg.OutputDir
	if outputDir == "" {
		outputDir = "./input"
	}
	os.MkdirAll(outputDir, 0755)

	// 파일명 생성
	safeTitle := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, video.Title)
	if len(safeTitle) > 30 {
		safeTitle = safeTitle[:30]
	}

	filename := fmt.Sprintf("%s_%s.mp4", safeTitle, video.ID)
	outputPath := filepath.Join(outputDir, filename)

	// 다운로드
	fmt.Printf("  📥 다운로드: %s\n", video.URL)

	resp, err := s.client.Get(video.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}

	video.FilePath = outputPath
	fmt.Printf("  ✅ 저장됨: %s\n", outputPath)

	return nil
}

// SearchAndDownload 검색 후 다운로드
func (s *StockVideoCollector) SearchAndDownload(ctx context.Context, source StockSource, query string, count int) ([]*CollectedVideo, error) {
	var videos []*CollectedVideo
	var err error

	switch source {
	case StockPexels:
		videos, err = s.SearchPexels(ctx, query, count)
	case StockPixabay:
		videos, err = s.SearchPixabay(ctx, query, count)
	default:
		return nil, fmt.Errorf("지원하지 않는 소스: %s", source)
	}

	if err != nil {
		return nil, err
	}

	var downloaded []*CollectedVideo
	for _, video := range videos {
		if err := s.DownloadStockVideo(ctx, video); err != nil {
			fmt.Printf("  ⚠️ 다운로드 실패: %v\n", err)
			continue
		}
		downloaded = append(downloaded, video)
	}

	return downloaded, nil
}

// GetPopularCategories 인기 카테고리 반환
func (s *StockVideoCollector) GetPopularCategories() map[string][]string {
	return map[string][]string{
		"satisfying": {
			"satisfying",
			"asmr",
			"slime",
			"soap cutting",
			"sand",
			"kinetic sand",
		},
		"nature": {
			"nature",
			"ocean waves",
			"sunset",
			"rain",
			"waterfall",
			"forest",
			"mountains",
		},
		"food": {
			"cooking",
			"food",
			"coffee",
			"baking",
			"chocolate",
		},
		"aesthetic": {
			"aesthetic",
			"neon lights",
			"city lights",
			"abstract",
			"minimalist",
		},
		"motivation": {
			"workout",
			"running",
			"success",
			"meditation",
			"yoga",
		},
		"animals": {
			"cute animals",
			"cat",
			"dog",
			"wildlife",
			"birds",
		},
		"technology": {
			"technology",
			"coding",
			"computer",
			"smartphone",
			"futuristic",
		},
	}
}

// CollectYouTubeCC YouTube Creative Commons 영상 수집
func (s *StockVideoCollector) CollectYouTubeCC(ctx context.Context, query string, count int) ([]*CollectedVideo, error) {
	// YouTube Creative Commons 라이선스 영상 검색
	// 참고: YouTube Data API로 creativeCommon 필터 사용 가능

	ytdlpPath := s.cfg.YtDlpPath
	if ytdlpPath == "" {
		ytdlpPath = "yt-dlp"
	}

	outputDir := s.cfg.OutputDir
	if outputDir == "" {
		outputDir = "./input"
	}
	os.MkdirAll(outputDir, 0755)

	// Creative Commons 영상 검색 (ytsearch와 함께 CC 필터는 API 필요)
	// 대안: "creative commons" 키워드 추가
	searchQuery := fmt.Sprintf("%s creative commons", query)

	args := []string{
		fmt.Sprintf("ytsearch%d:%s", count, searchQuery),
		"-o", filepath.Join(outputDir, "cc_%(title).40s_%(id)s.%(ext)s"),
		"-f", "best[ext=mp4][vcodec!=none][acodec!=none]/best",
		"--match-filter", fmt.Sprintf("duration <= %d", s.cfg.MaxDuration),
		"--max-downloads", fmt.Sprintf("%d", count),
		"--no-warnings",
		"--ignore-errors",
	}

	fmt.Printf("  🔍 YouTube CC 검색: %s\n", query)

	cmd := exec.CommandContext(ctx, ytdlpPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	return s.scanDownloaded(outputDir)
}

// scanDownloaded 다운로드된 파일 스캔
func (s *StockVideoCollector) scanDownloaded(dir string) ([]*CollectedVideo, error) {
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
			Source:      "stock",
			Title:       strings.TrimSuffix(entry.Name(), ext),
			FilePath:    filepath.Join(dir, entry.Name()),
			CollectedAt: time.Now(),
			Tags:        []string{"royalty-free"},
		}

		videos = append(videos, video)
	}

	return videos, nil
}

// GetLicenseInfo 라이선스 정보 반환
func GetLicenseInfo(source StockSource) string {
	licenses := map[StockSource]string{
		StockPexels:  "✅ Pexels License: 무료, 상업용 가능, 크레딧 권장 (필수 아님)",
		StockPixabay: "✅ Pixabay License: 무료, 상업용 가능, 크레딧 불필요, CC0 유사",
		StockCoverr:  "✅ Coverr License: 무료, 상업용 가능, 크레딧 불필요",
		StockMixkit:  "✅ Mixkit License: 무료, 상업용 가능, 크레딧 불필요",
	}

	if info, exists := licenses[source]; exists {
		return info
	}
	return "⚠️ 라이선스 확인 필요"
}
