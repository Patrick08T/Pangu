import { readFile, writeFile } from 'node:fs/promises';
import openapiTS, { astToString } from 'openapi-typescript';

const generated = astToString(await openapiTS(new URL('../../api/openapi.yaml', import.meta.url)));
const destination = new URL('../src/api/schema.d.ts', import.meta.url);
if (process.argv.includes('--check')) {
  if (await readFile(destination, 'utf8') !== generated) {
    throw new Error('API 类型与契约不一致，请运行 npm run generate 并提交结果。');
  }
} else {
  await writeFile(destination, generated);
}
