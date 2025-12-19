.PHONY: build run clean test install

# 변수
BINARY_NAME=clipbot
MAIN_PATH=./cmd/clipbot

# 빌드
build:
	go build -o $(BINARY_NAME) $(MAIN_PATH)

# Windows 빌드
build-windows:
	GOOS=windows GOARCH=amd64 go build -o $(BINARY_NAME).exe $(MAIN_PATH)

# macOS 빌드
build-mac:
	GOOS=darwin GOARCH=amd64 go build -o $(BINARY_NAME)-mac $(MAIN_PATH)

# Linux 빌드
build-linux:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME)-linux $(MAIN_PATH)

# 모든 플랫폼 빌드
build-all: build-windows build-mac build-linux

# 실행
run:
	go run $(MAIN_PATH)

# 전체 파이프라인 실행
run-full:
	go run $(MAIN_PATH) run

# 스케줄러 실행
run-schedule:
	go run $(MAIN_PATH) schedule

# 상태 확인
status:
	go run $(MAIN_PATH) status

# 테스트
test:
	go test -v ./...

# 커버리지
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# 의존성 설치
deps:
	go mod tidy
	go mod download

# 정리
clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME).exe $(BINARY_NAME)-mac $(BINARY_NAME)-linux
	rm -f coverage.out
	rm -rf temp/*

# 설치 (시스템 PATH에 복사)
install: build
	cp $(BINARY_NAME) /usr/local/bin/

# 디렉토리 초기화
init-dirs:
	mkdir -p input output temp archive data

# 설정 파일 복사
init-config:
	cp config.example.yaml config.yaml

# 전체 초기화
init: deps init-dirs init-config
	@echo "✅ ClipBot 초기화 완료!"
	@echo "📝 config.yaml 파일을 편집하여 API 키를 설정하세요."

# 도움말
help:
	@echo "ClipBot Makefile 명령어:"
	@echo ""
	@echo "  make build        - 현재 OS용 빌드"
	@echo "  make build-all    - 모든 플랫폼용 빌드"
	@echo "  make run          - 개발 모드 실행"
	@echo "  make run-full     - 전체 파이프라인 실행"
	@echo "  make run-schedule - 스케줄러 실행"
	@echo "  make status       - 상태 확인"
	@echo "  make test         - 테스트 실행"
	@echo "  make init         - 프로젝트 초기화"
	@echo "  make clean        - 빌드 파일 정리"
	@echo "  make help         - 도움말 표시"

