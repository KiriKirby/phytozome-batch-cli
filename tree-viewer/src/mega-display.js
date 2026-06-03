const EPSILON = 1e-10;

export function megaDefaultDisplayNewick(newick, metadata) {
  const text = String(newick || '').trim();
  if (!text) return text;
  const policy = megaDefaultDisplayPolicy(metadata);
  if (!policy.applyAdapter) return text;
  try {
    const parsed = parseNewick(text);
    const order = taxonOrder(metadata, parsed);
    const imported = importAsMegaTreeListTree(parsed, order);
    let displayTree = imported.root;
    annotateDisplayTree(displayTree, order);
    if (shouldApplyMidpointRoot(policy, imported, metadata)) {
      displayTree = makeRootOnMidPoint(displayTree, imported.nodes);
      annotateDisplayTree(displayTree, order);
    }
    if (policy.arrangeTaxa === 'balanced_shape') {
      sortClusterForMegaShape(displayTree, null, 'root');
    } else if (policy.arrangeTaxa === 'input_order') {
      sortClusterInInputOrder(displayTree);
    }
    annotateDisplayTree(displayTree, order);
    return `${writeNewick(displayTree)};`;
  } catch {
    return text;
  }
}

export function megaDefaultDisplayPolicy(metadata) {
  const source = String(metadata?.tree_computation_source || '').trim().toLowerCase();
  const method = normalizeToken(metadata?.tree_method);
  const treeCount = Number(metadata?.tree_count || 1);
  const isRuntimeTree = source === 'mega-phgo-runtime' || source.startsWith('mega-phgo-runtime/');
  if (!isRuntimeTree || !method) {
    return {
      applyAdapter: false,
      rootOnMidpoint: false,
      arrangeTaxa: 'none',
      reason: 'not a MEGA PHgo runtime inferred tree',
    };
  }
  if (Number.isFinite(treeCount) && treeCount > 1) {
    return {
      applyAdapter: true,
      rootOnMidpoint: false,
      arrangeTaxa: 'input_order',
      reason: 'MEGA Tree Explorer multi-tree view disables midpoint root and uses input order',
    };
  }
  if (!megaInferredTreeMethods.has(method)) {
    return {
      applyAdapter: false,
      rootOnMidpoint: false,
      arrangeTaxa: 'none',
      reason: 'not an ordinary inferred tree method',
    };
  }
  return {
    applyAdapter: true,
    rootOnMidpoint: true,
    rootedByMethod: method === 'upgma',
    arrangeTaxa: 'balanced_shape',
    reason: 'MEGA Tree Explorer default for ordinary inferred NJ/ME/UPGMA/ML/MP trees',
  };
}

const megaInferredTreeMethods = new Set([
  'neighbor_joining',
  'minimum_evolution',
  'upgma',
  'maximum_likelihood',
  'maximum_parsimony',
]);

function normalizeToken(value) {
  return String(value || '').trim().toLowerCase().replace(/[-\s]+/g, '_');
}

function shouldApplyMidpointRoot(policy, imported, metadata) {
  if (!policy.rootOnMidpoint || !imported.hasBranchLengths || imported.totalBranchLength <= EPSILON) return false;
  const explicitRooted = metadata?.tree_is_rooted ?? metadata?.is_rooted ?? metadata?.rooted;
  if (typeof explicitRooted === 'boolean') return !explicitRooted;
  if (policy.rootedByMethod) return false;
  return true;
}

function parseNewick(text) {
  let index = 0;
  const source = text.trim();

  const skipSpace = () => {
    while (/\s/.test(source[index] || '')) index += 1;
  };

  const parseLabel = () => {
    skipSpace();
    if (source[index] === "'") {
      index += 1;
      let label = '';
      while (index < source.length) {
        if (source[index] === "'" && source[index + 1] === "'") {
          label += "'";
          index += 2;
          continue;
        }
        if (source[index] === "'") {
          index += 1;
          break;
        }
        label += source[index];
        index += 1;
      }
      return label;
    }
    let label = '';
    while (index < source.length && !'(),:;'.includes(source[index])) {
      label += source[index];
      index += 1;
    }
    return label.trim();
  };

  const parseLength = () => {
    skipSpace();
    if (source[index] !== ':') return { length: 0, hasLength: false };
    index += 1;
    let raw = '';
    while (index < source.length && !'(),;'.includes(source[index])) {
      raw += source[index];
      index += 1;
    }
    const length = Number(raw.trim());
    return { length: Number.isFinite(length) ? length : 0, hasLength: true };
  };

  const parseNode = () => {
    skipSpace();
    const node = { name: '', length: 0, hasLength: false, children: [] };
    if (source[index] === '(') {
      index += 1;
      while (index < source.length) {
        node.children.push(parseNode());
        skipSpace();
        if (source[index] === ',') {
          index += 1;
          continue;
        }
        if (source[index] === ')') {
          index += 1;
          break;
        }
        throw new Error(`invalid Newick at ${index}`);
      }
      node.name = parseLabel();
      Object.assign(node, parseLength());
      return node;
    }
    node.name = parseLabel();
    Object.assign(node, parseLength());
    return node;
  };

  const root = parseNode();
  skipSpace();
  if (source[index] === ';') index += 1;
  skipSpace();
  if (index < source.length) throw new Error(`trailing Newick content at ${index}`);
  return root;
}

function taxonOrder(metadata, root) {
  const order = new Map();
  for (const record of metadata?.records || []) {
    const taxon = String(record?.taxon_id || '').trim();
    if (taxon && !order.has(taxon)) order.set(taxon, order.size + 1);
  }
  visitTree(root, (node) => {
    if (node.children.length || !node.name || order.has(node.name)) return;
    order.set(node.name, order.size + 1);
  });
  return order;
}

function visitTree(node, visit) {
  visit(node);
  for (const child of node.children || []) visitTree(child, visit);
}

function importAsMegaTreeListTree(parsed, order) {
  const leavesByName = new Map();
  const internalNodes = [];
  let fallbackOrder = order.size + 1;
  let isRooted = true;
  let hasBranchLengths = false;
  let totalBranchLength = 0;

  const markLength = (source, target) => {
    target.length = source.hasLength ? source.length : 0;
    target.hasLength = source.hasLength;
    if (source.hasLength) {
      hasBranchLengths = true;
      totalBranchLength += Math.max(0, target.length);
    }
  };

  const leafIndex = (name) => {
    const key = String(name || '');
    if (!order.has(key)) {
      order.set(key, fallbackOrder);
      fallbackOrder += 1;
    }
    return order.get(key);
  };

  const decodeNodeData = (source) => {
    if (!source.children.length) {
      const key = source.name || '';
      let leaf = leavesByName.get(key);
      if (!leaf) {
        leaf = makeMegaNode({ name: key, otu: true, index: leafIndex(key) });
        leavesByName.set(key, leaf);
      }
      const edgeLeaf = cloneBareNode(leaf);
      markLength(source, edgeLeaf);
      return edgeLeaf;
    }

    if (source.children.length < 2) throw new Error('MEGA Newick importer requires binary or multifurcating groups');
    if (source.children.length > 2) isRooted = false;

    let left = decodeNodeData(source.children[0]);
    let right = decodeNodeData(source.children[1]);
    let current = makeInternalNode(left, right, internalNodes);
    for (let i = 2; i < source.children.length; i += 1) {
      const previous = current;
      const next = decodeNodeData(source.children[i]);
      previous.length = 0;
      previous.hasLength = hasBranchLengths;
      current = makeInternalNode(previous, next, internalNodes);
      isRooted = false;
    }
    current.name = source.name || current.name;
    markLength(source, current);
    return current;
  };

  const root = decodeNodeData(parsed);
  root.length = 0;
  root.hasLength = false;
  root.parent = null;

  const leaves = [...leavesByName.values()].sort((a, b) => a.index - b.index);
  const maxLeafIndex = leaves.reduce((max, leaf) => Math.max(max, leaf.index), 0);
  const nodes = Array(maxLeafIndex + internalNodes.length + 1).fill(null);
  for (const leaf of leaves) nodes[leaf.index] = leaf;
  internalNodes.forEach((node, index) => {
    node.index = maxLeafIndex + index + 1;
    nodes[node.index] = node;
  });
  relinkCanonicalLeafNodes(root, leavesByName);

  return { root, nodes, isRooted, hasBranchLengths, totalBranchLength };
}

function makeMegaNode({ name = '', otu = false, index = 0 } = {}) {
  return {
    name,
    index,
    otu,
    length: 0,
    hasLength: false,
    maxlen1: 0,
    maxlen2: 0,
    parent: null,
    children: [],
  };
}

function cloneBareNode(source) {
  return {
    ...makeMegaNode({ name: source.name, otu: source.otu, index: source.index }),
  };
}

function makeInternalNode(left, right, internalNodes) {
  const node = makeMegaNode({ otu: false });
  node.children = [left, right];
  left.parent = node;
  right.parent = node;
  internalNodes.push(node);
  return node;
}

function relinkCanonicalLeafNodes(root, leavesByName) {
  visitTree(root, (node) => {
    if (node.children.length) return;
    const canonical = leavesByName.get(node.name);
    if (!canonical) return;
    canonical.length = node.length;
    canonical.hasLength = node.hasLength;
    canonical.parent = node.parent;
    if (node.parent) {
      const idx = node.parent.children.indexOf(node);
      if (idx >= 0) node.parent.children[idx] = canonical;
    }
  });
}

function makeRootOnMidPoint(root, nodes = null) {
  const searchNodes = nodes || indexedNodes(root);
  setMaxLength(root);
  const midpointNode = searchMidPoint(root, searchNodes);
  if (!midpointNode || midpointNode === root || !midpointNode.parent) return root;
  return moveRootOnBranch(root, midpointNode);
}

function indexedNodes(root) {
  const nodes = [];
  visitTree(root, (node) => {
    if (Number.isInteger(node.index) && node.index > 0) nodes[node.index] = node;
  });
  return nodes;
}

function setMaxLength(root) {
  if (!root?.children?.[0] || !root?.children?.[1]) return;

  const setMaxLen2 = (node) => {
    if (!node.children.length) {
      node.maxlen2 = node.length;
      return;
    }
    setMaxLen2(node.children[0]);
    setMaxLen2(node.children[1]);
    node.maxlen2 = Math.max(node.children[0].maxlen2, node.children[1].maxlen2) + node.length;
  };

  const sibling = (node) => {
    const parent = node.parent;
    if (!parent) return null;
    return parent.children[0] === node ? parent.children[1] : parent.children[0];
  };

  const setMaxLen1 = (node) => {
    const other = sibling(node);
    node.maxlen1 = Math.max(node.parent?.maxlen1 || 0, other?.maxlen2 || 0) + node.length;
    if (node.children.length) {
      setMaxLen1(node.children[0]);
      setMaxLen1(node.children[1]);
    }
  };

  setMaxLen2(root.children[0]);
  setMaxLen2(root.children[1]);
  root.children[0].maxlen1 = root.children[1].maxlen2 + root.children[0].length;
  root.children[1].maxlen1 = root.children[0].maxlen2 + root.children[1].length;
  if (root.children[0].children.length) {
    setMaxLen1(root.children[0].children[0]);
    setMaxLen1(root.children[0].children[1]);
  }
  if (root.children[1].children.length) {
    setMaxLen1(root.children[1].children[0]);
    setMaxLen1(root.children[1].children[1]);
  }
}

function searchMidPoint(root, nodes) {
  let result = root.children[0];
  for (let i = 1; i < nodes.length; i += 1) {
    const node = nodes[i];
    if (!node || node === root) continue;
    const r = node.length - Math.abs(node.maxlen1 - node.maxlen2);
    if (r >= -EPSILON) result = node;
  }
  return result;
}

function moveRootOnBranch(root, selected) {
  if (!selected.parent || selected === root) return root;
  if (selected.parent === root) {
    changeRootAtRootChild(root, selected);
    return root;
  }

  let p = selected;
  while (p.parent !== root) p = p.parent;
  const rootmode = p === p.parent.children[0] ? 1 : 2;

  p = selected;
  if (rootmode === 1) {
    while (p.parent !== root) {
      if (p === p.parent.children[0]) swapChildren(p.parent);
      p = p.parent;
    }
  } else {
    while (p.parent !== root) {
      if (p === p.parent.children[1]) swapChildren(p.parent);
      p = p.parent;
    }
  }

  const q = siblingNode(p);
  const b0 = branchOf(q);
  b0.maxlen2 = p.maxlen1;
  b0.length = root.children[0].length + root.children[1].length;

  let d = selected;
  p = d.parent;
  let a = p.parent;
  let b2 = branchOf(d);
  while (p !== root) {
    const b1 = branchOf(p);
    assignBranch(p, b2);
    swapMaxlen(p);
    b2 = b1;
    p.parent = d;
    if (d === p.children[0]) {
      p.children[0] = a;
    } else {
      p.children[1] = a;
    }
    d = p;
    p = a;
    a = a.parent;
  }

  if (d === p.children[0]) {
    p.children[1].parent = d;
    assignBranch(p.children[1], b0);
    if (p === d.children[0]) {
      d.children[0] = p.children[1];
    } else {
      d.children[1] = p.children[1];
    }
  } else {
    p.children[0].parent = d;
    assignBranch(p.children[0], b0);
    if (p === d.children[0]) {
      d.children[0] = p.children[0];
    } else {
      d.children[1] = p.children[0];
    }
  }

  p = selected.parent;
  p.parent = root;
  selected.parent = root;
  root.children = rootmode === 1 ? [selected, p] : [p, selected];

  const b0Selected = branchOf(selected);
  const b1 = branchOf(selected);
  const b2Final = branchOf(selected);
  if (Math.abs(b0Selected.maxlen1 - b0Selected.maxlen2) <= b0Selected.length + EPSILON) {
    b1.maxlen1 = (b0Selected.maxlen1 + b0Selected.maxlen2 - b0Selected.length) / 2;
    b1.length = b0Selected.maxlen2 - b1.maxlen1;
    b2Final.length = b0Selected.maxlen1 - b1.maxlen1;
  } else {
    const denominator = b0Selected.maxlen1 + b0Selected.maxlen2;
    b1.length = denominator <= EPSILON ? 0 : b0Selected.length * b0Selected.maxlen2 / denominator;
    b2Final.length = denominator <= EPSILON ? b0Selected.length : b0Selected.length * b0Selected.maxlen1 / denominator;
  }
  b1.maxlen1 = b0Selected.maxlen2;
  b1.maxlen2 = b0Selected.maxlen1 - b2Final.length;
  b2Final.maxlen1 = b0Selected.maxlen1;
  b2Final.maxlen2 = b0Selected.maxlen2 - b1.length;
  assignBranch(p, b1);
  assignBranch(selected, b2Final);
  return root;
}

function changeRootAtRootChild(root, selected) {
  let target = selected;
  if (target === root.children[0] && root.children[0].maxlen2 < root.children[1].maxlen2) {
    target = root.children[1];
  } else if (target === root.children[1] && root.children[1].maxlen2 < root.children[0].maxlen2) {
    target = root.children[0];
  }

  let b1Length = 0;
  let b2Length = target.length;
  if (Math.abs(target.maxlen1 - target.maxlen2) <= target.length + EPSILON) {
    const b0Length = (target.maxlen1 + target.maxlen2 - target.length) / 2;
    b1Length = target.maxlen2 - b0Length;
    b2Length = target.maxlen1 - b0Length;
  }
  if (target === root.children[0]) {
    root.children[1].length += b1Length;
    root.children[1].maxlen2 += b1Length;
    target.length = b2Length;
    target.maxlen2 -= b1Length;
  } else {
    root.children[0].length += b1Length;
    root.children[0].maxlen2 += b1Length;
    target.length = b2Length;
    target.maxlen2 -= b1Length;
  }
}

function siblingNode(node) {
  if (!node?.parent) return null;
  return node.parent.children[0] === node ? node.parent.children[1] : node.parent.children[0];
}

function swapChildren(node) {
  node.children = [node.children[1], node.children[0]];
}

function branchOf(node) {
  return {
    length: node?.length || 0,
    hasLength: !!node?.hasLength,
    maxlen1: node?.maxlen1 || 0,
    maxlen2: node?.maxlen2 || 0,
  };
}

function assignBranch(node, branch) {
  node.length = branch.length;
  node.hasLength = branch.hasLength;
  node.maxlen1 = branch.maxlen1;
  node.maxlen2 = branch.maxlen2;
}

function swapMaxlen(node) {
  const old = node.maxlen1;
  node.maxlen1 = node.maxlen2;
  node.maxlen2 = old;
}

function annotateDisplayTree(node, order) {
  if (!node.children.length) {
    node.size = 1;
    node.minOTU = order.get(node.name) || Number.POSITIVE_INFINITY;
    return;
  }
  for (const child of node.children) annotateDisplayTree(child, order);
  node.size = node.children.reduce((sum, child) => sum + child.size, 0);
  node.minOTU = Math.min(...node.children.map((child) => child.minOTU));
}

function sortClusterForMegaShape(node, parent, side) {
  if (!node.children.length) return;
  if (node.children.length === 2) {
    const [left, right] = node.children;
    let swap = false;
    if (!parent || side === 'des1') {
      swap = left.size < right.size;
    } else if (side === 'des2') {
      swap = left.size > right.size;
    }
    if (left.size === right.size && left.minOTU > right.minOTU) {
      swap = true;
    }
    if (swap) node.children = [right, left];
  } else {
    node.children.sort((a, b) => (b.size - a.size) || (a.minOTU - b.minOTU));
  }
  node.children.forEach((child, index) => sortClusterForMegaShape(child, node, index === 0 ? 'des1' : 'des2'));
}

function sortClusterInInputOrder(node) {
  if (!node.children.length) return;
  if (node.children.length === 2 && node.children[0].minOTU > node.children[1].minOTU) {
    node.children = [node.children[1], node.children[0]];
  } else if (node.children.length > 2) {
    node.children.sort((a, b) => a.minOTU - b.minOTU);
  }
  node.children.forEach(sortClusterInInputOrder);
}

function writeNewick(node) {
  const children = node.children.length ? `(${node.children.map(writeNewick).join(',')})` : '';
  const label = node.name ? quoteLabel(node.name) : '';
  const length = node.hasLength ? `:${formatLength(node.length)}` : '';
  return `${children}${label}${length}`;
}

function quoteLabel(label) {
  const value = String(label || '');
  if (value === '') return '';
  if (/[\s(),:;']/.test(value)) return `'${value.replace(/'/g, "''")}'`;
  return value;
}

function formatLength(value) {
  if (!Number.isFinite(value)) return '0';
  if (Math.abs(value) <= EPSILON) return '0.0';
  return Number(value.toPrecision(15)).toString();
}
