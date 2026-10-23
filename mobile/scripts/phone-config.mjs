export const DEFAULT_PHONE_API_URL = 'http://172.23.10.97:5174';

export function phoneConfig(value = DEFAULT_PHONE_API_URL) {
  const url = new URL(value);
  const octets = url.hostname.split('.').map(Number);
  const privateIPv4 = octets.length === 4 && octets.every(n => Number.isInteger(n) && n >= 0 && n <= 255) &&
    (octets[0] === 10 || octets[0] === 172 && octets[1] >= 16 && octets[1] <= 31 || octets[0] === 192 && octets[1] === 168);
  if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || url.search || url.hash || url.pathname !== '/' || url.protocol === 'http:' && !privateIPv4) {
    throw new Error('Phone API must be an HTTPS origin or an explicit private IPv4 HTTP origin.');
  }
  const domain = url.protocol === 'http:' ? `\n    <domain-config cleartextTrafficPermitted="true">\n        <domain includeSubdomains="false">${url.hostname}</domain>\n    </domain-config>` : '';
  return {
    apiURL: url.origin,
    networkSecurityXML: `<?xml version="1.0" encoding="utf-8"?>\n<network-security-config>\n    <base-config cleartextTrafficPermitted="false" />${domain}\n</network-security-config>\n`,
  };
}
