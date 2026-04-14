import { synchronize } from "@nozbe/watermelondb/sync";

import { database } from "@/model";
import { META_TABLES, SYNC_TABLES, type SyncTableName } from "@/model/tables";
import type { SyncChanges, SyncRecord, SyncTableChanges } from "@/sync/types";

// WatermelonDB sync is not re-entrant for our use case. We keep the active
// task here so repeated button taps can reuse the same in-flight sync.
let activeSyncTask: Promise<void> | null = null;

/**
 * Public sync entry for a single visible space.
 *
 * Even though pull is scoped by `spaceId`, push still sends WatermelonDB's
 * global local changes. This mirrors the current backend contract:
 * - Pull: `GET /sync?space_id=...`
 * - Push: global changes, with the server enforcing table constraints
 *
 * We also serialize sync calls here so the local database cannot be mutated by
 * multiple concurrent synchronize() executions.
 */
export async function syncSpace(spaceId: string): Promise<void> {
  if (!spaceId.trim()) {
    throw new Error("syncSpace requires a non-empty spaceId.");
  }

  if (activeSyncTask) {
    return activeSyncTask;
  }

  const syncTask = runSync(spaceId).finally(() => {
    if (activeSyncTask === syncTask) {
      activeSyncTask = null;
    }
  });

  activeSyncTask = syncTask;
  return syncTask;
}

/**
 * Runs one full WatermelonDB synchronization round.
 *
 * Implementation notes:
 * - WatermelonDB always executes pull before push
 * - `pullChanges` is scoped by the current `spaceId`
 * - `pushChanges` sends global changes because WatermelonDB generates them
 *   globally instead of filtering by `space_id`
 */
async function runSync(spaceId: string): Promise<void> {
  const apiBaseUrl = getApiBaseUrl();

  await synchronize({
    database,
    // Pull is where "space isolation" really happens in the current design.
    pullChanges: async ({ lastPulledAt, schemaVersion, migration }) => {
      const params = new URLSearchParams({
        space_id: spaceId,
        last_pulled_at: String(lastPulledAt ?? 0),
        schema_version: String(schemaVersion),
        migration: JSON.stringify(migration ?? null),
      });

      const response = await fetch(`${apiBaseUrl}/api/v1/sync?${params}`);
      if (!response.ok) {
        throw new Error(await buildSyncErrorMessage("pull", response));
      }

      // During integration the backend may omit empty tables, so we normalize
      // the payload into a complete 7-table changes object before returning it
      // to WatermelonDB.
      const payload = (await response.json()) as {
        changes?: Partial<Record<SyncTableName, Partial<SyncTableChanges>>>;
        timestamp?: number;
      };

      if (typeof payload.timestamp !== "number") {
        throw new Error(
          "Sync pull response does not contain a valid timestamp.",
        );
      }

      return {
        changes: normalizeChanges(payload.changes),
        timestamp: payload.timestamp,
      };
    },
    // Push keeps WatermelonDB's default global behavior. We only sanitize the
    // payload to enforce project-specific rules before sending it out.
    pushChanges: async ({ changes, lastPulledAt }) => {
      const sanitizedChanges = sanitizePushChanges(
        changes as Partial<Record<SyncTableName, Partial<SyncTableChanges>>>,
      );

      const response = await fetch(
        `${apiBaseUrl}/api/v1/sync?last_pulled_at=${String(lastPulledAt ?? 0)}`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify(sanitizedChanges),
        },
      );

      if (!response.ok) {
        throw new Error(await buildSyncErrorMessage("push", response));
      }
    },
    migrationsEnabledAtVersion: 1,
  });
}

/**
 * Reads and normalizes the backend base URL from Expo env vars.
 *
 * The trailing slash is removed so route concatenation stays stable regardless
 * of whether `.env` uses `http://host` or `http://host/`.
 */
function getApiBaseUrl(): string {
  const apiUrl = process.env.EXPO_PUBLIC_API_URL?.trim();
  if (!apiUrl) {
    throw new Error("EXPO_PUBLIC_API_URL is not configured.");
  }

  return apiUrl.replace(/\/+$/, "");
}

/**
 * Builds an empty `changes` object that contains every sync table.
 *
 * WatermelonDB and our backend are both easier to integrate when the payload
 * shape is stable, so we always start from a complete object instead of
 * conditionally constructing table entries later.
 */
function createEmptyChanges(): SyncChanges {
  return Object.fromEntries(
    SYNC_TABLES.map((tableName) => [
      tableName,
      { created: [], updated: [], deleted: [] },
    ]),
  ) as unknown as SyncChanges;
}

/**
 * Normalizes a pull response into the exact shape expected by WatermelonDB.
 *
 * Why this exists:
 * - the backend may omit empty tables while still being logically correct
 * - malformed or partial payloads should degrade to empty arrays instead of
 *   crashing inside `synchronize()`
 *
 * This function is intentionally structural, not business-aware. It only makes
 * sure that every table has `created`, `updated` and `deleted` arrays.
 */
function normalizeChanges(
  rawChanges?: Partial<Record<SyncTableName, Partial<SyncTableChanges>>>,
): SyncChanges {
  const normalizedChanges = createEmptyChanges();

  for (const tableName of SYNC_TABLES) {
    const tableChanges = rawChanges?.[tableName];
    normalizedChanges[tableName] = {
      created: pickRecords(tableChanges?.created),
      updated: pickRecords(tableChanges?.updated),
      deleted: pickIds(tableChanges?.deleted),
    };
  }

  return normalizedChanges;
}

/**
 * Sanitizes WatermelonDB's global push payload before it is sent to the server.
 *
 * The project has one important rule here:
 * - `users`, `spaces`, `space_members` must not participate in delete sync
 *
 * WatermelonDB itself does not know that rule, so we defensively drop delete
 * entries for meta tables on the client side before the request is sent.
 */
function sanitizePushChanges(
  rawChanges?: Partial<Record<SyncTableName, Partial<SyncTableChanges>>>,
): SyncChanges {
  const sanitizedChanges = createEmptyChanges();

  for (const tableName of SYNC_TABLES) {
    const tableChanges = rawChanges?.[tableName];
    const isMetaTable = isMetaTableName(tableName);

    sanitizedChanges[tableName] = {
      created: pickRecords(tableChanges?.created),
      updated: pickRecords(tableChanges?.updated),
      // Meta tables do not participate in delete sync for this project, so we
      // drop accidental deletes defensively before sending them to the server.
      deleted: isMetaTable ? [] : pickIds(tableChanges?.deleted),
    };
  }

  return sanitizedChanges;
}

/**
 * Accepts only object-like records from a candidate `created` / `updated` list.
 *
 * This keeps the sync payload tolerant to partially malformed backend data
 * without trying to perform full domain validation in the client.
 */
function pickRecords(records: unknown): SyncRecord[] {
  if (!Array.isArray(records)) {
    return [];
  }

  return records.filter(isSyncRecord);
}

/**
 * Accepts only string ids from a candidate `deleted` list.
 *
 * In WatermelonDB sync protocol, delete payloads are arrays of record ids
 * rather than full objects.
 */
function pickIds(ids: unknown): string[] {
  if (!Array.isArray(ids)) {
    return [];
  }

  return ids.filter((value): value is string => typeof value === "string");
}

/**
 * Lightweight structural guard for sync records.
 *
 * We only need to distinguish "plain object-like record" from other JSON-ish
 * values here. Business-level field validation belongs closer to API/domain
 * boundaries, not this transport normalization layer.
 */
function isSyncRecord(value: unknown): value is SyncRecord {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/**
 * Small helper to identify whether a table belongs to the meta-table group.
 *
 * Keeping this check in one function makes the payload sanitization logic more
 * readable and avoids repeating project-specific table grouping rules.
 */
function isMetaTableName(tableName: SyncTableName): boolean {
  return META_TABLES.some((metaTableName) => metaTableName === tableName);
}

/**
 * Builds a readable sync error message from a failed HTTP response.
 *
 * We include:
 * - which phase failed (`pull` or `push`)
 * - the HTTP status code
 * - the response body when available
 *
 * This keeps UI messages and console logs actionable during integration.
 */
async function buildSyncErrorMessage(
  phase: "pull" | "push",
  response: Response,
): Promise<string> {
  const responseText = await response.text();
  const suffix = responseText ? `: ${responseText}` : "";
  return `Sync ${phase} failed with ${response.status}${suffix}`;
}
