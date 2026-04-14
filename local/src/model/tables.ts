export const META_TABLES = ["users", "spaces", "space_members"] as const;
export const CONTENT_TABLES = [
  "photos",
  "expenses",
  "comments",
  "posts",
] as const;

export const SYNC_TABLES = [...META_TABLES, ...CONTENT_TABLES] as const;

export type MetaTableName = (typeof META_TABLES)[number];
export type ContentTableName = (typeof CONTENT_TABLES)[number];
export type SyncTableName = (typeof SYNC_TABLES)[number];
