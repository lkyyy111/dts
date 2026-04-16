import { Q } from "@nozbe/watermelondb";

import { database } from "@/model";
import Comment from "@/model/Comment";
import Expense from "@/model/Expense";
import Photo from "@/model/Photo";
import Post from "@/model/Post";
import Space from "@/model/Space";
import SpaceMember from "@/model/SpaceMember";
import User from "@/model/User";
import { markSpaceAsDisbanded } from "@/lib/disbandedSpaces";
import { nowTimestamp } from "@/lib/ids";
import {
  assignModelId,
  assignTimestamps,
  dateToTimestamp,
} from "@/lib/watermelon";
import type { UserProfileData } from "@/features/travel/userDb";

export type JoinedSpaceSummary = {
  id: string;
  code: string;
  name: string;
  createdAt: number;
  memberCount: number;
  photoCount: number;
  updatedAt: number;
};

export type SpaceUserView = {
  id: string;
  nickname: string;
  avatar_local_uri: string;
  avatar_remote_url: string;
  avatar_display_uri: string;
  created_at: number;
  updated_at: number;
};

export type SpaceMemberView = {
  id: string;
  space_id: string;
  user_id: string;
  created_at: number;
  updated_at: number;
};

export type SpaceData = {
  id: string;
  name: string;
  code: string;
  space: {
    id: string;
    name: string;
    created_at: number;
    updated_at: number;
  };
  users: SpaceUserView[];
  spaceMembers: SpaceMemberView[];
};

async function getAvatarFields(
  userId: string,
  currentProfile?: UserProfileData | null,
) {
  return {
    // 当前版本统一使用“蓝底 + 昵称首字”的文字头像，因此不再读取任何头像图片。
    avatar_local_uri: "",
    avatar_remote_url: "",
    avatar_display_uri: currentProfile?.id === userId ? "" : "",
  };
}

function pickLatestTimestamp(currentMax: number, nextValue: number) {
  return nextValue > currentMax ? nextValue : currentMax;
}

export async function listJoinedSpacesFromDb(
  userId: string,
): Promise<JoinedSpaceSummary[]> {
  if (!userId.trim()) {
    return [];
  }

  const spaceCollection = database.collections.get<Space>("spaces");
  const memberCollection =
    database.collections.get<SpaceMember>("space_members");
  const photoCollection = database.collections.get<Photo>("photos");
  const postCollection = database.collections.get<Post>("posts");
  const commentCollection = database.collections.get<Comment>("comments");
  const expenseCollection = database.collections.get<Expense>("expenses");

  const joinedMembers = await memberCollection
    .query(Q.where("user_id", userId))
    .fetch();
  if (joinedMembers.length === 0) {
    return [];
  }

  const joinedSpaceIds = Array.from(
    new Set(joinedMembers.map((item) => item.spaceId).filter(Boolean)),
  );

  const [spaces, allMembers, photos, posts, comments, expenses] =
    await Promise.all([
      spaceCollection.query().fetch(),
      memberCollection
        .query(Q.where("space_id", Q.oneOf(joinedSpaceIds)))
        .fetch(),
      photoCollection
        .query(Q.where("space_id", Q.oneOf(joinedSpaceIds)))
        .fetch(),
      postCollection
        .query(Q.where("space_id", Q.oneOf(joinedSpaceIds)))
        .fetch(),
      commentCollection
        .query(Q.where("space_id", Q.oneOf(joinedSpaceIds)))
        .fetch(),
      expenseCollection
        .query(Q.where("space_id", Q.oneOf(joinedSpaceIds)))
        .fetch(),
    ]);

  const spaceMap = new Map(
    spaces
      .filter((item) => joinedSpaceIds.includes(item.id))
      .map((item) => [item.id, item]),
  );
  const memberCountBySpace = new Map<string, number>();
  const photoCountBySpace = new Map<string, number>();
  const updatedAtBySpace = new Map<string, number>();

  for (const spaceId of joinedSpaceIds) {
    const space = spaceMap.get(spaceId);
    if (!space) {
      continue;
    }

    updatedAtBySpace.set(spaceId, dateToTimestamp(space.updatedAt));
  }

  for (const member of allMembers) {
    memberCountBySpace.set(
      member.spaceId,
      (memberCountBySpace.get(member.spaceId) ?? 0) + 1,
    );
    updatedAtBySpace.set(
      member.spaceId,
      pickLatestTimestamp(
        updatedAtBySpace.get(member.spaceId) ?? 0,
        dateToTimestamp(member.updatedAt),
      ),
    );
  }

  for (const photo of photos) {
    photoCountBySpace.set(
      photo.spaceId,
      (photoCountBySpace.get(photo.spaceId) ?? 0) + 1,
    );
    updatedAtBySpace.set(
      photo.spaceId,
      pickLatestTimestamp(
        updatedAtBySpace.get(photo.spaceId) ?? 0,
        dateToTimestamp(photo.updatedAt),
      ),
    );
  }

  for (const post of posts) {
    updatedAtBySpace.set(
      post.spaceId,
      pickLatestTimestamp(
        updatedAtBySpace.get(post.spaceId) ?? 0,
        dateToTimestamp(post.updatedAt),
      ),
    );
  }

  for (const comment of comments) {
    updatedAtBySpace.set(
      comment.spaceId,
      pickLatestTimestamp(
        updatedAtBySpace.get(comment.spaceId) ?? 0,
        dateToTimestamp(comment.updatedAt),
      ),
    );
  }

  for (const expense of expenses) {
    updatedAtBySpace.set(
      expense.spaceId,
      pickLatestTimestamp(
        updatedAtBySpace.get(expense.spaceId) ?? 0,
        dateToTimestamp(expense.updatedAt),
      ),
    );
  }

  return joinedSpaceIds
    .map((spaceId) => {
      const space = spaceMap.get(spaceId);
      if (!space) {
        return null;
      }

      return {
        id: space.id,
        code: space.id,
        name: space.name,
        createdAt: dateToTimestamp(space.createdAt),
        memberCount: memberCountBySpace.get(space.id) ?? 0,
        photoCount: photoCountBySpace.get(space.id) ?? 0,
        updatedAt:
          updatedAtBySpace.get(space.id) ?? dateToTimestamp(space.updatedAt),
      };
    })
    .filter((item): item is JoinedSpaceSummary => item !== null)
    .sort((left, right) => right.updatedAt - left.updatedAt);
}

export async function getSpaceSnapshotFromDb(
  spaceId: string,
  currentProfile?: UserProfileData | null,
): Promise<SpaceData | null> {
  const cleanSpaceId = spaceId.trim();
  if (!cleanSpaceId) {
    return null;
  }

  const spaceCollection = database.collections.get<Space>("spaces");
  const memberCollection =
    database.collections.get<SpaceMember>("space_members");
  const userCollection = database.collections.get<User>("users");

  let space: Space | null = null;
  try {
    space = await spaceCollection.find(cleanSpaceId);
  } catch {
    return null;
  }

  const members = await memberCollection
    .query(Q.where("space_id", cleanSpaceId), Q.sortBy("created_at", Q.asc))
    .fetch();

  const memberIds = Array.from(
    new Set(members.map((item) => item.userId).filter(Boolean)),
  );
  const allUsers = await userCollection.query().fetch();
  const rawUsers = allUsers.filter((item) => memberIds.includes(item.id));
  const rawUserMap = new Map(rawUsers.map((item) => [item.id, item]));

  const users = await Promise.all(
    memberIds.map(async (userId) => {
      const rawUser = rawUserMap.get(userId);
      const avatarFields = await getAvatarFields(userId, currentProfile);
      const currentProfileName =
        currentProfile?.id === userId ? currentProfile?.nickname || "" : "";
      return {
        id: userId,
        nickname: rawUser?.nickname || currentProfileName || "成员",
        ...avatarFields,
        created_at: rawUser ? dateToTimestamp(rawUser.createdAt) : 0,
        updated_at: rawUser ? dateToTimestamp(rawUser.updatedAt) : 0,
      };
    }),
  );

  return {
    id: space.id,
    name: space.name,
    code: space.id,
    space: {
      id: space.id,
      name: space.name,
      created_at: dateToTimestamp(space.createdAt),
      updated_at: dateToTimestamp(space.updatedAt),
    },
    users,
    spaceMembers: members.map((item) => ({
      id: item.id,
      space_id: item.spaceId,
      user_id: item.userId,
      created_at: dateToTimestamp(item.createdAt),
      updated_at: dateToTimestamp(item.updatedAt),
    })),
  };
}

export async function leaveSpaceLocally(spaceId: string, userId: string) {
  const memberCollection =
    database.collections.get<SpaceMember>("space_members");
  const records = await memberCollection
    .query(Q.where("space_id", spaceId), Q.where("user_id", userId))
    .fetch();

  if (records.length === 0) {
    return false;
  }

  await database.write(async () => {
    for (const record of records) {
      await record.markAsDeleted();
    }
  });

  return true;
}

export async function createSpaceLocally(params: {
  spaceId: string;
  name: string;
  userId: string;
}) {
  const cleanSpaceId = params.spaceId.trim();
  const cleanName = params.name.trim();
  const cleanUserId = params.userId.trim();
  if (!cleanSpaceId || !cleanName || !cleanUserId) {
    throw new Error("createSpaceLocally requires spaceId, name and userId.");
  }

  const spaceCollection = database.collections.get<Space>("spaces");
  const memberCollection =
    database.collections.get<SpaceMember>("space_members");

  let existingSpace: Space | null = null;
  try {
    existingSpace = await spaceCollection.find(cleanSpaceId);
  } catch {
    existingSpace = null;
  }

  const existingMembers = await memberCollection
    .query(Q.where("space_id", cleanSpaceId), Q.where("user_id", cleanUserId))
    .fetch();

  await database.write(async () => {
    if (!existingSpace) {
      await spaceCollection.create((space) => {
        assignModelId(space, cleanSpaceId);
        space.name = cleanName;
        const now = nowTimestamp();
        assignTimestamps(space, now, now);
      });
    }

    if (existingMembers.length === 0) {
      await memberCollection.create((member) => {
        assignModelId(member, `${cleanSpaceId}_${cleanUserId}`);
        member.spaceId = cleanSpaceId;
        member.userId = cleanUserId;
        const now = nowTimestamp();
        assignTimestamps(member, now, now);
      });
    }
  });

  return cleanSpaceId;
}

export async function deleteExpenseLocally(expenseId: string) {
  const cleanExpenseId = expenseId.trim();
  if (!cleanExpenseId) {
    return false;
  }

  const expenseCollection = database.collections.get<Expense>("expenses");

  let expense: Expense | null = null;
  try {
    expense = await expenseCollection.find(cleanExpenseId);
  } catch {
    return false;
  }

  await database.write(async () => {
    if (expense) {
      await expense.markAsDeleted();
    }
  });

  return true;
}

export async function disbandSpaceLocally(spaceId: string) {
  const spaceCollection = database.collections.get<Space>("spaces");
  const memberCollection =
    database.collections.get<SpaceMember>("space_members");
  const expenseCollection = database.collections.get<Expense>("expenses");
  const postCollection = database.collections.get<Post>("posts");
  const photoCollection = database.collections.get<Photo>("photos");
  const commentCollection = database.collections.get<Comment>("comments");

  let space: Space | null = null;
  try {
    space = await spaceCollection.find(spaceId);
  } catch {
    return false;
  }

  const [members, expenses, posts, photos, comments] = await Promise.all([
    memberCollection.query(Q.where("space_id", spaceId)).fetch(),
    expenseCollection.query(Q.where("space_id", spaceId)).fetch(),
    postCollection.query(Q.where("space_id", spaceId)).fetch(),
    photoCollection.query(Q.where("space_id", spaceId)).fetch(),
    commentCollection.query(Q.where("space_id", spaceId)).fetch(),
  ]);

  await database.write(async () => {
    for (const comment of comments) {
      await comment.markAsDeleted();
    }
    for (const photo of photos) {
      await photo.markAsDeleted();
    }
    for (const post of posts) {
      await post.markAsDeleted();
    }
    for (const expense of expenses) {
      await expense.markAsDeleted();
    }
    for (const member of members) {
      await member.markAsDeleted();
    }
    if (space) {
      await space.markAsDeleted();
    }
  });

  await markSpaceAsDisbanded(spaceId);

  return true;
}
