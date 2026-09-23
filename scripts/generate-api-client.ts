// Regenerates packages/api-client/src/generated/singgah.d.ts from
// contracts/openapi/singgah.yaml. Output is committed; CI fails on drift.
// Erasable-syntax only so plain `node` runs this file (Node >= 24).
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import openapiTS, { astToString } from 'openapi-typescript';

const root = path.resolve(fileURLToPath(new URL('.', import.meta.url)), '..');
const spec = new URL('../contracts/openapi/singgah.yaml', import.meta.url);
const outDir = path.join(root, 'packages/api-client/src/generated');
const outFile = path.join(outDir, 'singgah.d.ts');

const ast = await openapiTS(spec);
const banner =
	'/* eslint-disable */\n' +
	'// AUTO-GENERATED from contracts/openapi/singgah.yaml — do not edit.\n' +
	'// Regenerate with: pnpm client:generate\n';

await mkdir(outDir, { recursive: true });
await writeFile(outFile, banner + astToString(ast));
console.log(`generated ${path.relative(root, outFile)}`);
