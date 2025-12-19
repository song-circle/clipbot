package scheduler

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/clipbot/internal/config"
	"github.com/clipbot/internal/uploader"
	"github.com/robfig/cron/v3"
)

// Scheduler 업로드 스케줄러
type Scheduler struct {
	cfg        *config.ScheduleConfig
	storageCfg *config.StorageConfig
	uploader   *uploader.MultiUploader
	cron       *cron.Cron
	jobs       map[string]cron.EntryID
	mu         sync.RWMutex
	running    bool
	stopChan   chan struct{}
}

// ScheduledJob 예약된 작업
type ScheduledJob struct {
	ID          string    `json:"id"`
	VideoPath   string    `json:"video_path"`
	Platforms   []string  `json:"platforms"`
	ScheduledAt time.Time `json:"scheduled_at"`
	Status      string    `json:"status"` // pending, running, completed, failed
	Error       string    `json:"error,omitempty"`
}

// NewScheduler 새 스케줄러 생성
func NewScheduler(cfg *config.ScheduleConfig, storageCfg *config.StorageConfig, up *uploader.MultiUploader) *Scheduler {
	loc, _ := time.LoadLocation(cfg.Timezone)
	if loc == nil {
		loc = time.Local
	}

	return &Scheduler{
		cfg:        cfg,
		storageCfg: storageCfg,
		uploader:   up,
		cron:       cron.New(cron.WithLocation(loc)),
		jobs:       make(map[string]cron.EntryID),
		stopChan:   make(chan struct{}),
	}
}

// Start 스케줄러 시작
func (s *Scheduler) Start() error {
	if s.running {
		return fmt.Errorf("스케줄러가 이미 실행 중입니다")
	}

	// 설정된 시간에 업로드 작업 추가
	for _, postTime := range s.cfg.PostTimes {
		parts := strings.Split(postTime, ":")
		if len(parts) != 2 {
			continue
		}

		// cron 표현식 생성 (분 시 * * *)
		cronExpr := fmt.Sprintf("%s %s * * *", parts[1], parts[0])

		entryID, err := s.cron.AddFunc(cronExpr, func() {
			s.processScheduledUpload()
		})
		if err != nil {
			return fmt.Errorf("크론 작업 추가 실패 (%s): %w", postTime, err)
		}

		s.jobs[postTime] = entryID
		fmt.Printf("📅 업로드 예약: %s\n", postTime)
	}

	s.cron.Start()
	s.running = true

	fmt.Println("✅ 스케줄러 시작됨")
	fmt.Printf("⏰ 타임존: %s\n", s.cfg.Timezone)
	fmt.Printf("📊 일일 업로드 횟수: %d\n", s.cfg.PostsPerDay)

	// 무한 대기
	<-s.stopChan
	return nil
}

// Stop 스케줄러 중지
func (s *Scheduler) Stop() {
	if !s.running {
		return
	}

	s.cron.Stop()
	s.running = false
	close(s.stopChan)
	fmt.Println("⏹ 스케줄러 중지됨")
}

// processScheduledUpload 예약된 업로드 처리
func (s *Scheduler) processScheduledUpload() {
	// 주말 일시정지 확인
	if s.cfg.WeekendPause {
		weekday := time.Now().Weekday()
		if weekday == time.Saturday || weekday == time.Sunday {
			fmt.Println("⏸ 주말 - 업로드 건너뜀")
			return
		}
	}

	// 랜덤 딜레이 적용
	if s.cfg.RandomDelay > 0 {
		delay := time.Duration(rand.Intn(s.cfg.RandomDelay)) * time.Minute
		fmt.Printf("⏳ 랜덤 딜레이: %v\n", delay)
		time.Sleep(delay)
	}

	// 업로드할 영상 찾기
	videoPath, err := s.findNextVideo()
	if err != nil {
		fmt.Printf("❌ 영상 찾기 실패: %v\n", err)
		return
	}

	if videoPath == "" {
		fmt.Println("📭 업로드할 영상이 없습니다")
		return
	}

	fmt.Printf("🎬 업로드 시작: %s\n", filepath.Base(videoPath))

	// 모든 활성화된 플랫폼에 업로드
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	opts := &uploader.UploadOptions{
		Title:    extractTitleFromFilename(videoPath),
		Hashtags: []string{"#fyp", "#viral", "#trending"},
	}

	results := s.uploader.UploadToAll(ctx, videoPath, opts)

	// 결과 출력
	for platform, result := range results {
		if result.Success {
			fmt.Printf("✅ %s 업로드 완료: %s\n", platform, result.URL)
		} else {
			fmt.Printf("❌ %s 업로드 실패: %s\n", platform, result.Error)
		}
	}

	// 업로드된 영상을 아카이브로 이동
	s.archiveVideo(videoPath)
}

// findNextVideo 다음 업로드할 영상 찾기
func (s *Scheduler) findNextVideo() (string, error) {
	entries, err := os.ReadDir(s.storageCfg.OutputDir)
	if err != nil {
		// output 디렉토리가 없으면 input 디렉토리 확인
		entries, err = os.ReadDir(s.storageCfg.InputDir)
		if err != nil {
			return "", err
		}
		
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if isVideoFile(entry.Name()) {
				return filepath.Join(s.storageCfg.InputDir, entry.Name()), nil
			}
		}
		return "", nil
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if isVideoFile(entry.Name()) {
			return filepath.Join(s.storageCfg.OutputDir, entry.Name()), nil
		}
	}

	return "", nil
}

// archiveVideo 영상을 아카이브로 이동
func (s *Scheduler) archiveVideo(videoPath string) error {
	if err := os.MkdirAll(s.storageCfg.ArchiveDir, 0755); err != nil {
		return err
	}

	archivePath := filepath.Join(s.storageCfg.ArchiveDir, filepath.Base(videoPath))
	return os.Rename(videoPath, archivePath)
}

// ScheduleUpload 특정 시간에 업로드 예약
func (s *Scheduler) ScheduleUpload(videoPath string, platforms []string, scheduleAt time.Time) (*ScheduledJob, error) {
	job := &ScheduledJob{
		ID:          fmt.Sprintf("job_%d", time.Now().UnixNano()),
		VideoPath:   videoPath,
		Platforms:   platforms,
		ScheduledAt: scheduleAt,
		Status:      "pending",
	}

	// 예약 시간까지 대기 후 실행
	duration := time.Until(scheduleAt)
	if duration < 0 {
		return nil, fmt.Errorf("예약 시간이 과거입니다")
	}

	go func() {
		time.Sleep(duration)
		
		job.Status = "running"
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		opts := &uploader.UploadOptions{
			Title:    extractTitleFromFilename(videoPath),
			Hashtags: []string{"#fyp", "#viral"},
		}

		results := s.uploader.UploadTo(ctx, videoPath, uploader.ParsePlatforms(platforms), opts)

		allSuccess := true
		var errors []string
		for platform, result := range results {
			if !result.Success {
				allSuccess = false
				errors = append(errors, fmt.Sprintf("%s: %s", platform, result.Error))
			}
		}

		if allSuccess {
			job.Status = "completed"
		} else {
			job.Status = "failed"
			job.Error = strings.Join(errors, "; ")
		}
	}()

	return job, nil
}

// GetOptimalPostTimes 최적의 업로드 시간 추천
func (s *Scheduler) GetOptimalPostTimes(platform string) []string {
	// 플랫폼별 최적 업로드 시간 (한국 시간 기준)
	optimalTimes := map[string][]string{
		"tiktok":    {"07:00", "12:00", "19:00", "22:00"},
		"youtube":   {"14:00", "17:00", "20:00"},
		"instagram": {"08:00", "12:00", "18:00", "21:00"},
		"facebook":  {"09:00", "13:00", "16:00", "20:00"},
		"twitter":   {"08:00", "12:00", "17:00", "21:00"},
	}

	if times, exists := optimalTimes[platform]; exists {
		return times
	}
	return []string{"09:00", "15:00", "21:00"}
}

// GetNextScheduledTime 다음 예약된 업로드 시간
func (s *Scheduler) GetNextScheduledTime() time.Time {
	entries := s.cron.Entries()
	if len(entries) == 0 {
		return time.Time{}
	}

	var nextTime time.Time
	for _, entry := range entries {
		if nextTime.IsZero() || entry.Next.Before(nextTime) {
			nextTime = entry.Next
		}
	}
	return nextTime
}

// GetStatus 스케줄러 상태
func (s *Scheduler) GetStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"running":        s.running,
		"timezone":       s.cfg.Timezone,
		"posts_per_day":  s.cfg.PostsPerDay,
		"scheduled_times": s.cfg.PostTimes,
		"next_upload":    s.GetNextScheduledTime(),
		"active_jobs":    len(s.cron.Entries()),
	}
}

// isVideoFile 영상 파일인지 확인
func isVideoFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	videoExts := map[string]bool{
		".mp4":  true,
		".mov":  true,
		".avi":  true,
		".mkv":  true,
		".webm": true,
		".m4v":  true,
	}
	return videoExts[ext]
}

// extractTitleFromFilename 파일명에서 제목 추출
func extractTitleFromFilename(path string) string {
	filename := filepath.Base(path)
	ext := filepath.Ext(filename)
	title := strings.TrimSuffix(filename, ext)
	
	// 특수문자 정리
	title = strings.ReplaceAll(title, "_", " ")
	title = strings.ReplaceAll(title, "-", " ")
	
	return title
}

