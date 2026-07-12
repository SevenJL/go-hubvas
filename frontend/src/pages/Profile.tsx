import { useEffect, useRef, useState, type DragEvent, type FormEvent } from 'react';
import { Camera, Globe2, KeyRound, Laptop, ShieldCheck, Trash2, UploadCloud, X } from 'lucide-react';
import { Layout } from '../components/layout/Layout';
import { Avatar, Button, useToast } from '../components/ui';
import { useAuth } from '../store/authStore';
import { authService } from '../services/auth';
import { mediaService, type Crop } from '../services/media';
import { useI18n } from '../i18n';
import type { AuthSession } from '../types';

const MAX_AVATAR_BYTES = 5 * 1024 * 1024;
const ACCEPTED_TYPES = new Set(['image/jpeg', 'image/png', 'image/webp']);

type PendingAvatar = { file: File; previewURL: string; width: number; height: number };

async function imageDimensions(file: File) {
  if ('createImageBitmap' in window) {
    const bitmap = await createImageBitmap(file);
    const dimensions = { width: bitmap.width, height: bitmap.height };
    bitmap.close();
    return dimensions;
  }
  const url = URL.createObjectURL(file);
  try {
    return await new Promise<{ width: number; height: number }>((resolve, reject) => {
      const image = new Image();
      image.onload = () => resolve({ width: image.naturalWidth, height: image.naturalHeight });
      image.onerror = () => reject(new Error('Invalid image'));
      image.src = url;
    });
  } finally {
    URL.revokeObjectURL(url);
  }
}

export function Profile() {
  const { user, setUser, logout } = useAuth();
  const { t } = useI18n();
  const toast = useToast();
  const input = useRef<HTMLInputElement>(null);
  const [displayName, setDisplayName] = useState(user?.display_name || user?.username || '');
  const [bio, setBio] = useState(user?.bio || '');
  const [website, setWebsite] = useState(user?.website || '');
  const [saving, setSaving] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [removing, setRemoving] = useState(false);
  const [progress, setProgress] = useState(0);
  const [pending, setPending] = useState<PendingAvatar | null>(null);
  const [cropX, setCropX] = useState(50);
  const [cropY, setCropY] = useState(50);
  const [zoom, setZoom] = useState(1);
  const [sessions, setSessions] = useState<AuthSession[]>([]);
  const [sessionsLoading, setSessionsLoading] = useState(true);
  const [revokingSession, setRevokingSession] = useState<string | null>(null);
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [changingPassword, setChangingPassword] = useState(false);

  useEffect(() => () => {
    if (pending) URL.revokeObjectURL(pending.previewURL);
  }, [pending]);

  useEffect(() => {
    let cancelled = false;
    authService.sessions()
      .then(items => { if (!cancelled) setSessions(items); })
      .catch(error => { if (!cancelled) toast.error(error instanceof Error ? error.message : t('Could not load sessions')); })
      .finally(() => { if (!cancelled) setSessionsLoading(false); });
    return () => { cancelled = true; };
  }, [t, toast]);

  if (!user) return <Layout><div className="mx-auto max-w-3xl p-8">{t('Please log in first.')}</div></Layout>;

  const closeCrop = () => {
    setPending(current => {
      if (current) URL.revokeObjectURL(current.previewURL);
      return null;
    });
    setProgress(0);
    if (input.current) input.current.value = '';
  };

  const selectFile = async (file?: File) => {
    if (!file) return;
    if (!ACCEPTED_TYPES.has(file.type)) {
      toast.error(t('Choose a JPEG, PNG or WebP image.'));
      return;
    }
    if (file.size > MAX_AVATAR_BYTES) {
      toast.error(t('Avatar must be smaller than 5 MB'));
      return;
    }
    try {
      const dimensions = await imageDimensions(file);
      const previewURL = URL.createObjectURL(file);
      setPending(current => {
        if (current) URL.revokeObjectURL(current.previewURL);
        return { file, previewURL, ...dimensions };
      });
      setCropX(50);
      setCropY(50);
      setZoom(1);
      setProgress(0);
    } catch {
      toast.error(t('The selected file is not a valid image.'));
    }
  };

  const cropFor = ({ width, height }: PendingAvatar): Crop => {
    const side = Math.min(width, height) / zoom;
    const maxX = width - side;
    const maxY = height - side;
    return {
      x: maxX === 0 ? 0 : (maxX * cropX / 100) / width,
      y: maxY === 0 ? 0 : (maxY * cropY / 100) / height,
      width: side / width,
      height: side / height,
    };
  };

  const upload = async () => {
    if (!pending || uploading) return;
    setUploading(true);
    setProgress(0);
    try {
      await mediaService.uploadAvatar(pending.file, cropFor(pending), setProgress);
      const fresh = await authService.me();
      setUser(fresh);
      toast.success(t('Avatar updated'));
      closeCrop();
    } catch (error) {
      toast.error({ title: t('Upload failed'), message: error instanceof Error ? error.message : t('Please retry') });
    } finally {
      setUploading(false);
    }
  };

  const drop = (event: DragEvent) => {
    event.preventDefault();
    void selectFile(event.dataTransfer.files[0]);
  };

  const save = async (event: FormEvent) => {
    event.preventDefault();
    setSaving(true);
    try {
      const updated = await authService.updateProfile({ display_name: displayName, bio, website });
      setUser(updated);
      toast.success(t('Profile updated'));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Update failed'));
    } finally {
      setSaving(false);
    }
  };

  const remove = async () => {
    setRemoving(true);
    try {
      await mediaService.remove();
      const fresh = await authService.me();
      setUser(fresh);
      toast.success(t('Avatar removed'));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Could not remove avatar'));
    } finally {
      setRemoving(false);
    }
  };

  const revokeSession = async (id: string, current: boolean) => {
    setRevokingSession(id);
    try {
      await authService.revokeSession(id);
      if (current) {
        logout();
        window.location.assign('/login');
        return;
      }
      setSessions(items => items.map(item => item.id === id ? { ...item, revoked_at: new Date().toISOString() } : item));
      toast.success(t('Session revoked'));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Could not revoke session'));
    } finally {
      setRevokingSession(null);
    }
  };

  const changePassword = async (event: FormEvent) => {
    event.preventDefault();
    if (newPassword !== confirmPassword) {
      toast.error(t('New passwords do not match'));
      return;
    }
    setChangingPassword(true);
    try {
      await authService.changePassword(currentPassword, newPassword);
      toast.success(t('Password changed. Please sign in again.'));
      logout();
      window.location.assign('/login');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Could not change password'));
    } finally {
      setChangingPassword(false);
    }
  };

  return <Layout>
    <main className="mx-auto max-w-5xl px-4 py-10">
      <div className="mb-8">
        <p className="text-sm font-medium text-indigo-600">{t('ACCOUNT')}</p>
        <h1 className="mt-1 text-3xl font-bold text-slate-950">{t('Profile settings')}</h1>
        <p className="mt-2 text-slate-500">{t('Manage how people recognize you across Hubvas.')}</p>
      </div>
      <div className="grid gap-8 lg:grid-cols-[300px_1fr]">
        <section className="rounded-3xl border border-slate-200 bg-slate-950 p-6 text-white">
          <div className="flex flex-col items-center text-center">
            <Avatar size="xl" name={displayName || user.username} src={user.avatar_url} />
            <h2 className="mt-4 text-xl font-semibold">{displayName || user.username}</h2>
            <p className="text-sm text-slate-400">@{user.username}</p>
          </div>
          <div onDragOver={event => event.preventDefault()} onDrop={drop} onClick={() => input.current?.click()} className="mt-7 cursor-pointer rounded-2xl border border-dashed border-slate-600 p-5 text-center transition hover:border-indigo-400">
            <UploadCloud className="mx-auto mb-2" />
            <p className="text-sm font-medium">{t('Drop an image or choose a file')}</p>
            <p className="mt-1 text-xs text-slate-400">{t('JPEG, PNG or WebP · max 5 MB')}</p>
          </div>
          <input ref={input} hidden type="file" accept="image/jpeg,image/png,image/webp" onChange={event => void selectFile(event.target.files?.[0])} />
          <div className="mt-3 flex gap-2">
            <Button className="flex-1" onClick={() => input.current?.click()} disabled={uploading}><Camera size={15} /> {t('Change')}</Button>
            {user.avatar_url && <Button variant="secondary" onClick={() => void remove()} loading={removing} title={t('Remove avatar')}><Trash2 size={15} /></Button>}
          </div>
        </section>

        <form onSubmit={save} className="rounded-3xl border border-slate-200 bg-white p-7 shadow-sm">
          <div className="grid gap-6">
            <label className="block"><span className="text-sm font-semibold text-slate-800">{t('Username')}</span><input disabled value={user.username} className="mt-2 w-full rounded-xl border border-slate-200 bg-slate-50 px-4 py-3 text-slate-500" /><span className="mt-1 block text-xs text-slate-400">{t('Your permanent account identifier.')}</span></label>
            <label><span className="text-sm font-semibold text-slate-800">{t('Display name')}</span><input required maxLength={50} value={displayName} onChange={event => setDisplayName(event.target.value)} className="mt-2 w-full rounded-xl border border-slate-300 px-4 py-3 focus:border-indigo-500 focus:outline-none" /></label>
            <label><span className="text-sm font-semibold text-slate-800">{t('Bio')}</span><textarea maxLength={500} rows={5} value={bio} onChange={event => setBio(event.target.value)} className="mt-2 w-full resize-none rounded-xl border border-slate-300 px-4 py-3 focus:border-indigo-500 focus:outline-none" /><span className="float-right text-xs text-slate-400">{bio.length}/500</span></label>
            <label><span className="flex items-center gap-2 text-sm font-semibold text-slate-800"><Globe2 size={15} /> {t('Website')}</span><input type="url" placeholder="https://" value={website} onChange={event => setWebsite(event.target.value)} className="mt-2 w-full rounded-xl border border-slate-300 px-4 py-3 focus:border-indigo-500 focus:outline-none" /></label>
            <div className="flex justify-end"><Button type="submit" loading={saving}>{t('Save profile')}</Button></div>
          </div>
        </form>
      </div>

      <section className="mt-8 rounded-3xl border border-slate-200 bg-white p-6 shadow-sm sm:p-7">
        <div className="flex items-start gap-3">
          <span className="rounded-xl bg-indigo-50 p-2.5 text-indigo-600"><ShieldCheck size={22} /></span>
          <div><h2 className="text-xl font-bold text-slate-950">{t('Account security')}</h2><p className="mt-1 text-sm text-slate-500">{t('Manage your password and signed-in devices.')}</p></div>
        </div>
        <div className="mt-7 grid gap-8 lg:grid-cols-2">
          <form onSubmit={changePassword} className="rounded-2xl border border-slate-200 p-5">
            <h3 className="flex items-center gap-2 font-semibold text-slate-900"><KeyRound size={17} />{t('Change password')}</h3>
            <div className="mt-5 grid gap-4">
              <label className="text-sm font-medium text-slate-700">{t('Current password')}<input required type="password" autoComplete="current-password" value={currentPassword} onChange={event => setCurrentPassword(event.target.value)} className="mt-1.5 w-full rounded-xl border border-slate-300 px-3 py-2.5 focus:border-indigo-500 focus:outline-none" /></label>
              <label className="text-sm font-medium text-slate-700">{t('New password')}<input required minLength={8} maxLength={128} type="password" autoComplete="new-password" value={newPassword} onChange={event => setNewPassword(event.target.value)} className="mt-1.5 w-full rounded-xl border border-slate-300 px-3 py-2.5 focus:border-indigo-500 focus:outline-none" /></label>
              <label className="text-sm font-medium text-slate-700">{t('Confirm new password')}<input required minLength={8} maxLength={128} type="password" autoComplete="new-password" value={confirmPassword} onChange={event => setConfirmPassword(event.target.value)} className="mt-1.5 w-full rounded-xl border border-slate-300 px-3 py-2.5 focus:border-indigo-500 focus:outline-none" /></label>
              <p className="text-xs leading-5 text-slate-500">{t('Changing your password signs out every device, including this one.')}</p>
              <Button type="submit" loading={changingPassword}>{t('Update password')}</Button>
            </div>
          </form>

          <div className="rounded-2xl border border-slate-200 p-5">
            <h3 className="flex items-center gap-2 font-semibold text-slate-900"><Laptop size={17} />{t('Signed-in devices')}</h3>
            <p className="mt-1 text-xs text-slate-500">{t('Revoke any session you do not recognize.')}</p>
            <div className="mt-4 divide-y divide-slate-100">
              {sessionsLoading && <p className="py-5 text-sm text-slate-500">{t('Loading sessions...')}</p>}
              {!sessionsLoading && sessions.length === 0 && <p className="py-5 text-sm text-slate-500">{t('No active sessions found.')}</p>}
              {sessions.map(session => <div key={session.id} className="flex items-start justify-between gap-3 py-4 first:pt-1">
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium text-slate-800">{session.user_agent || t('Unknown device')} {session.current && <span className="ml-1 rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] text-emerald-700">{t('Current')}</span>}</p>
                  <p className="mt-1 text-xs text-slate-500">{session.ip_address || t('Unknown IP')} · {t('Last used')} {new Date(session.last_used_at).toLocaleString()}</p>
                  {session.revoked_at && <p className="mt-1 text-xs font-medium text-rose-600">{t('Revoked')}</p>}
                </div>
                {!session.revoked_at && <Button size="sm" variant={session.current ? 'danger' : 'secondary'} loading={revokingSession === session.id} onClick={() => void revokeSession(session.id, session.current)}>{t('Revoke')}</Button>}
              </div>)}
            </div>
          </div>
        </div>
      </section>
    </main>

    {pending && <div className="fixed inset-0 z-50 grid place-items-center bg-slate-950/70 p-4" role="dialog" aria-modal="true" aria-label={t('Crop avatar')}>
      <div className="w-full max-w-lg rounded-3xl bg-white p-6 shadow-2xl">
        <div className="flex items-start justify-between"><div><h2 className="text-xl font-bold text-slate-950">{t('Crop avatar')}</h2><p className="mt-1 text-sm text-slate-500">{t('Position and zoom your image inside the square.')}</p></div><button onClick={closeCrop} disabled={uploading} className="rounded-full p-2 text-slate-400 hover:bg-slate-100"><X size={20} /></button></div>
        <div className="mx-auto mt-6 h-72 w-72 overflow-hidden rounded-2xl bg-slate-100 ring-1 ring-slate-200">
          <img src={pending.previewURL} alt={t('Avatar preview')} className="h-full w-full object-cover" style={{ objectPosition: `${cropX}% ${cropY}%`, transform: `scale(${zoom})` }} />
        </div>
        <div className="mt-6 grid gap-4">
          <label className="text-sm font-medium text-slate-700">{t('Horizontal position')}<input className="mt-2 w-full accent-indigo-600" type="range" min="0" max="100" value={cropX} onChange={event => setCropX(Number(event.target.value))} /></label>
          <label className="text-sm font-medium text-slate-700">{t('Vertical position')}<input className="mt-2 w-full accent-indigo-600" type="range" min="0" max="100" value={cropY} onChange={event => setCropY(Number(event.target.value))} /></label>
          <label className="text-sm font-medium text-slate-700">{t('Zoom')}<input className="mt-2 w-full accent-indigo-600" type="range" min="1" max="2" step="0.05" value={zoom} onChange={event => setZoom(Number(event.target.value))} /></label>
        </div>
        {uploading && <div className="mt-5"><div className="mb-1 flex justify-between text-xs text-slate-500"><span>{t('Uploading and processing...')}</span><span>{progress}%</span></div><div className="h-2 overflow-hidden rounded-full bg-slate-100"><div className="h-full bg-indigo-600 transition-all" style={{ width: `${progress}%` }} /></div></div>}
        <div className="mt-6 flex justify-end gap-3"><Button variant="secondary" onClick={closeCrop} disabled={uploading}>{t('Cancel')}</Button><Button onClick={() => void upload()} loading={uploading}>{uploading ? t('Uploading...') : t('Upload avatar')}</Button></div>
      </div>
    </div>}
  </Layout>;
}
