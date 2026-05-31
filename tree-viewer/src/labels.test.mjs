import assert from 'node:assert/strict';
import { reactreeLabelMap, reactreeLabelMaps, relabelFasta, relabelNewick, sanitizeReactreeLabel } from './labels.js';

const metadata = {
  records: [
    { taxon_id: 'PHGOT000001', display_name: 'PAL 1' },
    { taxon_id: 'PHGOT000002', display_name: 'PAL_1' },
    { taxon_id: 'PHGOT000003', display_name: 'PAL:1(beta)' },
    { taxon_id: 'PHGOT000004', display_name: '' },
    { taxon_id: 'PHGOT000005', display_name: "O'Brien, PAL; 2" },
  ],
};

assert.equal(sanitizeReactreeLabel(' PAL: 1; (beta) '), 'PAL: 1; (beta)');

const labels = reactreeLabelMap(metadata);
assert.equal(labels.get('PHGOT000001'), 'PAL 1');
assert.equal(labels.get('PHGOT000002'), 'PAL_1');
assert.equal(labels.get('PHGOT000003'), 'PAL:1(beta)');
assert.equal(labels.get('PHGOT000004'), 'PHGOT000004');
assert.equal(labels.get('PHGOT000005'), "O'Brien, PAL; 2");
assert.equal(new Set(labels.values()).size, labels.size);

const { reverse } = reactreeLabelMaps(metadata);
assert.equal(reverse.get('PAL 1'), 'PHGOT000001');
assert.equal(reverse.get('PAL_1'), 'PHGOT000002');
assert.equal(reverse.get("O'Brien, PAL; 2"), 'PHGOT000005');

assert.equal(
  relabelNewick('(PHGOT000001:0.1,(PHGOT000002:0.2,PHGOT000003:0.3),PHGOT000004,PHGOT000005);', metadata),
  "('PAL 1':0.1,('PAL_1':0.2,'PAL:1(beta)':0.3),'PHGOT000004','O''Brien, PAL; 2');",
);

assert.equal(
  relabelFasta('>PHGOT000001\nAAAA\n>PHGOT000002 some source note\nCCCC\n>PHGOT000005\nTTTT\n>UNKNOWN\nGGGG', metadata),
  ">PAL 1\nAAAA\n>PAL_1\nCCCC\n>O'Brien, PAL; 2\nTTTT\n>UNKNOWN\nGGGG",
);
