import type { SyncTableName } from "@/model/tables";

export type SyncRecord = Record<string, unknown>;

export type SyncTableChanges = {
  created: SyncRecord[];
  updated: SyncRecord[];
  deleted: string[];
};

export type SyncChanges = Record<SyncTableName, SyncTableChanges>;

export type RawSyncChanges = Partial<
  Record<SyncTableName, Partial<SyncTableChanges>>
>;

export type SyncContext = {
  userId: string;
  spaceId: string;
};

export type PullChangesResponse = {
  changes?: RawSyncChanges;
  timestamp?: number;
};

export type PushChangesRequest = {
  last_pulled_at: number;
  changes: SyncChanges;
};

export type PhotoUploadResponse = {
  remote_url?: string;
};
