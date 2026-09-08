// Syntax feedback only; the client service also validates public IPs and resolved DNS addresses.
export function validCFEndpointSyntax(raw: string): boolean {
  const value = raw.trim().replace(/\.$/, '');
  if (!value || value.length > 253) return false;
  if (value.includes(':')) {
    try { return new URL(`https://[${value}]/`).hostname.startsWith('['); } catch { return false; }
  }
  if (/^[\d.]+$/.test(value)) {
    const parts = value.split('.');
    return parts.length === 4 && parts.every((part) => /^(0|[1-9]\d{0,2})$/.test(part) && Number(part) <= 255);
  }
  return value.includes('.') && value.split('.').every((label) =>
    label.length <= 63 && /^[a-z\d](?:[a-z\d-]*[a-z\d])?$/i.test(label));
}
