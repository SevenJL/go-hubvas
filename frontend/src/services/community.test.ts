import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('./api', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    postIdempotent: vi.fn(),
    delete: vi.fn(),
  },
}));

import { api } from './api';
import { communityService } from './community';

const mockedPostIdempotent = vi.mocked(api.postIdempotent);

describe('communityService.postComment', () => {
  beforeEach(() => vi.clearAllMocks());

  it('preserves the HTTP status for stale reply recovery', async () => {
    mockedPostIdempotent.mockResolvedValueOnce({ code: 404, message: 'parent comment not found' });

    const request = communityService.postComment('7', 'reply', '999');
    await expect(request).rejects.toMatchObject({
      name: 'CommunityRequestError',
      message: 'parent comment not found',
      status: 404,
    });
  });
});
