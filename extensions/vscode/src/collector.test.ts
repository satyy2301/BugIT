import { collectorBase, pickLatest } from './collector';

function assert(cond: boolean, msg: string) {
  if (!cond) {
    throw new Error(msg);
  }
}

function testCollectorBase() {
  assert(collectorBase('http://localhost:8080/') === 'http://localhost:8080', 'trailing slash');
  assert(collectorBase('http://localhost:8080') === 'http://localhost:8080', 'no slash');
}

function testPickLatest() {
  const snaps = [
    { id: 'a', captured_at: '2026-01-01T00:00:00Z' },
    { id: 'b', captured_at: '2026-01-02T00:00:00Z' },
  ];
  const latest = pickLatest(snaps);
  assert(latest?.id === 'b', 'latest by captured_at');
}

testCollectorBase();
testPickLatest();
console.log('collector tests passed');
