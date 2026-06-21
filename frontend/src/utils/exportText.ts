const WINDOWS_1252_EXTRA: Record<number, number> = {
  0x20ac: 0x80,
  0x201a: 0x82,
  0x0192: 0x83,
  0x201e: 0x84,
  0x2026: 0x85,
  0x2020: 0x86,
  0x2021: 0x87,
  0x02c6: 0x88,
  0x2030: 0x89,
  0x0160: 0x8a,
  0x2039: 0x8b,
  0x0152: 0x8c,
  0x017d: 0x8e,
  0x2018: 0x91,
  0x2019: 0x92,
  0x201c: 0x93,
  0x201d: 0x94,
  0x2022: 0x95,
  0x2013: 0x96,
  0x2014: 0x97,
  0x02dc: 0x98,
  0x2122: 0x99,
  0x0161: 0x9a,
  0x203a: 0x9b,
  0x0153: 0x9c,
  0x017e: 0x9e,
  0x0178: 0x9f,
};

const mojibakeMarkerPattern = /[\u00c3\u00c2\u00e2\u00f0\u00ef]/;

function decodeWindows1252AsUtf8(value: string): string | null {
  const bytes: number[] = [];

  for (const char of value) {
    const code = char.codePointAt(0);
    if (code === undefined) continue;

    if (code <= 0xff) {
      bytes.push(code);
      continue;
    }

    const mapped = WINDOWS_1252_EXTRA[code];
    if (mapped === undefined) {
      return null;
    }
    bytes.push(mapped);
  }

  try {
    return new TextDecoder('utf-8', { fatal: true }).decode(new Uint8Array(bytes));
  } catch {
    return null;
  }
}

export function cleanExportText(value: unknown): string {
  let cleaned = value === null || value === undefined ? '' : String(value);

  for (let i = 0; i < 4 && mojibakeMarkerPattern.test(cleaned); i += 1) {
    const decoded = decodeWindows1252AsUtf8(cleaned);
    if (!decoded || decoded === cleaned) break;
    cleaned = decoded;
  }

  return cleaned
    .replace(/[\u2010-\u2014\u2212]/g, '-')
    .replace(/[\u2018\u2019]/g, "'")
    .replace(/[\u201c\u201d]/g, '"')
    .replace(/\u2026/g, '...')
    .replace(/\u00a0/g, ' ')
    .replace(/[\u{1f000}-\u{1faff}\u2600-\u27bf\ufe0e\ufe0f\u200d\u20e3]/gu, '')
    .replace(/[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f-\u009f]/g, '');
}

export function cleanCsvCell(value: unknown): string {
  return cleanExportText(value).trim();
}

export function cleanCsvContent(content: string): string {
  return cleanExportText(content);
}
