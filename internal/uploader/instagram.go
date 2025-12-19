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

// InstagramUploader 인스타그램 업로더
type InstagramUploader struct {
	cfg           *config.InstagramConfig
	browser       *rod.Browser
	authenticated bool
}

// NewInstagramUploader 새 인스타그램 업로더 생성
func NewInstagramUploader(cfg *config.InstagramConfig) *InstagramUploader {
	return &InstagramUploader{
		cfg: cfg,
	}
}

// Platform 플랫폼 이름 반환
func (i *InstagramUploader) Platform() Platform {
	return PlatformInstagram
}

// IsAuthenticated 인증 상태 확인
func (i *InstagramUploader) IsAuthenticated() bool {
	return i.authenticated
}

// Authenticate 인증 수행
func (i *InstagramUploader) Authenticate(ctx context.Context) error {
	// 브라우저 시작
	path, _ := launcher.LookPath()
	u := launcher.New().
		Bin(path).
		Headless(false).
		MustLaunch()

	i.browser = rod.New().ControlURL(u).MustConnect()

	// 인스타그램 로그인 페이지로 이동
	page := i.browser.MustPage("https://www.instagram.com/accounts/login/")
	page.MustWaitStable()

	// 세션 데이터가 있으면 복원
	if i.cfg.SessionData != "" {
		// 쿠키 복원 로직
		cookies := []*proto.NetworkCookieParam{
			{
				Name:   "sessionid",
				Value:  i.cfg.SessionData,
				Domain: ".instagram.com",
				Path:   "/",
			},
		}
		page.MustSetCookies(cookies...)
		page.MustReload()
	} else if i.cfg.Username != "" && i.cfg.Password != "" {
		// 자동 로그인
		time.Sleep(2 * time.Second)

		// 쿠키 동의 닫기
		if allowBtn, err := page.Timeout(3 * time.Second).Element("button:has-text('허용')"); err == nil {
			allowBtn.MustClick()
		}

		// 로그인 폼 입력
		usernameInput := page.MustElement("input[name='username']")
		usernameInput.MustInput(i.cfg.Username)

		passwordInput := page.MustElement("input[name='password']")
		passwordInput.MustInput(i.cfg.Password)

		// 로그인 버튼 클릭
		loginBtn := page.MustElement("button[type='submit']")
		loginBtn.MustClick()

		// 로그인 완료 대기
		time.Sleep(5 * time.Second)
	}

	// 로그인 상태 확인
	page.MustNavigate("https://www.instagram.com/")
	page.MustWaitStable()

	// 로그인 되어있는지 확인
	if _, err := page.Timeout(5 * time.Second).Element("svg[aria-label='새로운 게시물']"); err == nil {
		i.authenticated = true
		fmt.Println("✅ 인스타그램 로그인 완료!")
		return nil
	}

	// 수동 로그인 대기
	fmt.Println("⏳ 인스타그램 로그인을 완료해주세요...")
	for j := 0; j < 24; j++ {
		time.Sleep(5 * time.Second)
		if _, err := page.Timeout(3 * time.Second).Element("svg[aria-label='새로운 게시물']"); err == nil {
			i.authenticated = true
			fmt.Println("✅ 인스타그램 로그인 완료!")
			return nil
		}
	}

	return fmt.Errorf("인스타그램 로그인 시간 초과")
}

// Upload 릴스 업로드
func (i *InstagramUploader) Upload(ctx context.Context, videoPath string, opts *UploadOptions) (*UploadResult, error) {
	if !i.authenticated {
		if err := i.Authenticate(ctx); err != nil {
			return nil, err
		}
	}

	// 파일 존재 확인
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("영상 파일을 찾을 수 없습니다: %s", videoPath)
	}

	page := i.browser.MustPage("https://www.instagram.com/")
	page.MustWaitStable()

	// 새 게시물 버튼 클릭
	newPostBtn := page.MustElement("svg[aria-label='새로운 게시물']")
	newPostBtn.MustClick()
	time.Sleep(2 * time.Second)

	// 파일 선택
	fileInput := page.MustElement("input[type='file']")
	fileInput.MustSetFiles(videoPath)
	time.Sleep(3 * time.Second)

	// 릴스 선택 (비디오인 경우 자동으로 릴스 옵션 표시)
	if reelsBtn, err := page.Timeout(3 * time.Second).Element("button:has-text('릴스')"); err == nil {
		reelsBtn.MustClick()
		time.Sleep(1 * time.Second)
	}

	// 다음 버튼 클릭
	nextBtn := page.MustElement("button:has-text('다음')")
	nextBtn.MustClick()
	time.Sleep(2 * time.Second)

	// 필터/편집 단계 건너뛰기
	if nextBtn2, err := page.Timeout(3 * time.Second).Element("button:has-text('다음')"); err == nil {
		nextBtn2.MustClick()
		time.Sleep(2 * time.Second)
	}

	// 캡션 입력
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

	// 기본 태그 추가
	for _, tag := range i.cfg.DefaultTags {
		if !strings.HasPrefix(tag, "#") {
			tag = "#" + tag
		}
		caption += tag + " "
	}

	captionArea := page.MustElement("textarea[aria-label='문구를 입력하세요...']")
	captionArea.MustInput(caption)

	// 위치 추가
	if opts.Location != "" {
		locationBtn := page.MustElement("button:has-text('위치 추가')")
		locationBtn.MustClick()
		time.Sleep(1 * time.Second)

		locationInput := page.MustElement("input[placeholder='위치 검색']")
		locationInput.MustInput(opts.Location)
		time.Sleep(2 * time.Second)

		// 첫 번째 결과 선택
		if firstLocation, err := page.Timeout(3 * time.Second).Element("div[role='button']"); err == nil {
			firstLocation.MustClick()
		}
	}

	// 공유 버튼 클릭
	shareBtn := page.MustElement("button:has-text('공유')")
	shareBtn.MustClick()

	fmt.Println("⏳ 릴스 업로드 중...")

	// 업로드 완료 대기
	time.Sleep(15 * time.Second)

	result := &UploadResult{
		Platform:   PlatformInstagram,
		Success:    true,
		UploadedAt: time.Now(),
	}

	// 완료 메시지 확인
	if _, err := page.Timeout(30 * time.Second).Element("span:has-text('릴스가 공유되었습니다')"); err == nil {
		fmt.Println("✅ 인스타그램 릴스 업로드 완료!")
	}

	return result, nil
}

// GetStatus 업로드 상태 확인
func (i *InstagramUploader) GetStatus(ctx context.Context, videoID string) (*UploadResult, error) {
	// Instagram은 API 접근이 제한적이므로 기본 정보만 반환
	return &UploadResult{
		Platform: PlatformInstagram,
		VideoID:  videoID,
		Success:  true,
	}, nil
}

// Delete 영상 삭제
func (i *InstagramUploader) Delete(ctx context.Context, videoID string) error {
	// Instagram 삭제는 수동으로 해야 함
	return fmt.Errorf("인스타그램은 자동 삭제를 지원하지 않습니다")
}

// Close 브라우저 닫기
func (i *InstagramUploader) Close() {
	if i.browser != nil {
		i.browser.MustClose()
	}
}

