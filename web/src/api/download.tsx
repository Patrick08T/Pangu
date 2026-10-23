import {useAction} from './hooks';
import {APIError} from './request';
import {ErrorNotice} from '../console/components';

export async function download(path:string,filename:string,signal=AbortSignal.timeout(120000)) {
  const response=await fetch(`/api${path}`,{credentials:'same-origin',cache:'no-store',signal:AbortSignal.any([signal,AbortSignal.timeout(120000)])});
  if(response.status===401)window.dispatchEvent(new Event('pangu:unauthenticated'));
  if(!response.ok){
    const value:unknown=await response.json().catch(()=>undefined);
    const message=value&&typeof value==='object'&&'error' in value&&typeof value.error==='string'?value.error:`下载失败（${response.status}）`;
    throw new APIError(message,response.status,response.headers.get('X-Request-ID')??'');
  }
  const blob=await response.blob();signal.throwIfAborted();
  const url=URL.createObjectURL(blob),link=document.createElement('a');
  link.href=url;link.download=filename;document.body.appendChild(link);link.click();link.remove();setTimeout(()=>URL.revokeObjectURL(url),1000);
}

export function Download({path,filename,children}:{path:string;filename:string;children:string}) {
  const action=useAction();
  return <span className="download"><button disabled={action.busy} onClick={()=>void action.run(signal=>download(path,filename,signal))}>{action.busy?'下载中…':children}</button><ErrorNotice error={action.error}/></span>;
}
