import type { ChildProcess } from "node:child_process";

function hasExited(child: ChildProcess): boolean {
  return child.exitCode !== null || child.signalCode !== null;
}

// Register before signalling so even an immediate exit cannot be missed. Each
// wait is bounded and removes its listeners on success, failure or timeout.
function signalAndWait(
  child: ChildProcess,
  signal: NodeJS.Signals,
  timeout: number,
): Promise<boolean> {
  return new Promise((resolve, reject) => {
    const finish = (exited: boolean, error?: Error) => {
      clearTimeout(timer);
      child.removeListener("exit", onExit);
      child.removeListener("error", onError);
      if (error) reject(error);
      else resolve(exited);
    };
    const onExit = () => finish(true);
    const onError = (error: NodeJS.ErrnoException) =>
      finish(
        hasExited(child) || error.code === "ESRCH",
        error.code === "ESRCH" ? undefined : error,
      );
    const timer = setTimeout(() => finish(hasExited(child)), timeout);
    child.once("exit", onExit);
    child.once("error", onError);
    try {
      if (hasExited(child)) finish(true);
      else child.kill(signal);
    } catch (error) {
      finish(false, error instanceof Error ? error : new Error(String(error)));
    }
  });
}

// Covers partial setup, failed spawn and earlier signal exits. A server that
// ignores graceful termination gets one bounded SIGKILL attempt before failing.
export async function stopServerProcess(
  child: ChildProcess | undefined,
  timeout = 1_000,
): Promise<void> {
  if (child === undefined || child.pid === undefined || hasExited(child))
    return;
  if (await signalAndWait(child, "SIGTERM", timeout)) return;
  if (await signalAndWait(child, "SIGKILL", timeout)) return;
  throw new Error(`server process ${child.pid} did not exit after SIGKILL`);
}
