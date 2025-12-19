package video

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/clipbot/internal/config"
)

// SubtitleGenerator 자막 생성기
type SubtitleGenerator struct {
	cfg          *config.SubtitleConfig
	aiConfig     *config.AIConfig
	whisperPath  string
}

// SubtitleSegment 자막 세그먼트
type SubtitleSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// WhisperResult Whisper 출력 결과
type WhisperResult struct {
	Text     string            `json:"text"`
	Segments []SubtitleSegment `json:"segments"`
	Language string            `json:"language"`
}

// NewSubtitleGenerator 새 자막 생성기 생성
func NewSubtitleGenerator(cfg *config.SubtitleConfig, aiCfg *config.AIConfig) *SubtitleGenerator {
	return &SubtitleGenerator{
		cfg:         cfg,
		aiConfig:    aiCfg,
		whisperPath: "whisper", // 또는 whisper.cpp 경로
	}
}

// Generate 영상에서 자막 생성
func (s *SubtitleGenerator) Generate(videoPath string) ([]SubtitleSegment, error) {
	// 먼저 오디오 추출
	audioPath := strings.TrimSuffix(videoPath, filepath.Ext(videoPath)) + "_audio.wav"
	defer os.Remove(audioPath)

	if err := s.extractAudio(videoPath, audioPath); err != nil {
		return nil, fmt.Errorf("오디오 추출 실패: %w", err)
	}

	// Whisper로 자막 생성
	segments, err := s.transcribeWithWhisper(audioPath)
	if err != nil {
		return nil, fmt.Errorf("음성 인식 실패: %w", err)
	}

	return segments, nil
}

// GenerateToSRT SRT 파일로 자막 생성
func (s *SubtitleGenerator) GenerateToSRT(videoPath, outputPath string) error {
	segments, err := s.Generate(videoPath)
	if err != nil {
		return err
	}

	return s.SaveAsSRT(segments, outputPath)
}

// GenerateToASS ASS 파일로 자막 생성 (스타일링 지원)
func (s *SubtitleGenerator) GenerateToASS(videoPath, outputPath string) error {
	segments, err := s.Generate(videoPath)
	if err != nil {
		return err
	}

	return s.SaveAsASS(segments, outputPath)
}

// SaveAsSRT SRT 형식으로 저장
func (s *SubtitleGenerator) SaveAsSRT(segments []SubtitleSegment, outputPath string) error {
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	for i, seg := range segments {
		startTime := formatSRTTime(seg.Start)
		endTime := formatSRTTime(seg.End)

		fmt.Fprintf(f, "%d\n", i+1)
		fmt.Fprintf(f, "%s --> %s\n", startTime, endTime)
		fmt.Fprintf(f, "%s\n\n", seg.Text)
	}

	return nil
}

// SaveAsASS ASS 형식으로 저장 (틱톡 스타일)
func (s *SubtitleGenerator) SaveAsASS(segments []SubtitleSegment, outputPath string) error {
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// ASS 헤더
	fmt.Fprintln(f, "[Script Info]")
	fmt.Fprintln(f, "ScriptType: v4.00+")
	fmt.Fprintln(f, "PlayResX: 1080")
	fmt.Fprintln(f, "PlayResY: 1920")
	fmt.Fprintln(f, "")

	// 스타일 정의
	fmt.Fprintln(f, "[V4+ Styles]")
	fmt.Fprintln(f, "Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding")

	// 틱톡 스타일 자막
	style := s.getASSStyle()
	fmt.Fprintln(f, style)
	fmt.Fprintln(f, "")

	// 이벤트
	fmt.Fprintln(f, "[Events]")
	fmt.Fprintln(f, "Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text")

	for _, seg := range segments {
		startTime := formatASSTime(seg.Start)
		endTime := formatASSTime(seg.End)

		// 단어별 하이라이트가 활성화된 경우
		text := seg.Text
		if s.cfg.WordByWord {
			text = s.applyWordByWordEffect(seg.Text)
		}

		fmt.Fprintf(f, "Dialogue: 0,%s,%s,TikTok,,0,0,0,,%s\n", startTime, endTime, text)
	}

	return nil
}

// getASSStyle 스타일별 ASS 스타일 생성
func (s *SubtitleGenerator) getASSStyle() string {
	switch s.cfg.Style {
	case "modern":
		// 틱톡 스타일: 굵은 폰트, 그림자, 중앙 정렬
		return "Style: TikTok,Arial Black,60,&H00FFFFFF,&H000000FF,&H00000000,&H80000000,1,0,0,0,100,100,0,0,1,3,2,2,30,30,100,1"
	case "minimal":
		// 미니멀 스타일: 얇은 폰트, 투명 배경
		return "Style: TikTok,Arial,48,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,0,2,30,30,100,1"
	case "classic":
		// 클래식 스타일: 기본 폰트, 검정 배경
		return "Style: TikTok,Arial,52,&H00FFFFFF,&H000000FF,&H00000000,&HC0000000,1,0,0,0,100,100,0,0,3,2,0,2,30,30,100,1"
	case "neon":
		// 네온 스타일: 발광 효과
		return "Style: TikTok,Arial Black,58,&H00FF00FF,&H00FF00FF,&H00FF00FF,&H00000000,1,0,0,0,100,100,0,0,1,4,3,2,30,30,100,1"
	case "gradient":
		// 그라디언트 스타일
		return "Style: TikTok,Arial Black,56,&H00FFFFFF,&H00FFD700,&H00000000,&H80000000,1,0,0,0,100,100,0,0,1,3,2,2,30,30,100,1"
	default:
		return "Style: TikTok,Arial Black,60,&H00FFFFFF,&H000000FF,&H00000000,&H80000000,1,0,0,0,100,100,0,0,1,3,2,2,30,30,100,1"
	}
}

// applyWordByWordEffect 단어별 하이라이트 효과 적용
func (s *SubtitleGenerator) applyWordByWordEffect(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	// ASS 카라오케 태그 사용
	var result strings.Builder
	for i, word := range words {
		if i > 0 {
			result.WriteString(" ")
		}
		// {\k50} = 500ms 동안 하이라이트
		result.WriteString(fmt.Sprintf("{\\k50}%s", word))
	}

	return result.String()
}

// extractAudio 영상에서 오디오 추출
func (s *SubtitleGenerator) extractAudio(videoPath, audioPath string) error {
	args := []string{
		"-i", videoPath,
		"-vn",
		"-acodec", "pcm_s16le",
		"-ar", "16000",
		"-ac", "1",
		"-y",
		audioPath,
	}

	cmd := exec.Command("ffmpeg", args...)
	return cmd.Run()
}

// transcribeWithWhisper Whisper로 음성 인식
func (s *SubtitleGenerator) transcribeWithWhisper(audioPath string) ([]SubtitleSegment, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	model := s.aiConfig.WhisperModel
	if model == "" {
		model = "base"
	}

	outputDir := filepath.Dir(audioPath)
	outputBase := strings.TrimSuffix(filepath.Base(audioPath), filepath.Ext(audioPath))

	args := []string{
		audioPath,
		"--model", model,
		"--language", s.cfg.Language,
		"--output_format", "json",
		"--output_dir", outputDir,
	}

	cmd := exec.CommandContext(ctx, s.whisperPath, args...)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("Whisper 실행 실패: %w", err)
	}

	// JSON 결과 읽기
	jsonPath := filepath.Join(outputDir, outputBase+".json")
	defer os.Remove(jsonPath)

	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("결과 파일 읽기 실패: %w", err)
	}

	var result WhisperResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("JSON 파싱 실패: %w", err)
	}

	return result.Segments, nil
}

// formatSRTTime SRT 시간 형식으로 변환
func formatSRTTime(seconds float64) string {
	hours := int(seconds) / 3600
	minutes := (int(seconds) % 3600) / 60
	secs := int(seconds) % 60
	millis := int((seconds - float64(int(seconds))) * 1000)

	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, secs, millis)
}

// formatASSTime ASS 시간 형식으로 변환
func formatASSTime(seconds float64) string {
	hours := int(seconds) / 3600
	minutes := (int(seconds) % 3600) / 60
	secs := int(seconds) % 60
	centisecs := int((seconds - float64(int(seconds))) * 100)

	return fmt.Sprintf("%d:%02d:%02d.%02d", hours, minutes, secs, centisecs)
}

// GenerateKaraokeSubtitle 카라오케 스타일 자막 생성 (틱톡에서 인기)
func (s *SubtitleGenerator) GenerateKaraokeSubtitle(videoPath, outputPath string) error {
	segments, err := s.Generate(videoPath)
	if err != nil {
		return err
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// ASS 카라오케 헤더
	fmt.Fprintln(f, "[Script Info]")
	fmt.Fprintln(f, "ScriptType: v4.00+")
	fmt.Fprintln(f, "PlayResX: 1080")
	fmt.Fprintln(f, "PlayResY: 1920")
	fmt.Fprintln(f, "")
	fmt.Fprintln(f, "[V4+ Styles]")
	fmt.Fprintln(f, "Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding")
	// 카라오케: 노래하는 부분이 색이 바뀜
	fmt.Fprintln(f, "Style: Karaoke,Arial Black,64,&H00FFFFFF,&H0000FFFF,&H00000000,&H80000000,1,0,0,0,100,100,0,0,1,4,2,2,30,30,100,1")
	fmt.Fprintln(f, "")
	fmt.Fprintln(f, "[Events]")
	fmt.Fprintln(f, "Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text")

	for _, seg := range segments {
		startTime := formatASSTime(seg.Start)
		endTime := formatASSTime(seg.End)

		// 카라오케 효과
		words := strings.Fields(seg.Text)
		duration := (seg.End - seg.Start) / float64(len(words)) * 100 // centiseconds per word

		var karaokeText strings.Builder
		for _, word := range words {
			karaokeText.WriteString(fmt.Sprintf("{\\kf%.0f}%s ", duration, word))
		}

		fmt.Fprintf(f, "Dialogue: 0,%s,%s,Karaoke,,0,0,0,,%s\n", startTime, endTime, karaokeText.String())
	}

	return nil
}

