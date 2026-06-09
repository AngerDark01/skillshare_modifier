import { QueryClient } from '@tanstack/react-query';
import { describe, expect, it } from 'vitest';
import { getCachedAuditResult, isAuditCacheFresh } from './auditCache';
import { queryKeys } from './queryKeys';
import type { AuditAllResponse } from '../api/client';

describe('auditCache', () => {
  it('treats invalidated cache as stale', () => {
    expect(isAuditCacheFresh({ dataUpdatedAt: Date.now(), isInvalidated: true })).toBe(false);
  });

  it('ignores cached audit results when installed counts no longer match', () => {
    const queryClient = new QueryClient();
    const payload: AuditAllResponse = {
      results: [],
      summary: {
        total: 1,
        passed: 1,
        warning: 0,
        failed: 0,
        critical: 0,
        high: 0,
        medium: 0,
        low: 0,
        info: 0,
        threshold: 'HIGH',
        riskScore: 0,
        riskLabel: 'clean',
      },
    };

    queryClient.setQueryData(queryKeys.audit.all('skills'), payload);

    expect(getCachedAuditResult(queryClient, 'skills', 0)).toBeNull();
    expect(getCachedAuditResult(queryClient, 'skills', 1)).toEqual(payload);
  });
});
