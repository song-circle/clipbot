package uploader

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/clipbot/internal/config"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// TikTokUploader 틱톡 업로더
type TikTokUploader struct {
	cfg           *config.TikTokConfig
	browser       *rod.Browser
	authenticated bool
}

// NewTikTokUploader 새 틱톡 업로더 생성
func NewTikTokUploader(cfg *config.TikTokConfig) *TikTokUploader {
	return &TikTokUploader{
		cfg: cfg,
	}
}

// Platform 플랫폼 이름 반환
func (t *TikTokUploader) Platform() Platform {
	return PlatformTikTok
}

// IsAuthenticated 인증 상태 확인
func (t *TikTokUploader) IsAuthenticated() bool {
	return t.authenticated
}

// Authenticate 인증 수행
func (t *TikTokUploader) Authenticate(ctx context.Context) error {
	// 브라우저 시작
	path, _ := launcher.LookPath()
	u := launcher.New().
		Bin(path).
		Headless(false). // 로그인을 위해 headless 비활성화
		MustLaunch()

	t.browser = rod.New().ControlURL(u).MustConnect()

	// 틱톡 로그인 페이지로 이동
	page := t.browser.MustPage("https://www.tiktok.com/login")

	// 세션 쿠키가 있으면 설정
	if t.cfg.SessionID != "" {
		cookies := []*proto.NetworkCookieParam{
			{
				Name:   "sessionid",
				Value:  t.cfg.SessionID,
				Domain: ".tiktok.com",
				Path:   "/",
			},
		}
		page.MustSetCookies(cookies...)
		page.MustReload()
	}

	// 로그인 상태 확인
	page.MustWaitStable()

	// 로그인 되어있는지 확인
	_, err := page.Timeout(5 * time.Second).Element("[data-e2e='upload-icon']")
	if err == nil {
		t.authenticated = true
		return nil
	}

	// 수동 로그인 대기 (최대 2분)
	fmt.Println("⏳ 틱톡 로그인을 완료해주세요...")
	for i := 0; i < 24; i++ {
		time.Sleep(5 * time.Second)
		_, err := page.Timeout(3 * time.Second).Element("[data-e2e='upload-icon']")
		if err == nil {
			t.authenticated = true
			fmt.Println("✅ 틱톡 로그인 완료!")
			return nil
		}
	}

	return fmt.Errorf("틱톡 로그인 시간 초과")
}

// Upload 영상 업로드
func (t *TikTokUploader) Upload(ctx context.Context, videoPath string, opts *UploadOptions) (*UploadResult, error) {
	if !t.authenticated {
		if err := t.Authenticate(ctx); err != nil {
			return nil, err
		}
	}

	// 파일 존재 확인
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("영상 파일을 찾을 수 없습니다: %s", videoPath)
	}

	// 업로드 페이지로 이동
	page := t.browser.MustPage("https://www.tiktok.com/upload")
	page.MustWaitStable()

	// 파일 업로드
	fileInput := page.MustElement("input[type='file']")
	fileInput.MustSetFiles(videoPath)

	// 업로드 완료 대기
	fmt.Println("📤 영상 업로드 중...")
	page.MustWaitStable()
	time.Sleep(5 * time.Second)

	// 제목/설명 입력
	caption := opts.Title
	if opts.Description != "" {
		caption += "\n\n" + opts.Description
	}

	// 해시태그 추가
	if len(opts.Hashtags) > 0 {
		caption += "\n\n"
		for _, tag := range opts.Hashtags {
			if !strings.HasPrefix(tag, "#") {
				tag = "#" + tag
			}
			caption += tag + " "
		}
	}

	// 기본 해시태그 추가
	if len(t.cfg.DefaultTags) > 0 {
		for _, tag := range t.cfg.DefaultTags {
			if !strings.HasPrefix(tag, "#") {
				tag = "#" + tag
			}
			caption += tag + " "
		}
	}

	// 캡션 입력
	captionEditor := page.MustElement("[data-e2e='caption-editor']")
	captionEditor.MustClick()
	captionEditor.MustInput(caption)

	// 공개 설정
	if opts.Privacy != "" && opts.Privacy != "public" {
		// 공개 설정 드롭다운 클릭
		privacyBtn := page.MustElement("[data-e2e='privacy-level']")
		privacyBtn.MustClick()
		time.Sleep(500 * time.Millisecond)

		switch opts.Privacy {
		case "private":
			page.MustElement("[data-e2e='privacy-private']").MustClick()
		case "friends":
			page.MustElement("[data-e2e='privacy-friends']").MustClick()
		}
	}

	// 듀엣/스티치 설정
	if !opts.Duet {
		duetToggle := page.MustElement("[data-e2e='duet-toggle']")
		duetToggle.MustClick()
	}
	if !opts.Stitch {
		stitchToggle := page.MustElement("[data-e2e='stitch-toggle']")
		stitchToggle.MustClick()
	}

	// 댓글 설정
	if !opts.Comments {
		commentToggle := page.MustElement("[data-e2e='comment-toggle']")
		commentToggle.MustClick()
	}

	// 게시 버튼 클릭
	time.Sleep(2 * time.Second)
	postBtn := page.MustElement("[data-e2e='post-btn']")
	postBtn.MustClick()

	// 업로드 완료 대기
	fmt.Println("⏳ 게시 처리 중...")
	time.Sleep(10 * time.Second)

	result := &UploadResult{
		Platform:   PlatformTikTok,
		Success:    true,
		UploadedAt: time.Now(),
	}

	// URL 가져오기 시도
	page.MustWaitStable()
	currentURL := page.MustInfo().URL
	if strings.Contains(currentURL, "/video/") {
		result.URL = currentURL
		// Video ID 추출
		parts := strings.Split(currentURL, "/video/")
		if len(parts) > 1 {
			result.VideoID = strings.Split(parts[1], "?")[0]
		}
	}

	fmt.Printf("✅ 틱톡 업로드 완료! %s\n", result.URL)
	return result, nil
}

// GetStatus 업로드 상태 확인
func (t *TikTokUploader) GetStatus(ctx context.Context, videoID string) (*UploadResult, error) {
	if !t.authenticated {
		return nil, fmt.Errorf("인증되지 않음")
	}

	url := fmt.Sprintf("https://www.tiktok.com/@%s/video/%s", t.cfg.Username, videoID)
	page := t.browser.MustPage(url)
	page.MustWaitStable()

	result := &UploadResult{
		Platform: PlatformTikTok,
		VideoID:  videoID,
		URL:      url,
		Success:  true,
	}

	// 조회수, 좋아요, 댓글 수 가져오기 시도
	// TikTok의 DOM 구조에 따라 선택자 조정 필요
	if viewsEl, err := page.Element("[data-e2e='video-views']"); err == nil {
		viewsText := viewsEl.MustText()
		result.Views = parseMetricValue(viewsText)
	}

	if likesEl, err := page.Element("[data-e2e='like-count']"); err == nil {
		likesText := likesEl.MustText()
		result.Likes = parseMetricValue(likesText)
	}

	if commentsEl, err := page.Element("[data-e2e='comment-count']"); err == nil {
		commentsText := commentsEl.MustText()
		result.Comments = parseMetricValue(commentsText)
	}

	return result, nil
}

// Delete 영상 삭제
func (t *TikTokUploader) Delete(ctx context.Context, videoID string) error {
	if !t.authenticated {
		return fmt.Errorf("인증되지 않음")
	}

	url := fmt.Sprintf("https://www.tiktok.com/@%s/video/%s", t.cfg.Username, videoID)
	page := t.browser.MustPage(url)
	page.MustWaitStable()

	// 더보기 버튼 클릭
	moreBtn := page.MustElement("[data-e2e='more-btn']")
	moreBtn.MustClick()
	time.Sleep(500 * time.Millisecond)

	// 삭제 버튼 클릭
	deleteBtn := page.MustElement("[data-e2e='delete-btn']")
	deleteBtn.MustClick()
	time.Sleep(500 * time.Millisecond)

	// 확인 버튼 클릭
	confirmBtn := page.MustElement("[data-e2e='confirm-delete']")
	confirmBtn.MustClick()

	time.Sleep(2 * time.Second)
	return nil
}

// Close 브라우저 닫기
func (t *TikTokUploader) Close() {
	if t.browser != nil {
		t.browser.MustClose()
	}
}

// parseMetricValue 조회수/좋아요 수 파싱 (K, M 등 처리)
func parseMetricValue(text string) int64 {
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, ",", "")

	multiplier := int64(1)
	if strings.HasSuffix(text, "K") || strings.HasSuffix(text, "k") {
		multiplier = 1000
		text = text[:len(text)-1]
	} else if strings.HasSuffix(text, "M") || strings.HasSuffix(text, "m") {
		multiplier = 1000000
		text = text[:len(text)-1]
	} else if strings.HasSuffix(text, "B") || strings.HasSuffix(text, "b") {
		multiplier = 1000000000
		text = text[:len(text)-1]
	}

	var value float64
	fmt.Sscanf(text, "%f", &value)
	return int64(value * float64(multiplier))
}

