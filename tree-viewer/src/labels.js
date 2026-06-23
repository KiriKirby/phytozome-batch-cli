export function displayLabelMap(metadata, options = {}) {
  const showPhgoCoords = Boolean(options.showPhgoCoords);
  const map = new Map();
  for (const record of metadata?.records || []) {
    const taxon = String(record.taxon_id || '').trim();
    if (!taxon) continue;
    const name = String(record.display_name || taxon).trim() || taxon;
    const prefix = showPhgoCoords ? String(record.display_prefix || '').trim() : '';
    map.set(taxon, prefix ? `${prefix} ${name}` : name);
  }
  return map;
}

export function reactreeLabelMap(metadata, options = {}) {
  return reactreeLabelMaps(metadata, options).forward;
}

export function reactreeLabelMaps(metadata, options = {}) {
  const displayNames = displayLabelMap(metadata, options);
  const forward = new Map();
  const reverse = new Map();
  for (const [taxon, label] of displayNames) {
    const taxonLabel = sanitizeReactreeLabel(taxon) || taxon;
    const safe = sanitizeReactreeLabel(label) || taxonLabel;
    forward.set(taxon, safe);
    if (!reverse.has(safe)) reverse.set(safe, taxon);
  }
  return { forward, reverse };
}

export function sanitizeReactreeLabel(label) {
  return String(label || '').trim();
}

export function relabelNewick(newick, metadata, options = {}) {
  let out = String(newick || '');
  const entries = [...reactreeLabelMap(metadata, options).entries()].sort((a, b) => b[0].length - a[0].length);
  for (const [taxon, label] of entries) {
    out = out.replace(new RegExp(escapeRegExp(taxon) + '(?=[:),;])', 'g'), quoteNewickLabel(label));
  }
  return out;
}

export function relabelFasta(fasta, metadata, options = {}) {
  const names = reactreeLabelMap(metadata, options);
  return String(fasta || '').split(/\r?\n/).map((line) => {
    if (!line.startsWith('>')) return line;
    const id = line.slice(1).trim().split(/\s+/)[0];
    const label = names.get(id);
    return label ? `>${label}` : line;
  }).join('\n');
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function quoteNewickLabel(value) {
  return `'${String(value || '').replace(/'/g, "''")}'`;
}
