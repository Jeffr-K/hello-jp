# Hello JP 작업 문서

이 문서는 현재 시스템의 완료 내역, 즉시 해야 할 작업, 장기 백로그를 한 곳에서 관리하기 위한 기준 문서다.

## 1. 현재 제품 방향

- 메인 화면은 ChatGPT형 일본어 RAG 챗봇이다.
- 오늘의 학습은 챗 UI가 아닌 모니터링 대시보드형 화면이다.
- 콘텐츠는 대화, 문장, 문법, 단어, 문화·역사, 시각 자료, 연습 문제로 확장한다.
- 사용자는 로그인 후 자신의 학습 주제를 설정한다.
- 학습 주제는 오늘 즉시 적용하거나 내일부터 적용하도록 예약할 수 있다.
- 인증되지 않은 사용자는 챗봇과 개인 학습 데이터에 접근할 수 없다.

## 2. 완료된 작업

### UI/프론트엔드

- React 19 + Vite 프론트엔드 구성
- 개발 포트 `5200`, Docker 운영 포트 `8082`
- ChatGPT형 메시지 스트리밍 UI
- SSE 응답 스트리밍
- 응답 생성 시간 표시
- 응답 생성 중 로딩바 표시
- 새 학습 세션 생성 및 대화 히스토리 연결
- 대화 히스토리 검색 및 hover 삭제 아이콘
- 반응형 좌측/우측 패널
- 우측 패널의 오늘의 학습 링크
- 헤더 지구본 SVG 아이콘 및 언어 메뉴
- 헤더 다크/라이트 테마 SVG 아이콘
- 테마 설정 `localStorage` 저장
- 오늘의 학습 대시보드형 그리드
- 오늘의 학습 중앙 영역 전체 폭 사용
- 오늘의 학습 내부 스크롤
- 모바일 1열, 태블릿 2열, 데스크톱 다중 그리드

### 백엔드/API

- Go HTTP 서버를 `internal/web`에서 `internal/api`로 분리
- 애플리케이션 진입점을 `cmd/jp/main.go`로 이동
- 채팅 일반 응답 API
- 채팅 SSE API
- 세션 히스토리 API
- 오늘의 콘텐츠 API
- 학습 상태 조회/저장 API
- PostgreSQL 메모리 저장소
- 학습 상태 저장 테이블
- Go 런타임과 React production asset 연결

### 콘텐츠/RAG

- 오늘의 콘텐츠 모델 확장
- 상황 기반 대화 모델
- 문화·역사 모델
- 시각 콘텐츠 모델
- 연습 문제 모델
- 콘텐츠를 대화/문화/문법/단어/문장 단위로 검색 문서화
- Qdrant 컬렉션 생성 및 벡터 upsert/search/delete
- OpenAI 임베딩 지원
- Ollama 임베딩 지원
- Ollama 기본 임베딩 모델 `nomic-embed-text`
- 생성된 일일 콘텐츠의 Qdrant 자동 색인

### 실행/운영

- PostgreSQL Docker Compose 구성
- Qdrant Docker Compose 구성
- React를 Docker build 단계에서 production bundle로 생성
- GitHub Actions CI 구성
  - Go vet
  - Go test
  - React build
  - Docker build
- 루트 `i18n` 리소스 구성
  - `ko.json`
  - `ja.json`
  - `en.json`

## 3. 즉시 해야 할 작업

### P0 — 인증 기반 구축

- [x] 사용자 테이블 생성
  - `id`
  - `email`
  - `password_hash`
  - `name`
  - `created_at`
  - `updated_at`
- [x] 회원가입 API
  - 이메일 형식 검증
  - 비밀번호 길이/복잡도 검증
  - 중복 이메일 차단
  - bcrypt 또는 Argon2id 해시 저장
- [x] 로그인 API
  - 이메일/비밀번호 검증
  - JWT access token 발급
  - 만료 시간 설정
- [x] JWT 검증 미들웨어
  - 서명 알고리즘 고정
  - 만료 시간 검증
  - issuer/audience 검증
  - `Authorization: Bearer <token>` 처리
- [ ] 로그아웃 처리
  - 클라이언트 토큰 제거
  - refresh token을 도입할 경우 서버 폐기 처리
- [x] 인증 필요 API 적용
  - 채팅
  - SSE 채팅
  - 히스토리
  - 학습 상태
  - 내 정보
  - 학습 설정
- [x] 현재 임시 사용자 ID `web-user` 제거

### P0 — 인증 화면

- [x] 로그인 페이지
- [x] 회원가입 폼
- [ ] 인증 실패/중복 이메일/만료 토큰 오류 메시지
- [ ] 로그인 상태 유지
- [ ] 인증 만료 시 로그인 페이지로 이동
- [ ] 모바일 로그인 레이아웃

### P0 — 내 정보 및 주제 설정

- [x] 내 정보 페이지
  - 이름
  - 이메일
  - 레벨
  - 학습 목표
  - 가입일
- [x] 오늘의 학습 설정 페이지
- [x] 학습 주제 입력 및 저장
- [x] 주제 적용 시점 선택
  - 오늘 즉시 적용
  - 내일부터 적용
- [x] 현재 적용 중인 주제 표시
- [x] 예약된 다음 주제 표시
- [ ] 주제 변경 이력 저장
- [ ] 사용자별 기본 레벨 저장

### P0 — 오늘 콘텐츠 생성 개인화

- [x] 사용자별 학습 설정 스키마 설계
- [x] `topic_settings` 테이블 생성
- [x] 오늘 적용 주제와 내일 적용 주제 분리 저장
- [ ] 스케줄러가 사용자별 예약 주제를 조회
- [ ] 사용자별 콘텐츠 생성 전략 결정
  - 사용자마다 별도 JSON 생성
  - 공통 주제 콘텐츠와 개인 설정을 조합
- [ ] 콘텐츠 ID에 사용자/날짜/주제 버전 포함
- [ ] 생성 성공/실패 상태 저장
- [ ] 생성 실패 재시도 정책 추가
- [ ] 생성 완료 후 Qdrant 색인
- [ ] 사용자가 저장한 주제가 다음 생성 요청에 실제 반영되는지 검증

## 4. 인증/JWT 설계 기준

### 토큰

- Access token 만료 시간: 짧게 설정한다. 예: 15분
- Refresh token을 사용할 경우 DB에 해시만 저장한다.
- JWT secret은 환경변수로만 주입한다.
- 개발용 기본 secret을 코드에 두지 않는다.
- 토큰 payload에는 최소 정보만 저장한다.
  - `sub`: 사용자 ID
  - `iss`: 발급자
  - `aud`: 대상
  - `iat`: 발급 시각
  - `exp`: 만료 시각

### 보안

- 비밀번호 평문 저장 금지
- JWT를 URL query string에 넣지 않는다.
- 브라우저 저장은 가능하면 HttpOnly/Secure/SameSite 쿠키를 우선 검토한다.
- CORS 허용 origin을 명시한다.
- 인증 실패 응답에서 이메일 존재 여부를 노출하지 않는다.
- 로그인 시도 rate limit을 추가한다.
- 모든 개인 API에서 path/query의 사용자 ID를 신뢰하지 않고 JWT subject를 사용한다.

## 5. 데이터 모델 백로그

```text
users
user_sessions
refresh_tokens
learning_profiles
learning_settings
learning_topic_schedules
daily_lessons
daily_lesson_chunks
learning_states
agent_messages
learning_episodes
```

권장 관계:

```text
users
 ├─ learning_profiles
 ├─ learning_settings
 ├─ learning_topic_schedules
 ├─ daily_lessons
 ├─ learning_states
 ├─ agent_messages
 └─ learning_episodes
```

## 6. 스케줄러 백로그

- [ ] 서버 재시작 시 누락된 오늘 콘텐츠 확인
- [ ] 컴퓨터가 꺼져 있던 기간의 콘텐츠 보정 생성
- [ ] 스케줄러 실행 상태 health endpoint
- [ ] 마지막 실행 시각/성공 여부 저장
- [ ] 생성 실패 알림
- [ ] 주제별 생성 큐
- [ ] 중복 생성 방지용 idempotency key
- [ ] Qdrant 연결 실패 시 재시도
- [ ] Ollama 모델 미설치 시 명확한 오류 표시
- [ ] 오늘 콘텐츠가 없을 때 가장 최근 콘텐츠를 임시 표시할지 정책 결정

## 7. RAG 백로그

- [ ] 사용자별 문서 namespace 분리
- [ ] 콘텐츠 버전 및 source metadata 추가
- [ ] 사용자 수준/학습 상태를 검색 필터에 반영
- [ ] 최근 학습 문서에 가중치 적용
- [ ] 중복 chunk 제거
- [ ] 검색 결과 citation/source 표시
- [ ] 검색 실패 시 일반 대화 fallback
- [ ] Qdrant collection migration 전략
- [ ] 임베딩 모델 변경 시 전체 재색인 명령
- [ ] retrieval 품질 평가 세트 작성

## 8. 품질/운영 백로그

- [ ] 인증 API 단위 테스트
- [ ] JWT 만료/위조/재사용 테스트
- [ ] 회원가입·로그인 e2e 테스트
- [ ] 오늘 주제 즉시 적용 e2e 테스트
- [ ] 내일 주제 예약 e2e 테스트
- [ ] 사용자 간 데이터 격리 테스트
- [ ] 반응형 주요 breakpoint 시각 검증
- [ ] 접근성: keyboard navigation, focus, aria-label
- [ ] i18n 리소스를 실제 React 번역 함수와 연결
- [ ] 로그에 API key/JWT/비밀번호가 노출되지 않는지 검증
- [ ] Docker Compose startup/readiness 검증
- [ ] CI에서 migration 검증

## 9. 완료 기준

이번 인증/개인화 작업은 아래 조건을 모두 만족해야 완료로 본다.

- 신규 사용자가 회원가입할 수 있다.
- 사용자가 로그인하면 JWT가 발급된다.
- 인증되지 않은 사용자는 개인 API를 사용할 수 없다.
- 로그인 사용자는 자신의 내 정보와 학습 설정을 볼 수 있다.
- 오늘 주제를 저장하면 오늘 콘텐츠 생성 요청에 즉시 반영된다.
- 내일 주제를 저장하면 다음 날짜 생성부터 반영된다.
- 다른 사용자의 대화/학습 상태/주제 설정을 조회할 수 없다.
- 생성된 개인화 콘텐츠가 Qdrant에 색인된다.
- 챗봇 RAG 검색이 현재 사용자 학습 콘텐츠를 우선 활용한다.
- Go 테스트, React 빌드, Docker 빌드, CI가 모두 통과한다.

## 10. 현재 알려진 운영 상태

- 오늘 콘텐츠는 `content/YYYY-MM-DD.json`이 있어야 조회된다.
- 로컬 스케줄러는 컴퓨터가 꺼져 있으면 실행되지 않는다.
- 현재 스케줄러는 환경변수 `DAILY_TOPIC` 기반이며, 사용자별 예약 주제 처리는 아직 구현 전이다.
- 오늘 주제 저장 시 인증 사용자별 콘텐츠 사본을 즉시 생성하는 경로는 구현되었다.
- 내일 예약 주제를 스케줄러가 사용자별로 읽어 생성하는 작업은 아직 남아 있다.
