// Validates contracts/openapi/singgah.yaml (structure + $refs) via
// @scalar/openapi-parser. Erasable-syntax only so plain `node` runs this.
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { bundle } from '@scalar/json-magic/bundle';
import { parseYaml, readFiles } from '@scalar/json-magic/bundle/plugins/node';
import { validate } from '@scalar/openapi-parser';

const root = path.resolve(fileURLToPath(new URL('.', import.meta.url)), '..');
const spec = path.join(root, 'contracts/openapi/singgah.yaml');

// bundle() loads the entry file plus every external $ref before validation,
// so a broken or unresolvable reference fails here.
const bundled = await bundle(spec, { plugins: [readFiles(), parseYaml()] });
const result = await validate(bundled);

if (!result.valid) {
	console.error(`OpenAPI validation failed for ${path.relative(root, spec)}:`);
	for (const err of result.errors ?? []) {
		console.error(` - ${typeof err === 'string' ? err : (err.message ?? String(err))}`);
	}
	process.exit(1);
}

console.log(`OpenAPI ${result.version ?? ''} valid: ${path.relative(root, spec)}`);
