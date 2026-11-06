import { copy } from "./copy";

const numberFormat = new Intl.NumberFormat();
const timeFormat = new Intl.DateTimeFormat(undefined, {
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
});
const dayTimeFormat = new Intl.DateTimeFormat(undefined, {
  month: "short",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
});

export function formatNumber(value: number): string {
  return numberFormat.format(value);
}

export function formatPercent(value: number): string {
  return `${value.toFixed(1)}%`;
}

export function formatLatency(value: number): string {
  return `${Math.round(value)} ms`;
}

export function formatClock(ms: number): string {
  return timeFormat.format(new Date(ms));
}

export function formatDayTime(ms: number): string {
  return dayTimeFormat.format(new Date(ms));
}

export function formatRangeTime(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return iso;
  }
  return dayTimeFormat.format(date);
}

export function formatUptime(seconds: number): string {
  const whole = Math.max(0, Math.floor(seconds));
  const days = Math.floor(whole / 86400);
  const hours = Math.floor((whole % 86400) / 3600);
  const minutes = Math.floor((whole % 3600) / 60);
  if (days > 0) {
    return `${days}d ${hours}h ${minutes}m`;
  }
  if (hours > 0) {
    return `${hours}h ${minutes}m`;
  }
  return `${minutes}m`;
}

export function actionLabel(action: string): string {
  switch (action) {
    case "block":
      return copy.actions.block;
    case "shadow":
      return copy.actions.shadow;
    case "ratelimited":
      return copy.actions.ratelimited;
    default:
      return copy.actions.allow;
  }
}

export function actionClass(action: string): string {
  switch (action) {
    case "block":
      return "badge badge-block";
    case "shadow":
      return "badge badge-shadow";
    case "ratelimited":
      return "badge badge-ratelimited";
    default:
      return "badge badge-allow";
  }
}

export function siteStatusLabel(status: string): string {
  if (status === "up") {
    return copy.status.healthy;
  }
  if (status === "down") {
    return copy.status.unhealthy;
  }
  return copy.status.unknown;
}
