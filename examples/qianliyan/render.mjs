import {readFile, writeFile} from 'node:fs/promises';
import {pathToFileURL} from 'node:url';

const entities = {'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'};
const escape = text => text.replace(/[&<>"']/g, character => entities[character]);

function sourceLine(text) {
  let result = '';
  let offset = 0;
  for (const match of text.matchAll(/https?:\/\/[^\s`<>；]+/gi)) {
    result += escape(text.slice(offset, match.index));
    let url;
    try {
      url = new URL(match[0]);
    } catch {
      // 无法解析的来源地址仍按原文显示。
    }
    const label = escape(match[0]);
    result += url && !url.username && !url.password
      ? '<a href="' + escape(url.href) + '" rel="noreferrer">' + label + '</a>'
      : label;
    offset = match.index + match[0].length;
  }
  return result + escape(text.slice(offset));
}

export function renderBody(markdown) {
  if (typeof markdown !== 'string' || !markdown.trim()) throw new Error('报告为空');
  if (Buffer.byteLength(markdown, 'utf8') > 1024 * 1024) throw new Error('报告超过 1 MiB');
  let sources = false;
  return markdown.split(/\r?\n/).map(line => {
    const heading = /^(#{1,3})\s+(.+)$/.exec(line);
    if (heading) {
      sources = heading[2] === '来源记录（平台生成）';
      const tag = heading[1].length === 3 ? 'h3' : 'h2';
      return '<' + tag + '>' + escape(heading[2]) + '</' + tag + '>';
    }
    if (!line.trim()) return '';
    return '<p>' + (sources ? sourceLine(line) : escape(line)) + '</p>';
  }).join('\n');
}

export function render(markdown) {
  const body = renderBody(markdown);
  return `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'">
<title>千里眼行业简报</title><style>
body{margin:0;background:#f3f6fa;color:#1b2534;font:16px/1.7 system-ui,sans-serif}
main{max-width:880px;margin:auto;padding:2rem;background:#fff}h1,h2,h3{line-height:1.35}
h1{color:#153a65}h2{border-top:1px solid #dbe3ee;padding-top:1rem;margin-top:2rem}
a{color:#075caa;overflow-wrap:anywhere}p{overflow-wrap:anywhere}
</style></head><body><main><h1>千里眼行业简报</h1>${body}</main></body></html>\n`;
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const [, , source, target] = process.argv;
  if (!source || !target) throw new Error('用法：node render.mjs <报告.md> <简报.html>');
  await writeFile(target, render(await readFile(source, 'utf8')), {flag: 'wx'});
}
