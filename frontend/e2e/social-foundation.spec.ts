import { expect, test, type Page, type Route } from '@playwright/test';

const ok = (data: unknown) => ({ code: 0, message: 'ok', data });
const user = {
  id: '1', username: 'owner', email: 'owner@example.com', display_name: 'Owner', bio: '', website: '', avatar_url: '',
  account_role: 'admin' as const, status: 'active' as const, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z',
};

async function authenticate(page: Page) {
  await page.addInitScript(() => {
    localStorage.setItem('access_token', 'test-token');
    localStorage.setItem('refresh_token', 'test-refresh');
  });
}

async function json(route: Route, data: unknown, status = 200) {
  await route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(ok(data)) });
}

async function installCommonRoutes(page: Page, currentUser = user) {
  await page.route('**/api/auth/me', route => json(route, currentUser));
  await page.route('**/api/notifications/unread-count', route => json(route, { count: 0 }));
  await page.route('**/api/auth/refresh', route => route.fulfill({ status: 401, contentType: 'application/json', body: JSON.stringify({ code: 401, message: 'expired' }) }));
}

test('uploads an avatar through presigned storage and edits profile fields', async ({ page }) => {
  await authenticate(page);
  let avatarUpdated = false;
  let profileUpdated = false;
  await installCommonRoutes(page);
  await page.unroute('**/api/auth/me');
  await page.route('**/api/auth/me', route => json(route, { ...user, avatar_url: avatarUpdated ? '/avatars/1/2.webp' : '', display_name: profileUpdated ? 'Production Owner' : 'Owner' }));
  await page.route('**/api/media/avatars/presign', route => json(route, { upload_id: 'upload-1', upload_url: 'https://storage.test/avatar', expires_at: '2026-07-12T12:00:00Z', headers: { 'Content-Type': 'image/png' } }));
  await page.route('https://storage.test/avatar', route => route.fulfill({ status: 200 }));
  await page.route('**/api/media/avatars/complete', route => { avatarUpdated = true; return json(route, { avatar_url: '/avatars/1/2.webp', avatar_version: 2 }); });
  await page.route('**/api/auth/profile', async route => { profileUpdated = true; await json(route, { ...user, display_name: 'Production Owner' }); });

  await page.goto('/profile');
  await expect(page.getByRole('heading', { name: 'Profile settings' })).toBeVisible();
  await page.locator('input[type=file]').setInputFiles({
    name: 'avatar.png', mimeType: 'image/png',
    buffer: Buffer.from('iVBORw0KGgoAAAANSUhEUgAAACAAAAAgCAIAAAD8GO2jAAAAO0lEQVR4nO3RQREAMAjEwKNaK6KykVAJ4cMvK+CYCXVfZ9NZXY8HBvwBMhEyETIRMhEyETIRMhEyUcgHVtUCDMm1K3cAAAAASUVORK5CYII=', 'base64'),
  });
  await expect(page.getByText('Crop avatar')).toBeVisible();
  await page.getByRole('button', { name: 'Upload avatar' }).click();
  await expect(page.getByText('Avatar updated')).toBeVisible();

  await page.getByLabel('Display name').fill('Production Owner');
  await page.getByLabel('Bio').fill('A production-ready profile');
  await page.getByLabel('Website').fill('https://example.com');
  await page.getByRole('button', { name: 'Save profile' }).click();
  await expect(page.getByText('Profile updated')).toBeVisible();
  expect(profileUpdated).toBe(true);
});

test('follows and blocks a public profile, then opens following feed', async ({ page }) => {
  await authenticate(page);
  await installCommonRoutes(page);
  let following = false;
  let blocked = false;
  const profile = () => ({
    id: '2', username: 'alice', display_name: 'Alice', avatar_url: '', bio: 'Creator', website: '', published_count: 1,
    followers_count: following ? 2 : 1, following_count: 0, is_following: following, is_blocked: blocked, is_blocked_by: false, joined_at: '2026-01-01T00:00:00Z',
  });
  await page.route('**/api/users/alice', route => json(route, profile()));
  await page.route('**/api/users/alice/canvases*', route => json(route, { items: [], total_count: 0, page: 1, page_size: 20 }));
  await page.route('**/api/users/2/follow', async route => { following = route.request().method() === 'POST'; await json(route, { following }); });
  await page.route('**/api/users/2/block', async route => { blocked = route.request().method() === 'POST'; if (blocked) following = false; await json(route, { blocked }); });
  await page.route('**/api/community/following*', route => json(route, { items: [{ canvas_id: '10', author_id: '2', author_name: 'Alice', author_username: 'alice', author_avatar_url: '', title: 'Followed work', snapshot_url: '', tags: [], like_count: 0, is_liked: false, comment_count: 0, fork_count: 0, published_at: 1_700_000_000 }], total_count: 1, page: 1, page_size: 20 }));

  await page.goto('/users/alice');
  await page.getByRole('button', { name: 'Follow', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Unfollow', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Block', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Unblock', exact: true })).toBeVisible();

  await page.goto('/community');
  await page.getByRole('button', { name: 'Following', exact: true }).click();
  await expect(page.getByText('Followed work')).toBeVisible();
});

test('replies to a comment and marks a notification read', async ({ page }) => {
  await authenticate(page);
  await installCommonRoutes(page);
  const root = { id: '5', canvas_id: '10', author_id: '2', author_name: 'Alice', author_username: 'alice', author_avatar_url: '', content: 'Root comment', deleted: false, moderation_status: 'visible', created_at: 1_700_000_000 };
  await page.route('**/api/canvases/10', route => json(route, { id: '10', owner_id: '2', title: 'Discussion', visibility: 'published', member_count: 1, online_count: 0, created_at: '', updated_at: '' }));
  await page.route('**/api/community/10', route => json(route, { canvas_id: '10', author_id: '2', author_name: 'Alice', author_username: 'alice', author_avatar_url: '', title: 'Discussion', snapshot_url: '', tags: [], like_count: 0, is_liked: false, comment_count: 1, fork_count: 0, published_at: 1_700_000_000 }));
  await page.route('**/api/canvases/10/comments*', async route => {
    if (route.request().method() === 'POST') {
      const body = route.request().postDataJSON();
      await json(route, { ...root, id: '6', author_id: '1', author_name: 'Owner', author_username: 'owner', parent_comment_id: body.parent_comment_id, content: body.content });
    } else await json(route, { items: [root], total: 1, page: 1, page_size: 20 });
  });
  await page.route('**/api/canvases/10/like-status', route => json(route, { liked: false, like_count: 0 }));
  await page.route('**/api/canvases/10/snapshot', route => json(route, { data: null }));

  let notificationRead = false;
  await page.route('**/api/notifications?page=1', route => json(route, { items: [{ id: '9', event_type: 'comment', actor: { id: '2', username: 'alice', display_name: 'Alice', avatar_url: '' }, target_type: 'comment', target_id: '5', data: { canvas_id: '10' }, created_at: '2026-07-12T00:00:00Z' }], total: 1, page: 1, page_size: 20 }));
  await page.route('**/api/notifications/9/read', async route => { notificationRead = true; await json(route, { read: true }); });

  await page.goto('/canvas/10');
  await page.getByRole('button', { name: 'Reply' }).click();
  await expect(page.getByText('Replying to a comment')).toBeVisible();
  await page.getByPlaceholder('Write a comment...').fill('@alice Thanks');
  await page.getByRole('button', { name: 'Post comment' }).click();
  await expect(page.getByText('@alice Thanks')).toBeVisible();

  await page.goto('/notifications');
  await page.getByText('commented on your canvas').click();
  await expect.poll(() => notificationRead).toBe(true);
});

test('administrator applies target moderation and resolves a report', async ({ page }) => {
  await authenticate(page);
  await installCommonRoutes(page);
  const report = { id: '4', reporter_id: '1', target_type: 'comment', target_id: '5', reason: 'harassment', details: 'abuse', status: 'pending', review_note: '', created_at: '2026-07-12T00:00:00Z' };
  let hidden = false;
  let resolved = false;
  await page.route('**/api/admin/reports?status=pending', route => json(route, { items: resolved ? [] : [report], total: resolved ? 0 : 1, page: 1, page_size: 20 }));
  await page.route('**/api/admin/comments/5/moderation', async route => { hidden = route.request().postDataJSON().status === 'hidden'; await json(route, { status: 'hidden' }); });
  await page.route('**/api/admin/reports/4', async route => { resolved = route.request().postDataJSON().status === 'resolved'; await json(route, { ...report, status: 'resolved' }); });
  page.on('dialog', dialog => dialog.accept('handled'));

  await page.goto('/admin');
  await page.getByRole('button', { name: 'Hide comment' }).click();
  await expect.poll(() => hidden).toBe(true);
  await page.getByRole('button', { name: 'Resolve', exact: true }).click();
  await expect(page.getByText('No reports in this queue.')).toBeVisible();
  expect(resolved).toBe(true);
});
