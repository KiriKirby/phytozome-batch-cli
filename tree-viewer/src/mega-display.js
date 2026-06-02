const EPSILON = 1e-10;

export function megaDefaultDisplayNewick(newick, metadata) {
  const text = String(newick || '').trim();
  if (!text) return text;
  try {
    const parsed = parseNewick(text);
    const order = taxonOrder(metadata, parsed);
    const graph = buildGraph(parsed, order);
    if (graph.leaves.length < 2) return text;
    let displayTree = parsed;
    if (hasPositiveLength(graph.edges)) {
      displayTree = midpointRootedTree(graph);
    }
    annotateDisplayTree(displayTree, order);
    sortClusterForMegaShape(displayTree, null, 'root');
    annotateDisplayTree(displayTree, order);
    return `${writeNewick(displayTree)};`;
  } catch {
    return text;
  }
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

function buildGraph(root, order) {
  const nodes = [];
  const edges = [];

  const addNode = (source, parentID = -1) => {
    const id = nodes.length;
    const leafOrder = source.children.length ? Number.POSITIVE_INFINITY : (order.get(source.name) || Number.POSITIVE_INFINITY);
    nodes.push({ id, name: source.name || '', order: leafOrder });
    if (parentID >= 0) edges.push({ a: parentID, b: id, length: source.hasLength ? source.length : 0, hasLength: source.hasLength });
    for (const child of source.children) addNode(child, id);
  };

  addNode(root);
  const adjacency = nodes.map(() => []);
  for (const edge of edges) {
    adjacency[edge.a].push({ to: edge.b, length: edge.length, hasLength: edge.hasLength });
    adjacency[edge.b].push({ to: edge.a, length: edge.length, hasLength: edge.hasLength });
  }
  const leaves = nodes.filter((node) => node.name && adjacency[node.id].length <= 1).map((node) => node.id);
  return { nodes, edges, adjacency, leaves };
}

function hasPositiveLength(edges) {
  return edges.some((edge) => edge.hasLength && edge.length > EPSILON);
}

function midpointRootedTree(graph) {
  const start = graph.leaves[0];
  const first = farthestLeaf(graph, start).leaf;
  const second = farthestLeaf(graph, first);
  const path = pathToRoot(second.parent, second.leaf).reverse();
  const midpoint = second.distance[second.leaf] / 2;
  let offset = 0;
  for (let i = 0; i < path.length - 1; i += 1) {
    const from = path[i];
    const to = path[i + 1];
    const edge = graph.adjacency[from].find((candidate) => candidate.to === to);
    const nextOffset = offset + edge.length;
    if (nextOffset + EPSILON >= midpoint) {
      const fromLength = Math.max(0, midpoint - offset);
      const toLength = Math.max(0, nextOffset - midpoint);
      if (fromLength <= EPSILON) return orientAtExistingNode(graph, from);
      if (toLength <= EPSILON) return orientAtExistingNode(graph, to);
      return {
        name: '',
        length: 0,
        hasLength: false,
        children: [
          orientFromGraph(graph, from, to, fromLength, true),
          orientFromGraph(graph, to, from, toLength, true),
        ],
      };
    }
    offset = nextOffset;
  }
  return orientAtExistingNode(graph, path[Math.max(0, path.length - 1)]);
}

function farthestLeaf(graph, start) {
  const distance = Array(graph.nodes.length).fill(Number.NEGATIVE_INFINITY);
  const parent = Array(graph.nodes.length).fill(-1);
  distance[start] = 0;
  const stack = [start];
  while (stack.length) {
    const current = stack.pop();
    for (const edge of graph.adjacency[current]) {
      if (edge.to === parent[current]) continue;
      distance[edge.to] = distance[current] + Math.max(0, edge.length);
      parent[edge.to] = current;
      stack.push(edge.to);
    }
  }
  let leaf = start;
  for (const candidate of graph.leaves) {
    if (distance[candidate] > distance[leaf] + EPSILON) leaf = candidate;
  }
  return { leaf, distance, parent };
}

function pathToRoot(parent, leaf) {
  const path = [];
  for (let node = leaf; node >= 0; node = parent[node]) path.push(node);
  return path;
}

function orientAtExistingNode(graph, rootID) {
  return {
    name: graph.nodes[rootID].name,
    length: 0,
    hasLength: false,
    children: graph.adjacency[rootID].map((edge) => orientFromGraph(graph, edge.to, rootID, edge.length, edge.hasLength)),
  };
}

function orientFromGraph(graph, nodeID, parentID, length, hasLength) {
  const graphNode = graph.nodes[nodeID];
  const node = {
    name: graphNode.name,
    length,
    hasLength,
    children: [],
  };
  for (const edge of graph.adjacency[nodeID]) {
    if (edge.to === parentID) continue;
    node.children.push(orientFromGraph(graph, edge.to, nodeID, edge.length, edge.hasLength));
  }
  return node;
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
