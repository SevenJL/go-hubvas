import { useEffect, useState } from 'react';
import { Trash2, UserPlus } from 'lucide-react';
import { canvasService } from '../../services/canvas';
import { Avatar, Button, ConfirmDialog, InlineLoader, Modal, useToast } from '../ui';
import type { CanvasMember } from '../../types';

type AssignableRole = Exclude<CanvasMember['role'], 'owner'>;
const roles: AssignableRole[] = ['editor', 'viewer', 'commenter'];

export function MemberManager({ canvasId, onClose }: { canvasId: string; onClose: () => void }) {
  const toast = useToast();
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
      .catch(err => toast.error({ title: 'Could not load members', message: err instanceof Error ? err.message : 'Please try again.' }))
      .finally(() => setLoading(false));
  }, [canvasId, toast]);

  const addMember = async () => {
    if (!username.trim() || saving) return;
    setSaving(true);
    try {
      const member = await canvasService.addMember(canvasId, username.trim(), role);
      setMembers(prev => [...prev.filter(item => item.user_id !== member.user_id), member]);
      setUsername('');
      toast.success({ title: 'Member invited', message: `${member.username || username.trim()} can now access this canvas.` });
    } catch (err) {
      toast.error({ title: 'Invitation failed', message: err instanceof Error ? err.message : 'Please try again.' });
    } finally {
      setSaving(false);
    }
  };

  const updateRole = async (member: CanvasMember, nextRole: AssignableRole) => {
    try {
      const updated = await canvasService.updateMemberRole(canvasId, member.user_id, nextRole);
      setMembers(prev => prev.map(item => item.user_id === updated.user_id ? updated : item));
      toast.success({ title: 'Role updated', message: `${updated.username || `User ${updated.user_id}`} is now ${updated.role}.` });
    } catch (err) {
      toast.error({ title: 'Role update failed', message: err instanceof Error ? err.message : 'Please try again.' });
    }
  };

  const removeMember = async () => {
    if (!removeTarget) return;
    setRemoving(true);
    try {
      await canvasService.removeMember(canvasId, removeTarget.user_id);
      setMembers(prev => prev.filter(item => item.user_id !== removeTarget.user_id));
      toast.success({ title: 'Member removed', message: `${removeTarget.username || `User ${removeTarget.user_id}`} no longer has access.` });
      setRemoveTarget(null);
    } catch (err) {
      toast.error({ title: 'Could not remove member', message: err instanceof Error ? err.message : 'Please try again.' });
    } finally {
      setRemoving(false);
    }
  };

  return (
    <>
      <Modal open title="Canvas members" description="Invite registered users and control their access." onClose={onClose} size="md">
        <div className="mb-4 flex gap-2">
          <input className="input-field flex-1" value={username} onChange={event => setUsername(event.target.value)} onKeyDown={event => event.key === 'Enter' && void addMember()} placeholder="Username" autoFocus />
          <select className="input-field w-28" value={role} onChange={event => setRole(event.target.value as AssignableRole)} aria-label="Member role">
            {roles.map(item => <option key={item} value={item}>{item}</option>)}
          </select>
          <Button onClick={() => void addMember()} loading={saving} disabled={!username.trim()} title="Add member"><UserPlus size={17} /></Button>
        </div>

        {loading ? (
          <InlineLoader label="Loading members..." />
        ) : members.length === 0 ? (
          <div className="rounded-xl bg-slate-50 py-8 text-center text-sm text-slate-400">No members yet.</div>
        ) : (
          <div className="max-h-80 divide-y divide-slate-100 overflow-y-auto">
            {members.map(member => (
              <div key={member.user_id} className="flex items-center gap-3 py-3">
                <Avatar name={member.username || `User ${member.user_id}`} />
                <div className="min-w-0 flex-1">
                  <div className="truncate text-sm font-medium text-slate-900">{member.username || `User ${member.user_id}`}</div>
                  <div className="text-[11px] text-slate-400">ID {member.user_id}</div>
                </div>
                {member.role === 'owner' ? (
                  <span className="rounded-full bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-600">owner</span>
                ) : (
                  <>
                    <select className="rounded-md border border-slate-200 px-2 py-1.5 text-xs" value={member.role} onChange={event => void updateRole(member, event.target.value as AssignableRole)} aria-label={`Role for ${member.username || member.user_id}`}>
                      {roles.map(item => <option key={item} value={item}>{item}</option>)}
                    </select>
                    <button onClick={() => setRemoveTarget(member)} className="rounded-md p-1.5 text-slate-400 hover:bg-red-50 hover:text-red-500" title="Remove member"><Trash2 size={15} /></button>
                  </>
                )}
              </div>
            ))}
          </div>
        )}
      </Modal>

      <ConfirmDialog
        open={Boolean(removeTarget)}
        title="Remove canvas member?"
        description={removeTarget ? `${removeTarget.username || `User ${removeTarget.user_id}`} will immediately lose access to this canvas.` : ''}
        confirmLabel="Remove member"
        danger
        loading={removing}
        onClose={() => setRemoveTarget(null)}
        onConfirm={() => void removeMember()}
      />
    </>
  );
}
