import type { ApiResponse } from '../types';
const BASE_URL = '/api';
let accessToken: string | null = null;
// Remove credentials written by older builds. Refresh sessions now live only in an HttpOnly cookie.
localStorage.removeItem('access_token');
localStorage.removeItem('refresh_token');
export function setTokens(access: string, legacyRefresh?: string) { accessToken = access; void legacyRefresh }
export function clearTokens() { accessToken = null; localStorage.removeItem('access_token'); localStorage.removeItem('refresh_token') }
export function getAccessToken() { return accessToken }
let refreshPromise: Promise<boolean> | null = null;
async function tryRefreshToken() {
  if (refreshPromise) return refreshPromise;
  refreshPromise = (async () => { try {
    const res = await fetch(`${BASE_URL}/auth/refresh`, { method:'POST', credentials:'same-origin' });
    const body: ApiResponse<{access_token:string}> = await res.json();
    if (!res.ok || body.code !== 0 || !body.data) { clearTokens(); return false }
    setTokens(body.data.access_token); return true;
  } catch { return false } finally { refreshPromise = null } })();
  return refreshPromise;
}
async function perform(path: string, init: RequestInit, retry = true): Promise<Response> {
  const headers = new Headers(init.headers);
  if (accessToken) headers.set('Authorization', `Bearer ${accessToken}`);
  const res = await fetch(path.startsWith('http') ? path : `${BASE_URL}${path}`, {...init, headers, credentials:'same-origin'});
  if (res.status === 401 && retry && !path.endsWith('/auth/refresh') && await tryRefreshToken()) return perform(path, init, false);
  return res;
}
async function request<T>(method:string,path:string,body?:unknown,extraHeaders?:Record<string,string>): Promise<ApiResponse<T>> {
  const res = await perform(path,{method,headers:{'Content-Type':'application/json',...extraHeaders},body:body === undefined ? undefined : JSON.stringify(body)});
  try { return await res.json() } catch { return {code:res.status||500,message:`Request failed (${res.status})`} }
}
function idempotencyKey() {
  return globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`;
}
export async function uploadForm<T>(path:string, form:FormData):Promise<ApiResponse<T>> { const res=await perform(path,{method:'POST',headers:{'Idempotency-Key':idempotencyKey()},body:form}); try{return await res.json()}catch{return {code:res.status||500,message:`Request failed (${res.status})`}} }
export function putFile(url:string,file:File,headers:Record<string,string>,onProgress:(n:number)=>void):Promise<void>{
  return new Promise((resolve,reject)=>{const xhr=new XMLHttpRequest();xhr.open('PUT',url);Object.entries(headers).forEach(([k,v])=>xhr.setRequestHeader(k,v));xhr.upload.onprogress=e=>e.lengthComputable&&onProgress(Math.round(e.loaded/e.total*100));xhr.onload=()=>xhr.status>=200&&xhr.status<300?resolve():reject(new Error(`Upload failed (${xhr.status})`));xhr.onerror=()=>reject(new Error('Upload failed'));xhr.send(file)})
}
export const api={get:<T>(p:string)=>request<T>('GET',p),post:<T>(p:string,b?:unknown)=>request<T>('POST',p,b),postIdempotent:<T>(p:string,b?:unknown)=>request<T>('POST',p,b,{'Idempotency-Key':idempotencyKey()}),put:<T>(p:string,b?:unknown)=>request<T>('PUT',p,b),patch:<T>(p:string,b?:unknown)=>request<T>('PATCH',p,b),patchIdempotent:<T>(p:string,b?:unknown)=>request<T>('PATCH',p,b,{'Idempotency-Key':idempotencyKey()}),delete:<T>(p:string)=>request<T>('DELETE',p)};
