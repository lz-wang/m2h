import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { LoginPage } from "./login-page";

function renderLogin(overrides: Partial<Parameters<typeof LoginPage>[0]> = {}) {
  const onLogin =
    vi.fn<(username: string, password: string) => Promise<boolean>>();
  render(
    <LoginPage
      onLogin={onLogin}
      loginPending={false}
      loginError={null}
      {...overrides}
    />,
  );
  return { onLogin };
}

describe("LoginPage", () => {
  it("submits entered credentials", async () => {
    const { onLogin } = renderLogin();
    onLogin.mockResolvedValue(true);

    await userEvent.type(screen.getByLabelText("用户名"), "alice");
    await userEvent.type(screen.getByLabelText("密码"), "password-1");
    await userEvent.click(screen.getByRole("button", { name: "登录" }));

    await waitFor(() =>
      expect(onLogin).toHaveBeenCalledWith("alice", "password-1"),
    );
  });

  it("does not submit incomplete credentials", async () => {
    const { onLogin } = renderLogin();
    await userEvent.type(screen.getByLabelText("用户名"), "alice");
    await userEvent.click(screen.getByRole("button", { name: "登录" }));

    expect(onLogin).not.toHaveBeenCalled();
    expect(screen.getByLabelText("密码").getAttribute("aria-invalid")).toBe(
      "true",
    );
  });

  it("shows the login error and keeps entered input", () => {
    renderLogin({ loginError: "用户名或密码错误。" });
    expect(screen.getByRole("alert").textContent).toContain(
      "用户名或密码错误。",
    );
  });

  it("disables the submit button while a login is pending", () => {
    renderLogin({ loginPending: true });
    const button = screen.getByRole("button", {
      name: "正在登录…",
    }) as HTMLButtonElement;
    expect(button.disabled).toBe(true);
  });
});
