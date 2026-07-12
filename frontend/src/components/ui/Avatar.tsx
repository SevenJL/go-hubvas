import { useEffect, useState } from 'react';
interface AvatarProps { name:string; src?:string; size?:'sm'|'md'|'lg'|'xl'; className?:string }
const sizeClasses={sm:'w-7 h-7 text-[10px]',md:'w-9 h-9 text-xs',lg:'w-12 h-12 text-sm',xl:'w-24 h-24 text-2xl'};
export function Avatar({name,src,size='md',className=''}:AvatarProps){
 const [failed,setFailed]=useState(false); useEffect(()=>setFailed(false),[src]);
 const initials=(name||'?').split(/\s+/).map(w=>w[0]).join('').toUpperCase().slice(0,2);
 if(src&&!failed)return <img src={src} alt={name} onError={()=>setFailed(true)} className={`${sizeClasses[size]} rounded-full object-cover bg-slate-100 ring-1 ring-slate-200 ${className}`}/>;
 return <div title={name} className={`${sizeClasses[size]} rounded-full bg-gradient-to-br from-indigo-100 to-violet-100 flex items-center justify-center font-semibold text-indigo-700 ring-1 ring-indigo-200 ${className}`}>{initials||'?'}</div>;
}
