export function fileMatchesAccept(file: { name: string; type: string }, accept?: string) {
  if (!accept?.trim()) return true;
  return accept.split(',').some((entry) => {
    const type = entry.trim().toLowerCase();
    if (!type) return false;
    if (type.startsWith('.')) return file.name.toLowerCase().endsWith(type);
    if (type.endsWith('/*')) return file.type.toLowerCase().startsWith(type.slice(0, -1));
    return file.type.toLowerCase() === type;
  });
}
