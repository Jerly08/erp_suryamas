const originalEmitWarning = process.emitWarning;

process.emitWarning = function (warning, ...args) {
    if (warning.name === 'DeprecationWarning') {
        return;
    }
    return originalEmitWarning(warning, ...args);
};
