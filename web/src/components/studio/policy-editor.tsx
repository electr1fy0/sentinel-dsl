import * as React from "react"

const KEYWORDS = new Set([
  "policy",
  "rule",
  "when",
  "and",
  "or",
  "then",
  "allow",
  "deny",
  "default",
  "matches",
  "in",
])
const FIELDS = new Set([
  "method",
  "path",
  "host",
  "scheme",
  "role",
  "remote_addr",
  "user_agent",
  "headers",
  "query",
  "fields",
])
const BRACKETS: Record<string, string> = { "(": ")", "[": "]", "{": "}" }
const CLOSE_TO_OPEN: Record<string, string> = { ")": "(", "]": "[", "}": "{" }
const TOKEN_PATTERN =
  /("(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|\b\d+(?:\.\d+)?\b|\b[A-Za-z_][\w.]*\b|==|!=|<=|>=|[{}()[\];,]|\S)/g

export interface PolicyEditorProps {
  value: string
  onChange: (value: string) => void
  className?: string
  placeholder?: string
}

function tokenClass(token: string): string {
  if (token.startsWith('"') || token.startsWith("'")) return "text-emerald-500"
  if (/^\d/.test(token)) return "text-amber-500"
  if (KEYWORDS.has(token)) return "font-semibold text-sky-500"
  if (FIELDS.has(token.split(".")[0] ?? "")) return "text-violet-500"
  if (/^(==|!=|<=|>=|[{}()[\];,])$/.test(token)) return "text-muted-foreground"
  return "text-foreground"
}

function highlightSource(
  source: string,
  matching: Set<number>
): React.ReactNode[] {
  const nodes: React.ReactNode[] = []
  TOKEN_PATTERN.lastIndex = 0
  let cursor = 0
  let match: RegExpExecArray | null
  let key = 0
  while ((match = TOKEN_PATTERN.exec(source)) !== null) {
    const token = match[0]
    const start = match.index
    if (start > cursor)
      nodes.push(
        <React.Fragment key={`raw-${key++}`}>
          {source.slice(cursor, start)}
        </React.Fragment>
      )
    if (matching.has(start)) {
      nodes.push(
        <mark
          key={`match-${key++}`}
          className="rounded bg-primary/20 text-foreground"
        >
          {token}
        </mark>
      )
    } else {
      nodes.push(
        <span key={`token-${key++}`} className={tokenClass(token)}>
          {token}
        </span>
      )
    }
    cursor = start + token.length
  }
  if (cursor < source.length)
    nodes.push(
      <React.Fragment key={`tail-${key++}`}>
        {source.slice(cursor)}
      </React.Fragment>
    )
  return nodes
}

function matchingBrackets(source: string, caret: number): Set<number> {
  const positions = new Set<number>()
  const candidate = [caret - 1, caret].find(
    (index) =>
      BRACKETS[source[index] ?? ""] || CLOSE_TO_OPEN[source[index] ?? ""]
  )
  if (candidate === undefined) return positions
  const char = source[candidate] ?? ""
  const direction = BRACKETS[char] ? 1 : -1
  const target = BRACKETS[char] || CLOSE_TO_OPEN[char]
  let depth = 0
  for (
    let index = candidate;
    direction > 0 ? index < source.length : index >= 0;
    index += direction
  ) {
    const current = source[index]
    if (current === char) depth += 1
    if (current === target) {
      depth -= 1
      if (depth === 0) {
        positions.add(candidate)
        positions.add(index)
        break
      }
    }
  }
  return positions
}

export function formatPolicy(source: string): string {
  let indent = 0
  const lines = source.replaceAll("\r\n", "\n").split("\n")
  return lines
    .map((line) => {
      const trimmed = line.trim()
      if (!trimmed) return ""
      if (trimmed.startsWith("}")) indent = Math.max(0, indent - 1)
      const formatted = `${"  ".repeat(indent)}${trimmed}`
      if (trimmed.endsWith("{")) indent += 1
      return formatted
    })
    .join("\n")
    .replace(/\n{3,}/g, "\n\n")
}

const SNIPPETS = [
  {
    label: "Policy skeleton",
    value: `policy NewPolicy {\n  rule AllowGet when\n    method == "GET"\n  then allow;\n\n  default deny;\n}`,
  },
  {
    label: "Rule skeleton",
    value: `rule NewRule when\n  method == "GET"\nthen allow;`,
  },
  { label: "Allow action", value: "then allow;" },
  { label: "Deny action", value: 'then deny 403 "Forbidden";' },
]

export const PolicyEditor = React.forwardRef<
  HTMLTextAreaElement,
  PolicyEditorProps
>(function PolicyEditor({ value, onChange, className, placeholder }, ref) {
  const [scroll, setScroll] = React.useState({ left: 0, top: 0 })
  const [matching, setMatching] = React.useState<Set<number>>(new Set())
  const [searchOpen, setSearchOpen] = React.useState(false)
  const [completionsOpen, setCompletionsOpen] = React.useState(false)
  const [query, setQuery] = React.useState("")
  const [replacement, setReplacement] = React.useState("")
  const textareaRef = React.useRef<HTMLTextAreaElement | null>(null)
  const setRefs = (node: HTMLTextAreaElement | null) => {
    textareaRef.current = node
    if (typeof ref === "function") ref(node)
    else if (ref) ref.current = node
  }

  const updateCaret = () => {
    const textarea = textareaRef.current
    if (!textarea) return
    setMatching(matchingBrackets(value, textarea.selectionStart))
  }

  const replaceSelection = (insert: string, selectionStart = insert.length) => {
    const textarea = textareaRef.current
    if (!textarea) return
    const start = textarea.selectionStart
    const end = textarea.selectionEnd
    const next = `${value.slice(0, start)}${insert}${value.slice(end)}`
    onChange(next)
    requestAnimationFrame(() => {
      textarea.focus()
      textarea.setSelectionRange(start + selectionStart, start + selectionStart)
      updateCaret()
    })
  }

  const handleKeyDown = (event: React.KeyboardEvent<HTMLTextAreaElement>) => {
    const modifier = event.metaKey || event.ctrlKey
    if (modifier && event.key.toLowerCase() === "f") {
      event.preventDefault()
      setSearchOpen(true)
      return
    }
    if (modifier && event.key === " ") {
      event.preventDefault()
      setCompletionsOpen((open) => !open)
      return
    }
    if (event.key === "Escape") {
      setSearchOpen(false)
      setCompletionsOpen(false)
      return
    }
    if (event.key === "Tab") {
      event.preventDefault()
      replaceSelection("  ")
      return
    }
    if (event.key === "Enter") {
      event.preventDefault()
      const textarea = textareaRef.current
      if (!textarea) return
      const lineStart = value.lastIndexOf("\n", textarea.selectionStart - 1) + 1
      const before = value.slice(lineStart, textarea.selectionStart)
      const currentIndent = before.match(/^\s*/)?.[0] ?? ""
      const extraIndent = before.trimEnd().endsWith("{") ? "  " : ""
      const nextIndent = before.trimStart().startsWith("}")
        ? currentIndent.slice(2)
        : currentIndent
      replaceSelection(`\n${nextIndent}${extraIndent}`)
    }
  }

  const matches = query ? value.split(query).length - 1 : 0
  const replaceAll = () => {
    if (!query) return
    onChange(value.split(query).join(replacement))
  }

  return (
    <div className={`relative flex min-h-0 flex-1 flex-col ${className ?? ""}`}>
      {(searchOpen || completionsOpen) && (
        <div className="absolute inset-x-2 top-2 z-30 flex flex-wrap items-center gap-1 rounded-xl bg-popover p-1.5">
          {searchOpen && (
            <>
              <InputLike
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="Find"
                autoFocus
                aria-label="Find text"
              />
              <InputLike
                value={replacement}
                onChange={(event) => setReplacement(event.target.value)}
                placeholder="Replace"
                aria-label="Replace text"
              />
              <span className="px-1 text-[10px] text-muted-foreground">
                {matches} match{matches === 1 ? "" : "es"}
              </span>
              <button
                type="button"
                onClick={replaceAll}
                className="rounded px-1.5 py-1 text-[10px] text-muted-foreground hover:bg-muted hover:text-foreground"
              >
                Replace all
              </button>
            </>
          )}
          {completionsOpen && (
            <div className="flex flex-wrap items-center gap-1">
              <span className="px-1 text-[10px] text-muted-foreground">
                Insert snippet:
              </span>
              {SNIPPETS.map((snippet) => (
                <button
                  key={snippet.label}
                  type="button"
                  onClick={() => {
                    replaceSelection(snippet.value)
                    setCompletionsOpen(false)
                  }}
                  className="rounded bg-muted/70 px-1.5 py-1 text-[10px] text-muted-foreground hover:bg-accent hover:text-foreground"
                >
                  {snippet.label}
                </button>
              ))}
            </div>
          )}
          <button
            type="button"
            onClick={() => {
              setSearchOpen(false)
              setCompletionsOpen(false)
            }}
            className="ml-auto px-1 text-xs text-muted-foreground hover:text-foreground"
            aria-label="Close editor tools"
          >
            ×
          </button>
        </div>
      )}
      <div className="relative min-h-0 flex-1 overflow-hidden">
        <pre
          aria-hidden="true"
          className="pointer-events-none absolute inset-0 m-0 overflow-hidden p-4 font-mono text-[13px] leading-[22px] break-words whitespace-pre-wrap"
          style={{
            transform: `translate(${-scroll.left}px, ${-scroll.top}px)`,
          }}
        >
          {highlightSource(value, matching)}
        </pre>
        <textarea
          ref={setRefs}
          value={value}
          onChange={(event) => onChange(event.target.value)}
          onScroll={(event) =>
            setScroll({
              left: event.currentTarget.scrollLeft,
              top: event.currentTarget.scrollTop,
            })
          }
          onClick={updateCaret}
          onKeyUp={updateCaret}
          onSelect={updateCaret}
          onKeyDown={handleKeyDown}
          className="absolute inset-0 z-10 h-full w-full resize-none overflow-auto rounded-none bg-transparent p-4 font-mono text-[13px] leading-[22px] text-transparent caret-foreground outline-none selection:bg-primary/20 placeholder:text-muted-foreground/50"
          placeholder={placeholder}
          style={{ color: "transparent" }}
          spellCheck={false}
          aria-label="Sentinel policy source"
        />
      </div>
      <div className="flex h-5 shrink-0 items-center justify-between bg-muted/40 px-2 text-[10px] text-muted-foreground">
        <span>
          {matching.size === 2
            ? "Matching bracket highlighted"
            : "Sentinel DSL"}
        </span>
        <span className="hidden sm:inline">
          ⌘/Ctrl+F to search · Tab to indent
        </span>
      </div>
    </div>
  )
})

function InputLike(props: React.InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      {...props}
      className="h-6 w-28 rounded bg-muted/70 px-1.5 font-mono text-[10px] outline-none focus:bg-accent focus:ring-2 focus:ring-ring/30"
    />
  )
}
