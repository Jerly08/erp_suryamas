const fs = require('fs');

const findKeys = (file, keyName) => {
    const content = fs.readFileSync(file, 'utf8');
    const lines = content.split('\n');
    lines.forEach((line, index) => {
        if (line.includes(`"${keyName}":`)) {
            console.log(`Line ${index + 1}: ${line.trim()}`);
        }
    });
};

console.log('--- Checking id.json ---');
findKeys('src/translations/id.json', 'settings');
console.log('\n--- Checking en.json ---');
findKeys('src/translations/en.json', 'settings');
