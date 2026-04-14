import type { SyncTableName } from "@/model/tables";

export type SyncRecord = Record<string, unknown>;

export type SyncTableChanges = {
  created: SyncRecord[];
  updated: SyncRecord[];
  deleted: string[];
};

export type SyncChanges = Record<SyncTableName, SyncTableChanges>;
