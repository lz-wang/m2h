import { Tabs } from "@base-ui/react/tabs";
import { KeyRound, Share2 } from "lucide-react";
import { ShareSettings } from "@/components/share/share-dialogs";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { TokenSettings } from "./token-settings";

const tabClassName =
  "flex min-h-10 flex-col items-center justify-center gap-1 rounded-md px-1 py-2 text-xs whitespace-nowrap text-muted-foreground outline-none hover:bg-muted/60 focus-visible:ring-2 focus-visible:ring-ring data-active:bg-muted data-active:font-medium data-active:text-foreground sm:flex-row sm:justify-start sm:gap-2 sm:px-3 sm:text-sm";
const panelClassName =
  "flex min-w-0 flex-1 flex-col gap-3 overflow-auto p-3 outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset sm:p-5 [&[hidden]]:hidden";

export function ManagementDialog({
  open,
  onOpenChange,
  tokensEnabled,
  sharesEnabled,
}: {
  open: boolean;
  onOpenChange(open: boolean): void;
  tokensEnabled: boolean;
  sharesEnabled: boolean;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        aria-describedby={undefined}
        className="top-[10dvh] h-[80dvh] max-h-[80dvh] sm:max-w-5xl"
      >
        <DialogTitle className="border-b px-4 py-4 pr-12 sm:px-5">
          管理面板
        </DialogTitle>
        <Tabs.Root
          orientation="vertical"
          defaultValue={tokensEnabled ? "tokens" : "shares"}
          className="flex min-h-0 flex-1"
        >
          <Tabs.List
            aria-label="管理类别"
            className="flex w-20 shrink-0 flex-col gap-1 border-r bg-muted/25 p-2 sm:w-40 sm:p-3"
          >
            {tokensEnabled ? (
              <Tabs.Tab value="tokens" className={tabClassName}>
                <KeyRound aria-hidden="true" className="size-4" />
                凭据管理
              </Tabs.Tab>
            ) : null}
            {sharesEnabled ? (
              <Tabs.Tab value="shares" className={tabClassName}>
                <Share2 aria-hidden="true" className="size-4" />
                分享管理
              </Tabs.Tab>
            ) : null}
          </Tabs.List>
          {tokensEnabled ? (
            <Tabs.Panel value="tokens" keepMounted className={panelClassName}>
              <TokenSettings />
            </Tabs.Panel>
          ) : null}
          {sharesEnabled ? (
            <Tabs.Panel value="shares" keepMounted className={panelClassName}>
              <ShareSettings />
            </Tabs.Panel>
          ) : null}
        </Tabs.Root>
      </DialogContent>
    </Dialog>
  );
}
