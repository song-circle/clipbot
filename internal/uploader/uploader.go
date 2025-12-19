package uploader

import (
	"context"
	"fmt"
	"time"

	"github.com/clipbot/internal/config"
)

// Platform 플랫폼 타입
type Platform string

const (
	PlatformTikTok    Platform = "tiktok"
	PlatformYouTube   Platform = "youtube"
	PlatformInstagram Platform = "instagram"
	PlatformFacebook  Platform = "facebook"
	PlatformTwitter   Platform = "twitter"
)

// UploadResult 업로드 결과
type UploadResult struct {
	Platform    Platform  `json:"platform"`
	Success     bool      `json:"success"`
	VideoID     string    `json:"video_id"`
	URL         string    `json:"url"`
	UploadedAt  time.Time `json:"uploaded_at"`
	Error       string    `json:"error,omitempty"`
	Views       int64     `json:"views"`
	Likes       int64     `json:"likes"`
	Comments    int64     `json:"comments"`
	Shares      int64     `json:"shares"`
}

// UploadOptions 업로드 옵션
type UploadOptions struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Hashtags    []string `json:"hashtags"`
	Thumbnail   string   `json:"thumbnail"`    // 썸네일 이미지 경로
	Privacy     string   `json:"privacy"`      // public, private, unlisted
	ScheduleAt  *time.Time `json:"schedule_at"` // 예약 업로드 시간
	Location    string   `json:"location"`     // 위치 태그
	Music       string   `json:"music"`        // 음악 ID (틱톡)
	Duet        bool     `json:"duet"`         // 듀엣 허용 (틱톡)
	Stitch      bool     `json:"stitch"`       // 스티치 허용 (틱톡)
	Comments    bool     `json:"comments"`     // 댓글 허용
	CategoryID  string   `json:"category_id"`  // 카테고리 (유튜브)
}

// Uploader 업로더 인터페이스
type Uploader interface {
	// Platform 플랫폼 이름 반환
	Platform() Platform

	// Upload 영상 업로드
	Upload(ctx context.Context, videoPath string, opts *UploadOptions) (*UploadResult, error)

	// GetStatus 업로드 상태 확인
	GetStatus(ctx context.Context, videoID string) (*UploadResult, error)

	// Delete 영상 삭제
	Delete(ctx context.Context, videoID string) error

	// IsAuthenticated 인증 상태 확인
	IsAuthenticated() bool

	// Authenticate 인증 수행
	Authenticate(ctx context.Context) error
}

// MultiUploader 멀티 플랫폼 업로더
type MultiUploader struct {
	uploaders map[Platform]Uploader
	cfg       *config.PlatformsConfig
}

// NewMultiUploader 새 멀티 업로더 생성
func NewMultiUploader(cfg *config.PlatformsConfig) *MultiUploader {
	mu := &MultiUploader{
		uploaders: make(map[Platform]Uploader),
		cfg:       cfg,
	}

	// 활성화된 플랫폼 업로더 등록
	if cfg.TikTok.Enabled {
		mu.uploaders[PlatformTikTok] = NewTikTokUploader(&cfg.TikTok)
	}
	if cfg.YouTube.Enabled {
		mu.uploaders[PlatformYouTube] = NewYouTubeUploader(&cfg.YouTube)
	}
	if cfg.Instagram.Enabled {
		mu.uploaders[PlatformInstagram] = NewInstagramUploader(&cfg.Instagram)
	}
	if cfg.Facebook.Enabled {
		mu.uploaders[PlatformFacebook] = NewFacebookUploader(&cfg.Facebook)
	}
	if cfg.Twitter.Enabled {
		mu.uploaders[PlatformTwitter] = NewTwitterUploader(&cfg.Twitter)
	}

	return mu
}

// UploadToAll 모든 활성화된 플랫폼에 업로드
func (m *MultiUploader) UploadToAll(ctx context.Context, videoPath string, opts *UploadOptions) map[Platform]*UploadResult {
	results := make(map[Platform]*UploadResult)

	for platform, uploader := range m.uploaders {
		result, err := uploader.Upload(ctx, videoPath, opts)
		if err != nil {
			results[platform] = &UploadResult{
				Platform: platform,
				Success:  false,
				Error:    err.Error(),
			}
		} else {
			results[platform] = result
		}
	}

	return results
}

// UploadTo 특정 플랫폼들에 업로드
func (m *MultiUploader) UploadTo(ctx context.Context, videoPath string, platforms []Platform, opts *UploadOptions) map[Platform]*UploadResult {
	results := make(map[Platform]*UploadResult)

	for _, platform := range platforms {
		uploader, exists := m.uploaders[platform]
		if !exists {
			results[platform] = &UploadResult{
				Platform: platform,
				Success:  false,
				Error:    fmt.Sprintf("플랫폼 %s이(가) 활성화되지 않았습니다", platform),
			}
			continue
		}

		result, err := uploader.Upload(ctx, videoPath, opts)
		if err != nil {
			results[platform] = &UploadResult{
				Platform: platform,
				Success:  false,
				Error:    err.Error(),
			}
		} else {
			results[platform] = result
		}
	}

	return results
}

// GetUploader 특정 플랫폼 업로더 가져오기
func (m *MultiUploader) GetUploader(platform Platform) (Uploader, bool) {
	uploader, exists := m.uploaders[platform]
	return uploader, exists
}

// GetEnabledPlatforms 활성화된 플랫폼 목록
func (m *MultiUploader) GetEnabledPlatforms() []Platform {
	platforms := make([]Platform, 0, len(m.uploaders))
	for p := range m.uploaders {
		platforms = append(platforms, p)
	}
	return platforms
}

// AuthenticateAll 모든 플랫폼 인증
func (m *MultiUploader) AuthenticateAll(ctx context.Context) map[Platform]error {
	errors := make(map[Platform]error)
	for platform, uploader := range m.uploaders {
		if err := uploader.Authenticate(ctx); err != nil {
			errors[platform] = err
		}
	}
	return errors
}

// ParsePlatforms 문자열 배열을 Platform 배열로 변환
func ParsePlatforms(names []string) []Platform {
	platforms := make([]Platform, 0, len(names))
	for _, name := range names {
		switch name {
		case "tiktok":
			platforms = append(platforms, PlatformTikTok)
		case "youtube":
			platforms = append(platforms, PlatformYouTube)
		case "instagram":
			platforms = append(platforms, PlatformInstagram)
		case "facebook":
			platforms = append(platforms, PlatformFacebook)
		case "twitter", "x":
			platforms = append(platforms, PlatformTwitter)
		case "all":
			return []Platform{PlatformTikTok, PlatformYouTube, PlatformInstagram, PlatformFacebook, PlatformTwitter}
		}
	}
	return platforms
}

// DefaultUploadOptions 기본 업로드 옵션
func DefaultUploadOptions() *UploadOptions {
	return &UploadOptions{
		Privacy:  "public",
		Duet:     true,
		Stitch:   true,
		Comments: true,
	}
}

