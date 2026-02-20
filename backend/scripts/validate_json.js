const fs = require('fs');

const filename = process.argv[2];
console.log(`Validating ${filename}...`);

try {
    const content = fs.readFileSync(filename, 'utf8');

    // Check for duplicates
    // We can use JSON.parse with a reviver to check duplicates, but that's tricky.
    // Instead, let's parse linearly and watch for keys at the same level.
    // Actually, just find the specific keys I know I duplicated: selectCashGLAccount

    let lines = content.split('\n');
    let keys = ['selectCashGLAccount', 'selectCashGLPlaceholder'];

    keys.forEach(key => {
        let count = 0;
        lines.forEach((line, index) => {
            if (line.includes(`"${key}"`)) {
                count++;
                console.log(`Found "${key}" at line ${index + 1}: ${line.trim()}`);
            }
        });
        if (count > 1) {
            console.error(`ERROR: Duplicate key "${key}" found! Count: ${count}`);
            process.exit(1);
        }
    });

    JSON.parse(content);
    console.log("JSON is valid structure.");
} catch (e) {
    console.error("JSON Error:", e.message);
    process.exit(1);
}
