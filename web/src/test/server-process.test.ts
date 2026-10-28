import { type ChildProcess, spawn } from "node:child_process";
import { once } from "node:events";
import { afterEach, expect, it } from "vitest";

import { stopServerProcess } from "../../e2e/server-process";

const children: ChildProcess[] = [];

afterEach(async () => {
  for (const child of children.splice(0)) await stopServerProcess(child);
});

async function running(ignoreTerm = false): Promise<ChildProcess> {
  const child = spawn(process.execPath, [
    "-e",
    `${ignoreTerm ? 'process.on("SIGTERM", () => {});' : ""} setInterval(() => {}, 1000); process.stdout.write("ready");`,
  ]);
  children.push(child);
  if (child.stdout === null) throw new Error("missing child stdout");
  await once(child.stdout, "data");
  return child;
}

it("stops a running process and removes exit/error listeners", async () => {
  const child = await running();
  await stopServerProcess(child);
  expect(child.signalCode).toBe("SIGTERM");
  expect(child.listenerCount("exit")).toBe(0);
  expect(child.listenerCount("error")).toBe(0);
});

it("does not await a second exit after an earlier signal termination", async () => {
  const child = await running();
  const exited = once(child, "exit");
  child.kill("SIGTERM");
  await exited;
  expect(child.exitCode).toBeNull();
  await stopServerProcess(child);
  expect(child.listenerCount("exit")).toBe(0);
});

it("forces a process that ignores graceful termination to exit", async () => {
  const child = await running(true);
  await stopServerProcess(child, 100);
  expect(child.signalCode).toBe("SIGKILL");
  expect(child.listenerCount("exit")).toBe(0);
  expect(child.listenerCount("error")).toBe(0);
});

it("handles incomplete setup, failed spawn and an earlier normal exit", async () => {
  await stopServerProcess(undefined);
  const failed = spawn("/missing/m2h-test-server");
  await once(failed, "error");
  await stopServerProcess(failed);
  const child = spawn(process.execPath, ["-e", "process.exit(7)"]);
  await once(child, "exit");
  expect(child.exitCode).toBe(7);
  await stopServerProcess(child);
});
