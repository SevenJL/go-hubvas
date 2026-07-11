import { useEffect, useState } from 'react';
import { X, UserPlus, Trash2 } from 'lucide-react';
import { canvasService } from '../../services/canvas';
import type { CanvasMember } from '../../types';

type AssignableRole = Exclude<CanvasMember['role'], 'owner'>;
const roles: AssignableRole[] = ['editor', 'viewer', 'commenter'];

export function MemberManager({ canvasId, onClose }: { canvasId: string; onClose: () => void }) {
  const [members, setMembers] = useState<CanvasMember[]>([]);
  const [username, setUsername] = useState('');
  const [role, setRole] = useState<AssignableRole>('editor');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    canvasService.listMembers(canvasId)
      .then(setMembers)
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to load members'))
      .finally(() => setLoading(false));
  }, [canvasId]);

  const addMember = async () => {
    if (!username.trim() || saving) return;
    setSaving(true);
    setError('');
    try {
      const member = await canvasService.addMember(canvasId, username.trim(), role);
      setMembers(prev => [...prev.filter(item => item.user_id !== member.user_id), member]);
      setUsername('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add member');
    } finally { setSaving(false); }
  };

  const updateRole = async (member: CanvasMember, nextRole: AssignableRole) => {
    setError('');
    try {
      const updated = await canvasService.updateMemberRole(canvasId, member.user_id, nextRole);
      setMembers(prev => prev.map(item => item.user_id === updated.user_id ? updated : item));
    } catch (err) { setError(err instanceof Error ? err.message : 'Failed to update role'); }
  };

  const removeMember = async (member: CanvasMember) => {
    if (!confirm(`Remove ${member.username || member.user_id} from this canvas?`)) return;
    setError('');
    try {
      await canvasService.removeMember(canvasId, member.user_id);
      setMembers(prev => prev.filter(item => item.user_id !== member.user_id));
    } catch (err) { setError(err instanceof Error ? err.message : 'Failed to remove member'); }
  };

  return (
    <div className="fixed inset-0 z-[2000] bg-black/30 flex items-center justify-center p-4" onMouseDown={onClose}>
      <div className="bg-white rounded-xl shadow-xl w-full max-w-lg p-5" onMouseDown={event => event.stopPropagation()}>
        <div className="flex items-center justify-between mb-4">
          <div>
            <h3 className="font-semibold text-gray-900">Canvas members</h3>
            <p className="text-xs text-gray-500 mt-0.5">Invite registered users and control their access.</p>
          </div>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-700"><X size={18} /></button>
        </div>

        <div className="flex gap-2 mb-4">
          <input className="input-field flex-1" value={username} onChange={event => setUsername(event.target.value)}
            onKeyDown={event => event.key === 'Enter' && void addMember()} placeholder="Username" />
          <select className="input-field w-28" value={role} onChange={event => setRole(event.target.value as AssignableRole)}>
            {roles.map(item => <option key={item} value={item}>{item}</option>)}
          </select>
          <button className="btn-primary px-3" onClick={() => void addMember()} disabled={!username.trim() || saving} title="Add member">
            <UserPlus size={17} />
          </button>
        </div>

        {error && <div className="mb-3 text-xs text-red-600 bg-red-50 rounded px-3 py-2">{error}</div>}
        {loading ? <div className="py-8 text-center text-sm text-gray-400">Loading members...</div> : (
          <div className="divide-y divide-gray-100 max-h-80 overflow-y-auto">
            {members.map(member => (
              <div key={member.user_id} className="flex items-center gap-3 py-3">
                <div className="w-8 h-8 rounded-full bg-indigo-100 text-indigo-700 flex items-center justify-center text-xs font-semibold">
                  {(member.username || member.user_id).slice(0, 2).toUpperCase()}
                </div>
                <div className="min-w-0 flex-1">
                  <div className="text-sm font-medium text-gray-900 truncate">{member.username || `User ${member.user_id}`}</div>
                  <div className="text-[11px] text-gray-400">ID {member.user_id}</div>
                </div>
                {member.role === 'owner' ? (
                  <span className="text-xs text-gray-500 px-2">owner</span>
                ) : (
                  <>
                    <select className="text-xs border border-gray-200 rounded-md px-2 py-1.5" value={member.role}
                      onChange={event => void updateRole(member, event.target.value as AssignableRole)}>
                      {roles.map(item => <option key={item} value={item}>{item}</option>)}
                    </select>
                    <button onClick={() => void removeMember(member)} className="text-gray-400 hover:text-red-500" title="Remove member">
                      <Trash2 size={15} />
                    </button>
                  </>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
