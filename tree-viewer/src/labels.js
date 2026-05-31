export function displayLabelMap(metadata) {
  const map = new Map();
  for (const record of metadata?.records || []) {
    const taxon = String(record.taxon_id || '').trim();
    if (!taxon) continue;
    map.set(taxon, String(record.display_name || taxon).trim() || taxon);
  }
  return map;
}

export function reactreeLabelMap(metadata) {
  return reactreeLabelMaps(metadata).forward;
}

export function reactreeLabelMaps(metadata) {
  const displayNames = displayLabelMap(metadata);
  const used = new Set();
  const forward = new Map();
  const reverse = new Map();
  for (const [taxon, label] of displayNames) {
    const taxonLabel = sanitizeReactreeLabel(taxon) || taxon;
    const safeBase = sanitizeReactreeLabel(label) || taxonLabel;
    let safe = safeBase;
    if (used.has(safe.toLowerCase())) {
      safe = `${safeBase} [${taxonLabel}]`;
      let counter = 2;
      while (used.has(safe.toLowerCase())) {
        safe = `${safeBase} [${taxonLabel} ${counter}]`;
        counter += 1;
      }
    }
    used.add(safe.toLowerCase());
    forward.set(taxon, safe);
    reverse.set(safe, taxon);
  }
  return { forward, reverse };
}

export function sanitizeReactreeLabel(label) {
  return String(label || '').trim();
}

export function relabelNewick(newick, metadata) {
  let out = String(newick || '');
  const entries = [...reactreeLabelMap(metadata).entries()].sort((a, b) => b[0].length - a[0].length);
  for (const [taxon, label] of entries) {
    out = out.replace(new RegExp(escapeRegExp(taxon) + '(?=[:),;])', 'g'), quoteNewickLabel(label));
  }
  return out;
}

export function relabelFasta(fasta, metadata) {
  const names = reactreeLabelMap(metadata);
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
