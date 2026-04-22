import { afterEach, describe, expect, it, vi } from "vitest";

describe("API_BASE_URL", () => {
  afterEach(() => {
    vi.resetModules();
    vi.unstubAllEnvs();
  });

  it("falls back to localhost when VITE_API_URL is empty", async () => {
    vi.stubEnv("VITE_API_URL", "");
    const mod = await import("./api");
    expect(mod.API_BASE_URL).toBe("http://localhost:8080");
  });

  it("uses VITE_API_URL when defined", async () => {
    vi.stubEnv("VITE_API_URL", "https://api.example.com");
    const mod = await import("./api");
    expect(mod.API_BASE_URL).toBe("https://api.example.com");
  });
});
