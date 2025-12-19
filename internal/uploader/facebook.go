package uploader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/clipbot/internal/config"
)

// FacebookUploader 페이스북 업로더
type FacebookUploader struct {
	cfg           *config.FacebookConfig
	client        *http.Client
	authenticated bool
}

// NewFacebookUploader 새 페이스북 업로더 생성
func NewFacebookUploader(cfg *config.FacebookConfig) *FacebookUploader {
	return &FacebookUploader{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Minute},
	}
}

// Platform 플랫폼 이름 반환
func (f *FacebookUploader) Platform() Platform {
	return PlatformFacebook
}

// IsAuthenticated 인증 상태 확인
func (f *FacebookUploader) IsAuthenticated() bool {
	return f.authenticated || f.cfg.AccessToken != ""
}

// Authenticate 인증 수행
func (f *FacebookUploader) Authenticate(ctx context.Context) error {
	if f.cfg.AccessToken == "" {
		return fmt.Errorf("Facebook Access Token이 필요합니다. https://developers.facebook.com 에서 발급받으세요")
	}

	// 토큰 유효성 검사
	url := fmt.Sprintf("https://graph.facebook.com/me?access_token=%s", f.cfg.AccessToken)
	resp, err := f.client.Get(url)
	if err != nil {
		return fmt.Errorf("토큰 확인 실패: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("유효하지 않은 Access Token입니다")
	}

	f.authenticated = true
	return nil
}

// Upload 릴스 업로드
func (f *FacebookUploader) Upload(ctx context.Context, videoPath string, opts *UploadOptions) (*UploadResult, error) {
	if !f.IsAuthenticated() {
		if err := f.Authenticate(ctx); err != nil {
			return nil, err
		}
	}

	// 파일 존재 확인
	file, err := os.Open(videoPath)
	if err != nil {
		return nil, fmt.Errorf("파일 열기 실패: %w", err)
	}
	defer file.Close()

	fileInfo, _ := file.Stat()

	// 설명 준비
	description := opts.Title
	if opts.Description != "" {
		description += "\n\n" + opts.Description
	}

	// 해시태그 추가
	if len(opts.Hashtags) > 0 {
		description += "\n\n"
		for _, tag := range opts.Hashtags {
			if !strings.HasPrefix(tag, "#") {
				tag = "#" + tag
			}
			description += tag + " "
		}
	}

	for _, tag := range f.cfg.DefaultTags {
		if !strings.HasPrefix(tag, "#") {
			tag = "#" + tag
		}
		description += tag + " "
	}

	// 1. 업로드 세션 시작
	sessionURL := fmt.Sprintf(
		"https://graph.facebook.com/v18.0/%s/video_reels?upload_phase=start&access_token=%s",
		f.cfg.PageID,
		f.cfg.AccessToken,
	)

	sessionResp, err := f.client.Post(sessionURL, "application/json", nil)
	if err != nil {
		return nil, fmt.Errorf("업로드 세션 시작 실패: %w", err)
	}
	defer sessionResp.Body.Close()

	var sessionResult struct {
		VideoID  string `json:"video_id"`
		UploadID string `json:"upload_id"`
	}
	if err := json.NewDecoder(sessionResp.Body).Decode(&sessionResult); err != nil {
		return nil, fmt.Errorf("세션 응답 파싱 실패: %w", err)
	}

	// 2. 파일 업로드
	fmt.Println("📤 Facebook에 업로드 중...")

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("video_file_chunk", filepath.Base(videoPath))
	if err != nil {
		return nil, err
	}
	io.Copy(part, file)

	writer.WriteField("upload_phase", "transfer")
	writer.WriteField("start_offset", "0")
	writer.WriteField("upload_id", sessionResult.UploadID)
	writer.Close()

	uploadURL := fmt.Sprintf(
		"https://graph.facebook.com/v18.0/%s/video_reels?access_token=%s",
		f.cfg.PageID,
		f.cfg.AccessToken,
	)

	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	uploadResp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("파일 업로드 실패: %w", err)
	}
	defer uploadResp.Body.Close()

	// 3. 업로드 완료
	finishData := map[string]interface{}{
		"upload_phase": "finish",
		"video_id":     sessionResult.VideoID,
		"title":        opts.Title,
		"description":  description,
		"video_state":  "PUBLISHED",
	}

	finishJSON, _ := json.Marshal(finishData)
	finishURL := fmt.Sprintf(
		"https://graph.facebook.com/v18.0/%s/video_reels?access_token=%s",
		f.cfg.PageID,
		f.cfg.AccessToken,
	)

	finishResp, err := f.client.Post(finishURL, "application/json", bytes.NewReader(finishJSON))
	if err != nil {
		return nil, fmt.Errorf("업로드 완료 실패: %w", err)
	}
	defer finishResp.Body.Close()

	var finishResult struct {
		Success bool   `json:"success"`
		VideoID string `json:"id"`
	}
	json.NewDecoder(finishResp.Body).Decode(&finishResult)

	result := &UploadResult{
		Platform:   PlatformFacebook,
		Success:    true,
		VideoID:    sessionResult.VideoID,
		URL:        fmt.Sprintf("https://www.facebook.com/reel/%s", sessionResult.VideoID),
		UploadedAt: time.Now(),
	}

	_ = fileInfo // 나중에 진행률 표시에 사용 가능

	fmt.Printf("✅ Facebook Reels 업로드 완료! %s\n", result.URL)
	return result, nil
}

// GetStatus 업로드 상태 확인
func (f *FacebookUploader) GetStatus(ctx context.Context, videoID string) (*UploadResult, error) {
	if !f.IsAuthenticated() {
		return nil, fmt.Errorf("인증되지 않음")
	}

	url := fmt.Sprintf(
		"https://graph.facebook.com/v18.0/%s?fields=views,likes.summary(true),comments.summary(true)&access_token=%s",
		videoID,
		f.cfg.AccessToken,
	)

	resp, err := f.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var videoData struct {
		Views int64 `json:"views"`
		Likes struct {
			Summary struct {
				TotalCount int64 `json:"total_count"`
			} `json:"summary"`
		} `json:"likes"`
		Comments struct {
			Summary struct {
				TotalCount int64 `json:"total_count"`
			} `json:"summary"`
		} `json:"comments"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&videoData); err != nil {
		return nil, err
	}

	return &UploadResult{
		Platform: PlatformFacebook,
		Success:  true,
		VideoID:  videoID,
		Views:    videoData.Views,
		Likes:    videoData.Likes.Summary.TotalCount,
		Comments: videoData.Comments.Summary.TotalCount,
	}, nil
}

// Delete 영상 삭제
func (f *FacebookUploader) Delete(ctx context.Context, videoID string) error {
	if !f.IsAuthenticated() {
		return fmt.Errorf("인증되지 않음")
	}

	url := fmt.Sprintf(
		"https://graph.facebook.com/v18.0/%s?access_token=%s",
		videoID,
		f.cfg.AccessToken,
	)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("삭제 실패: %d", resp.StatusCode)
	}

	return nil
}

