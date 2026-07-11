import { useEffect, useState } from 'react';
import { Trash2, UserPlus } from 'lucide-react';
import { canvasService } from '../../services/canvas';
import { Avatar, Button, ConfirmDialog, InlineLoader, Modal, useToast } from '../ui';
import type { CanvasMember } from '../../types';
import { useI18n } from '../../i18n';

type AssignableRole = Exclude<CanvasMember['role'], 'owner'>;
const roles: AssignableRole[] = ['editor', 'viewer', 'commenter'];

export function MemberManager({ canvasId, onClose }: { canvasId: string; onClose: () => void }) {
  const toast = useToast();
  const { t } = useI18n();
  const [members, setMembers] = useState<CanvasMember[]>([]);
  const [username, setUsername] = useState('');
  const [role, setRole] = useState<AssignableRole>('editor');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [removeTarget, setRemoveTarget] = useState<CanvasMember | null>(null);
  const [removing, setRemoving] = useState(false);

  useEffect(() => {
    canvasService.listMembers(canvasId)
      .then(setMembers)
      .catch(err => toast.error({ title: t('Could not load members'), message: err instanceof Error ? err.message : t('Please try again.') }))
      .finally(() => setLoading(false));
  }, [canvasId, t, toast]);

  const addMember = async () => {
    if (!username.trim() || saving) return;
    setSaving(true);
    try {
      const member = await canvasService.addMember(canvasId, username.trim(), role);
      setMembers(prev => [...prev.filter(item => item.user_id !== member.user_id), member]);
      setUsername('');
      toast.success({ title: t('Member invited'), message: t('{name} can now access this canvas.', { name: member.username || username.trim() }) });
    } catch (err) {
      toast.error({ title: t('Invitation failed'), message: err instanceof Error ? err.message : t('Please try again.') });
    } finally {
      setSaving(false);
    }
  };

  const updateRole = async (member: CanvasMember, nextRole: AssignableRole) => {
    try {
      const updated = await canvasService.updateMemberRole(canvasId, member.user_id, nextRole);
      setMembers(prev => prev.map(item => item.user_id === updated.user_id ? updated : item));
      toast.success({ title: t('Role updated'), message: t('{name} is now {role}.', { name: updated.username || t('User {id}', { id: updated.user_id }), role: t(updated.role) }) });
    } catch (err) {
      toast.error({ title: t('Role update failed'), message: err instanceof Error ? err.message : t('Please try again.') });
    }
  };

  const removeMember = async () => {
    if (!removeTarget) return;
    setRemoving(true);
    try {
      await canvasService.removeMember(canvasId, removeTarget.user_id);
      setMembers(prev => prev.filter(item => item.user_id !== removeTarget.user_id));
      toast.success({ title: t('Member removed'), message: t('{name} no longer has access.', { name: removeTarget.username || t('User {id}', { id: removeTarget.user_id }) }) });
      setRemoveTarget(null);
    } catch (err) {
      toast.error({ title: t('Could not remove member'), message: err instanceof Error ? err.message : t('Please try again.') });
    } finally {
      setRemoving(false);
    }
  };

  return (
    <>
      <Modal open title={t('Canvas members')} description={t('Invite registered users and control their access.')} onClose={onClose} size="md">
        <div className="mb-4 flex gap-2">
          <input className="input-field flex-1" value={username} onChange={event => setUsername(event.target.value)} onKeyDown={event => event.key === 'Enter' && void addMember()} placeholder={t('Username')} autoFocus />
          <select className="input-field w-28" value={role} onChange={event => setRole(event.target.value as AssignableRole)} aria-label={t('Member role')}>
            {roles.map(item => <option key={item} value={item}>{t(item)}</option>)}
          </select>
          <Button onClick={() => void addMember()} loading={saving} disabled={!username.trim()} title={t('Add member')}><UserPlus size={17} /></Button>
        </div>

        {loading ? (
          <InlineLoader label={t('Loading members...')} />
        ) : members.length === 0 ? (
          <div className="rounded-xl bg-slate-50 py-8 text-center text-sm text-slate-400">{t('No members yet.')}</div>
        ) : (
          <div className="max-h-80 divide-y divide-slate-100 overflow-y-auto">
            {members.map(member => (
              <div key={member.user_id} className="flex items-center gap-3 py-3">
                <Avatar name={member.username || t('User {id}', { id: member.user_id })} />
                <div className="min-w-0 flex-1">
                  <div className="truncate text-sm font-medium text-slate-900">{member.username || t('User {id}', { id: member.user_id })}</div>
                  <div className="text-[11px] text-slate-400">{t('ID {id}', { id: member.user_id })}</div>
                </div>
                {member.role === 'owner' ? (
                  <span className="rounded-full bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-600">{t('owner')}</span>
                ) : (
                  <>
                    <select className="rounded-md border border-slate-200 px-2 py-1.5 text-xs" value={member.role} onChange={event => void updateRole(member, event.target.value as AssignableRole)} aria-label={t('Role for {name}', { name: member.username || member.user_id })}>
                      {roles.map(item => <option key={item} value={item}>{t(item)}</option>)}
                    </select>
                    <button onClick={() => setRemoveTarget(member)} className="rounded-md p-1.5 text-slate-400 hover:bg-red-50 hover:text-red-500" title={t('Remove member')}><Trash2 size={15} /></button>
                  </>
                )}
              </div>
            ))}
          </div>
        )}
      </Modal>

      <ConfirmDialog
        open={Boolean(removeTarget)}
        title={t('Remove canvas member?')}
        description={removeTarget ? t('{name} will immediately lose access to this canvas.', { name: removeTarget.username || t('User {id}', { id: removeTarget.user_id }) }) : ''}
        confirmLabel={t('Remove member')}
        danger
        loading={removing}
        onClose={() => setRemoveTarget(null)}
        onConfirm={() => void removeMember()}
      />
    </>
  );
}
