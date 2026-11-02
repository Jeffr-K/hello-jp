import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent, type ReactNode, type RefObject } from 'react'

type Role = 'user' | 'assistant'
type ChatMessage = { role: Role; content: string; elapsedMs?: number }
type ChatSession = { id: string; title: string; preview: string }
type DailyContent = {
  date: string; level: string; theme: string
  words: { word: string; reading: string; meaning: string; example: string; example_translation: string }[]
  sentences: { japanese: string; reading: string; translation: string }[]
  grammar: { pattern: string; meaning: string; explanation: string; examples: { japanese: string; reading: string; translation: string }[] }
  moments_text: string
  situation?: string
  dialogue?: { speaker: string; japanese: string; reading: string; translation: string }[]
  culture?: { title: string; summary: string; period?: string; source?: string }
  visual?: { prompt: string; alt: string; url?: string }
  practice?: { type: string; prompt: string; answer: string }[]
}
type LearningState = { lesson_id: string; viewed: boolean; practiced: boolean; correct: number; attempts: number; mastered: boolean; last_seen_at?: string }

const historyKey = 'today-japanese-chat-history'
const sessionKey = 'today-japanese-session-id'
const newSessionId = () => 'session-' + Date.now()

function Icon({ name }: { name: 'menu' | 'panel-left' | 'panel-right' | 'plus' | 'x' | 'send' | 'globe' | 'moon' | 'sun' }) {
  const paths = {
    menu: <><line x1="4" x2="20" y1="6" y2="6" /><line x1="4" x2="20" y1="12" y2="12" /><line x1="4" x2="20" y1="18" y2="18" /></>,
    'panel-left': <><rect width="18" height="18" x="3" y="3" rx="2" /><line x1="9" x2="9" y1="3" y2="21" /></>,
    'panel-right': <><rect width="18" height="18" x="3" y="3" rx="2" /><line x1="15" x2="15" y1="3" y2="21" /></>,
    plus: <><line x1="12" x2="12" y1="5" y2="19" /><line x1="5" x2="19" y1="12" y2="12" /></>,
    x: <><line x1="6" x2="18" y1="6" y2="18" /><line x1="18" x2="6" y1="6" y2="18" /></>,
    send: <><path d="m4 4 16 8-16 8 3-8-3-8Z" /><path d="M7 12h13" /></>,
    globe: <><circle cx="12" cy="12" r="9" /><path d="M3 12h18M12 3c2.5 2.5 3.5 5.5 3.5 9S14.5 18.5 12 21c-2.5-2.5-3.5-5.5-3.5-9S9.5 5.5 12 3Z" /></>,
    moon: <path d="M20.5 15.2A8.5 8.5 0 0 1 8.8 3.5 8.5 8.5 0 1 0 20.5 15.2Z" />,
    sun: <><circle cx="12" cy="12" r="4" /><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" /></>,
  }
  return <svg className="icon" viewBox="0 0 24 24" aria-hidden="true">{paths[name]}</svg>
}

function readSessions(): ChatSession[] {
  try { return JSON.parse(localStorage.getItem(historyKey) || '[]') as ChatSession[] } catch { return [] }
}

export function App() {
  const [path, setPath] = useState(window.location.pathname)
  const [sessionId, setSessionId] = useState(() => new URLSearchParams(window.location.search).get('session_id') || localStorage.getItem(sessionKey) || newSessionId())
  const [sessions, setSessions] = useState<ChatSession[]>(readSessions)
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [leftOpen, setLeftOpen] = useState(false)
  const [rightOpen, setRightOpen] = useState(false)
  const [leftFolded, setLeftFolded] = useState(false)
  const [rightFolded, setRightFolded] = useState(false)
  const [darkMode, setDarkMode] = useState(() => localStorage.getItem('today-japanese-theme') === 'dark')
  const [languageOpen, setLanguageOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [daily, setDaily] = useState<DailyContent | null>(null)
  const [dailyError, setDailyError] = useState('')
  const [learningState, setLearningState] = useState<LearningState | null>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const bottomRef = useRef<HTMLDivElement>(null)

  const go = useCallback((nextPath: string, id = sessionId) => {
    const suffix = nextPath === '/' ? '?session_id=' + encodeURIComponent(id) : ''
    window.history.pushState({}, '', nextPath + suffix)
    setPath(nextPath)
    setSessionId(id)
    localStorage.setItem(sessionKey, id)
    setLeftOpen(false); setRightOpen(false)
  }, [sessionId])

  const loadHistory = useCallback(async (id: string) => {
    try {
      const response = await fetch('/api/chat/history?session_id=' + encodeURIComponent(id))
      if (!response.ok) return setMessages([])
      const history = await response.json() as { role: string; content: string }[]
      setMessages(history.filter((message): message is ChatMessage => message.role === 'user' || message.role === 'assistant'))
    } catch { setMessages([]) }
  }, [])

  useEffect(() => { localStorage.setItem(sessionKey, sessionId); if (path === '/') void loadHistory(sessionId) }, [loadHistory, path, sessionId])
  useEffect(() => {
    document.body.dataset.theme = darkMode ? 'dark' : 'light'
    localStorage.setItem('today-japanese-theme', darkMode ? 'dark' : 'light')
  }, [darkMode])
  useEffect(() => {
    if (path !== '/today') return
    fetch('/api/content/today').then((response) => response.ok ? response.json() : Promise.reject(new Error('오늘의 콘텐츠가 없습니다.'))).then(async (content: DailyContent) => {
      setDaily(content); setDailyError('')
      const stateResponse = await fetch('/api/learning-state?lesson_id=' + encodeURIComponent(content.date))
      if (stateResponse.ok) {
        const state = await stateResponse.json() as LearningState
        setLearningState(state)
        if (!state.viewed) {
          const next = { ...state, lesson_id: content.date, viewed: true, last_seen_at: new Date().toISOString() }
          setLearningState(next)
          void fetch('/api/learning-state?lesson_id=' + encodeURIComponent(content.date), { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(next) })
        }
      }
    }).catch((error: Error) => setDailyError(error.message))
  }, [path])
  useEffect(() => { bottomRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [messages, loading])

  const visibleSessions = useMemo(() => {
    const value = query.trim().toLowerCase()
    return sessions.filter((session) => !value || (session.title + ' ' + session.preview).toLowerCase().includes(value))
  }, [query, sessions])
  const persistSessions = (next: ChatSession[]) => { setSessions(next); localStorage.setItem(historyKey, JSON.stringify(next)) }
  const createSession = () => {
    const id = newSessionId()
    persistSessions([{ id, title: '새 학습 세션', preview: '대화를 시작해보세요' }, ...sessions])
    setMessages([]); go('/', id); inputRef.current?.focus()
  }
  const deleteSession = (id: string) => {
    const remaining = sessions.filter((session) => session.id !== id)
    if (id === sessionId) {
      const nextId = newSessionId()
      persistSessions([{ id: nextId, title: '새 학습 세션', preview: '대화를 시작해보세요' }, ...remaining])
      setMessages([])
      go('/', nextId)
      return
    }
    persistSessions(remaining)
  }

  const sendMessage = async (event: FormEvent) => {
    event.preventDefault()
    const text = input.trim()
    if (!text || loading) return
    setInput(''); setLoading(true)
    setMessages((current) => [...current, { role: 'user', content: text }])
    let assistant = ''
    const started = performance.now()
    try {
      const response = await fetch('/api/chat/stream', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ user_id: 'web-user', session_id: sessionId, input: text }) })
      if (!response.ok || !response.body) throw new Error('chat stream failed')
      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      while (true) {
        const result = await reader.read()
        if (result.done) break
        buffer += decoder.decode(result.value, { stream: true })
        const chunks = buffer.split('\n\n')
        buffer = chunks.pop() || ''
        for (const chunk of chunks) {
          const dataLine = chunk.split('\n').find((line) => line.startsWith('data: '))
          if (!dataLine) continue
          const payload = JSON.parse(dataLine.slice(6)) as { content?: string; elapsed_ms?: number; message?: string }
          if (chunk.startsWith('event: delta')) {
            assistant += payload.content || ''
            setMessages((current) => {
              const last = current[current.length - 1]
              return last?.role === 'assistant' ? [...current.slice(0, -1), { ...last, content: assistant }] : [...current, { role: 'assistant', content: assistant }]
            })
          }
          if (chunk.startsWith('event: done')) {
            const elapsedMs = payload.elapsed_ms || Math.round(performance.now() - started)
            setMessages((current) => current.map((message, index) => index === current.length - 1 && message.role === 'assistant' ? { ...message, elapsedMs } : message))
          }
          if (chunk.startsWith('event: error')) throw new Error(payload.message || 'chat failed')
        }
      }
      setSessions((current) => {
        const next = current.length ? current.map((session) => session.id === sessionId ? { ...session, title: session.title === '새 학습 세션' ? text.slice(0, 34) : session.title, preview: text } : session) : [{ id: sessionId, title: text.slice(0, 34), preview: text }]
        localStorage.setItem(historyKey, JSON.stringify(next)); return next
      })
    } catch { setMessages((current) => [...current, { role: 'assistant', content: '응답을 불러오지 못했어요. 잠시 후 다시 시도해 주세요.' }]) }
    finally { setLoading(false); inputRef.current?.focus() }
  }

  return <div className={'app-shell ' + (path === '/today' ? 'today-mode ' : '') + (darkMode ? 'theme-dark ' : '') + (leftFolded ? 'left-folded ' : '') + (rightFolded ? 'right-folded' : '')}>
    <header className="topbar">
      <button className="icon-button desktop-only" onClick={() => setLeftFolded((value) => !value)} aria-label="왼쪽 패널 접기"><Icon name="panel-left" /></button>
      <button className="icon-button mobile-only" onClick={() => setLeftOpen(true)} aria-label="메뉴 열기"><Icon name="menu" /></button>
      <button className="brand" onClick={() => go('/')}><span>今日の日本語 · Today Japanese</span><small>매일 조금씩, 오래 기억하는 일본어</small></button>
      <div className="header-actions">
        <div className="language-picker">
          <button className="icon-button" onClick={() => setLanguageOpen((value) => !value)} aria-label="언어 설정" title="언어 설정"><Icon name="globe" /></button>
          {languageOpen && <div className="language-menu"><button onClick={() => setLanguageOpen(false)}>한국어</button><button onClick={() => setLanguageOpen(false)}>日本語</button><button onClick={() => setLanguageOpen(false)}>English</button></div>}
        </div>
        <button className="icon-button" onClick={() => setDarkMode((value) => !value)} aria-label={darkMode ? '라이트 테마' : '다크 테마'} title={darkMode ? '라이트 테마' : '다크 테마'}><Icon name={darkMode ? 'sun' : 'moon'} /></button>
        <button className="icon-button" onClick={() => window.innerWidth <= 1200 ? setRightOpen(true) : setRightFolded((value) => !value)} aria-label="오늘의 학습 패널"><Icon name="panel-right" /></button>
      </div>
    </header>
    <div className="layout">
      <aside className={'sidebar left-sidebar ' + (leftOpen ? 'drawer-open' : '')}>
        <div className="sidebar-heading"><h2>Chats</h2><button className="small-icon-button" onClick={createSession} aria-label="새 학습 세션"><Icon name="plus" /></button></div>
        <input className="chat-search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="대화 검색" aria-label="대화 검색" />
        <div className="session-list">{visibleSessions.map((session) => <div className={'session-item ' + (session.id === sessionId ? 'active' : '')} key={session.id}><button className="session-open" onClick={() => { go('/', session.id); void loadHistory(session.id) }}><strong>{session.title}</strong><span>{session.preview}</span></button><button className="session-delete" onClick={() => deleteSession(session.id)} aria-label="대화 삭제"><Icon name="x" /></button></div>)}</div>
        <button className="drawer-close mobile-only" onClick={() => setLeftOpen(false)}>닫기</button>
      </aside>
      <main className="main-content">{path === '/today' ? <TodayPage content={daily} error={dailyError} state={learningState} setState={setLearningState} /> : <ChatPage messages={messages} loading={loading} input={input} setInput={setInput} sendMessage={sendMessage} inputRef={inputRef} bottomRef={bottomRef} />}</main>
      <aside className={'sidebar right-sidebar ' + (rightOpen ? 'drawer-open' : '')}><a className="today-link" href="/today" onClick={(event) => { event.preventDefault(); go('/today') }}>오늘의 학습</a><button className="drawer-close mobile-only" onClick={() => setRightOpen(false)}>닫기</button></aside>
    </div>
  </div>
}

function ChatPage({ messages, loading, input, setInput, sendMessage, inputRef, bottomRef }: { messages: ChatMessage[]; loading: boolean; input: string; setInput: (value: string) => void; sendMessage: (event: FormEvent) => void; inputRef: RefObject<HTMLTextAreaElement | null>; bottomRef: RefObject<HTMLDivElement | null> }) {
  return <section className="chat-page">
    {messages.length === 0 && !loading && <div className="chat-welcome"><span className="eyebrow">Japanese RAG Tutor</span><h1>무엇을 공부할까요?</h1><p>대화 기록과 오늘의 학습 콘텐츠를 바탕으로 일본어를 연습해보세요.</p></div>}
    <div className="messages">{messages.map((message, index) => <div className={'message-row ' + message.role} key={index}><div className="message-content">{message.content}</div>{message.elapsedMs && <small className="elapsed">응답 생성 {(message.elapsedMs / 1000).toFixed(1)}초</small>}</div>)}{loading && <div className="message-loading"><span>응답 생성 중</span><i /></div>}<div ref={bottomRef} /></div>
    <form className="composer" onSubmit={sendMessage}><textarea ref={inputRef} value={input} onChange={(event) => setInput(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); event.currentTarget.form?.requestSubmit() } }} placeholder="일본어 학습에 대해 물어보세요" rows={1} /><button type="submit" disabled={loading || !input.trim()} aria-label="메시지 보내기"><Icon name="send" /></button></form>
  </section>
}

function TodayPage({ content, error, state, setState }: { content: DailyContent | null; error: string; state: LearningState | null; setState: (state: LearningState) => void }) {
  if (error) return <div className="today-page"><p className="error">{error}</p></div>
  if (!content) return <div className="today-page"><p>오늘의 학습을 불러오는 중…</p></div>
  const markPracticed = () => {
    const next = { ...(state || { lesson_id: content.date, viewed: true, practiced: false, correct: 0, attempts: 0, mastered: false }), practiced: true, attempts: (state?.attempts || 0) + 1, last_seen_at: new Date().toISOString() }
    setState(next)
    void fetch('/api/learning-state?lesson_id=' + encodeURIComponent(content.date), { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(next) })
  }
  return <section className="today-page">
    <header className="today-header"><span className="eyebrow">Daily Japanese</span><h1>{content.theme}</h1><p>{content.date} · {content.level}{content.situation ? ' · ' + content.situation : ''}</p></header>
    <div className="today-overview">
      {content.visual && <div className="visual-card">{content.visual.url ? <img src={content.visual.url} alt={content.visual.alt} /> : <div><span>오늘의 장면</span><strong>{content.visual.alt || content.visual.prompt}</strong></div>}</div>}
      <div className="lesson-status"><span className="card-kicker">학습 현황</span><strong>{state?.practiced ? '오늘 학습 완료' : '학습 진행 중'}</strong><div className="status-metrics"><span><b>{content.level}</b>레벨</span><span><b>{content.words.length}</b>단어</span><span><b>{content.sentences.length}</b>문장</span><span><b>{state?.attempts || 0}</b>회 학습</span></div></div>
    </div>
    <div className="today-grid">
      <TodaySection className="today-dialogue" title="오늘의 대화">{content.dialogue?.length ? content.dialogue.map((turn, index) => <div className="dialogue-turn" key={index}><b>{turn.speaker}</b><strong>{turn.japanese}</strong><em>{turn.reading}</em><span>{turn.translation}</span></div>) : <div className="conversation">{content.moments_text}</div>}</TodaySection>
      <TodaySection title="오늘의 문장">{content.sentences.map((sentence) => <Sentence key={sentence.japanese} sentence={sentence} />)}</TodaySection>
      <TodaySection title={'오늘의 문법 · ' + content.grammar.pattern}><p>{content.grammar.meaning}</p><pre>{content.grammar.explanation}</pre>{content.grammar.examples.map((sentence) => <Sentence key={sentence.japanese} sentence={sentence} />)}</TodaySection>
      {content.culture && <TodaySection title={'문화·역사: ' + content.culture.title}><p>{content.culture.summary}</p>{content.culture.period && <small>{content.culture.period}</small>}{content.culture.source && <small>출처: {content.culture.source}</small>}</TodaySection>}
      <TodaySection className="today-words" title="오늘의 단어"><div className="word-grid">{content.words.map((word) => <div className="word-row" key={word.word}><strong>{word.word}</strong><em>{word.reading}</em><span>{word.meaning}</span><small>예문: {word.example}<br />{word.example_translation}</small></div>)}</div></TodaySection>
      {content.practice?.length ? <TodaySection title="연습 문제">{content.practice.map((item, index) => <details className="practice-item" key={index}><summary>{item.prompt}</summary><p>{item.answer}</p></details>)}</TodaySection> : null}
    </div>
    <button className="practice-complete" onClick={markPracticed}>{state?.practiced ? '학습 완료됨' : '오늘의 학습 완료'}</button>
  </section>
}

function TodaySection({ title, children, className = '' }: { title: string; children: ReactNode; className?: string }) { return <section className={'today-section ' + className}><h2>{title}</h2>{children}</section> }
function Sentence({ sentence }: { sentence: { japanese: string; reading: string; translation: string } }) { return <div className="sentence"><strong>{sentence.japanese}</strong><em>{sentence.reading}</em><span>{sentence.translation}</span></div> }
