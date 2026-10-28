import {useEffect,useRef,useState} from 'react';

function decimal(text:string):string {
  const match=/^(-?)(0|[1-9]\d*)(?:\.(\d+))?(?:[eE]([+-]?\d+))?$/.exec(text);
  if(!match||text.length>256)return '';
  const digits=(match[2]+(match[3]??'')).replace(/^0+/,'');
  if(!digits)return '0';
  const trimmed=digits.replace(/0+$/,'');
  return match[1]+trimmed+'e'+(BigInt(match[4]??'0')-BigInt((match[3]??'').length)+BigInt(digits.length-trimmed.length));
}

export function exactNumber(text:string):boolean {
  const value=Number(text);
  return !!decimal(text)&&Number.isFinite(value)&&decimal(text)===decimal(String(value));
}

export function preciseJSON(text:string):boolean {
  return [...text.matchAll(/"(?:\\.|[^"\\])*"|(-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?)/g)].every(match=>!match[1]||exactNumber(match[1]));
}

export function NumberField({value,change,label,integer,required,disabled}:{value:unknown;change:(value:unknown)=>void;label:string;integer:boolean;required:boolean;disabled:boolean}) {
  const [raw,setRaw]=useState(value===undefined?'':String(value)),last=useRef(value);
  useEffect(()=>{if(value!==last.current){setRaw(value===undefined?'':String(value));last.current=value;}},[value]);
  const error=raw!==''&&(!exactNumber(raw)||(integer&&!Number.isInteger(Number(raw))))?'数字精度或类型不符合要求，请填写可精确表示的数字。':'';
  function update(text:string){
    setRaw(text);
    if(text===''&&required||text!==''&&(!exactNumber(text)||(integer&&!Number.isInteger(Number(text)))))return;
    const next=text===''?undefined:Number(text);
    last.current=next;change(next);
  }
  return <><label className="field">{label}<input inputMode="decimal" required={required} disabled={disabled} value={raw} onChange={e=>update(e.target.value)} ref={element=>{element?.setCustomValidity(error);}}/></label>{error&&<p role="alert">{error}</p>}</>;
}
