# Today Japanese 프로젝트 컨텍스트

## 프로젝트 위치

```text
/Users/jeffrey/Desktop/go-playground/jp
```

Go module:

```text
github.com/meshestra/go-playground/jp
```

에디터와 터미널은 상위 `go-playground`가 아니라 반드시 `jp` 폴더를 기준으로 사용한다.

## 프로젝트 목표

매일 일본어 단어·문장·문법 콘텐츠를 생성하고, 사용자의 학습 이력을 기억하며, 적절한 Tool을 선택하는 AI 일본어 학습 애플리케이션이다.

콘텐츠는 웹 학습 화면, 일본어 발음 재생, HelloTalk Moments 게시글, 사용자별 복습에 사용한다.

## 현재 구현된 기능

### Agent Runtime

- `llm.Provider` 추상화 및 OpenAI Provider
- Context Loading
- Learning Profile 및 최근 Episode 로딩
- 다중 Tool Call 및 Tool 결과 재전달
- 최대 반복 횟수 제한
- Context Cancellation·Timeout 전파
- Structured Logging
- Tool 실행 오류 전달

### Memory

- Session별 Short-term Memory
- 최근 메시지 제한 조회
- 사용자별 Learning Profile
- Episodic Memory
- 사용자별 데이터 격리
- 동시성 안전한 In-memory Store
- PostgreSQL Store 및 Migration

### Tools

- `translate`
- `grammar`
- `vocabulary`
- `review`
- Tool Registry
- JSON 객체·필수 필드·타입·추가 필드 검증

### 콘텐츠·웹·Docker

- 날짜별 일본어 콘텐츠 JSON 생성 및 저장
- Go `net/http` 웹 서버
- 반응형 학습 카드 UI
- 브라우저 일본어 음성 재생
- HelloTalk Moments 복사
- `Dockerfile`
- `docker-compose.yml`
- PostgreSQL 16 및 데이터 볼륨

기존 콘텐츠 생성기는 `internal/src-agent/agent.go`에 유지하고, 새 Tool-calling Runtime은 `internal/src-agent/runtime.go`로 분리했다.

## 주요 구조

```text
.
├── cmd/jp/main.go
├── Justfile
├── Dockerfile
├── docker-compose.yml
├── migrations/001_agent_memory.sql
├── internal/config/
├── internal/content/
├── internal/llm/
├── internal/memory/
├── internal/repository/postgres/
├── internal/tools/
├── internal/src-api/
│   ├── presentation/http/
│   ├── application/
│   ├── domain/
│   └── infrastructure/
├── internal/src-agent/
└── internal/src-batch/
```

## 환경변수

```env
OPENAI_API_KEY=sk-...
DATABASE_URL=
LLM_PROVIDER=openai
LLM_MODEL=gpt-5-mini
CONTENT_DIR=content
WEB_ADDR=:8080
DAILY_LEVEL=N3
TTS_PROVIDER=browser

POSTGRES_DB=today_japanese
POSTGRES_USER=today_japanese
POSTGRES_PASSWORD=today_japanese_dev
POSTGRES_PORT=5432
WEB_PORT=8080
```

API 키 변수명은 `OPEN_API_KEY`가 아니라 반드시 `OPENAI_API_KEY`다.

채팅에 API 키가 노출된 적이 있으므로 실제 키는 폐기하고 새 키를 사용하는 것이 안전하다.

## 로컬 실행

```bash
cd /Users/jeffrey/Desktop/go-playground/jp
just check
just generate
just serve
```

웹 주소:

```text
http://localhost:8080
```

Agent Runtime 실행:

```bash
go run ./cmd/jp -mode agent \
  -user-id demo-user \
  -session-id default \
  -input "오늘 공부할 일본어를 추천해줘"
```

주제 지정:

```bash
just generate topic="운동과 관련된 일본어"
```

## Docker 실행

Docker Desktop을 실행한 뒤:

```bash
cd /Users/jeffrey/Desktop/go-playground/jp
just docker-up
just docker-generate
just docker-down
```

Compose는 PostgreSQL을 `db` 서비스로 실행하고 앱 컨테이너에 `DATABASE_URL`을 주입한다. `OPENAI_API_KEY`는 호스트 `.env`에서 앱 컨테이너로 전달된다.

## 검증

```bash
GOCACHE=/tmp/today-japanese-go-cache go test ./...
GOCACHE=/tmp/today-japanese-go-cache go test -race ./...
GOCACHE=/tmp/today-japanese-go-cache go vet ./...
just check
just build
```

Go 테스트, race 테스트, vet은 통과한 상태다. Docker Desktop이 꺼져 있으면 Compose 이미지 빌드와 컨테이너 기동은 실행할 수 없다.

## 모듈 경로 문제

상위 디렉터리에도 다른 프로젝트의 `go.mod`가 있다. 에디터는 반드시 다음 폴더만 연다.

```text
/Users/jeffrey/Desktop/go-playground/jp
```

확인 명령:

```bash
cd /Users/jeffrey/Desktop/go-playground/jp
go env GOMOD
```

정상 결과:

```text
/Users/jeffrey/Desktop/go-playground/jp/go.mod
```

## 남은 확장 작업

- 실제 PostgreSQL 연결을 사용하는 Agent/Web 운영 검증
- 일일 자동 생성 스케줄러
- TTS 파일 생성 및 저장
- 복습 일정과 학습 결과의 구조화된 저장
- 사용자 인증 및 데이터 삭제
