// Go's time.Time crosses the bridge as an RFC 3339 string.

const pad = (n: number) => String(n).padStart(2, '0')

// Always DD/MM/YYYY HH:MM[:SS] in local time, so every row has the same width.
export function formatDate(value: unknown, seconds = true): string {
  if (!value) return ''
  const d = new Date(value as string)
  if (Number.isNaN(d.getTime())) return ''
  const day = `${pad(d.getDate())}/${pad(d.getMonth() + 1)}/${d.getFullYear()}`
  const time = `${pad(d.getHours())}:${pad(d.getMinutes())}` + (seconds ? `:${pad(d.getSeconds())}` : '')
  return `${day} ${time}`
}

// A line that starts its own item: a list bullet, a number or a trailer like "Co-Authored-By:".
const ownLine = /^([-*•+] |\d+[.)] |[A-Za-z-]+: )/

// Joins the hard 72-column wraps of a commit body so it fills the width it is shown at.
export function reflow(body: string): string {
  return body
    .split(/\n{2,}/)
    .map((para) => {
      const lines = para.split('\n')
      // A paragraph indented throughout is code or a quote, kept as written.
      if (lines.every((l) => /^\s/.test(l))) return para
      return lines.reduce((out, line, i) => {
        if (!i) return line
        return ownLine.test(line) ? `${out}\n${line}` : `${out} ${line.trim()}`
      }, '')
    })
    .join('\n\n')
}
