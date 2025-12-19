package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/clipbot/internal/config"
	"github.com/sashabaranov/go-openai"
)

// ContentGenerator AI 기반 콘텐츠 생성기
type ContentGenerator struct {
	cfg    *config.AIConfig
	client *openai.Client
}

// GeneratedContent 생성된 콘텐츠
type GeneratedContent struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Hashtags    []string `json:"hashtags"`
	Hook        string   `json:"hook"`        // 초반 훅 문구
	CTA         string   `json:"cta"`         // Call to Action
	Platform    string   `json:"platform"`    // 플랫폼별 최적화
}

// TrendingTopic 트렌딩 토픽
type TrendingTopic struct {
	Topic       string   `json:"topic"`
	Hashtags    []string `json:"hashtags"`
	Description string   `json:"description"`
	Score       float64  `json:"score"`
}

// VideoScript 영상 스크립트
type VideoScript struct {
	Intro      string   `json:"intro"`
	MainPoints []string `json:"main_points"`
	Outro      string   `json:"outro"`
	Duration   int      `json:"duration"` // 예상 길이 (초)
}

// NewContentGenerator 새 콘텐츠 생성기 생성
func NewContentGenerator(cfg *config.AIConfig) *ContentGenerator {
	var client *openai.Client
	if cfg.OpenAIKey != "" {
		client = openai.NewClient(cfg.OpenAIKey)
	}

	return &ContentGenerator{
		cfg:    cfg,
		client: client,
	}
}

// GenerateTitle 제목 생성
func (g *ContentGenerator) GenerateTitle(ctx context.Context, topic, platform string) (string, error) {
	if g.client == nil {
		return topic, nil
	}

	prompt := fmt.Sprintf(`당신은 바이럴 콘텐츠 전문가입니다.
다음 주제로 %s에 올릴 영상의 제목을 만들어주세요.

주제: %s

규칙:
- 최대 100자
- 호기심을 자극하는 훅
- 이모지 1-2개 포함
- 클릭을 유도하는 문구

제목만 출력하세요:`, platform, topic)

	resp, err := g.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: g.cfg.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		MaxTokens:   100,
		Temperature: 0.8,
	})
	if err != nil {
		return topic, err
	}

	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

// GenerateHashtags 해시태그 생성
func (g *ContentGenerator) GenerateHashtags(ctx context.Context, topic, platform string, count int) ([]string, error) {
	if g.client == nil {
		return []string{"#viral", "#fyp", "#trending"}, nil
	}

	prompt := fmt.Sprintf(`당신은 SNS 마케팅 전문가입니다.
다음 주제로 %s에 올릴 영상의 해시태그를 %d개 만들어주세요.

주제: %s

규칙:
- 트렌딩 해시태그 포함
- 니치 해시태그 포함
- 플랫폼 특화 해시태그 포함
- JSON 배열 형식으로 출력

예시: ["#tag1", "#tag2", "#tag3"]

해시태그 배열만 출력:`, platform, count, topic)

	resp, err := g.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: g.cfg.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		MaxTokens:   200,
		Temperature: 0.7,
	})
	if err != nil {
		return nil, err
	}

	var hashtags []string
	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	if err := json.Unmarshal([]byte(content), &hashtags); err != nil {
		// JSON 파싱 실패 시 줄 단위로 파싱
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "#") {
				hashtags = append(hashtags, strings.Split(line, " ")[0])
			}
		}
	}

	return hashtags, nil
}

// GenerateDescription 설명 생성
func (g *ContentGenerator) GenerateDescription(ctx context.Context, topic, platform string) (string, error) {
	if g.client == nil {
		return topic, nil
	}

	maxLength := 150
	switch platform {
	case "youtube":
		maxLength = 500
	case "tiktok":
		maxLength = 150
	case "instagram":
		maxLength = 2200
	case "facebook":
		maxLength = 500
	case "twitter":
		maxLength = 280
	}

	prompt := fmt.Sprintf(`당신은 SNS 콘텐츠 전문가입니다.
다음 주제로 %s에 올릴 영상의 설명을 작성해주세요.

주제: %s

규칙:
- 최대 %d자
- 시청자 참여 유도 (댓글, 좋아요 등)
- Call to Action 포함
- 이모지 적절히 사용

설명만 출력:`, platform, topic, maxLength)

	resp, err := g.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: g.cfg.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		MaxTokens:   300,
		Temperature: 0.7,
	})
	if err != nil {
		return topic, err
	}

	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

// GenerateFullContent 전체 콘텐츠 생성 (제목, 설명, 해시태그)
func (g *ContentGenerator) GenerateFullContent(ctx context.Context, topic, platform string) (*GeneratedContent, error) {
	if g.client == nil {
		return &GeneratedContent{
			Title:       topic,
			Description: topic,
			Hashtags:    []string{"#viral", "#fyp"},
		}, nil
	}

	prompt := fmt.Sprintf(`당신은 바이럴 콘텐츠 전문가입니다.
다음 주제로 %s에 올릴 영상의 콘텐츠를 생성해주세요.

주제: %s

다음 JSON 형식으로 출력해주세요:
{
  "title": "영상 제목 (100자 이내, 이모지 포함)",
  "description": "영상 설명 (플랫폼에 맞는 길이)",
  "hashtags": ["#해시태그1", "#해시태그2", ...],
  "hook": "영상 초반 3초 훅 문구",
  "cta": "구독/팔로우 유도 문구"
}

JSON만 출력:`, platform, topic)

	resp, err := g.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: g.cfg.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		MaxTokens:   500,
		Temperature: 0.8,
	})
	if err != nil {
		return nil, fmt.Errorf("AI 생성 실패: %w", err)
	}

	var content GeneratedContent
	responseText := resp.Choices[0].Message.Content
	
	// JSON 추출 (코드 블록 제거)
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimPrefix(responseText, "```")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	if err := json.Unmarshal([]byte(responseText), &content); err != nil {
		return nil, fmt.Errorf("JSON 파싱 실패: %w", err)
	}

	content.Platform = platform
	return &content, nil
}

// GenerateScript 영상 스크립트 생성
func (g *ContentGenerator) GenerateScript(ctx context.Context, topic string, durationSecs int) (*VideoScript, error) {
	if g.client == nil {
		return &VideoScript{
			Intro:      topic,
			MainPoints: []string{topic},
			Outro:      "좋아요와 구독 부탁드립니다!",
			Duration:   durationSecs,
		}, nil
	}

	prompt := fmt.Sprintf(`당신은 숏폼 콘텐츠 전문가입니다.
다음 주제로 %d초짜리 영상 스크립트를 작성해주세요.

주제: %s

다음 JSON 형식으로 출력:
{
  "intro": "0-3초: 강력한 훅 (시선 끌기)",
  "main_points": [
    "포인트 1",
    "포인트 2",
    "포인트 3"
  ],
  "outro": "마무리 및 CTA",
  "duration": %d
}

JSON만 출력:`, durationSecs, topic, durationSecs)

	resp, err := g.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: g.cfg.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		MaxTokens:   500,
		Temperature: 0.7,
	})
	if err != nil {
		return nil, err
	}

	var script VideoScript
	responseText := resp.Choices[0].Message.Content
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimPrefix(responseText, "```")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	if err := json.Unmarshal([]byte(responseText), &script); err != nil {
		return nil, err
	}

	return &script, nil
}

// SuggestContentIdeas 콘텐츠 아이디어 제안
func (g *ContentGenerator) SuggestContentIdeas(ctx context.Context, niche string, count int) ([]string, error) {
	if g.client == nil {
		return []string{niche + " 관련 콘텐츠"}, nil
	}

	prompt := fmt.Sprintf(`당신은 바이럴 콘텐츠 전문가입니다.
다음 니치/분야에서 바이럴 가능성이 높은 숏폼 콘텐츠 아이디어를 %d개 제안해주세요.

분야: %s

규칙:
- 트렌드 반영
- 호기심 유발
- 제작 가능한 현실적인 아이디어
- JSON 배열 형식

예시: ["아이디어 1", "아이디어 2"]

아이디어 배열만 출력:`, count, niche)

	resp, err := g.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: g.cfg.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		MaxTokens:   500,
		Temperature: 0.9,
	})
	if err != nil {
		return nil, err
	}

	var ideas []string
	responseText := strings.TrimSpace(resp.Choices[0].Message.Content)
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimPrefix(responseText, "```")
	responseText = strings.TrimSuffix(responseText, "```")
	
	if err := json.Unmarshal([]byte(responseText), &ideas); err != nil {
		// 실패 시 줄 단위로 파싱
		lines := strings.Split(responseText, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			line = strings.TrimPrefix(line, "- ")
			line = strings.TrimPrefix(line, "* ")
			if line != "" && !strings.HasPrefix(line, "[") && !strings.HasPrefix(line, "]") {
				ideas = append(ideas, line)
			}
		}
	}

	return ideas, nil
}

// OptimizeForPlatform 플랫폼별 최적화
func (g *ContentGenerator) OptimizeForPlatform(ctx context.Context, content *GeneratedContent, platform string) (*GeneratedContent, error) {
	if g.client == nil {
		return content, nil
	}

	contentJSON, _ := json.Marshal(content)

	prompt := fmt.Sprintf(`다음 콘텐츠를 %s 플랫폼에 최적화해주세요.

현재 콘텐츠:
%s

%s 특성:
- 틱톡: 15-60초, 트렌디한 해시태그, 음악 중요
- 유튜브: Shorts 60초, SEO 중요, 상세 설명
- 인스타그램: 릴스 90초, 비주얼 중심, 위치태그
- 페이스북: 스토리텔링, 공유 유도
- 트위터: 280자 제한, 짧고 임팩트

최적화된 JSON 출력:`, platform, string(contentJSON), platform)

	resp, err := g.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: g.cfg.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		MaxTokens:   500,
		Temperature: 0.6,
	})
	if err != nil {
		return content, err
	}

	var optimized GeneratedContent
	responseText := resp.Choices[0].Message.Content
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimPrefix(responseText, "```")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	if err := json.Unmarshal([]byte(responseText), &optimized); err != nil {
		return content, nil
	}

	optimized.Platform = platform
	return &optimized, nil
}

// AnalyzeVideoContent 영상 내용 분석 (자막 기반)
func (g *ContentGenerator) AnalyzeVideoContent(ctx context.Context, transcript string) (map[string]interface{}, error) {
	if g.client == nil {
		return nil, fmt.Errorf("OpenAI API 키가 필요합니다")
	}

	prompt := fmt.Sprintf(`다음 영상 자막을 분석해주세요:

자막:
%s

다음 JSON 형식으로 분석 결과 출력:
{
  "summary": "영상 요약 (2-3문장)",
  "topics": ["주제1", "주제2"],
  "mood": "영상 분위기 (재미있는/진지한/감동적인 등)",
  "target_audience": "타겟 시청자층",
  "viral_score": 1-10,
  "improvement_suggestions": ["개선점1", "개선점2"]
}

JSON만 출력:`, transcript)

	resp, err := g.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: g.cfg.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		MaxTokens:   500,
		Temperature: 0.5,
	})
	if err != nil {
		return nil, err
	}

	var analysis map[string]interface{}
	responseText := resp.Choices[0].Message.Content
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimPrefix(responseText, "```")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	if err := json.Unmarshal([]byte(responseText), &analysis); err != nil {
		return nil, err
	}

	return analysis, nil
}

