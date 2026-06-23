import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const root = resolve(here, '..');
const dist = resolve(root, 'node_modules/reactreejs/dist');

const esm = readFileSync(resolve(dist, 'index.mjs'), 'utf8');
const dts = readFileSync(resolve(dist, 'index.d.ts'), 'utf8');
const dmts = readFileSync(resolve(dist, 'index.d.mts'), 'utf8');

assert.match(
  esm,
  /function Reactree\(\{ newick, defaultHeight = 520, fasta, initialState, onStateChange, onViewerSnapshot, labelMetadata, showPhgoCoords = false, setShowPhgoCoords \}\)/,
);
assert.match(esm, /function buildPhgoDisplayTreeLabelResolver\(metadata, showPhgoCoords\)/);
assert.match(esm, /const displayLabelResolver = useMemo\(\(\) => buildPhgoDisplayTreeLabelResolver\(labelMetadata, showPhgoCoords\), \[labelMetadata, showPhgoCoords\]\)/);
assert.match(esm, /showPhgoCoords,\n\s+setShowPhgoCoords,\n\s+collapsedNodes,/);
assert.match(esm, /const coordItem = \{ key: "phgoCoords", label: "Coord"/);
assert.match(esm, /children: \[alignmentItem\.node, alignItem\.node, coordItem\.node, labelItem\.node, expandItem\.node\]/);
assert.match(esm, /showPhgoCoords,\n\s+containerH,/);
assert.match(
  esm,
  /\[treeData, history, layout, renderStyle, treeType, labelMode, alignLabels, truncateNames, showAlignment, showPhgoCoords,/,
);
assert.match(esm, /showPhgoCoords,\n\s+setShowPhgoCoords,\n\s+collapsedNodes,/);

for (const types of [dts, dmts]) {
  assert.match(types, /labelMetadata\?: Record<string, unknown>;/);
  assert.match(types, /showPhgoCoords\?: boolean;/);
  assert.match(types, /setShowPhgoCoords\?: \(updater: boolean \| \(\(value: boolean\) => boolean\)\) => void;/);
  assert.match(
    types,
    /declare function Reactree\(\{ newick, defaultHeight, fasta, initialState, onStateChange, onViewerSnapshot, labelMetadata, showPhgoCoords, setShowPhgoCoords \}: ReactreeProps\)/,
  );
}
