import { beforeEach, describe, expect, test, vi } from "vitest"
import {
  ApiError,
  analyzeAutomata,
  explainPolicy,
  fetchExamples,
  validatePolicy,
} from "./lib/api-client"

const request = {
  method: "GET",
  path: "/api/test",
  host: "localhost",
  scheme: "http",
  role: "admin",
  headers: {},
  query: {},
  fields: {},
  remote_addr: "127.0.0.1",
  user_agent: "test",
}

function jsonResponse(value: unknown, status = 200): Response {
  return new Response(JSON.stringify(value), {
    status,
    headers: { "Content-Type": "application/json" },
  })
}

describe("Sentinel Studio API client", () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  test("uses the API response for validation", async () => {
    const response = { valid: true, diagnostics: [] }
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(response))
    vi.stubGlobal("fetch", fetchMock)

    await expect(validatePolicy("policy Demo {}")).resolves.toEqual(response)
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/validate",
      expect.objectContaining({ method: "POST" })
    )
  })

  test("surfaces API failures instead of fabricating local results", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("offline")))

    await expect(validatePolicy("policy Demo {}")).rejects.toMatchObject({
      kind: "network",
      endpoint: "/validate",
    })
  })

  test("preserves typed HTTP errors", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockImplementation(() => jsonResponse({ error: "bad policy" }, 422))
    )

    await expect(validatePolicy("bad")).rejects.toBeInstanceOf(ApiError)
    await expect(validatePolicy("bad")).rejects.toMatchObject({
      kind: "http",
      status: 422,
      message: "bad policy",
    })
  })

  test("evaluates, loads examples, and analyzes automata through the API", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        jsonResponse({
          valid: true,
          diagnostics: [],
          decision: { allowed: true, decision: "ALLOW", default: false },
        })
      )
      .mockResolvedValueOnce(jsonResponse([]))
      .mockResolvedValueOnce(
        jsonResponse({ valid: true, pattern: "a", input: "a", accepted: true })
      )
    vi.stubGlobal("fetch", fetchMock)

    await expect(
      explainPolicy("policy Demo {}", request)
    ).resolves.toMatchObject({ valid: true })
    await expect(fetchExamples()).resolves.toEqual([])
    await expect(analyzeAutomata("a", "a")).resolves.toMatchObject({
      accepted: true,
    })
  })

  test("normalizes nullable automata collections from the API", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        jsonResponse({
          valid: true,
          pattern: "",
          accepted: true,
          diagnostics: null,
          nfa: { alphabet: null, nodes: null, edges: null },
          dfa: { alphabet: null, nodes: null, edges: null },
          minimized: { alphabet: null, nodes: null, edges: null },
          nfa_sim_steps: [{ step_index: 0, active_states: null }],
        })
      )
    )

    await expect(analyzeAutomata("", "")).resolves.toMatchObject({
      diagnostics: [],
      minimized: { alphabet: [], nodes: [], edges: [] },
      nfa_sim_steps: [{ active_states: [] }],
    })
  })
})
