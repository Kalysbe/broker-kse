export type MobileDevice = "android" | "ios" | "phone" | "desktop";

export function detectMobileDevice(): MobileDevice {
  const ua = navigator.userAgent;
  if (/iPhone|iPod/i.test(ua)) return "ios";
  if (/Android/i.test(ua) && /Mobile/i.test(ua)) return "android";
  const mobileHint = (navigator as Navigator & { userAgentData?: { mobile: boolean } }).userAgentData?.mobile;
  if (mobileHint || window.matchMedia("(max-width: 767px) and (pointer: coarse)").matches) return "phone";
  return "desktop";
}
