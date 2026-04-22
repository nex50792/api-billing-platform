import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { RouterProvider, createMemoryRouter } from "react-router";
import ForgotPassword, { action } from "./forgot-password";

function renderForgotPassword() {
  const router = createMemoryRouter(
    [{ path: "/forgot-password", element: <ForgotPassword />, action }],
    { initialEntries: ["/forgot-password"] },
  );
  return render(<RouterProvider router={router} />);
}

describe("ForgotPassword", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("renders the email input field", () => {
    renderForgotPassword();
    expect(screen.getByRole("heading", { name: "パスワードをお忘れの方" })).toBeInTheDocument();
    expect(screen.getByLabelText("登録済みのメールアドレス")).toBeRequired();
  });

  it("shows success message when the backend returns ok", async () => {
    const fetchMock = vi.mocked(fetch);
    fetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ message: "送信しました" }),
    } as Response);

    renderForgotPassword();
    await userEvent.type(
      screen.getByLabelText("登録済みのメールアドレス"),
      "user@example.com",
    );
    await userEvent.click(screen.getByRole("button", { name: /送信/ }));

    await waitFor(() => {
      expect(screen.getByRole("status")).toHaveTextContent("送信しました");
    });
  });

  it("shows error message when the backend fails", async () => {
    const fetchMock = vi.mocked(fetch);
    fetchMock.mockResolvedValueOnce({ ok: false } as Response);

    renderForgotPassword();
    await userEvent.type(
      screen.getByLabelText("登録済みのメールアドレス"),
      "user@example.com",
    );
    await userEvent.click(screen.getByRole("button", { name: /送信/ }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(/リクエストに失敗/);
    });
  });
});
