package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/clipbot/internal/ai"
	"github.com/clipbot/internal/collector"
	"github.com/clipbot/internal/config"
	"github.com/clipbot/internal/scheduler"
	"github.com/clipbot/internal/uploader"
	"github.com/clipbot/internal/video"
)

// App 메인 애플리케이션
type App struct {
	cfg       *config.Config
	editor    *video.Editor
	subtitle  *video.SubtitleGenerator
	uploader  *uploader.MultiUploader
	scheduler *scheduler.Scheduler
	ai        *ai.ContentGenerator
	collector *collector.Collector
}

// New 새 애플리케이션 생성
func New(cfg *config.Config) *App {
	// 디렉토리 생성
	cfg.EnsureDirectories()

	// 컴포넌트 초기화
	editor := video.NewEditor(&cfg.Video)
	subtitleGen := video.NewSubtitleGenerator(&cfg.Video.Subtitle, &cfg.AI)
	multiUploader := uploader.NewMultiUploader(&cfg.Platforms)
	sched := scheduler.NewScheduler(&cfg.Schedule, &cfg.Storage, multiUploader)
	contentAI := ai.NewContentGenerator(&cfg.AI)
	videoCollector := collector.NewCollector(&cfg.Collector)

	return &App{
		cfg:       cfg,
		editor:    editor,
		subtitle:  subtitleGen,
		uploader:  multiUploader,
		scheduler: sched,
		ai:        contentAI,
		collector: videoCollector,
	}
}

// Run 전체 파이프라인 실행
func (a *App) Run() error {
	fmt.Println("🚀 ClipBot 시작!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// 1. 입력 디렉토리의 영상 처리
	videos, err := a.findVideos(a.cfg.Storage.InputDir)
	if err != nil {
		return fmt.Errorf("영상 찾기 실패: %w", err)
	}

	if len(videos) == 0 {
		fmt.Println("📭 처리할 영상이 없습니다")
		fmt.Printf("💡 %s 폴더에 영상을 넣어주세요\n", a.cfg.Storage.InputDir)
		return nil
	}

	fmt.Printf("📹 발견된 영상: %d개\n", len(videos))

	for _, videoPath := range videos {
		if err := a.processVideo(videoPath); err != nil {
			fmt.Printf("❌ 처리 실패 (%s): %v\n", filepath.Base(videoPath), err)
			continue
		}
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("✅ 모든 작업 완료!")

	return nil
}

// processVideo 단일 영상 처리 (편집 -> 업로드)
func (a *App) processVideo(inputPath string) error {
	filename := filepath.Base(inputPath)
	fmt.Printf("\n🎬 처리 중: %s\n", filename)

	// 출력 경로 설정
	ext := filepath.Ext(filename)
	baseName := strings.TrimSuffix(filename, ext)
	outputPath := filepath.Join(a.cfg.Storage.OutputDir, baseName+"_shorts.mp4")

	// 1. 영상 편집 (쇼츠 형식으로 변환)
	fmt.Println("  📐 영상 편집 중...")
	if err := a.editor.ConvertToShorts(inputPath, outputPath); err != nil {
		return fmt.Errorf("편집 실패: %w", err)
	}

	// 2. 자막 생성 (활성화된 경우)
	if a.cfg.Video.Subtitle.Enabled {
		fmt.Println("  📝 자막 생성 중...")
		subtitlePath := filepath.Join(a.cfg.Storage.TempDir, baseName+".ass")
		if err := a.subtitle.GenerateToASS(outputPath, subtitlePath); err != nil {
			fmt.Printf("  ⚠️ 자막 생성 실패: %v\n", err)
		} else {
			// 자막을 영상에 합성
			subtitledPath := filepath.Join(a.cfg.Storage.OutputDir, baseName+"_subtitled.mp4")
			if err := a.editor.AddSubtitles(outputPath, subtitlePath, subtitledPath); err != nil {
				fmt.Printf("  ⚠️ 자막 합성 실패: %v\n", err)
			} else {
				// 기존 파일을 자막 있는 버전으로 교체
				os.Remove(outputPath)
				os.Rename(subtitledPath, outputPath)
			}
		}
	}

	// 3. AI 콘텐츠 생성
	fmt.Println("  🤖 콘텐츠 생성 중...")
	ctx := context.Background()
	content, err := a.ai.GenerateFullContent(ctx, baseName, "tiktok")
	if err != nil {
		fmt.Printf("  ⚠️ AI 콘텐츠 생성 실패: %v\n", err)
		content = &ai.GeneratedContent{
			Title:       baseName,
			Description: baseName,
			Hashtags:    []string{"#fyp", "#viral", "#trending"},
		}
	}

	// 4. 업로드
	enabledPlatforms := a.cfg.GetEnabledPlatforms()
	if len(enabledPlatforms) == 0 {
		fmt.Println("  ⚠️ 활성화된 플랫폼이 없습니다")
		return nil
	}

	fmt.Printf("  📤 업로드 중 (%s)...\n", strings.Join(enabledPlatforms, ", "))

	opts := &uploader.UploadOptions{
		Title:       content.Title,
		Description: content.Description,
		Hashtags:    content.Hashtags,
	}

	results := a.uploader.UploadToAll(ctx, outputPath, opts)

	for platform, result := range results {
		if result.Success {
			fmt.Printf("  ✅ %s: %s\n", platform, result.URL)
		} else {
			fmt.Printf("  ❌ %s: %s\n", platform, result.Error)
		}
	}

	// 5. 원본을 아카이브로 이동
	archivePath := filepath.Join(a.cfg.Storage.ArchiveDir, filename)
	os.Rename(inputPath, archivePath)

	return nil
}

// EditVideo 영상 편집만 실행
func (a *App) EditVideo(inputPath, outputPath string) error {
	if outputPath == "" {
		ext := filepath.Ext(inputPath)
		baseName := strings.TrimSuffix(filepath.Base(inputPath), ext)
		outputPath = filepath.Join(a.cfg.Storage.OutputDir, baseName+"_edited.mp4")
	}

	return a.editor.ConvertToShorts(inputPath, outputPath)
}

// Upload 업로드만 실행
func (a *App) Upload(videoPath string, platformNames []string) error {
	platforms := uploader.ParsePlatforms(platformNames)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// AI로 콘텐츠 생성
	baseName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	content, _ := a.ai.GenerateFullContent(ctx, baseName, string(platforms[0]))

	opts := &uploader.UploadOptions{
		Title:       content.Title,
		Description: content.Description,
		Hashtags:    content.Hashtags,
	}

	results := a.uploader.UploadTo(ctx, videoPath, platforms, opts)

	for platform, result := range results {
		if result.Success {
			fmt.Printf("✅ %s: %s\n", platform, result.URL)
		} else {
			fmt.Printf("❌ %s: %s\n", platform, result.Error)
		}
	}

	return nil
}

// StartScheduler 스케줄러 시작
func (a *App) StartScheduler() error {
	return a.scheduler.Start()
}

// ShowStatus 상태 표시
func (a *App) ShowStatus() {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📊 ClipBot 상태")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// 활성화된 플랫폼
	platforms := a.cfg.GetEnabledPlatforms()
	fmt.Printf("\n🌐 활성화된 플랫폼: %s\n", strings.Join(platforms, ", "))

	// 대기 중인 영상 수
	videos, _ := a.findVideos(a.cfg.Storage.InputDir)
	fmt.Printf("📹 대기 중인 영상: %d개\n", len(videos))

	// 처리 완료 영상 수
	processed, _ := a.findVideos(a.cfg.Storage.OutputDir)
	fmt.Printf("✅ 처리 완료 영상: %d개\n", len(processed))

	// 아카이브 영상 수
	archived, _ := a.findVideos(a.cfg.Storage.ArchiveDir)
	fmt.Printf("📦 아카이브 영상: %d개\n", len(archived))

	// 스케줄러 상태
	schedStatus := a.scheduler.GetStatus()
	fmt.Printf("\n⏰ 스케줄러: ")
	if running, ok := schedStatus["running"].(bool); ok && running {
		fmt.Println("실행 중")
		if next, ok := schedStatus["next_upload"].(time.Time); ok && !next.IsZero() {
			fmt.Printf("   다음 업로드: %s\n", next.Format("2006-01-02 15:04:05"))
		}
	} else {
		fmt.Println("중지됨")
	}

	// 수익화 정보 (API 연동 시 표시)
	fmt.Println("\n💰 수익화 팁:")
	fmt.Println("   • TikTok: Creator Fund 가입 (팔로워 10K+)")
	fmt.Println("   • YouTube: 파트너 프로그램 (구독자 1K+, 시청 4000시간)")
	fmt.Println("   • Instagram: Reels Play 보너스 프로그램")
	fmt.Println("   • Facebook: 인스트림 광고 (팔로워 10K+)")
	fmt.Println("   • Twitter/X: 크리에이터 수익 공유")

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// findVideos 디렉토리에서 영상 파일 찾기
func (a *App) findVideos(dir string) ([]string, error) {
	var videos []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return videos, nil
		}
		return nil, err
	}

	videoExts := map[string]bool{
		".mp4": true, ".mov": true, ".avi": true,
		".mkv": true, ".webm": true, ".m4v": true,
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if videoExts[ext] {
			videos = append(videos, filepath.Join(dir, entry.Name()))
		}
	}

	return videos, nil
}

// GenerateContent AI 콘텐츠 생성
func (a *App) GenerateContent(topic, platform string) (*ai.GeneratedContent, error) {
	ctx := context.Background()
	return a.ai.GenerateFullContent(ctx, topic, platform)
}

// GetContentIdeas 콘텐츠 아이디어 제안
func (a *App) GetContentIdeas(niche string, count int) ([]string, error) {
	ctx := context.Background()
	return a.ai.SuggestContentIdeas(ctx, niche, count)
}

// StartCollector 영상 수집기 시작
func (a *App) StartCollector(once bool) error {
	fmt.Println("🔍 영상 수집기 시작!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	ctx := context.Background()

	if once {
		// 한 번만 수집
		for _, source := range a.cfg.Collector.Sources {
			if !source.Enabled {
				continue
			}
			fmt.Printf("\n📥 %s에서 수집 중...\n", source.Type)
			videos, err := a.collector.CollectFromSource(ctx, source)
			if err != nil {
				fmt.Printf("❌ 수집 실패: %v\n", err)
				continue
			}
			fmt.Printf("✅ %d개 영상 수집 완료!\n", len(videos))
		}
		return nil
	}

	// 지속적으로 수집
	return a.collector.Start(ctx)
}

// DownloadVideo 단일 URL 다운로드
func (a *App) DownloadVideo(url string) error {
	ctx := context.Background()
	fmt.Printf("📥 다운로드 중: %s\n", url)

	video, err := a.collector.DownloadSingle(ctx, url)
	if err != nil {
		return err
	}

	fmt.Printf("✅ 저장됨: %s\n", video.FilePath)
	return nil
}

// CollectStock 저작권 프리 스톡 영상 수집
func (a *App) CollectStock(source, query string, count int) error {
	fmt.Println("🎬 저작권 프리 스톡 영상 수집!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	ctx := context.Background()

	// 스톡 영상 수집기 생성
	stockCollector := collector.NewStockVideoCollector(
		&a.cfg.Collector,
		a.cfg.Collector.StockVideo.PexelsAPIKey,
		a.cfg.Collector.StockVideo.PixabayAPIKey,
	)

	var videos []*collector.CollectedVideo
	var err error

	switch source {
	case "pexels":
		fmt.Println("📦 소스: Pexels (무료, 상업용 가능)")
		fmt.Printf("🔍 검색: %s\n", query)
		videos, err = stockCollector.SearchAndDownload(ctx, collector.StockPexels, query, count)
	case "pixabay":
		fmt.Println("📦 소스: Pixabay (무료, 상업용 가능, CC0)")
		fmt.Printf("🔍 검색: %s\n", query)
		videos, err = stockCollector.SearchAndDownload(ctx, collector.StockPixabay, query, count)
	default:
		return fmt.Errorf("지원하지 않는 소스: %s (pexels, pixabay 사용 가능)", source)
	}

	if err != nil {
		return err
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("✅ %d개 저작권 프리 영상 수집 완료!\n", len(videos))
	fmt.Println("💡 이 영상들은 상업적으로 자유롭게 사용 가능합니다!")

	return nil
}
