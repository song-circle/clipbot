# 🎬 ClipBot

**자동 영상 편집 & 멀티 플랫폼 업로드 봇**

영상을 자동으로 쇼츠/릴스 형식으로 편집하고, 수익화 가능한 모든 SNS에 자동 업로드합니다.

## ✨ 주요 기능

### 1. 🔍 자동 영상 수집
- **YouTube**: 채널, 검색, 트렌딩, Shorts 자동 수집
- **Reddit**: 인기 서브레딧 바이럴 영상 수집
- **TikTok**: 해시태그, 트렌딩 영상 수집
- **Twitter/X**: 바이럴 영상 트윗 수집
- **필터링**: 조회수, 길이, 키워드 기반 자동 필터

### 2. 🎥 자동 영상 편집
- **세로 영상 변환**: 가로 영상을 9:16 비율로 자동 크롭
- **자동 자막 생성**: Whisper AI로 음성 인식 → 틱톡 스타일 자막
- **효과 적용**: 페이드, 줌, 트랜지션 등
- **배경 음악 추가**: 저작권 프리 음악 자동 삽입
- **썸네일 자동 생성**

### 3. 📤 멀티 플랫폼 업로드
| 플랫폼 | 수익화 방법 | 조건 |
|--------|------------|------|
| **TikTok** | Creator Fund | 팔로워 10K+, 조회수 100K+ |
| **YouTube Shorts** | 광고 수익 | 구독자 1K+, 시청 4000시간 |
| **Instagram Reels** | Reels Play 보너스 | 초대 기반 |
| **Facebook Reels** | 인스트림 광고 | 팔로워 10K+ |
| **Twitter/X** | Creator 수익 공유 | 블루 구독 |

### 4. 🤖 AI 콘텐츠 생성
- **자동 제목 생성**: 바이럴 가능한 제목 AI 생성
- **해시태그 추천**: 플랫폼별 최적화된 해시태그
- **설명 작성**: 참여 유도 문구 자동 생성
- **콘텐츠 아이디어**: 니치 기반 아이디어 제안

### 5. ⏰ 스케줄링
- **자동 업로드**: 설정한 시간에 자동 게시
- **최적 시간 분석**: 플랫폼별 최적 업로드 시간
- **랜덤 딜레이**: 봇 감지 방지
- **주말 일시정지**: 선택적 주말 휴무

## 🚀 시작하기

### 필수 요구사항

```bash
# Go 1.21 이상
go version

# yt-dlp 설치 (영상 수집용)
# Windows
pip install yt-dlp
# 또는 choco install yt-dlp

# macOS
brew install yt-dlp

# Linux
pip install yt-dlp

# FFmpeg 설치
# Windows (Chocolatey)
choco install ffmpeg

# macOS (Homebrew)
brew install ffmpeg

# Linux (apt)
sudo apt install ffmpeg

# Whisper 설치 (자막 생성용)
pip install openai-whisper
```

### 설치

```bash
# 저장소 클론
git clone https://github.com/your-username/clipbot.git
cd clipbot

# 의존성 설치
go mod tidy

# 빌드
go build -o clipbot ./cmd/clipbot
```

### 설정

```bash
# 설정 파일 복사
cp config.example.yaml config.yaml

# 설정 파일 편집
# API 키 및 인증 정보 입력
```

## 📖 사용법

### 전체 파이프라인 실행

```bash
# input 폴더의 모든 영상을 편집하고 업로드
./clipbot run
```

### 영상 자동 수집

```bash
# 설정된 소스에서 영상 수집 (지속 실행)
./clipbot collect

# 한 번만 수집 후 종료
./clipbot collect --once

# 특정 URL에서 영상 다운로드
./clipbot download https://youtube.com/shorts/xxxxx
./clipbot download https://www.tiktok.com/@user/video/xxxxx
./clipbot download https://www.reddit.com/r/videos/comments/xxxxx
```

### 영상 편집만

```bash
# 단일 영상 편집
./clipbot edit video.mp4 output.mp4

# 쇼츠 형식으로 자동 변환
./clipbot edit --shorts video.mp4
```

### 업로드만

```bash
# 특정 플랫폼에 업로드
./clipbot upload video.mp4 tiktok youtube

# 모든 플랫폼에 업로드
./clipbot upload video.mp4 all
```

### 스케줄러 실행

```bash
# 스케줄러 시작 (백그라운드 자동 업로드)
./clipbot schedule
```

### 상태 확인

```bash
# 현재 상태 및 수익 현황
./clipbot status
```

## ⚙️ 플랫폼별 설정 가이드

### TikTok

1. 브라우저에서 TikTok 로그인
2. 개발자 도구 → Application → Cookies
3. `sessionid` 값 복사
4. `config.yaml`에 붙여넣기

### YouTube

1. [Google Cloud Console](https://console.cloud.google.com) 접속
2. 새 프로젝트 생성
3. YouTube Data API v3 활성화
4. OAuth 2.0 클라이언트 ID 생성
5. `client_id`, `client_secret` 복사
6. 첫 실행 시 브라우저 인증 진행

### Instagram

1. ⚠️ 2단계 인증 비활성화 필요
2. `username`, `password` 입력
3. 또는 세션 쿠키 직접 입력

### Facebook

1. [Facebook Developers](https://developers.facebook.com) 접속
2. 앱 생성 → 페이지 액세스 토큰 발급
3. `access_token`, `page_id` 입력

### Twitter/X

1. [Twitter Developer Portal](https://developer.twitter.com) 접속
2. 앱 생성 → API 키 발급
3. `api_key`, `api_secret`, `access_token`, `access_secret` 입력

## 📁 폴더 구조

```
clipbot/
├── input/          # 원본 영상 넣는 곳
├── output/         # 편집된 영상
├── archive/        # 업로드 완료된 영상
├── temp/           # 임시 파일
├── data/           # 데이터베이스
├── config.yaml     # 설정 파일
└── clipbot         # 실행 파일
```

## 💰 수익화 전략

### 콘텐츠 전략

1. **일관성**: 매일 같은 시간에 업로드
2. **니치**: 특정 주제에 집중
3. **트렌드**: 현재 트렌드 활용
4. **호기심 유발**: 3초 안에 시선 끌기

### 최적 업로드 시간 (한국 기준)

| 플랫폼 | 최적 시간 |
|--------|----------|
| TikTok | 오전 7시, 오후 12시, 저녁 7시, 밤 10시 |
| YouTube | 오후 2시, 5시, 8시 |
| Instagram | 오전 8시, 오후 12시, 저녁 6시, 9시 |
| Facebook | 오전 9시, 오후 1시, 4시, 8시 |
| Twitter | 오전 8시, 오후 12시, 5시, 9시 |

### 수익화 요건

| 플랫폼 | 팔로워 | 추가 조건 |
|--------|--------|----------|
| TikTok | 10,000+ | 30일간 10만 조회 |
| YouTube | 1,000+ | 4,000 시청 시간 또는 1000만 Shorts 조회 |
| Instagram | 초대제 | 프로페셔널 계정 |
| Facebook | 10,000+ | 60만분 시청 시간 |
| Twitter | 블루 구독 | 500+ 팔로워 |

## ⚠️ 주의사항

1. **API 제한**: 각 플랫폼의 API 사용량 제한 확인
2. **저작권**: 음악, 영상 저작권 확인 필수
3. **커뮤니티 가이드라인**: 각 플랫폼 규정 준수
4. **계정 보안**: API 키는 환경변수로 관리 권장

```bash
# 환경변수 설정 예시
export CLIPBOT_OPENAI_KEY="your-openai-key"
export CLIPBOT_YOUTUBE_CLIENT_SECRET="your-secret"
export CLIPBOT_TIKTOK_SESSION="your-session-id"
```

## 🛠 개발

```bash
# 개발 모드 실행
go run ./cmd/clipbot

# 테스트
go test ./...

# 린트
golangci-lint run
```

## 📄 라이선스

MIT License

## 🤝 기여

Pull Request 환영합니다!

1. Fork
2. Feature 브랜치 생성 (`git checkout -b feature/amazing`)
3. 커밋 (`git commit -m 'Add amazing feature'`)
4. Push (`git push origin feature/amazing`)
5. Pull Request 생성

---

Made with ❤️ for content creators

