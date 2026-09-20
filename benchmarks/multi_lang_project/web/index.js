function formatHeader(title) {
    return `=== ${title.toUpperCase()} ===`;
}

function computeMetrics(items) {
    return items.reduce((acc, x) => acc + x, 0);
}

module.exports = { formatHeader, computeMetrics };
