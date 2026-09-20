const { formatHeader, computeMetrics } = require('./index.js');

if (formatHeader('vecto') !== '=== VECTO ===') {
    console.error('formatHeader failed');
    process.exit(1);
}

if (computeMetrics([1, 2, 3, 4]) !== 10) {
    console.error('computeMetrics failed');
    process.exit(1);
}

console.log('Web tests passed');
