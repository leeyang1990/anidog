import assert from 'node:assert/strict'
import { nonNegativeSetting } from '../src/utils/numericSetting.js'

for (const value of [0, '0', '0.0']) {
  assert.equal(nonNegativeSetting(value, 95), 0)
  assert.equal(nonNegativeSetting(String(nonNegativeSetting(value, 100)), 100), 0)
}
for (const value of [undefined, null, '', '  ', 'invalid', 'Infinity', Infinity]) {
  assert.equal(nonNegativeSetting(value, 100), 100)
}
assert.equal(nonNegativeSetting('-2', 100), 0)
assert.equal(nonNegativeSetting('12.5', 100), 12.5)
console.log('numeric setting tests passed')
