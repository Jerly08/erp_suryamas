const fs = require('fs');

const filename = process.argv[2];
console.log(`Analyzing duplicates in ${filename}...`);

try {
    const content = fs.readFileSync(filename, 'utf8');
    const lines = content.split('\n');
    let scopeStack = [new Set()]; // Stack of sets of seen keys at each level
    let indentStack = [0]; // Stack of indent levels

    lines.forEach((line, index) => {
        const lineNum = index + 1;
        // Skip empty lines or lines with only braces/brackets/commas
        if (!line.trim() || line.trim().match(/^[\}\],]+$/)) {
            // If line is just closing brace, pop scope?
            // Indentation-based logic is safer for malformed JSON?
            // Let's use strict indentation
            const currentIndent = line.search(/\S|$/);
            while (indentStack.length > 0 && currentIndent < indentStack[indentStack.length - 1]) {
                indentStack.pop();
                scopeStack.pop();
            }
            return;
        }

        const match = line.match(/^(\s*)"([^"]+)":/);
        if (match) {
            const indent = match[1].length;
            const key = match[2];

            // Adjust scope based on indent
            if (indent > indentStack[indentStack.length - 1]) {
                // New deeper scope
                indentStack.push(indent);
                scopeStack.push(new Set());
            } else {
                while (indentStack.length > 0 && indent < indentStack[indentStack.length - 1]) {
                    indentStack.pop();
                    scopeStack.pop();
                }
                if (indentStack.length === 0) { // Should not happen if well-formed or 0 indent
                    indentStack.push(indent);
                    scopeStack.push(new Set());
                } else if (indent > indentStack[indentStack.length - 1]) {
                    // This is slightly weird (indent mismatch), treat as new scope?
                    indentStack.push(indent);
                    scopeStack.push(new Set());
                }
            }

            // Check duplicate in CURRENT scope (top of stack)
            const currentScope = scopeStack[scopeStack.length - 1];
            if (currentScope.has(key)) {
                console.log(`Duplicate key found at line ${lineNum}: ${key}`);
            } else {
                currentScope.add(key);
            }
        }
    });

} catch (e) {
    console.error("Error:", e.message);
}
