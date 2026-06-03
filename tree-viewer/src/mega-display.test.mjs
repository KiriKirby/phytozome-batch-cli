import assert from 'node:assert/strict';
import { megaDefaultDisplayNewick, megaDefaultDisplayPolicy } from './mega-display.js';

function leafOrder(newick) {
  const out = [];
  let token = '';
  let inQuote = false;
  let inLength = false;
  const flush = () => {
    const value = token.trim();
    token = '';
    if (value) out.push(value.replace(/^'|'$/g, '').replace(/''/g, "'"));
  };
  for (const ch of String(newick || '')) {
    if (inLength) {
      if (ch === ',' || ch === ')' || ch === ';') inLength = false;
      else continue;
    }
    if (!inLength && ch === "'") {
      inQuote = !inQuote;
      token += ch;
      continue;
    }
    if (!inQuote && ch === ':') {
      flush();
      inLength = true;
      continue;
    }
    if (!inQuote && (ch === '(' || ch === ',' || ch === ')' || ch === ';')) {
      flush();
      continue;
    }
    token += ch;
  }
  return out;
}

const records = ['A', 'B', 'C', 'D', 'E'].map((taxon_id) => ({ taxon_id }));
const runtimeMeta = (tree_method, extra = {}) => ({
  tree_computation_source: 'mega-phgo-runtime',
  tree_method,
  records,
  ...extra,
});

const inferredMethods = [
  'neighbor_joining',
  'minimum_evolution',
  'upgma',
  'maximum_likelihood',
  'maximum_parsimony',
];

for (const method of inferredMethods) {
  const policy = megaDefaultDisplayPolicy(runtimeMeta(method));
  assert.equal(policy.applyAdapter, true, `${method} should use MEGA Tree Explorer adapter`);
  assert.equal(policy.rootOnMidpoint, true, `${method} should enable Root on Midpoint action`);
  assert.equal(policy.arrangeTaxa, 'balanced_shape', `${method} should default to balanced shape`);
}

const standaloneNewick = '(A:1,B:2,(C:3,D:4):5);';
assert.equal(
  megaDefaultDisplayNewick(standaloneNewick, { records }),
  standaloneNewick,
  'standalone imported Newick must not be rerooted or sorted as an inferred MEGA runtime tree',
);

const unrootedRuntimeNewick = '(A:1,B:2,(C:3,D:20):5,E:1);';
const mlDisplay = megaDefaultDisplayNewick(unrootedRuntimeNewick, runtimeMeta('maximum_likelihood'));
assert.notEqual(mlDisplay, unrootedRuntimeNewick, 'runtime ML tree should pass through MEGA default display adaptation');
for (const method of ['neighbor_joining', 'minimum_evolution', 'maximum_likelihood']) {
  assert.equal(
    megaDefaultDisplayNewick(unrootedRuntimeNewick, runtimeMeta(method)),
    mlDisplay,
    `${method} should share the ordinary unrooted branch-length display path`,
  );
}

const binaryRuntimeNewick = '((A:1,B:2):3,(C:4,D:20):5);';
for (const method of ['neighbor_joining', 'minimum_evolution', 'maximum_likelihood']) {
  assert.notEqual(
    megaDefaultDisplayNewick(binaryRuntimeNewick, runtimeMeta(method)),
    binaryRuntimeNewick,
    `${method} should still apply MEGA inferred-tree midpoint display behavior when runtime Newick has a binary root shape`,
  );
}

const rootedUPGMA = '((A:1,B:1):2,(C:1,D:1):2);';
assert.deepEqual(
  leafOrder(megaDefaultDisplayNewick(rootedUPGMA, runtimeMeta('upgma'))),
  ['A', 'B', 'C', 'D'],
  'rooted UPGMA Newick should keep its rooted topology and only apply default arrangement',
);

const topologyOnlyMP = '(A,B,(C,D),E);';
const mpDisplay = megaDefaultDisplayNewick(topologyOnlyMP, runtimeMeta('maximum_parsimony'));
assert.deepEqual(
  leafOrder(mpDisplay),
  ['A', 'B', 'C', 'D', 'E'],
  'topology-only MP trees do not midpoint-root because MEGA MakeRootOnMidPoint exits without branch lengths',
);
assert.equal(mpDisplay.includes(':'), false, 'topology-only display must not invent branch lengths');

const multiTreeDisplay = megaDefaultDisplayNewick(unrootedRuntimeNewick, runtimeMeta('maximum_likelihood', { tree_count: 2 }));
assert.deepEqual(
  leafOrder(multiTreeDisplay),
  ['A', 'B', 'C', 'D', 'E'],
  'MEGA multi-tree view disables midpoint rooting and arranges taxa by input order',
);

assert.equal(
  megaDefaultDisplayPolicy({ tree_computation_source: 'manual-import', tree_method: 'maximum_likelihood' }).applyAdapter,
  false,
  'manual imports are not ordinary runtime inferred trees',
);
