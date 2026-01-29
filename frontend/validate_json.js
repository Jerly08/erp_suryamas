const fs = require('fs');

const validate = (file) => {
    try {
        const content = fs.readFileSync(file, 'utf8');
        JSON.parse(content);
        console.log(`PASS: ${file} is valid JSON`);
    } catch (e) {
        console.error(`FAIL: ${file} is INVALID JSON`);
        console.error(e.message);
    }
};

validate('src/translations/id.json');
validate('src/translations/en.json');
