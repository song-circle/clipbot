package uploader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/clipbot/internal/config"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// YouTubeUploader 유튜브 업로더
type YouTubeUploader struct {
	cfg           *config.YouTubeConfig
	client        *http.Client
	authenticated bool
}

// YouTubeVideo 유튜브 비디오 정보
type YouTubeVideo struct {
	ID      string `json:"id"`
	Snippet struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Tags        []string `json:"tags"`
		CategoryID  string `json:"categoryId"`
	} `json:"snippet"`
	Status struct {
		PrivacyStatus string `json:"privacyStatus"`
	} `json:"status"`
	Statistics struct {
		ViewCount    string `json:"viewCount"`
		LikeCount    string `json:"likeCount"`
		CommentCount string `json:"commentCount"`
	} `json:"statistics"`
}

// NewYouTubeUploader 새 유튜브 업로더 생성
func NewYouTubeUploader(cfg *config.YouTubeConfig) *YouTubeUploader {
	return &YouTubeUploader{
		cfg: cfg,
	}
}

// Platform 플랫폼 이름 반환
func (y *YouTubeUploader) Platform() Platform {
	return PlatformYouTube
}

// IsAuthenticated 인증 상태 확인
func (y *YouTubeUploader) IsAuthenticated() bool {
	return y.authenticated
}

// Authenticate OAuth2 인증 수행
func (y *YouTubeUploader) Authenticate(ctx context.Context) error {
	config := &oauth2.Config{
		ClientID:     y.cfg.ClientID,
		ClientSecret: y.cfg.ClientSecret,
		Endpoint:     google.Endpoint,
		Scopes: []string{
			"https://www.googleapis.com/auth/youtube.upload",
			"https://www.googleapis.com/auth/youtube",
		},
		RedirectURL: "http://localhost:8080/callback",
	}

	// 이미 리프레시 토큰이 있는 경우
	if y.cfg.RefreshToken != "" {
		token := &oauth2.Token{
			RefreshToken: y.cfg.RefreshToken,
		}
		y.client = config.Client(ctx, token)
		y.authenticated = true
		return nil
	}

	// 새로운 인증 플로우
	authURL := config.AuthCodeURL("state", oauth2.AccessTypeOffline)
	fmt.Printf("🔑 브라우저에서 다음 URL을 열어 인증해주세요:\n%s\n", authURL)

	// 콜백 서버 시작
	codeChan := make(chan string)
	server := &http.Server{Addr: ":8080"}

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		codeChan <- code
		fmt.Fprintf(w, "✅ 인증 완료! 이 창을 닫아도 됩니다.")
	})

	go server.ListenAndServe()

	// 인증 코드 대기
	select {
	case code := <-codeChan:
		token, err := config.Exchange(ctx, code)
		if err != nil {
			return fmt.Errorf("토큰 교환 실패: %w", err)
		}
		y.client = config.Client(ctx, token)
		y.authenticated = true
		fmt.Printf("💾 리프레시 토큰을 config.yaml에 저장하세요: %s\n", token.RefreshToken)
	case <-time.After(5 * time.Minute):
		return fmt.Errorf("인증 시간 초과")
	}

	server.Shutdown(ctx)
	return nil
}

// Upload 영상 업로드 (YouTube Shorts)
func (y *YouTubeUploader) Upload(ctx context.Context, videoPath string, opts *UploadOptions) (*UploadResult, error) {
	if !y.authenticated {
		if err := y.Authenticate(ctx); err != nil {
			return nil, err
		}
	}

	// 파일 열기
	file, err := os.Open(videoPath)
	if err != nil {
		return nil, fmt.Errorf("파일 열기 실패: %w", err)
	}
	defer file.Close()

	// 메타데이터 준비
	title := opts.Title
	description := opts.Description

	// Shorts 해시태그 추가
	if y.cfg.ShortsHashtag {
		description = "#Shorts\n\n" + description
	}

	// 태그 추가
	allTags := append(opts.Tags, y.cfg.DefaultTags...)
	if len(opts.Hashtags) > 0 {
		for _, hashtag := range opts.Hashtags {
			allTags = append(allTags, strings.TrimPrefix(hashtag, "#"))
		}
	}

	// 카테고리 ID (기본: Entertainment = 24)
	categoryID := y.cfg.CategoryID
	if categoryID == "" {
		categoryID = "24"
	}

	// 공개 설정
	privacyStatus := y.cfg.PrivacyStatus
	if opts.Privacy != "" {
		privacyStatus = opts.Privacy
	}
	if privacyStatus == "" {
		privacyStatus = "public"
	}

	// 비디오 메타데이터
	video := map[string]interface{}{
		"snippet": map[string]interface{}{
			"title":       title,
			"description": description,
			"tags":        allTags,
			"categoryId":  categoryID,
		},
		"status": map[string]interface{}{
			"privacyStatus":           privacyStatus,
			"selfDeclaredMadeForKids": false,
		},
	}

	// 예약 업로드
	if opts.ScheduleAt != nil {
		video["status"].(map[string]interface{})["publishAt"] = opts.ScheduleAt.Format(time.RFC3339)
		video["status"].(map[string]interface{})["privacyStatus"] = "private"
	}

	videoJSON, _ := json.Marshal(video)

	// 멀티파트 업로드 요청 생성
	uploadURL := "https://www.googleapis.com/upload/youtube/v3/videos?uploadType=resumable&part=snippet,status"

	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, strings.NewReader(string(videoJSON)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// 업로드 세션 시작
	resp, err := y.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("업로드 세션 시작 실패: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("업로드 세션 시작 실패: %s", string(body))
	}

	// 업로드 URL 가져오기
	uploadLocation := resp.Header.Get("Location")
	if uploadLocation == "" {
		return nil, fmt.Errorf("업로드 URL을 받지 못했습니다")
	}

	// 실제 파일 업로드
	fileInfo, _ := file.Stat()
	uploadReq, err := http.NewRequestWithContext(ctx, "PUT", uploadLocation, file)
	if err != nil {
		return nil, err
	}
	uploadReq.Header.Set("Content-Type", "video/*")
	uploadReq.ContentLength = fileInfo.Size()

	fmt.Println("📤 YouTube에 업로드 중...")
	uploadResp, err := y.client.Do(uploadReq)
	if err != nil {
		return nil, fmt.Errorf("파일 업로드 실패: %w", err)
	}
	defer uploadResp.Body.Close()

	if uploadResp.StatusCode != http.StatusOK && uploadResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(uploadResp.Body)
		return nil, fmt.Errorf("파일 업로드 실패: %s", string(body))
	}

	// 응답 파싱
	var videoResp YouTubeVideo
	if err := json.NewDecoder(uploadResp.Body).Decode(&videoResp); err != nil {
		return nil, fmt.Errorf("응답 파싱 실패: %w", err)
	}

	result := &UploadResult{
		Platform:   PlatformYouTube,
		Success:    true,
		VideoID:    videoResp.ID,
		URL:        fmt.Sprintf("https://youtube.com/shorts/%s", videoResp.ID),
		UploadedAt: time.Now(),
	}

	fmt.Printf("✅ YouTube Shorts 업로드 완료! %s\n", result.URL)
	return result, nil
}

// GetStatus 업로드 상태 확인
func (y *YouTubeUploader) GetStatus(ctx context.Context, videoID string) (*UploadResult, error) {
	if !y.authenticated {
		return nil, fmt.Errorf("인증되지 않음")
	}

	url := fmt.Sprintf("https://www.googleapis.com/youtube/v3/videos?part=statistics,snippet&id=%s", videoID)
	resp, err := y.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response struct {
		Items []YouTubeVideo `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	if len(response.Items) == 0 {
		return nil, fmt.Errorf("영상을 찾을 수 없습니다")
	}

	video := response.Items[0]
	result := &UploadResult{
		Platform: PlatformYouTube,
		Success:  true,
		VideoID:  video.ID,
		URL:      fmt.Sprintf("https://youtube.com/shorts/%s", video.ID),
		Views:    parseMetricValue(video.Statistics.ViewCount),
		Likes:    parseMetricValue(video.Statistics.LikeCount),
		Comments: parseMetricValue(video.Statistics.CommentCount),
	}

	return result, nil
}

// Delete 영상 삭제
func (y *YouTubeUploader) Delete(ctx context.Context, videoID string) error {
	if !y.authenticated {
		return fmt.Errorf("인증되지 않음")
	}

	url := fmt.Sprintf("https://www.googleapis.com/youtube/v3/videos?id=%s", videoID)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	resp, err := y.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("삭제 실패: %d", resp.StatusCode)
	}

	return nil
}

// GetAnalytics 분석 데이터 가져오기
func (y *YouTubeUploader) GetAnalytics(ctx context.Context, videoID string) (map[string]interface{}, error) {
	if !y.authenticated {
		return nil, fmt.Errorf("인증되지 않음")
	}

	// YouTube Analytics API 호출
	url := fmt.Sprintf(
		"https://youtubeanalytics.googleapis.com/v2/reports?ids=channel==%s&startDate=2020-01-01&endDate=2030-12-31&metrics=views,likes,comments,shares,estimatedRevenue&filters=video==%s",
		y.cfg.ChannelID,
		videoID,
	)

	resp, err := y.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var analytics map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&analytics); err != nil {
		return nil, err
	}

	return analytics, nil
}

