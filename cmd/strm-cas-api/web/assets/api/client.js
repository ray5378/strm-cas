export async function request(url, options = {}) {
  const res = await fetch(url, options)
  const contentType = res.headers.get('content-type') || ''
  const data = contentType.includes('application/json') ? await res.json() : await res.text()
  if (!res.ok) {
    const message = typeof data === 'string' ? data : (data?.error || `HTTP ${res.status}`)
    throw new Error(message)
  }
  return data
}

export function getJSON(url) {
  return request(url)
}

export function postJSON(url, payload) {
  return request(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: payload === undefined ? undefined : JSON.stringify(payload),
  })
}
