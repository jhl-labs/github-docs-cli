# github-docs-cli

`github-docs-cli`는 **하나의 GitHub 저장소를 문서 도구로** 사용하기 위한 CLI입니다.
AI Agent와 자동화 스크립트가 저장소의 **이슈·디스커션·위키**, 그리고 GitHub Pages로 발행되는
**`docs/` 폴더**를 안정적으로 읽고/쓰기 위해 만듭니다.

> **상태: 초기 동작 버전 (alpha).**
> Go로 구현된 CLI가 동작합니다: `issue` / `discussion` / `docs` / `pages` / `wiki` / `note`.
> 표준 라이브러리만 사용하며(외부 의존성 0), 단일 바이너리로 크로스플랫폼 배포가 가능합니다.

## 왜 이 프로젝트가 필요한가

[`confluence-cli`](https://github.com/jhl-labs/confluence-cli)가 셀프호스팅 Confluence를 AI Agent의
문서 계층으로 감쌌다면, `github-docs-cli`는 같은 발상을 **GitHub**에 적용합니다.

개발 노트·연구 노트·의사결정 사항을 외부 도구에 흩어 두는 대신, 팀이 이미 사용하는 **GitHub 저장소**에
남기면 코드와 문서·이력이 한곳에 모입니다. 이를 위해 다음이 필요했습니다.

- AI Agent가 이슈/디스커션/위키/`docs/`를 일관된 서브커맨드로 다루는 **전용 CLI 계층**
- 인증(토큰), 재시도(429/5xx·secondary rate limit), Pages/Contents 처리를 일괄 래핑
- Skill이 필요한 순간에만 도구 정보를 불러올 수 있게 하는 의도적인 경량화
- AI Agent가 **hook**으로 의사결정/연구 노트를 GitHub에 기록하는 `note` 명령

> GitHub 공식 MCP 서버가 존재하지만, 사내 감사 추적·최소 권한·단일 정적 바이너리 배포 관점에서
> 자체 CLI 계층을 두는 것이 이 프로젝트의 선택입니다. GitHub.com과 **Enterprise Server**를 모두 지원합니다.

## 핵심 기능

구현 완료:

- `github-docs-cli issue` : 이슈 목록/조회/생성/수정/댓글/라벨 — 이슈를 문서·노트로 사용
- `github-docs-cli discussion` : 디스커션 목록/조회/생성/댓글/카테고리 (GraphQL API)
- `github-docs-cli docs` : `docs/` 파일 목록/트리/조회/작성/삭제 (Contents API)
- `github-docs-cli pages` : GitHub Pages 설정 조회/활성화/빌드 트리거
- `github-docs-cli wiki` : 위키 페이지 목록/조회/작성/삭제 (`<repo>.wiki.git` 클론, `git` 필요)
- `github-docs-cli note` : 개발/연구/의사결정 노트 기록 (AI Agent hook용)
- `github-docs-cli generate-workflow` : GitHub Actions 워크플로우 생성 (pages/jekyll/link-check)
- `github-docs-cli generate-skill` : AI 에이전트용 스킬 문서 생성 (claude/codex/gemini/opencode/generic)

계획 중 (로드맵 참고):

- `release` : 릴리스/태그 관리, `attachment`/`asset` 업로드
- 마크다운 ↔ 이슈/디스커션 동기화, 페이지네이션·대량 처리

## 설치

소개 페이지: <https://jhl-labs.github.io/github-docs-cli>

```bash
# 스크립트 설치 (Linux/macOS) — 최신 릴리스 바이너리를 받습니다
curl -fsSL https://jhl-labs.github.io/github-docs-cli/install.sh | sh
```

또는 [Releases](https://github.com/jhl-labs/github-docs-cli/releases/latest)에서
플랫폼별 단일 바이너리를 직접 내려받을 수 있습니다 (Linux/macOS amd64·arm64, Windows amd64).

## 소스에서 빌드

Go 1.26+ 환경이 필요합니다.

```bash
make build          # ./github-docs-cli 생성
# 또는
go build -o github-docs-cli .

make dist           # 전체 플랫폼 릴리스 바이너리 (dist/)
make test           # 테스트
```

크로스 컴파일은 Go 표준 방식 그대로 동작합니다.

```bash
GOOS=windows GOARCH=amd64 go build -o github-docs-cli.exe .
GOOS=darwin  GOARCH=arm64 go build -o github-docs-cli .
```

## 사용법

모든 명령은 `github-docs-cli <그룹> <액션> [flags]` 형태이며, 기본적으로 **JSON**을 출력합니다.
`--output text`로 사람이 읽기 쉬운 요약을 얻을 수 있습니다.

### issue — 이슈를 문서/노트로

```bash
github-docs-cli issue list --state open --label docs --limit 20
github-docs-cli issue get --number 42 --comments --output text
github-docs-cli issue create --title "Design doc" --body-file design.md --label docs
github-docs-cli issue update --number 42 --state closed
github-docs-cli issue comment --number 42 --body "Shipped in v1.2"
github-docs-cli issue label --number 42 --add "docs,decision" --remove "draft"
```

### discussion — 디스커션 (GraphQL)

```bash
github-docs-cli discussion categories
github-docs-cli discussion list --limit 20
github-docs-cli discussion get --number 7 --output text
github-docs-cli discussion create --category "Ideas" --title "RFC" --body-file rfc.md
github-docs-cli discussion comment --number 7 --body "Agreed."
```

### docs — `docs/` 파일 관리 (Contents API)

```bash
github-docs-cli docs tree --path docs
github-docs-cli docs list --path docs
github-docs-cli docs get --path docs/guide.md                 # 원본 파일 출력
github-docs-cli docs put --path docs/guide.md --body-file guide.md --message "docs: update guide"
github-docs-cli docs delete --path docs/old.md --message "docs: remove old page"
```

`docs put`은 기존 파일의 blob SHA를 자동으로 조회하여 **생성/수정**을 알아서 처리합니다.

### pages — GitHub Pages 발행

```bash
github-docs-cli pages get
github-docs-cli pages enable --path /docs      # docs/ 폴더로 빌드
github-docs-cli pages build                     # 재빌드 트리거
```

### wiki — 저장소 위키 (`git` 바이너리 필요)

위키는 별도 git 저장소(`<repo>.wiki.git`)이므로, 이 명령들은 위키를 임시 디렉터리에 클론한 뒤
변경하고 push 합니다.

```bash
github-docs-cli wiki list
github-docs-cli wiki get --page Home
github-docs-cli wiki set --page "Runbook" --body-file runbook.md
github-docs-cli wiki delete --page "Old Page"
```

### note — 개발/연구/의사결정 노트 기록 (AI Agent hook)

AI Agent가 hook에서 한 줄로 호출해 결정 사항을 GitHub에 남기는 용도입니다.
기본 동작은 타임스탬프가 붙은 Markdown 항목을 `docs/notes/DEVLOG.md`에 **append**(Contents API 커밋)합니다.

```bash
# 기본: docs/notes/DEVLOG.md 에 항목 추가
github-docs-cli note add --kind decision --title "Use Go" --body "Single static binary."

# 다른 파일로
github-docs-cli note add --kind research --title "Bench" --body-file findings.md --docs docs/notes/research.md

# 이슈/디스커션 댓글로
github-docs-cli note add --kind dev --title "Refactor" --body "..." --issue 42
github-docs-cli note add --kind decision --title "ADR-1" --body-file adr.md --new-issue
```

`--kind`는 `dev | research | decision | note` 중 하나입니다.

### generate-workflow / generate-skill

```bash
github-docs-cli generate-workflow pages        # .github/workflows/pages.yml (docs/ → Pages 배포)
github-docs-cli generate-workflow jekyll       # Jekyll 빌드 후 배포
github-docs-cli generate-workflow link-check   # PR에서 docs 링크 검사

github-docs-cli generate-skill                 # 범용(generic)
github-docs-cli generate-skill claude          # Claude SKILL.md (프론트매터 포함)
github-docs-cli generate-skill codex --stdout  # 파일 대신 표준출력
```

`generate-skill`은 이 CLI의 사용법(인증·명령어·Markdown 본문 주의사항)을 담은 스킬 문서를 생성합니다.

- 인자 없음 / `generic` : 프론트매터 없는 범용 마크다운
- `claude` : `name`/`description` YAML 프론트매터를 가진 Claude 스킬(`SKILL.md`로 사용)
- `codex` / `opencode` : `AGENTS.md` 컨벤션
- `gemini` : `GEMINI.md` 컨텍스트 파일 컨벤션

## 설정 우선순위

낮음 → 높음 순으로 덮어씁니다: **설정 파일 < 환경변수 < 커맨드라인 플래그**

- 설정 파일: `$GITHUB_DOCS_CONFIG` 또는 `~/.config/github-docs-cli/config.json`
  (Windows는 `%AppData%`, macOS는 `~/Library/Application Support` — `os.UserConfigDir` 기준)
- 예시는 [`config.example.json`](./config.example.json) 참고

## 인증

GitHub REST/GraphQL API는 토큰을 `Authorization: Bearer <token>` 헤더로 호출합니다.

- **Personal Access Token (권장)** — *Settings → Developer settings → Personal access tokens*에서 발급.
  Fine-grained 토큰으로 대상 저장소와 최소 권한만 부여하는 것을 권장합니다.
- **`GITHUB_TOKEN`** — GitHub Actions 안에서 자동 주입되는 토큰도 그대로 사용할 수 있습니다.

권한(scope): 이슈/콘텐츠/Pages에는 `repo`, 디스커션에는 `write:discussion`이 필요합니다.

### 환경변수 (예시)

| 변수 | 설명 | 예시 |
|---|---|---|
| `GITHUB_TOKEN` (또는 `GH_TOKEN`) | 인증 토큰 (Bearer) | `ghp_…` |
| `GITHUB_REPO` | 기본 저장소 `owner/name` (`--repo` 생략 가능) | `jhl-labs/github-docs-cli` |
| `GITHUB_API_URL` | REST 베이스 URL (기본 `https://api.github.com`) | `https://ghe.example.com/api/v3` |
| `GITHUB_INSECURE` | TLS 검증 생략 (사내 CA/GHE) | `false` |

```bash
# 토큰 동작 확인 예시
curl -H "Authorization: Bearer $GITHUB_TOKEN" \
  "$GITHUB_API_URL/repos/$GITHUB_REPO" | head
```

> 토큰은 셸 히스토리·로그에 남지 않도록 `.env`(gitignore) 또는 시크릿 매니저로 관리합니다.

## REST / GraphQL API 기준

- 이슈/라벨/콘텐츠/Pages: REST API (`{API_URL}/repos/{owner}/{repo}/...`)
- 디스커션: GraphQL API (`{API_URL}/graphql`, GHE는 `/api/graphql`)
- 본문 형식: GitHub-Flavored **Markdown** (이슈·디스커션·노트·위키 모두). HTML이 아닙니다.
- 공식 문서: <https://docs.github.com/rest>, <https://docs.github.com/graphql>

## AI Agent 연동 (Skill + Hook)

1. **Skill**: `github-docs-cli generate-skill claude --out .claude/skills/github-docs-cli/SKILL.md`로
   에이전트에게 도구 사용법을 주입합니다. (다른 플랫폼은 해당 플래버 사용)
2. **Hook**: 에이전트가 의사결정을 내릴 때마다 `github-docs-cli note add --kind decision ...`을 호출하면
   결정/연구/개발 노트가 저장소에 자동으로 누적됩니다.

## 프로젝트 구조

```
.
├── main.go                  # 엔트리포인트, 그룹 디스패치
├── common.go                # 공통 플래그(인증/출력) + 클라이언트 + 서브 디스패치
├── output.go                # JSON / text 출력
├── cmd_issue.go             # issue (list/get/create/update/comment/label)
├── cmd_discussion.go        # discussion (GraphQL)
├── cmd_docs.go              # docs (Contents API)
├── cmd_pages.go             # pages
├── cmd_wiki.go              # wiki (git 클론)
├── cmd_note.go              # note (AI hook)
├── cmd_generate_skill.go    # generate-skill
├── cmd_generate_workflow.go # generate-workflow
├── internal/
│   ├── config/              # 설정 로딩 (파일 < 환경변수 < 플래그)
│   └── github/              # REST/GraphQL 클라이언트 (auth, 재시도, 에러) + 이슈/디스커션/콘텐츠 API
└── Makefile                 # build / test / dist(크로스컴파일) / skills
```

## 로드맵

- [x] 언어/런타임 확정(Go) 및 프로젝트 스캐폴딩
- [x] 인증 계층 (토큰) + 설정 로딩(파일/환경변수/플래그)
- [x] `issue` / `discussion` 읽기·쓰기 명령
- [x] `docs`(Contents) / `pages` 발행 명령
- [x] `wiki` 관리 (git 클론 기반)
- [x] `note` hook 명령 (개발/연구/의사결정 기록)
- [x] `generate-skill` / `generate-workflow`
- [x] JSON 출력 표준화 + 에러/재시도(429·5xx·secondary rate limit, Retry-After) 전략
- [x] httptest 기반 단위 테스트
- [ ] `release`/`asset` 관리, 첨부 업로드
- [ ] 페이지네이션(`--page`/cursor) 및 대량 처리
- [ ] `examples/` 문서 보강

## 운영 원칙

- 토큰은 절대 코드나 로그에 노출하지 않음 (최소 권한 토큰 권장)
- 문서/업데이트는 최소 권한 원칙으로 수행
- 작업 단위는 명령형, 응답은 AI가 소비하기 쉬운 JSON으로 통일

## 변경 이력

[CHANGELOG.md](./CHANGELOG.md) 참고.

## 라이선스

[JHL License](./LICENSE) — 개인·교육·비상업적 용도로 자유롭게 사용/수정/배포할 수 있습니다.
**바이너리와 소스 코드 모두 상업적 사용은 금지**되며, 상업적 사용은 개발자(Licensor)의
사전 서면 허가가 있는 경우에만 허용됩니다. 자세한 내용은 LICENSE 파일을 참고하세요.
