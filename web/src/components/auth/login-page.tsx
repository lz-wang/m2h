import { Lock, LogIn } from "lucide-react";
import { type FormEvent, useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

export interface LoginPageProps {
  // Invoked with the entered credentials; resolves true on success (the
  // gate then mounts the reader).
  onLogin: (username: string, password: string) => Promise<boolean>;
  loginPending: boolean;
  loginError: string | null;
}

// LoginPage is the single login surface for both local and LDAP modes —
// they differ only in which identity provider verifies the pair. The page
// renders at the reader's own URL so a successful login restores the
// originally requested document and anchor without any redirect state.
export function LoginPage({
  onLogin,
  loginPending,
  loginError,
}: LoginPageProps) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [submitted, setSubmitted] = useState(false);

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (loginPending || username === "" || password === "") {
      setSubmitted(true);
      return;
    }
    void onLogin(username, password);
  };

  const usernameInvalid = submitted && username === "";
  const passwordInvalid = submitted && password === "";

  return (
    <div className="bg-background text-foreground fixed inset-0 grid place-items-center p-4">
      <div className="border-border bg-card w-full max-w-sm rounded-xl border p-8 shadow-sm">
        <div className="mb-6 flex flex-col items-center gap-3">
          <span className="bg-primary/10 text-primary grid size-12 place-items-center rounded-full">
            <Lock aria-hidden="true" className="size-5" />
          </span>
          <h1 className="text-xl font-semibold">登录 m2h</h1>
          <p className="text-muted-foreground text-center text-sm">
            此文档服务需要身份验证
          </p>
        </div>
        <form
          className="flex flex-col gap-4"
          onSubmit={handleSubmit}
          noValidate
        >
          <label className="flex flex-col gap-1.5" htmlFor="login-username">
            <span className="text-sm font-medium">用户名</span>
            <Input
              id="login-username"
              value={username}
              autoComplete="username"
              spellCheck={false}
              aria-invalid={usernameInvalid}
              onChange={(event) => setUsername(event.target.value)}
            />
          </label>
          <label className="flex flex-col gap-1.5" htmlFor="login-password">
            <span className="text-sm font-medium">密码</span>
            <Input
              id="login-password"
              type="password"
              value={password}
              autoComplete="current-password"
              aria-invalid={passwordInvalid}
              onChange={(event) => setPassword(event.target.value)}
            />
          </label>
          {loginError !== null ? (
            <p className="text-destructive text-sm" role="alert">
              {loginError}
            </p>
          ) : null}
          <Button
            type="submit"
            disabled={loginPending}
            className={cn("mt-1 h-9 w-full")}
          >
            <LogIn aria-hidden="true" data-icon="inline-start" />
            {loginPending ? "正在登录…" : "登录"}
          </Button>
        </form>
      </div>
    </div>
  );
}
