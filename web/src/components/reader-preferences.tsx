import { Menu } from "@base-ui/react/menu";
import {
  Check,
  ChevronRight,
  Columns3,
  Maximize2,
  Moon,
  Scaling,
  Sun,
  SunMoon,
} from "lucide-react";
import type { ReactNode } from "react";

import type { DocumentWidth, Mode } from "../model";
import { Button } from "./ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "./ui/tooltip";

const modes: Array<{ value: Mode; label: string; icon: typeof Sun }> = [
  { value: "auto", label: "跟随系统", icon: SunMoon },
  { value: "light", label: "浅色", icon: Sun },
  { value: "dark", label: "深色", icon: Moon },
];

export const accountThemeOptions = modes.map((option) => ({
  ...option,
  label: option.value === "auto" ? "系统主题" : `${option.label}主题`,
}));

export const documentWidths: Array<{
  value: DocumentWidth;
  label: string;
  icon: typeof Columns3;
}> = [
  { value: "standard", label: "标准", icon: Columns3 },
  { value: "wide", label: "宽", icon: Scaling },
  { value: "full", label: "全屏", icon: Maximize2 },
];

export function PreferenceSubmenu({
  label,
  icon: Icon,
  children,
}: {
  label: string;
  icon: typeof Sun;
  children: ReactNode;
}) {
  return (
    <Menu.SubmenuRoot>
      <Menu.SubmenuTrigger className="theme-menu-item">
        <Icon aria-hidden="true" />
        <span>{label}</span>
        <ChevronRight aria-hidden="true" />
      </Menu.SubmenuTrigger>
      <Menu.Portal>
        <Menu.Positioner
          className="theme-menu-positioner"
          side="left"
          align="start"
          sideOffset={4}
        >
          <Menu.Popup
            className="theme-menu-popup settings-submenu-popup"
            aria-label={label}
          >
            {children}
          </Menu.Popup>
        </Menu.Positioner>
      </Menu.Portal>
    </Menu.SubmenuRoot>
  );
}

export function PreferenceMenuOptions<Value extends string>({
  options,
  value,
  onChange,
}: {
  options: { value: Value; label: string; icon: typeof Sun }[];
  value: Value;
  onChange(value: Value): void;
}) {
  return (
    <Menu.RadioGroup
      value={value}
      onValueChange={(nextValue) => {
        const option = options.find((item) => item.value === nextValue);
        if (option) onChange(option.value);
      }}
    >
      {options.map(({ value, label, icon: Icon }) => (
        <Menu.RadioItem
          key={value}
          value={value}
          className="theme-menu-item"
          closeOnClick
        >
          <Icon aria-hidden="true" />
          <span>{label}</span>
          <Menu.RadioItemIndicator className="theme-menu-indicator">
            <Check aria-hidden="true" />
          </Menu.RadioItemIndicator>
        </Menu.RadioItem>
      ))}
    </Menu.RadioGroup>
  );
}

export function DocumentWidthMenu({
  width,
  onChange,
}: {
  width: DocumentWidth;
  onChange(width: DocumentWidth): void;
}) {
  const current =
    documentWidths.find((item) => item.value === width) ?? documentWidths[0];
  const CurrentIcon = current.icon;
  return (
    // Non-modal: toolbar dropdowns must not trap focus or lock document scroll.
    <Menu.Root modal={false}>
      <Tooltip>
        <Menu.Trigger
          render={
            <TooltipTrigger
              render={
                <Button
                  variant="ghost"
                  size="icon-sm"
                  className="document-width-control"
                  aria-label={`文档宽度：${current.label}`}
                />
              }
            />
          }
        >
          <CurrentIcon aria-hidden="true" />
        </Menu.Trigger>
        <TooltipContent side="bottom">调整文档宽度</TooltipContent>
      </Tooltip>
      <Menu.Portal>
        <Menu.Positioner
          className="theme-menu-positioner"
          align="end"
          sideOffset={6}
        >
          <Menu.Popup className="theme-menu-popup">
            <Menu.Group>
              <Menu.GroupLabel className="theme-menu-label">
                文档宽度
              </Menu.GroupLabel>
              <PreferenceMenuOptions
                options={documentWidths}
                value={width}
                onChange={onChange}
              />
            </Menu.Group>
          </Menu.Popup>
        </Menu.Positioner>
      </Menu.Portal>
    </Menu.Root>
  );
}

export function ThemeMenu({
  mode,
  onChange,
}: {
  mode: Mode;
  onChange(mode: Mode): void;
}) {
  const currentLabel = modes.find((item) => item.value === mode)?.label ?? mode;
  const CurrentIcon =
    modes.find((item) => item.value === mode)?.icon ?? SunMoon;

  return (
    // Non-modal: toolbar dropdowns must not trap focus or lock document scroll.
    <Menu.Root modal={false}>
      <Tooltip>
        <Menu.Trigger
          render={
            <TooltipTrigger
              render={
                <Button
                  variant="ghost"
                  size="icon-sm"
                  aria-label={`显示主题：${currentLabel}`}
                />
              }
            />
          }
        >
          <CurrentIcon aria-hidden="true" />
        </Menu.Trigger>
        <TooltipContent side="bottom">切换显示主题</TooltipContent>
      </Tooltip>
      <Menu.Portal>
        <Menu.Positioner
          className="theme-menu-positioner"
          align="end"
          sideOffset={6}
        >
          <Menu.Popup className="theme-menu-popup">
            <Menu.Group>
              <Menu.GroupLabel className="theme-menu-label">
                显示主题
              </Menu.GroupLabel>
              <PreferenceMenuOptions
                options={modes}
                value={mode}
                onChange={onChange}
              />
            </Menu.Group>
          </Menu.Popup>
        </Menu.Positioner>
      </Menu.Portal>
    </Menu.Root>
  );
}
