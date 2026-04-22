import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import AdminLogin from "./login";

describe("AdminLogin", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("renders email/password inputs with required attribute", () => {
    render(<AdminLogin />);
    const email = screen.getByLabelText("メールアドレス") as HTMLInputElement;
    const password = screen.getByLabelText("パスワード") as HTMLInputElement;
    expect(email).toBeRequired();
    expect(password).toBeRequired();
    expect(email.type).toBe("email");
    expect(password.type).toBe("password");
  });

  it("shows error message when login fails", async () => {
    const fetchMock = vi.mocked(fetch);
    fetchMock.mockResolvedValueOnce({ ok: false } as Response);

    render(<AdminLogin />);
    await userEvent.type(screen.getByLabelText("メールアドレス"), "admin@example.com");
    await userEvent.type(screen.getByLabelText("パスワード"), "wrong");
    await userEvent.click(screen.getByRole("button", { name: /ログイン/ }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("ログインに失敗しました");
    });
  });
});
