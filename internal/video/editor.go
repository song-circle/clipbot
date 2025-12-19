package video

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/clipbot/internal/config"
)

// Editor 영상 편집기
type Editor struct {
	cfg        *config.VideoConfig
	ffmpegPath string
	ffprobePath string
}

// VideoInfo 영상 정보
type VideoInfo struct {
	Duration    float64
	Width       int
	Height      int
	FPS         float64
	Codec       string
	AudioCodec  string
	Bitrate     int64
	HasAudio    bool
}

// EditOptions 편집 옵션
type EditOptions struct {
	StartTime     float64  // 시작 시간 (초)
	EndTime       float64  // 종료 시간 (초)
	Width         int
	Height        int
	FPS           int
	Speed         float64  // 속도 배율
	Volume        float64  // 볼륨 (1.0 = 100%)
	FadeIn        float64  // 페이드인 시간
	FadeOut       float64  // 페이드아웃 시간
	CropToVertical bool    // 9:16 비율로 크롭
	AddSubtitle   bool
	SubtitlePath  string
	OverlayText   string
	OverlayImage  string
	BGMusic       string   // 배경 음악 경로
	BGMusicVolume float64  // 배경 음악 볼륨
	Filters       []string // 추가 FFmpeg 필터
}

// NewEditor 새 편집기 생성
func NewEditor(cfg *config.VideoConfig) *Editor {
	ffmpegPath := cfg.FFmpegPath
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}

	return &Editor{
		cfg:         cfg,
		ffmpegPath:  ffmpegPath,
		ffprobePath: strings.Replace(ffmpegPath, "ffmpeg", "ffprobe", 1),
	}
}

// GetVideoInfo 영상 정보 가져오기
func (e *Editor) GetVideoInfo(inputPath string) (*VideoInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, e.ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		inputPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe 실행 실패: %w", err)
	}

	// JSON 파싱 (간단한 구현)
	info := &VideoInfo{}
	
	// duration 파싱
	if idx := strings.Index(string(output), `"duration"`); idx != -1 {
		sub := string(output)[idx:]
		if start := strings.Index(sub, `"`); start != -1 {
			sub = sub[start+1:]
			if end := strings.Index(sub, `"`); end != -1 {
				sub = sub[end+1:]
				if start2 := strings.Index(sub, `"`); start2 != -1 {
					sub = sub[start2+1:]
					if end2 := strings.Index(sub, `"`); end2 != -1 {
						info.Duration, _ = strconv.ParseFloat(sub[:end2], 64)
					}
				}
			}
		}
	}

	// width, height 파싱
	if idx := strings.Index(string(output), `"width"`); idx != -1 {
		sub := string(output)[idx:]
		if start := strings.Index(sub, ":"); start != -1 {
			sub = sub[start+1:]
			if end := strings.Index(sub, ","); end != -1 {
				info.Width, _ = strconv.Atoi(strings.TrimSpace(sub[:end]))
			}
		}
	}

	if idx := strings.Index(string(output), `"height"`); idx != -1 {
		sub := string(output)[idx:]
		if start := strings.Index(sub, ":"); start != -1 {
			sub = sub[start+1:]
			if end := strings.Index(sub, ","); end != -1 {
				info.Height, _ = strconv.Atoi(strings.TrimSpace(sub[:end]))
			}
		}
	}

	return info, nil
}

// Edit 영상 편집 실행
func (e *Editor) Edit(inputPath, outputPath string, opts *EditOptions) error {
	if opts == nil {
		opts = e.defaultOptions()
	}

	// 출력 경로가 없으면 자동 생성
	if outputPath == "" {
		ext := filepath.Ext(inputPath)
		base := strings.TrimSuffix(filepath.Base(inputPath), ext)
		outputPath = filepath.Join(e.cfg.OutputFormat, fmt.Sprintf("%s_edited%s", base, ext))
	}

	// 출력 디렉토리 생성
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("출력 디렉토리 생성 실패: %w", err)
	}

	args := e.buildFFmpegArgs(inputPath, outputPath, opts)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, e.ffmpegPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("FFmpeg 실행 실패: %w", err)
	}

	return nil
}

// ConvertToShorts 쇼츠/릴스/틱톡 형식으로 변환 (9:16, 60초 이내)
func (e *Editor) ConvertToShorts(inputPath, outputPath string) error {
	info, err := e.GetVideoInfo(inputPath)
	if err != nil {
		return err
	}

	opts := &EditOptions{
		Width:          e.cfg.DefaultWidth,
		Height:         e.cfg.DefaultHeight,
		FPS:            e.cfg.DefaultFPS,
		CropToVertical: true,
		FadeIn:         0.5,
		FadeOut:        0.5,
	}

	// 60초 이상이면 자름
	if info.Duration > float64(e.cfg.MaxDuration) {
		opts.EndTime = float64(e.cfg.MaxDuration)
	}

	return e.Edit(inputPath, outputPath, opts)
}

// AddSubtitles 자막 추가
func (e *Editor) AddSubtitles(inputPath, subtitlePath, outputPath string) error {
	opts := &EditOptions{
		AddSubtitle:  true,
		SubtitlePath: subtitlePath,
	}
	return e.Edit(inputPath, outputPath, opts)
}

// AddTextOverlay 텍스트 오버레이 추가
func (e *Editor) AddTextOverlay(inputPath, text, outputPath string, opts *TextOverlayOptions) error {
	if opts == nil {
		opts = &TextOverlayOptions{
			FontSize:  48,
			FontColor: "white",
			X:         "(w-text_w)/2",
			Y:         "h-100",
		}
	}

	editOpts := &EditOptions{
		OverlayText: text,
	}

	return e.Edit(inputPath, outputPath, editOpts)
}

// TextOverlayOptions 텍스트 오버레이 옵션
type TextOverlayOptions struct {
	FontSize  int
	FontColor string
	FontFile  string
	X         string
	Y         string
	StartTime float64
	EndTime   float64
}

// MergeVideos 여러 영상 합치기
func (e *Editor) MergeVideos(inputPaths []string, outputPath string) error {
	if len(inputPaths) < 2 {
		return fmt.Errorf("합칠 영상이 2개 이상 필요합니다")
	}

	// concat demuxer를 위한 임시 파일 생성
	listFile := filepath.Join(os.TempDir(), "clipbot_concat.txt")
	f, err := os.Create(listFile)
	if err != nil {
		return fmt.Errorf("임시 파일 생성 실패: %w", err)
	}
	defer os.Remove(listFile)

	for _, path := range inputPaths {
		absPath, _ := filepath.Abs(path)
		fmt.Fprintf(f, "file '%s'\n", absPath)
	}
	f.Close()

	args := []string{
		"-f", "concat",
		"-safe", "0",
		"-i", listFile,
		"-c", "copy",
		"-y",
		outputPath,
	}

	cmd := exec.Command(e.ffmpegPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// ExtractAudio 오디오 추출
func (e *Editor) ExtractAudio(inputPath, outputPath string) error {
	args := []string{
		"-i", inputPath,
		"-vn",
		"-acodec", "libmp3lame",
		"-q:a", "2",
		"-y",
		outputPath,
	}

	cmd := exec.Command(e.ffmpegPath, args...)
	return cmd.Run()
}

// AddBGMusic 배경 음악 추가
func (e *Editor) AddBGMusic(videoPath, musicPath, outputPath string, musicVolume float64) error {
	opts := &EditOptions{
		BGMusic:       musicPath,
		BGMusicVolume: musicVolume,
	}
	return e.Edit(videoPath, outputPath, opts)
}

// CreateThumbnail 썸네일 생성
func (e *Editor) CreateThumbnail(inputPath, outputPath string, atSecond float64) error {
	args := []string{
		"-i", inputPath,
		"-ss", fmt.Sprintf("%.2f", atSecond),
		"-vframes", "1",
		"-q:v", "2",
		"-y",
		outputPath,
	}

	cmd := exec.Command(e.ffmpegPath, args...)
	return cmd.Run()
}

// defaultOptions 기본 옵션
func (e *Editor) defaultOptions() *EditOptions {
	return &EditOptions{
		Width:          e.cfg.DefaultWidth,
		Height:         e.cfg.DefaultHeight,
		FPS:            e.cfg.DefaultFPS,
		Speed:          e.cfg.Effects.SpeedMultiplier,
		Volume:         1.0,
		FadeIn:         0.5,
		FadeOut:        0.5,
		CropToVertical: e.cfg.Effects.AutoCrop,
	}
}

// buildFFmpegArgs FFmpeg 인자 생성
func (e *Editor) buildFFmpegArgs(inputPath, outputPath string, opts *EditOptions) []string {
	args := []string{"-i", inputPath}

	// 시작/종료 시간
	if opts.StartTime > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.2f", opts.StartTime))
	}
	if opts.EndTime > 0 {
		args = append(args, "-t", fmt.Sprintf("%.2f", opts.EndTime-opts.StartTime))
	}

	// 배경 음악 입력
	if opts.BGMusic != "" {
		args = append(args, "-i", opts.BGMusic)
	}

	// 필터 구성
	var videoFilters []string
	var audioFilters []string

	// 크롭 (세로 영상으로)
	if opts.CropToVertical && opts.Width > 0 && opts.Height > 0 {
		videoFilters = append(videoFilters, 
			fmt.Sprintf("crop=ih*%d/%d:ih", opts.Width, opts.Height))
	}

	// 해상도 조정
	if opts.Width > 0 && opts.Height > 0 {
		videoFilters = append(videoFilters, 
			fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2",
				opts.Width, opts.Height, opts.Width, opts.Height))
	}

	// FPS 조정
	if opts.FPS > 0 {
		videoFilters = append(videoFilters, fmt.Sprintf("fps=%d", opts.FPS))
	}

	// 속도 조절
	if opts.Speed != 0 && opts.Speed != 1.0 {
		videoFilters = append(videoFilters, fmt.Sprintf("setpts=%.2f*PTS", 1.0/opts.Speed))
		audioFilters = append(audioFilters, fmt.Sprintf("atempo=%.2f", opts.Speed))
	}

	// 페이드 효과
	if opts.FadeIn > 0 {
		videoFilters = append(videoFilters, fmt.Sprintf("fade=t=in:st=0:d=%.2f", opts.FadeIn))
	}
	if opts.FadeOut > 0 {
		videoFilters = append(videoFilters, fmt.Sprintf("fade=t=out:st=%.2f:d=%.2f", 
			opts.EndTime-opts.FadeOut, opts.FadeOut))
	}

	// 자막
	if opts.AddSubtitle && opts.SubtitlePath != "" {
		videoFilters = append(videoFilters, 
			fmt.Sprintf("subtitles='%s'", strings.ReplaceAll(opts.SubtitlePath, "'", "\\'")))
	}

	// 볼륨
	if opts.Volume != 1.0 {
		audioFilters = append(audioFilters, fmt.Sprintf("volume=%.2f", opts.Volume))
	}

	// 배경 음악 믹싱
	if opts.BGMusic != "" {
		if opts.BGMusicVolume == 0 {
			opts.BGMusicVolume = 0.3
		}
		// 복잡한 필터 사용
		args = append(args, "-filter_complex",
			fmt.Sprintf("[0:a]volume=1[a1];[1:a]volume=%.2f[a2];[a1][a2]amix=inputs=2:duration=first[aout]",
				opts.BGMusicVolume))
		args = append(args, "-map", "0:v", "-map", "[aout]")
	} else {
		// 비디오 필터 적용
		if len(videoFilters) > 0 {
			args = append(args, "-vf", strings.Join(videoFilters, ","))
		}

		// 오디오 필터 적용
		if len(audioFilters) > 0 {
			args = append(args, "-af", strings.Join(audioFilters, ","))
		}
	}

	// 품질 설정
	quality := e.getQualitySettings()
	args = append(args, quality...)

	// 출력 포맷
	args = append(args, "-f", e.cfg.OutputFormat)

	// 덮어쓰기
	args = append(args, "-y")

	// 출력 파일
	args = append(args, outputPath)

	return args
}

func (e *Editor) getQualitySettings() []string {
	switch e.cfg.Quality {
	case "ultra":
		return []string{"-c:v", "libx264", "-preset", "slow", "-crf", "18", "-c:a", "aac", "-b:a", "192k"}
	case "high":
		return []string{"-c:v", "libx264", "-preset", "medium", "-crf", "22", "-c:a", "aac", "-b:a", "128k"}
	case "medium":
		return []string{"-c:v", "libx264", "-preset", "fast", "-crf", "26", "-c:a", "aac", "-b:a", "96k"}
	case "low":
		return []string{"-c:v", "libx264", "-preset", "veryfast", "-crf", "30", "-c:a", "aac", "-b:a", "64k"}
	default:
		return []string{"-c:v", "libx264", "-preset", "medium", "-crf", "22", "-c:a", "aac", "-b:a", "128k"}
	}
}

// BatchEdit 여러 영상 일괄 편집
func (e *Editor) BatchEdit(inputDir, outputDir string, opts *EditOptions) error {
	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return fmt.Errorf("입력 디렉토리 읽기 실패: %w", err)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("출력 디렉토리 생성 실패: %w", err)
	}

	videoExts := map[string]bool{".mp4": true, ".mov": true, ".avi": true, ".mkv": true, ".webm": true}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if !videoExts[ext] {
			continue
		}

		inputPath := filepath.Join(inputDir, entry.Name())
		outputPath := filepath.Join(outputDir, 
			strings.TrimSuffix(entry.Name(), ext)+"_edited.mp4")

		fmt.Printf("편집 중: %s\n", entry.Name())
		if err := e.Edit(inputPath, outputPath, opts); err != nil {
			fmt.Printf("편집 실패 %s: %v\n", entry.Name(), err)
			continue
		}
		fmt.Printf("완료: %s\n", outputPath)
	}

	return nil
}

