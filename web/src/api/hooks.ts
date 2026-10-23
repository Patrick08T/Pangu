import { useEffect, useRef, useState } from 'react';
import { apiRequest } from './request';

export function useQuery<T>(path: string | null, interval = 0) {
  const [revision, refresh] = useState(0);
  const [state, setState] = useState<{ data?: T; error?: Error; loading: boolean }>({ loading: !!path });
  useEffect(() => {
    if (!path) { setState({ loading: false }); return; }
    let active = true, pending = false;
    let controller: AbortController | undefined;
    setState({ loading: true });
    const load = async () => {
      if (pending || !active) return;
      pending = true; controller = new AbortController();
      const timer = setTimeout(() => controller?.abort(), 12000);
      try {
        const data = await apiRequest<T>(path, { signal: controller.signal });
        if (active) setState({ data, loading: false });
      } catch (error) {
        if (active) setState(previous => ({ ...previous, loading: false, error: error instanceof Error ? error : new Error('请求失败') }));
      } finally { clearTimeout(timer); pending = false; }
    };
    void load();
    const timer = interval ? setInterval(() => { if (!document.hidden) void load(); }, interval) : undefined;
    return () => { active = false; controller?.abort(); clearInterval(timer); };
  }, [path, interval, revision]);
  return { ...state, reload: () => refresh(value => value + 1) };
}

export function useAction() {
  const lock = useRef(false);
  const alive = useRef(true),controller=useRef<AbortController>(undefined);
  useEffect(()=>{alive.current=true;return()=>{alive.current=false;controller.current?.abort();};},[]);
  const [state, setState] = useState<{ busy: boolean; error?: Error; message?: string }>({ busy: false });
  async function run(action: (signal:AbortSignal) => Promise<void>, message = '操作已完成') {
    if (lock.current||!alive.current) return;
    lock.current = true; setState({ busy: true });
    controller.current=new AbortController();
    try { await action(controller.current.signal); if(alive.current)setState({ busy: false, message }); }
    catch (error) { if(alive.current)setState({ busy: false, error: error instanceof Error ? error : new Error('操作失败') }); }
    finally { lock.current = false; }
  }
  return { ...state, run, active:()=>alive.current, navigate:(path:string)=>{if(alive.current)window.location.hash=path;} };
}
