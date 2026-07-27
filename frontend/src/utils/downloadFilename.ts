const normalizeFilename = (value: string, fallbackName: string): string => {
  const filename = value
    .trim()
    .replace(/[\\/\u0000-\u001F]/g, "_");

  return filename || fallbackName;
};

const getDispositionValue = (
  disposition: string,
  parameterName: "filename" | "filename*",
): string | null => {
  const expression = new RegExp(
    `(?:^|;)\\s*${parameterName.replace("*", "\\*")}\\s*=\\s*(?:"([^"]*)"|([^;]*))`,
    "i",
  );
  const match = disposition.match(expression);

  return match ? (match[1] ?? match[2]).trim() : null;
};

/**
 * Reads a download filename without decoding ordinary filename= values.
 * Only RFC 5987 filename*= values are percent-decoded.
 */
export const getDownloadFilename = (
  disposition: string | null | undefined,
  fallbackName: string,
): string => {
  const fallback = normalizeFilename(fallbackName, "download");
  if (!disposition) return fallback;

  const extendedFilename = getDispositionValue(disposition, "filename*");
  if (extendedFilename) {
    const encodedFilename = extendedFilename.replace(/^[^']*'[^']*'/, "");
    try {
      return normalizeFilename(decodeURIComponent(encodedFilename), fallback);
    } catch {
      return normalizeFilename(encodedFilename, fallback);
    }
  }

  const filename = getDispositionValue(disposition, "filename");
  return filename ? normalizeFilename(filename, fallback) : fallback;
};
