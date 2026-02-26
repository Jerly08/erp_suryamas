export function parseCSVRow(text: string, delimiter: string = ','): string[] {
    const result: string[] = [];
    let current = '';
    let inQuotes = false;

    for (let i = 0; i < text.length; i++) {
        const char = text[i];

        if (char === '"') {
            if (inQuotes && text[i + 1] === '"') {
                current += '"';
                i++; // skip escaped quote
            } else {
                inQuotes = !inQuotes;
            }
        } else if (char === delimiter && !inQuotes) {
            result.push(current.trim().replace(/^"|"$/g, ''));
            current = '';
        } else {
            current += char;
        }
    }

    result.push(current.trim().replace(/^"|"$/g, ''));
    return result;
}

export function detectDelimiter(text: string): string {
    const firstLine = text.split(/\r?\n/)[0] || '';
    const commaCount = (firstLine.match(/,/g) || []).length;
    const semicolonCount = (firstLine.match(/;/g) || []).length;
    return semicolonCount > commaCount ? ';' : ',';
}

export function parseCSV(text: string): string[][] {
    const delimiter = detectDelimiter(text);
    const lines = text.split(/\r?\n/);
    return lines.filter(line => line.trim() !== '').map(line => parseCSVRow(line, delimiter));
}

export function cleanCurrencyString(val: string | undefined): number {
    if (!val) return 0;
    // Handle formats like " 2,650,944 ", " Rp100,000 "
    const cleaned = val.toString().replace(/Rp|\s|,/g, '');
    const num = parseFloat(cleaned);
    return isNaN(num) ? 0 : num;
}
