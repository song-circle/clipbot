package uploader

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/clipbot/internal/config"
)

// TwitterUploader 트위터/X 업로더
type TwitterUploader struct {
	cfg           *config.TwitterConfig
	client        *http.Client
	authenticated bool
}

// NewTwitterUploader 새 트위터 업로더 생성
func NewTwitterUploader(cfg *config.TwitterConfig) *TwitterUploader {
	return &TwitterUploader{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Minute},
	}
}

// Platform 플랫폼 이름 반환
func (t *TwitterUploader) Platform() Platform {
	return PlatformTwitter
}

// IsAuthenticated 인증 상태 확인
func (t *TwitterUploader) IsAuthenticated() bool {
	return t.authenticated || (t.cfg.APIKey != "" && t.cfg.AccessToken != "")
}

// Authenticate 인증 수행
func (t *TwitterUploader) Authenticate(ctx context.Context) error {
	if t.cfg.APIKey == "" || t.cfg.APISecret == "" {
		return fmt.Errorf("Twitter API 키가 필요합니다. https://developer.twitter.com 에서 발급받으세요")
	}
	if t.cfg.AccessToken == "" || t.cfg.AccessSecret == "" {
		return fmt.Errorf("Twitter Access Token이 필요합니다")
	}

	// 간단한 인증 확인
	req, _ := http.NewRequest("GET", "https://api.twitter.com/2/users/me", nil)
	t.signRequest(req, nil)

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("인증 확인 실패: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("유효하지 않은 인증 정보입니다")
	}

	t.authenticated = true
	return nil
}

// Upload 영상 업로드
func (t *TwitterUploader) Upload(ctx context.Context, videoPath string, opts *UploadOptions) (*UploadResult, error) {
	if !t.IsAuthenticated() {
		if err := t.Authenticate(ctx); err != nil {
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
	fileData, _ := io.ReadAll(file)

	// 1. INIT - 미디어 업로드 초기화
	fmt.Println("📤 Twitter에 업로드 중...")

	initParams := url.Values{
		"command":     {"INIT"},
		"total_bytes": {strconv.FormatInt(fileInfo.Size(), 10)},
		"media_type":  {"video/mp4"},
	}

	initReq, _ := http.NewRequest("POST", "https://upload.twitter.com/1.1/media/upload.json", strings.NewReader(initParams.Encode()))
	initReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	t.signRequest(initReq, initParams)

	initResp, err := t.client.Do(initReq)
	if err != nil {
		return nil, fmt.Errorf("미디어 초기화 실패: %w", err)
	}
	defer initResp.Body.Close()

	var initResult struct {
		MediaID       int64 `json:"media_id"`
		MediaIDString string `json:"media_id_string"`
	}
	if err := json.NewDecoder(initResp.Body).Decode(&initResult); err != nil {
		return nil, fmt.Errorf("초기화 응답 파싱 실패: %w", err)
	}

	// 2. APPEND - 청크 업로드
	chunkSize := 5 * 1024 * 1024 // 5MB
	for i := 0; i < len(fileData); i += chunkSize {
		end := i + chunkSize
		if end > len(fileData) {
			end = len(fileData)
		}
		chunk := fileData[i:end]
		segmentIndex := i / chunkSize

		appendParams := url.Values{
			"command":       {"APPEND"},
			"media_id":      {initResult.MediaIDString},
			"segment_index": {strconv.Itoa(segmentIndex)},
		}

		var body bytes.Buffer
		body.WriteString("--boundary\r\n")
		body.WriteString("Content-Disposition: form-data; name=\"command\"\r\n\r\n")
		body.WriteString("APPEND\r\n")
		body.WriteString("--boundary\r\n")
		body.WriteString(fmt.Sprintf("Content-Disposition: form-data; name=\"media_id\"\r\n\r\n%s\r\n", initResult.MediaIDString))
		body.WriteString("--boundary\r\n")
		body.WriteString(fmt.Sprintf("Content-Disposition: form-data; name=\"segment_index\"\r\n\r\n%d\r\n", segmentIndex))
		body.WriteString("--boundary\r\n")
		body.WriteString("Content-Disposition: form-data; name=\"media\"; filename=\"video.mp4\"\r\n")
		body.WriteString("Content-Type: video/mp4\r\n\r\n")
		body.Write(chunk)
		body.WriteString("\r\n--boundary--\r\n")

		appendReq, _ := http.NewRequest("POST", "https://upload.twitter.com/1.1/media/upload.json", &body)
		appendReq.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
		t.signRequest(appendReq, appendParams)

		_, err := t.client.Do(appendReq)
		if err != nil {
			return nil, fmt.Errorf("청크 업로드 실패: %w", err)
		}
	}

	// 3. FINALIZE - 업로드 완료
	finalParams := url.Values{
		"command":  {"FINALIZE"},
		"media_id": {initResult.MediaIDString},
	}

	finalReq, _ := http.NewRequest("POST", "https://upload.twitter.com/1.1/media/upload.json", strings.NewReader(finalParams.Encode()))
	finalReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	t.signRequest(finalReq, finalParams)

	finalResp, err := t.client.Do(finalReq)
	if err != nil {
		return nil, fmt.Errorf("업로드 완료 실패: %w", err)
	}
	defer finalResp.Body.Close()

	// 처리 상태 확인 (비동기)
	var finalResult struct {
		ProcessingInfo struct {
			State          string `json:"state"`
			CheckAfterSecs int    `json:"check_after_secs"`
		} `json:"processing_info"`
	}
	json.NewDecoder(finalResp.Body).Decode(&finalResult)

	// 처리 완료 대기
	for finalResult.ProcessingInfo.State == "pending" || finalResult.ProcessingInfo.State == "in_progress" {
		time.Sleep(time.Duration(finalResult.ProcessingInfo.CheckAfterSecs) * time.Second)

		statusParams := url.Values{
			"command":  {"STATUS"},
			"media_id": {initResult.MediaIDString},
		}
		statusReq, _ := http.NewRequest("GET", "https://upload.twitter.com/1.1/media/upload.json?"+statusParams.Encode(), nil)
		t.signRequest(statusReq, nil)

		statusResp, _ := t.client.Do(statusReq)
		json.NewDecoder(statusResp.Body).Decode(&finalResult)
		statusResp.Body.Close()
	}

	// 4. 트윗 게시
	tweetText := opts.Title
	if len(opts.Hashtags) > 0 {
		tweetText += "\n\n"
		for _, tag := range opts.Hashtags {
			if !strings.HasPrefix(tag, "#") {
				tag = "#" + tag
			}
			tweetText += tag + " "
		}
	}

	for _, tag := range t.cfg.DefaultTags {
		if !strings.HasPrefix(tag, "#") {
			tag = "#" + tag
		}
		tweetText += tag + " "
	}

	tweetData := map[string]interface{}{
		"text": tweetText,
		"media": map[string]interface{}{
			"media_ids": []string{initResult.MediaIDString},
		},
	}
	tweetJSON, _ := json.Marshal(tweetData)

	tweetReq, _ := http.NewRequest("POST", "https://api.twitter.com/2/tweets", bytes.NewReader(tweetJSON))
	tweetReq.Header.Set("Content-Type", "application/json")
	t.signRequest(tweetReq, nil)

	tweetResp, err := t.client.Do(tweetReq)
	if err != nil {
		return nil, fmt.Errorf("트윗 게시 실패: %w", err)
	}
	defer tweetResp.Body.Close()

	var tweetResult struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.NewDecoder(tweetResp.Body).Decode(&tweetResult)

	result := &UploadResult{
		Platform:   PlatformTwitter,
		Success:    true,
		VideoID:    tweetResult.Data.ID,
		URL:        fmt.Sprintf("https://twitter.com/i/status/%s", tweetResult.Data.ID),
		UploadedAt: time.Now(),
	}

	fmt.Printf("✅ Twitter 업로드 완료! %s\n", result.URL)
	return result, nil
}

// GetStatus 업로드 상태 확인
func (t *TwitterUploader) GetStatus(ctx context.Context, tweetID string) (*UploadResult, error) {
	if !t.IsAuthenticated() {
		return nil, fmt.Errorf("인증되지 않음")
	}

	url := fmt.Sprintf("https://api.twitter.com/2/tweets/%s?tweet.fields=public_metrics", tweetID)
	req, _ := http.NewRequest("GET", url, nil)
	t.signRequest(req, nil)

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tweetData struct {
		Data struct {
			PublicMetrics struct {
				LikeCount    int64 `json:"like_count"`
				ReplyCount   int64 `json:"reply_count"`
				RetweetCount int64 `json:"retweet_count"`
				ViewCount    int64 `json:"impression_count"`
			} `json:"public_metrics"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tweetData); err != nil {
		return nil, err
	}

	return &UploadResult{
		Platform: PlatformTwitter,
		Success:  true,
		VideoID:  tweetID,
		Views:    tweetData.Data.PublicMetrics.ViewCount,
		Likes:    tweetData.Data.PublicMetrics.LikeCount,
		Comments: tweetData.Data.PublicMetrics.ReplyCount,
		Shares:   tweetData.Data.PublicMetrics.RetweetCount,
	}, nil
}

// Delete 트윗 삭제
func (t *TwitterUploader) Delete(ctx context.Context, tweetID string) error {
	if !t.IsAuthenticated() {
		return fmt.Errorf("인증되지 않음")
	}

	url := fmt.Sprintf("https://api.twitter.com/2/tweets/%s", tweetID)
	req, _ := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	t.signRequest(req, nil)

	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("삭제 실패: %d", resp.StatusCode)
	}

	return nil
}

// signRequest OAuth 1.0a 서명
func (t *TwitterUploader) signRequest(req *http.Request, params url.Values) {
	nonce := generateNonce()
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	oauthParams := map[string]string{
		"oauth_consumer_key":     t.cfg.APIKey,
		"oauth_nonce":            nonce,
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_timestamp":        timestamp,
		"oauth_token":            t.cfg.AccessToken,
		"oauth_version":          "1.0",
	}

	// 모든 파라미터 수집
	allParams := make(map[string]string)
	for k, v := range oauthParams {
		allParams[k] = v
	}
	if params != nil {
		for k, v := range params {
			if len(v) > 0 {
				allParams[k] = v[0]
			}
		}
	}

	// 파라미터 정렬 및 인코딩
	var keys []string
	for k := range allParams {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var paramPairs []string
	for _, k := range keys {
		paramPairs = append(paramPairs, fmt.Sprintf("%s=%s", 
			url.QueryEscape(k), url.QueryEscape(allParams[k])))
	}
	paramString := strings.Join(paramPairs, "&")

	// 서명 베이스 문자열
	baseURL := req.URL.Scheme + "://" + req.URL.Host + req.URL.Path
	signatureBase := fmt.Sprintf("%s&%s&%s",
		req.Method,
		url.QueryEscape(baseURL),
		url.QueryEscape(paramString))

	// 서명 키
	signingKey := fmt.Sprintf("%s&%s",
		url.QueryEscape(t.cfg.APISecret),
		url.QueryEscape(t.cfg.AccessSecret))

	// HMAC-SHA1 서명
	h := hmac.New(sha1.New, []byte(signingKey))
	h.Write([]byte(signatureBase))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// Authorization 헤더 생성
	oauthParams["oauth_signature"] = signature
	var authPairs []string
	for k, v := range oauthParams {
		authPairs = append(authPairs, fmt.Sprintf(`%s="%s"`, k, url.QueryEscape(v)))
	}
	authHeader := "OAuth " + strings.Join(authPairs, ", ")

	req.Header.Set("Authorization", authHeader)
}

func generateNonce() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 32)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

