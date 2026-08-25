import { afterEach, describe, expect, it, vi } from "vitest";
import { api, ApiError } from "../api";

/** Minimal fetch Response double: http() only reads ok/status/json(). */
function jsonResponse(status: number, body: unknown): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as Response;
}

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});

describe("ApiError", () => {
  it("is a real Error subclass carrying code + message", () => {
    const err = new ApiError("INSUFFICIENT_STOCK", "insufficient stock on hand");
    expect(err).toBeInstanceOf(Error);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.code).toBe("INSUFFICIENT_STOCK");
    expect(err.message).toBe("insufficient stock on hand");
    expect(String(err)).toContain("insufficient stock on hand");
  });
});

describe("http error handling", () => {
  it("throws ApiError with backend code+message on non-2xx responses", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => jsonResponse(409, { code: "EMAIL_EXISTS", message: "email already registered" })),
    );

    let caught: unknown;
    try {
      await api.getFixedAsset(1);
    } catch (err) {
      caught = err;
    }

    expect(caught).toBeInstanceOf(ApiError);
    expect(caught).toBeInstanceOf(Error);
    const apiError = caught as ApiError;
    expect(apiError.code).toBe("EMAIL_EXISTS");
    expect(apiError.message).toBe("email already registered");
  });

  it("falls back to REQUEST_FAILED when the error body is not JSON-shaped", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => jsonResponse(500, {})),
    );

    await expect(api.getFixedAsset(1)).rejects.toMatchObject({
      code: "REQUEST_FAILED",
      message: "Something went wrong. Please try again.",
    });
  });

  it("throws ApiError for client-side validation before any request", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    await expect(api.login({ email: "", password: "" })).rejects.toBeInstanceOf(ApiError);
    await expect(api.login({ email: "", password: "" })).rejects.toMatchObject({
      code: "VALIDATION_ERROR",
    });
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

describe("api.login session shape", () => {
  it("exposes the tenant business so repeat logins skip onboarding", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
        if (init?.method === "POST") {
          return jsonResponse(200, {
            access_token: "access-1",
            refresh_token: "refresh-1",
            family_id: "fam-1",
          });
        }
        return jsonResponse(200, { id: "7", name: "PT Maju Bersama", role: "owner" });
      }),
    );

    const result = await api.login({ email: "owner@example.com", password: "Password!23" });

    expect(result.hasTenant).toBe(true);
    expect(result.business).not.toBeNull();
    expect(result.business?.id).toBe("7");
    expect(result.business?.name).toBe("PT Maju Bersama");
  });

  it("returns business null when the user has no tenant yet", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
        if (init?.method === "POST") {
          return jsonResponse(200, { access_token: "access-1", family_id: "fam-1" });
        }
        return jsonResponse(404, { code: "NO_TENANT", message: "no tenant" });
      }),
    );

    const result = await api.login({ email: "fresh@example.com", password: "Password!23" });

    expect(result.hasTenant).toBe(false);
    expect(result.business).toBeNull();
  });
});
