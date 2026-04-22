import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { RouterProvider, createMemoryRouter } from "react-router";
import ProviderLogin from "./login";

function renderProviderLogin() {
  const router = createMemoryRouter(
    [
      { path: "/provider/login", element: <ProviderLogin /> },
      { path: "/provider", element: <div>provider-home</div> },
    ],
    { initialEntries: ["/provider/login"] },
  );
  return render(<RouterProvider router={router} />);
}

describe("ProviderLogin", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("renders the login form with required fields", () => {
    renderProviderLogin();
    expect(screen.getByRole("heading", { name: "プロバイダーログイン" })).toBeInTheDocument();
    expect(screen.getByLabelText("メールアドレス")).toBeRequired();
    expect(screen.getByLabelText("パスワード")).toBeRequired();
  });

  it("shows error when backend rejects credentials", async () => {
    const fetchMock = vi.mocked(fetch);
    fetchMock.mockResolvedValueOnce({ ok: false } as Response);

    renderProviderLogin();
    await userEvent.type(screen.getByLabelText("メールアドレス"), "p@example.com");
    await userEvent.type(screen.getByLabelText("パスワード"), "bad");
    await userEvent.click(screen.getByRole("button", { name: /ログイン/ }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("ログインに失敗しました");
    });
  });
});
