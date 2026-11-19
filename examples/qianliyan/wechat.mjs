import {readFile, writeFile, open} from 'node:fs/promises';
import {pathToFileURL} from 'node:url';
import {createHash} from 'node:crypto';
import {renderBody} from './render.mjs';

export function createPublisher(config, request = fetch) {
  if (!config.appId || !config.secret || !config.cover) throw new Error('缺少公众号 AppID、AppSecret 或永久封面素材 ID');
  let token;
  async function call(endpoint, body) {
    const url = new URL('/cgi-bin/' + endpoint, 'https://api.weixin.qq.com');
    if (endpoint !== 'stable_token') url.searchParams.set('access_token', token);
    let result, response;
    try {
      response = await request(url, {
        method: 'POST', redirect: 'error', signal: AbortSignal.timeout(20_000),
        headers: {'Content-Type': 'application/json'}, body: JSON.stringify(body),
      });
      result = await response.json();
    } catch {
      throw new Error('微信公众号 ' + endpoint + ' 响应未知，请核对公众号后台');
    }
    if (!response.ok || !result || (result.errcode !== undefined && result.errcode !== 0)) {
      const code = Number.isSafeInteger(result?.errcode) ? result.errcode : response.status;
      throw new Error('微信公众号 ' + endpoint + ' 失败（错误码 ' + code + '）');
    }
    return result;
  }
  async function invoke(endpoint, body) {
    if (!token) {
      const result = await call('stable_token', {
        grant_type: 'client_credential', appid: config.appId, secret: config.secret, force_refresh: false,
      });
      token = result.access_token;
      if (typeof token !== 'string' || !token) throw new Error('公众号未返回有效访问凭据');
    }
    return call(endpoint, body);
  }
  return {
    async draft(title, content) {
      if (!content.trim() || content.length >= 20_000 || Buffer.byteLength(content) >= 1024 * 1024) {
        throw new Error('公众号正文为空或超出长度限制');
      }
      const result = await invoke('draft/add', {articles: [{title, content, thumb_media_id: config.cover}]});
      if (typeof result.media_id !== 'string' || !result.media_id) throw new Error('缺少草稿回执，请核对公众号后台');
      return result.media_id;
    },
    async submit(mediaId) {
      const result = await invoke('freepublish/submit', {media_id: mediaId});
      const id = result.publish_id;
      if (!(typeof id === 'string' && id.trim()) && !(Number.isSafeInteger(id) && id > 0)) {
        throw new Error('缺少发布回执，请核对公众号后台');
      }
      return id;
    },
    async status(id) {
      const result = await invoke('freepublish/get', {publish_id: id});
      if (result.publish_status === 0 && result.article_id) return 'published';
      if (result.publish_status === 1) return 'publishing';
      if (Number.isInteger(result.publish_status) && result.publish_status > 1) {
        throw new Error('公众号发布失败（状态 ' + result.publish_status + '）');
      }
      throw new Error('公众号发布状态未知');
    },
  };
}

async function updateReceipt(file, receipt) {
  const output = await open(file, 'a');
  try {
    await output.writeFile(JSON.stringify(receipt) + '\n');
    await output.sync();
  } finally {
    await output.close();
  }
}

async function publishFile(source, receiptFile, publisher, appId) {
  const markdown = await readFile(source, 'utf8');
  const content = renderBody(markdown);
  const hash = createHash('sha256').update(markdown).digest('hex');
  const date = new Intl.DateTimeFormat('sv-SE', {timeZone: 'Asia/Shanghai'}).format(new Date());
  const receipt = {app_id: appId, sha256: hash, status:'draft_pending'};
  await writeFile(receiptFile, JSON.stringify(receipt) + '\n', {flag:'wx'});
  receipt.media_id = await publisher.draft('千里眼行业简报 · ' + date, content);
  receipt.status = 'submit_pending';
  await updateReceipt(receiptFile, receipt);
  receipt.publish_id = await publisher.submit(receipt.media_id);
  receipt.status = 'publishing';
  await updateReceipt(receiptFile, receipt);
  console.log(JSON.stringify({status: receipt.status, publish_id: receipt.publish_id}));
}

async function checkFile(receiptFile, publisher, appId) {
  const body = await readFile(receiptFile, 'utf8');
  if (!body.endsWith('\n')) throw new Error('发布回执不完整，请先核对公众号后台');
  const receipt = JSON.parse(body.trimEnd().split('\n').at(-1));
  if (receipt.app_id !== appId || !receipt.publish_id) throw new Error('账号不匹配或没有发布回执，请核对公众号后台');
  receipt.status = await publisher.status(receipt.publish_id);
  await updateReceipt(receiptFile, receipt);
  console.log(JSON.stringify({status: receipt.status, publish_id: receipt.publish_id}));
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const [, , action, receipt, source] = process.argv;
  if (!receipt || !((action === 'publish' && source) || (action === 'status' && !source))) {
    throw new Error('用法：node wechat.mjs publish <回执.jsonl> <报告.md> 或 node wechat.mjs status <回执.jsonl>');
  }
  const appId = process.env.QIANLIYAN_WECHAT_APP_ID;
  const publisher = createPublisher({appId, secret: process.env.QIANLIYAN_WECHAT_APP_SECRET, cover: process.env.QIANLIYAN_WECHAT_COVER_MEDIA_ID});
  if (action === 'publish') await publishFile(source, receipt, publisher, appId);
  else await checkFile(receipt, publisher, appId);
}
