import { File, Paths } from "expo-file-system";
import { synchronize } from "@nozbe/watermelondb/sync";

import { database } from "@/model";
import Photo from "@/model/Photo";
import { META_TABLES, SYNC_TABLES, type SyncTableName } from "@/model/tables";
import type {
  PhotoUploadResponse,
  PullChangesResponse,
  PushChangesRequest,
  RawSyncChanges,
  SyncChanges,
  SyncContext,
  SyncRecord,
} from "@/sync/types";

// WatermelonDB sync is not re-entrant for our use case. We keep the active
// task here so repeated button taps can reuse the same in-flight sync.
let activeSyncTask: Promise<void> | null = null;

/**
 * Public sync entry for one "current sync context".
 *
 * The backend contract requires the frontend to send both:
 * - `X-User-Id`
 * - `X-Space-Id`
 *
 * Even though pull is scoped by `spaceId`, push still sends WatermelonDB's
 * global local changes. This mirrors the current backend contract:
 * - Pull: `GET /api/v1/sync` with `last_pulled_at` in query and ids in headers
 * - Push: `POST /api/v1/sync` with global `changes` in body and ids in headers
 *
 * We also serialize sync calls here so the local database cannot be mutated by
 * multiple concurrent synchronize() executions.
 */
export async function syncSpace(input: SyncContext): Promise<void> {
  const context = normalizeSyncContext(input);

  if (activeSyncTask) {
    return activeSyncTask;
  }

  const syncTask = runSync(context).finally(() => {
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
 * - after the database sync finishes, we run a second phase for photo file
 *   uploads because `/api/v1/photos` is separate from `/api/v1/sync`
 */
async function runSync(context: SyncContext): Promise<void> {
  const apiBaseUrl = getApiBaseUrl();

  await synchronize({
    database,
    // Pull is where "space isolation" really happens in the current design.
    //
    // WatermelonDB also passes `schemaVersion` and `migration` metadata into
    // this callback. We intentionally do not forward them because the current
    // backend contract exposed in Apifox only accepts `last_pulled_at` in the
    // query string and uses headers for user/space identity.
    pullChanges: async ({ lastPulledAt }) => {
      const params = new URLSearchParams({
        last_pulled_at: String(toSyncTimestamp(lastPulledAt)),
      });

      const response = await fetch(`${apiBaseUrl}/api/v1/sync?${params}`, {
        headers: buildSyncHeaders(context),
      });
      if (!response.ok) {
        throw new Error(await buildSyncErrorMessage("pull", response));
      }

      // During integration the backend may omit empty tables, so we normalize
      // the payload into a complete 7-table changes object before returning it
      // to WatermelonDB.
      const payload = (await response.json()) as PullChangesResponse;

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
      const sanitizedChanges = sanitizePushChanges(changes as RawSyncChanges);
      const requestBody: PushChangesRequest = {
        last_pulled_at: toSyncTimestamp(lastPulledAt),
        changes: sanitizedChanges,
      };

      const response = await fetch(`${apiBaseUrl}/api/v1/sync`, {
        method: "POST",
        headers: buildJsonHeaders(context),
        body: JSON.stringify(requestBody),
      });

      if (!response.ok) {
        throw new Error(await buildSyncErrorMessage("push", response));
      }
    },
    migrationsEnabledAtVersion: 1,
  });

  // Photo file upload is intentionally separated from WatermelonDB's record
  // sync:
  // - `/api/v1/sync` handles database rows
  // - `/api/v1/photos` handles the actual file binary
  //
  // We run it after the main sync so record creation reaches the server first.
  await uploadPendingPhotos(apiBaseUrl, context.userId);
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
 * Normalizes sync input before any request is made.
 *
 * We trim both ids once up front so the rest of the sync code can rely on a
 * stable header context without repeating validation in every request branch.
 */
function normalizeSyncContext(input: SyncContext): SyncContext {
  const userId = input.userId.trim();
  const spaceId = input.spaceId.trim();

  if (!userId) {
    throw new Error("syncSpace requires a non-empty userId.");
  }

  if (!spaceId) {
    throw new Error("syncSpace requires a non-empty spaceId.");
  }

  return { userId, spaceId };
}

/**
 * Builds the common sync headers required by the backend contract.
 *
 * These headers represent the sync context, not a client-side filter. We still
 * send WatermelonDB's global push payload; the backend decides how each record
 * should be validated and stored.
 */
function buildSyncHeaders(context: SyncContext): Record<string, string> {
  return {
    "X-User-Id": context.userId,
    "X-Space-Id": context.spaceId,
  };
}

/**
 * Builds JSON request headers for endpoints such as `POST /api/v1/sync`.
 *
 * Multipart photo uploads intentionally do not use this helper because fetch
 * needs to generate the multipart boundary automatically.
 */
function buildJsonHeaders(context: SyncContext): Record<string, string> {
  return {
    ...buildSyncHeaders(context),
    "Content-Type": "application/json",
  };
}

/**
 * Converts WatermelonDB's `lastPulledAt` value into the numeric field expected
 * by our backend.
 *
 * WatermelonDB uses `null` for the first sync. The server contract uses `0`
 * for "nothing has been pulled yet", so we normalize here once and reuse it
 * for both pull query strings and push request bodies.
 */
function toSyncTimestamp(lastPulledAt: number | null | undefined): number {
  return lastPulledAt ?? 0;
}

/**
 * Uploads every local photo file that still lacks a server `remote_url`.
 *
 * Two project-specific rules drive this implementation:
 * - we must scan all photos, not just the currently visible space
 * - each upload must still use the photo record's own `space_id` as header
 *
 * That second point matters because this post-sync compensation step may be
 * uploading files that belong to several different spaces in one pass.
 */
async function uploadPendingPhotos(
  apiBaseUrl: string,
  userId: string,
): Promise<void> {
  const photosCollection = database.collections.get<Photo>("photos");
  const allPhotos = await photosCollection.query().fetch();
  const pendingPhotos = allPhotos.filter(hasPendingPhotoUpload);

  // We upload sequentially on purpose. This keeps request ordering readable in
  // logs and avoids spiking memory/network usage with many simultaneous file
  // reads on mobile devices.
  for (const photo of pendingPhotos) {
    await uploadSinglePhoto(apiBaseUrl, userId, photo);
  }
}

/**
 * Decides whether a photo still needs binary upload compensation.
 *
 * The backend fills `remote_url` after `/api/v1/photos` succeeds. Until a
 * later pull brings that value back into WatermelonDB, the photo is still
 * considered pending on the client.
 */
function hasPendingPhotoUpload(photo: Photo): boolean {
  return !isNonEmptyString(photo.remoteUrl);
}

/**
 * Uploads one photo file to `POST /api/v1/photos`.
 *
 * We derive the expected local path from `data-design.md` rather than storing
 * `local_uri` in the database:
 * `${App storage}/photos/${photo_id}.jpg`
 *
 * If the database row exists but the file is missing, we do not fail the whole
 * sync round. That situation is already considered an abnormal local-file case
 * in the design docs, so we log it and let future UI cleanup handle it.
 */
async function uploadSinglePhoto(
  apiBaseUrl: string,
  userId: string,
  photo: Photo,
): Promise<void> {
  const localPhotoFile = getExpectedLocalPhotoFile(photo.id);
  if (!localPhotoFile.exists) {
    console.warn(
      `[sync] skip photo upload for ${photo.id}: local file missing at ${localPhotoFile.uri}`,
    );
    return;
  }

  const photoContext = normalizeSyncContext({
    userId,
    spaceId: photo.spaceId,
  });
  const requestBody = new FormData();
  requestBody.append("photo_id", photo.id);
  requestBody.append("file", localPhotoFile, localPhotoFile.name);

  const response = await fetch(`${apiBaseUrl}/api/v1/photos`, {
    method: "POST",
    headers: buildSyncHeaders(photoContext),
    body: requestBody,
  });

  if (!response.ok) {
    throw new Error(await buildSyncErrorMessage("photo upload", response));
  }

  const payload = (await response.json()) as PhotoUploadResponse;
  if (!isNonEmptyString(payload.remote_url)) {
    throw new Error(
      `Photo upload for ${photo.id} succeeded but response.remote_url is missing.`,
    );
  }

  // We intentionally do not write `remote_url` into WatermelonDB here.
  // According to the current design, the server becomes the source of truth for
  // that field and the client receives it on the next pull. This avoids
  // manufacturing an extra local "photo updated" change just because the file
  // upload API echoed the server URL back to us.
}

/**
 * Resolves the canonical local file location for a photo id.
 *
 * Keeping this path logic in one helper makes it easier to update later if the
 * project changes its on-device storage layout.
 */
function getExpectedLocalPhotoFile(photoId: string): File {
  return new File(Paths.document, "photos", `${photoId}.jpg`);
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
function normalizeChanges(rawChanges?: RawSyncChanges): SyncChanges {
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
function sanitizePushChanges(rawChanges?: RawSyncChanges): SyncChanges {
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
 * Small string guard used for ids and `remote_url`.
 *
 * We treat whitespace-only strings as empty because headers and URLs should not
 * carry meaningless blank values.
 */
function isNonEmptyString(value: unknown): value is string {
  return typeof value === "string" && value.trim().length > 0;
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
 * - which phase failed (`pull`, `push`, `photo upload`, ...)
 * - the HTTP status code
 * - the response body when available
 *
 * This keeps UI messages and console logs actionable during integration.
 */
async function buildSyncErrorMessage(
  phase: string,
  response: Response,
): Promise<string> {
  const responseText = await response.text();
  const suffix = responseText ? `: ${responseText}` : "";
  return `Sync ${phase} failed with ${response.status}${suffix}`;
}
