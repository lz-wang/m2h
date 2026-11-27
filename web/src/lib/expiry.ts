export const expiryOptions = [
  { id: "10m", days: 10 / 1440, label: "10 分钟" },
  { id: "30m", days: 30 / 1440, label: "30 分钟" },
  { id: "1h", days: 1 / 24, label: "1 小时" },
  { id: "6h", days: 6 / 24, label: "6 小时" },
  { id: "12h", days: 12 / 24, label: "12 小时" },
  { id: "24h", days: 1, label: "24 小时" },
  { id: "3d", days: 3, label: "3 天" },
  { id: "7d", days: 7, label: "7 天" },
  { id: "30d", days: 30, label: "30 天" },
  { id: "90d", days: 90, label: "90 天" },
  { id: "permanent", days: "permanent", label: "永久" },
] as const;
